// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/L11R/escobar/internal/configs"
	"github.com/L11R/escobar/internal/proxy"
	"github.com/L11R/escobar/internal/static"
	"github.com/jcmturner/gokrb5/v8/client"
	krb5config "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

func main() {
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

	// Init logger
	logger, err := zap.NewProduction()
	if len(config.Verbose) != 0 && config.Verbose[0] {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalln(err)
	}

	// Create Kerberos configuration for client
	r, err := config.Proxy.Kerberos.Reader()
	if err != nil {
		logger.Fatal("Cannot create Kerberos config", zap.Error(err))
	}

	kbr5conf, err := krb5config.NewFromReader(r)
	if err != nil {
		logger.Fatal("Cannot read Kerberos config", zap.Error(err))
	}

	// Create Kerberos client with user credentials if we are using Linux, macOS or something else.
	initKbr5 := func() *client.Client {
		if config.Proxy.DownstreamProxyAuth.Keytab != "" {
			kt, err := keytab.Load(config.Proxy.DownstreamProxyAuth.Keytab)
			if err != nil {
				logger.Fatal("Cannot read Keytab-file", zap.Error(err))
			}

			return client.NewWithKeytab(
				config.Proxy.DownstreamProxyAuth.User,
				config.Proxy.Kerberos.Realm,
				kt,
				kbr5conf,
				client.DisablePAFXFAST(true),
			)
		}

		return client.NewWithPassword(
			config.Proxy.DownstreamProxyAuth.User,
			config.Proxy.Kerberos.Realm,
			config.Proxy.DownstreamProxyAuth.Password,
			kbr5conf,
			client.DisablePAFXFAST(true),
		)
	}

	var krb5cl *client.Client
	if runtime.GOOS != "windows" || config.Proxy.ManualMode {
		krb5cl = initKbr5()
	}

	shutdown := make(chan error, 1)

	// Proxy server
	p := proxy.NewProxy(logger, config.Proxy, krb5cl)

	l, err := p.Listen()
	if err != nil {
		logger.Fatal("Cannot listen socket!", zap.Error(err))
	}

	go func(shutdown chan<- error) {
		shutdown <- p.Serve(l)
	}(shutdown)

	// Static server
	s := static.NewStatic(logger, config.Static, config.Proxy)

	if config.Static.Enabled {
		go func(shutdown chan<- error) {
			shutdown <- s.ListenAndServe()
		}(shutdown)
	}

	// Check auth against out real server
	if ok, err := p.CheckAuth(); err == nil && ok {
		// do nothing, it's ok
	} else if err != nil {
		logger.Error(
			"Cannot check proxy and credentials validity",
			zap.String("ping_url", config.Proxy.PingURL.String()),
			zap.Error(err),
		)
		shutdown <- err
	} else {
		logger.Error("Provided credentials are invalid")
		shutdown <- err
	}

	// Graceful shutdown block
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		logger.Info("Got the signal!", zap.Any("signal", s))
	case err := <-shutdown:
		logger.Error("Error running the application!", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	logger.Info("Stopping proxy...")

	// Close static server first, it shouldn't have many open connections
	if config.Static.Enabled {
		if err := s.Shutdown(ctx); err != nil {
			logger.Error("Error shutting down the static server!", zap.Error(err))
		}
	}

	if err := p.Shutdown(ctx); err != nil {
		logger.Error("Error shutting down the proxy server!", zap.Error(err))
	}

	logger.Info("Proxy stopped")
}
