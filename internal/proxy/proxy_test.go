// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// We need corporate proxy emulation to run tests
	proxy := goproxy.NewProxyHttpServer()
	auth.ProxyBasic(proxy, "EVIL.CORP NEEDS YOUR AUTH", func(user, password string) bool {
		return user == "test_user" && subtle.ConstantTimeCompare([]byte(password), []byte("test_password")) == 1
	})

	go func() {
		log.Fatal(http.ListenAndServe("localhost:9090", proxy))
	}()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestProxy_CheckAuth(t *testing.T) {
	logger := zap.NewNop()

	downstreamProxyURL, _ := url.Parse("http://localhost:9090/")
	httpPingURL, _ := url.Parse("http://checkip.amazonaws.com/")
	httpsPingURL, _ := url.Parse("https://checkip.amazonaws.com/")
	httpNoProxyPingURL, _ := url.Parse("http://api.ipify.org/")
	httpsNoProxyPingURL, _ := url.Parse("https://api.ipify.org/")

	config := &Config{
		Addr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 31280,
		},
		AddrString:         "127.0.0.1:31280",
		NoProxy:            "127.0.0.1,localhost,*.ipify.org",
		DownstreamProxyURL: downstreamProxyURL,
		DownstreamProxyAuth: DownstreamProxyAuth{
			User:     "test_user",
			Password: "test_password",
		},
		Timeouts: Timeouts{
			Server: ServerTimeouts{
				ReadHeaderTimeout: 30 * time.Second,
				WriteTimeout:      1 * time.Minute,
			},
			Client: ClientTimeouts{
				KeepAlivePeriod: 1 * time.Minute,
			},
			DownstreamProxy: DownstreamProxyTimeouts{
				DialTimeout:     10 * time.Second,
				KeepAlivePeriod: 1 * time.Minute,
			},
		},
		PingURL: httpPingURL,
		Mode:    BasicMode,
	}

	p := NewProxy(logger, config, nil)

	l, err := p.Listen()
	require.NoError(t, err)

	go func() {
		require.NoError(t, p.Serve(l))
	}()

	t.Run("http", func(t *testing.T) {
		ok, err := p.CheckAuth()
		require.NoError(t, err)
		require.True(t, ok)
	})

	p.config.PingURL = httpsPingURL

	t.Run("https", func(t *testing.T) {
		ok, err := p.CheckAuth()
		require.NoError(t, err)
		require.True(t, ok)
	})

	p.config.PingURL = httpNoProxyPingURL

	t.Run("http with no_proxy", func(t *testing.T) {
		ok, err := p.CheckAuth()
		require.NoError(t, err)
		require.True(t, ok)
	})

	p.config.PingURL = httpsNoProxyPingURL

	t.Run("https with no_proxy", func(t *testing.T) {
		ok, err := p.CheckAuth()
		require.NoError(t, err)
		require.True(t, ok)
	})

	require.NoError(t, p.Shutdown(context.Background()))
}
