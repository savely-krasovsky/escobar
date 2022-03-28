// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package configs

import (
	"os"
	"testing"
	"time"

	"github.com/L11R/escobar/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		t.Run("short", func(t *testing.T) {
			os.Args = []string{
				"./escobar",
				"-v",
				"-a", "localhost:31280",
				"-d", "http://10.0.0.1:9090",
				"-u", "ivanovii",
				"-p", "Qwerty123",
				"--proxy.kerberos.realm", "EVIL.CORP",
				"--proxy.ping-url", "https://www.google.com/",
				"--proxy.timeouts.server.read", "10s",
				"--proxy.timeouts.server.read-header", "130s",
				"--proxy.timeouts.server.write", "10s",
				"--proxy.timeouts.server.idle", "11m",
				"--proxy.timeouts.client.read", "10s",
				"--proxy.timeouts.client.write", "10s",
				"--proxy.timeouts.client.keepalive-period", "11m",
				"--proxy.timeouts.downstream.dial", "110s",
				"--proxy.timeouts.downstream.read", "10s",
				"--proxy.timeouts.downstream.write", "10s",
				"--proxy.timeouts.downstream.keepalive-period", "11m",
				"--static.addr", "localhost:31290",
			}

			config, err := Parse()
			require.NoError(t, err)

			assert.Equal(t, []bool{true}, config.Verbose)
			assert.Equal(t, "127.0.0.1:31280", config.Proxy.Addr.String())
			assert.Equal(t, "http://10.0.0.1:9090", config.Proxy.DownstreamProxyURL.String())
			assert.Equal(t, "ivanovii", config.Proxy.DownstreamProxyAuth.User)
			assert.Equal(t, "Qwerty123", config.Proxy.DownstreamProxyAuth.Password)
			assert.Equal(t, "EVIL.CORP", config.Proxy.Kerberos.Realm)
			assert.Equal(t, "https://www.google.com/", config.Proxy.PingURL.String())
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.Server.ReadTimeout)
			assert.Equal(t, 130*time.Second, config.Proxy.Timeouts.Server.ReadHeaderTimeout)
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.Server.WriteTimeout)
			assert.Equal(t, 11*time.Minute, config.Proxy.Timeouts.Server.IdleTimeout)
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.Client.ReadTimeout)
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.Client.WriteTimeout)
			assert.Equal(t, 11*time.Minute, config.Proxy.Timeouts.Client.KeepAlivePeriod)
			assert.Equal(t, 110*time.Second, config.Proxy.Timeouts.DownstreamProxy.DialTimeout)
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.DownstreamProxy.ReadTimeout)
			assert.Equal(t, 10*time.Second, config.Proxy.Timeouts.DownstreamProxy.WriteTimeout)
			assert.Equal(t, 11*time.Minute, config.Proxy.Timeouts.DownstreamProxy.KeepAlivePeriod)
			assert.Equal(t, "127.0.0.1:31290", config.Static.Addr.String())
		})

		t.Run("long", func(t *testing.T) {
			os.Args = []string{
				"./escobar",
				"--verbose",
				"--proxy.addr", "localhost:31280",
				"--proxy.downstream-proxy-url", "http://10.0.0.1:9090",
				"--proxy.downstream-proxy-auth.user", "ivanovii",
				"--proxy.downstream-proxy-auth.password", "Qwerty123",
				"--proxy.kerberos.realm", "EVIL.CORP",
			}

			config, err := Parse()
			require.NoError(t, err)

			assert.Equal(t, []bool{true}, config.Verbose)
			assert.Equal(t, "127.0.0.1:31280", config.Proxy.Addr.String())
			assert.Equal(t, "http://10.0.0.1:9090", config.Proxy.DownstreamProxyURL.String())
			assert.Equal(t, "ivanovii", config.Proxy.DownstreamProxyAuth.User)
			assert.Equal(t, "Qwerty123", config.Proxy.DownstreamProxyAuth.Password)
			assert.Equal(t, "EVIL.CORP", config.Proxy.Kerberos.Realm)
			assert.Equal(t, "https://www.google.com/", config.Proxy.PingURL.String())
		})

		t.Run("manual mode is on", func(t *testing.T) {
			os.Args = []string{
				"./escobar",
				"--verbose",
				"--proxy.addr", "localhost:31280",
				"--proxy.downstream-proxy-url", "http://10.0.0.1:9090",
				"--proxy.downstream-proxy-auth.user", "ivanovii",
				"--proxy.downstream-proxy-auth.password", "Qwerty123",
				"--proxy.kerberos.realm", "EVIL.CORP",
				"--proxy.kerberos.kdc", "10.0.0.1:88",
				"--proxy.mode", "manual",
			}

			config, err := Parse()
			require.NoError(t, err)

			assert.Equal(t, []bool{true}, config.Verbose)
			assert.Equal(t, "127.0.0.1:31280", config.Proxy.Addr.String())
			assert.Equal(t, "http://10.0.0.1:9090", config.Proxy.DownstreamProxyURL.String())
			assert.Equal(t, "ivanovii", config.Proxy.DownstreamProxyAuth.User)
			assert.Equal(t, "Qwerty123", config.Proxy.DownstreamProxyAuth.Password)
			assert.Equal(t, "EVIL.CORP", config.Proxy.Kerberos.Realm)
			assert.Equal(t, "10.0.0.1:88", config.Proxy.Kerberos.KDC.String())
			assert.Equal(t, "https://www.google.com/", config.Proxy.PingURL.String())
			assert.Equal(t, proxy.ManualMode, config.Proxy.Mode)
		})
	})
}
