// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
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

type escobar struct {
	logger       *zap.Logger
	systemLogger service.Logger
	config       *configs.Config

	proxy  *proxy.Proxy
	static *static.Static
}

// initKrb5 creates Kerberos client with user credentials if we are using Linux, macOS or something else.
func (e *escobar) initKbr5() (*client.Client, error) {
	// Create Kerberos configuration for client
	r, err := e.config.Proxy.Kerberos.Reader()
	if err != nil {
		return nil, fmt.Errorf("cannot create Kerberos config: %w", err)
	}

	kbr5conf, err := krb5config.NewFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read Kerberos config: %w", err)
	}

	if e.config.Proxy.DownstreamProxyAuth.Keytab != "" {
		kt, err := keytab.Load(e.config.Proxy.DownstreamProxyAuth.Keytab)
		if err != nil {
			return nil, fmt.Errorf("cannot read Keytab-file: %w", err)
		}

		return client.NewWithKeytab(
			e.config.Proxy.DownstreamProxyAuth.User,
			e.config.Proxy.Kerberos.Realm,
			kt,
			kbr5conf,
			client.DisablePAFXFAST(true),
		), nil
	}

	return client.NewWithPassword(
		e.config.Proxy.DownstreamProxyAuth.User,
		e.config.Proxy.Kerberos.Realm,
		e.config.Proxy.DownstreamProxyAuth.Password,
		kbr5conf,
		client.DisablePAFXFAST(true),
	), nil
}

func main() {
	escbr := &escobar{}

	svc, err := service.New(escbr, &service.Config{
		Name:        "Escobar Proxy",
		DisplayName: "Escobar Proxy",
		Description: "Escobar Proxy",
	})
	if err != nil {
		log.Fatalln(err)
	}

	sysLogger, err := svc.SystemLogger(nil)
	if err != nil {
		log.Fatal(err)
	}
	escbr.systemLogger = sysLogger

	if err := svc.Run(); err != nil {
		sysLogger.Error(err)
	}
}

func (e *escobar) Start(svc service.Service) error {
	go e.run(svc)
	return nil
}

func (e *escobar) Write(b []byte) (int, error) {
	var entry struct {
		Level zapcore.Level `json:"level"`
	}
	if err := json.Unmarshal(b, &entry); err != nil {
		return 0, err
	}

	switch entry.Level {
	case zapcore.DebugLevel:
		return 0, e.systemLogger.Info(string(b))
	case zapcore.InfoLevel:
		return 0, e.systemLogger.Info(string(b))
	case zapcore.WarnLevel:
		return 0, e.systemLogger.Warning(string(b))
	default:
		return 0, e.systemLogger.Error(string(b))
	}
}

func (e *escobar) run(svc service.Service) {
	// Parse command line arguments or environment variables
	config, err := configs.Parse()
	if err != nil {
		if err, ok := err.(*flags.Error); ok {
			fmt.Println(err)
			os.Exit(0)
		}

		fmt.Printf("Invalid args: %v\n", err)
		os.Exit(1)
	}

	e.config = config

	// Init logger
	enabler := zapcore.ErrorLevel
	if len(config.Verbose) != 0 && config.Verbose[0] {
		enabler = zapcore.DebugLevel
	}
	logger := zap.New(
		zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(os.Stdout), enabler),
			zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(e), enabler),
		),
	)
	e.logger = logger

	var krb5cl *client.Client
	if config.Proxy.Mode == proxy.ManualMode {
		krb5cl, err = e.initKbr5()
		if err != nil {
			log.Fatalln(err)
		}
	}

	p := proxy.NewProxy(logger, config.Proxy, krb5cl)
	e.proxy = p
	s := static.NewStatic(logger, config.Static, config.Proxy)
	e.static = s

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
		if ok {
			return
		}

		if err != nil {
			logger.Error(
				"Cannot check proxy and credentials validity",
				zap.String("ping_url", config.Proxy.PingURL.String()),
				zap.Error(err),
			)
		} else {
			logger.Error("Provided credentials are invalid")
		}
	}()

	go func() {
		logger.Error("Error while running proxy!", zap.Error(<-errChan))
	}()
}

func (e *escobar) Stop(_ service.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	e.logger.Info("Stopping proxy...")

	// Close static server first, it shouldn't have many open connections
	if err := e.static.Shutdown(ctx); err != nil {
		return fmt.Errorf("error while shutting down the static server: %w", err)
	}

	if err := e.proxy.Shutdown(ctx); err != nil {
		return fmt.Errorf("error while shutting down the proxy server: %w", err)
	}

	e.logger.Info("Proxy stopped")
	return nil
}
