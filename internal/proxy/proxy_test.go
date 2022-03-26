// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/L11R/httputil"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"github.com/jcmturner/gokrb5/v8/client"
	krb5config "github.com/jcmturner/gokrb5/v8/config"
	"github.com/stretchr/testify/assert"
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

func TestNewProxy(t *testing.T) {
	logger := zap.NewNop()

	downstreamProxyURL, _ := url.Parse("http://localhost:9090/")
	pingURL, _ := url.Parse("https://www.google.com/")

	config := &Config{
		Addr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 3128,
		},
		DownstreamProxyURL: downstreamProxyURL,
		DownstreamProxyAuth: DownstreamProxyAuth{
			User:     "test_user",
			Password: "test_password",
		},
		Kerberos: Kerberos{
			Realm: "EVIL.CORP",
			KDC: &net.TCPAddr{
				IP:   net.IPv4(10, 0, 0, 1),
				Port: 88,
			},
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
		PingURL: pingURL,
	}

	confReader, err := config.Kerberos.Reader()
	require.NoError(t, err)

	krb5conf, err := krb5config.NewFromReader(confReader)
	require.NoError(t, err)

	krb5cl := client.NewWithPassword(
		config.DownstreamProxyAuth.User,
		config.Kerberos.Realm,
		config.DownstreamProxyAuth.Password,
		krb5conf,
	)

	// Dummy patch to avoid different pointers comparing
	rp := httputil.NewForwardingProxy()
	patch := monkey.Patch(httputil.NewForwardingProxy, func() *httputil.ReverseProxy {
		return rp
	})
	defer patch.Unpatch()

	expected := &Proxy{
		logger:    logger,
		config:    config,
		krb5cl:    krb5cl,
		httpProxy: httputil.NewForwardingProxy(),
	}

	expected.httpProxy.ErrorLog = zap.NewStdLog(logger)
	expected.server = &http.Server{
		Addr:              config.Addr.String(),
		Handler:           expected,
		TLSNextProto:      make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		ReadTimeout:       config.Timeouts.Server.ReadTimeout,
		ReadHeaderTimeout: config.Timeouts.Server.ReadHeaderTimeout,
		WriteTimeout:      config.Timeouts.Server.WriteTimeout,
		IdleTimeout:       config.Timeouts.Server.IdleTimeout,
	}

	actual := NewProxy(logger, config, krb5cl)
	assert.Equal(t, expected, actual)
}

func TestProxy_CheckAuth(t *testing.T) {
	logger := zap.NewNop()

	downstreamProxyURL, _ := url.Parse("http://localhost:9090/")
	httpPingURL, _ := url.Parse("http://checkip.amazonaws.com/")
	httpsPingURL, _ := url.Parse("https://checkip.amazonaws.com/")

	config := &Config{
		Addr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 31280,
		},
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

	require.NoError(t, p.Shutdown(context.Background()))
}
