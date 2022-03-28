package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/L11R/escobar/internal/configs"
	"github.com/L11R/escobar/internal/proxy"
	"github.com/L11R/escobar/internal/static"
	"github.com/jcmturner/gokrb5/v8/client"
	krb5config "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jessevdk/go-flags"
	"github.com/kardianos/service"
	"github.com/shibukawa/configdir"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Daemon struct {
	SystemLogger service.Logger

	logger *zap.Logger
	config *configs.Config

	proxy  *proxy.Proxy
	static *static.Static
}

func New() *Daemon {
	d := &Daemon{
		SystemLogger: nil,
		logger:       nil,
		config:       nil,
		proxy:        nil,
		static:       nil,
	}

	return d
}

func (d *Daemon) Start(svc service.Service) error {
	go d.run(svc)
	return nil
}

func (d *Daemon) run(svc service.Service) {
	// Parse command line arguments, environment variables or from config file
	config, err := configs.Parse()
	if err != nil {
		if err, ok := err.(*flags.Error); ok {
			fmt.Println(err)
			os.Exit(0)
		}

		fmt.Printf("Invalid args: %v\n", err)
		os.Exit(1)
	}
	d.config = config

	// Init loggers
	enabler := zapcore.ErrorLevel
	if len(config.Verbose) != 0 && config.Verbose[0] {
		enabler = zapcore.DebugLevel
	}
	loggers := []zapcore.Core{
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(os.Stdout), enabler),
	}

	// Attach system logger if enabled
	syslogErrChan := make(chan error, 1)
	if config.UseSystemLogger {
		sysLogger, err := svc.SystemLogger(syslogErrChan)
		if err != nil {
			log.Fatalln(err)
		}
		d.SystemLogger = sysLogger

		loggers = append(
			loggers,
			zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(d), enabler),
		)
	} else {
		close(syslogErrChan)
	}
	logger := zap.New(zapcore.NewTee(loggers...))
	d.logger = logger
	go func() {
		for err := range syslogErrChan {
			logger.Error("Error from system logger received", zap.Error(err))
		}
	}()

	var krb5cl *client.Client
	if config.Proxy.Mode == proxy.ManualMode {
		krb5cl, err = d.initKrb5()
		if err != nil {
			log.Fatalln(err)
		}
	}

	p := proxy.NewProxy(logger, config.Proxy, krb5cl)
	d.proxy = p
	s := static.NewStatic(logger, config.Static, config.Proxy)
	d.static = s

	if config.Install {
		b, _ := json.MarshalIndent(config, "", "\t")

		configDirs := configdir.New("Escobar", "Escobar")
		folders := configDirs.QueryFolders(configdir.System)
		if err := folders[0].WriteFile("settings.json", b); err != nil {
			logger.Error("Error while trying to save config!", zap.Error(err))
		}

		if err := svc.Install(); err != nil {
			logger.Error("Error while trying to install service!", zap.Error(err))
		}
		return
	}
	if config.Uninstall {
		if err := svc.Uninstall(); err != nil {
			logger.Error("Error while trying to uninstall service!", zap.Error(err))
		}
		return
	}

	l, err := p.Listen()
	if err != nil {
		logger.Fatal("Cannot listen socket!", zap.Error(err))
	}

	errChan := make(chan error, 1)

	go func() {
		errChan <- p.Serve(l)
	}()

	go func() {
		errChan <- s.ListenAndServe()
	}()

	go func() {
		// Check auth against out real server
		ok, err := p.CheckAuth()
		if err != nil {
			logger.Error(
				"Cannot check downstream proxy!",
				zap.String("ping_url", config.Proxy.PingURL.String()),
				zap.Error(err),
			)

			errChan <- err
			return
		}

		if !ok {
			errChan <- errors.New("provided credentials for downstream proxy are invalid")
		}
	}()

	go func() {
		if err := <-errChan; err != nil {
			logger.Error("Error while running proxy!", zap.Error(err))
			d.Stop(nil)
			os.Exit(1)
		}
	}()
}

// Stop shutdowns proxy and static server
func (d *Daemon) Stop(_ service.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	d.logger.Info("Stopping proxy...")

	// Close static server first, it shouldn't have many open connections
	if err := d.static.Shutdown(ctx); err != nil {
		return fmt.Errorf("error while shutting down the static server: %w", err)
	}

	if err := d.proxy.Shutdown(ctx); err != nil {
		return fmt.Errorf("error while shutting down the proxy server: %w", err)
	}

	d.logger.Info("Proxy stopped")
	return nil
}

// Write implements io.Writer to create zap encoder
func (d *Daemon) Write(b []byte) (int, error) {
	var entry struct {
		Level zapcore.Level `json:"level"`
	}
	if err := json.Unmarshal(b, &entry); err != nil {
		return 0, err
	}

	switch entry.Level {
	case zapcore.DebugLevel:
		return 0, d.SystemLogger.Info(string(b))
	case zapcore.InfoLevel:
		return 0, d.SystemLogger.Info(string(b))
	case zapcore.WarnLevel:
		return 0, d.SystemLogger.Warning(string(b))
	default:
		return 0, d.SystemLogger.Error(string(b))
	}
}

// initKrb5 creates Kerberos client with user credentials if we are using Linux, macOS or something else.
func (d *Daemon) initKrb5() (*client.Client, error) {
	// Create Kerberos configuration for client
	r, err := d.config.Proxy.Kerberos.Reader()
	if err != nil {
		return nil, fmt.Errorf("cannot create Kerberos config: %w", err)
	}

	kbr5conf, err := krb5config.NewFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read Kerberos config: %w", err)
	}

	if d.config.Proxy.DownstreamProxyAuth.Keytab != "" {
		kt, err := keytab.Load(d.config.Proxy.DownstreamProxyAuth.Keytab)
		if err != nil {
			return nil, fmt.Errorf("cannot read Keytab-file: %w", err)
		}

		return client.NewWithKeytab(
			d.config.Proxy.DownstreamProxyAuth.User,
			d.config.Proxy.Kerberos.Realm,
			kt,
			kbr5conf,
			client.DisablePAFXFAST(true),
		), nil
	}

	return client.NewWithPassword(
		d.config.Proxy.DownstreamProxyAuth.User,
		d.config.Proxy.Kerberos.Realm,
		d.config.Proxy.DownstreamProxyAuth.Password,
		kbr5conf,
		client.DisablePAFXFAST(true),
	), nil
}
