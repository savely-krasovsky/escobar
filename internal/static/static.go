// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package static

import (
	"context"
	"errors"
	"net/http"

	"github.com/L11R/escobar/internal/proxy"
	"go.uber.org/zap"
)

type Static struct {
	logger      *zap.Logger
	config      *Config
	proxyConfig *proxy.Config
	server      *http.Server
}

func NewStatic(logger *zap.Logger, config *Config, proxyConfig *proxy.Config) *Static {
	s := &Static{
		logger:      logger,
		config:      config,
		proxyConfig: proxyConfig,
	}

	s.server = &http.Server{
		Addr:    config.Addr.String(),
		Handler: s.newRouter(),
	}

	return s
}

// ListenAndServe listens and serves HTTP requests.
func (s *Static) ListenAndServe() error {
	s.logger.Info("Listening and serving HTTP requests", zap.String("address", s.config.Addr.String()))

	if err := s.server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Error listening and serving HTTP requests!", zap.Error(err))
			return err
		}
	}

	return nil
}

// Shutdown shuts down the HTTP server.
func (s *Static) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Error shutting down HTTP server!", zap.Error(err))
		return err
	}

	return nil
}
