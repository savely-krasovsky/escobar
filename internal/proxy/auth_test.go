// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build !windows

package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"bou.ke/monkey"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/spnego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestProxy_setProxyAuthorizationHeader(t *testing.T) {
	u, _ := url.Parse("http://proxy.evil.corp:9090")

	p := NewProxy(
		zap.NewNop(),
		&Config{
			DownstreamProxyURL: u,
			DownstreamProxyAuth: DownstreamProxyAuth{
				User:     "test_user",
				Password: "test_password",
			},
		},
		&client.Client{},
	)

	req, _ := http.NewRequest("GET", "https://www.google.com/", nil)

	t.Run("default kerberos", func(t *testing.T) {
		expected := "Negotiate a2VyYmVyb3NfdGVzdF90b2tlbg=="
		patch := monkey.Patch(spnego.SetSPNEGOHeader, func(krb5cl *client.Client, req *http.Request, spn string) error {
			req.Header.Set(spnego.HTTPHeaderAuthRequest, expected)
			return nil
		})
		defer patch.Unpatch()
		if err := p.setProxyAuthorizationHeader(req); err != nil {
			assert.NoError(t, err)
		}

		actual := req.Header.Get(ProxyAuthorization)
		assert.Equal(t, expected, actual)
	})

	p.config.BasicMode = true

	t.Run("basic mode", func(t *testing.T) {
		expected := "Basic dGVzdF91c2VyOnRlc3RfcGFzc3dvcmQ="
		if err := p.setProxyAuthorizationHeader(req); err != nil {
			assert.NoError(t, err)
		}

		actual := req.Header.Get(ProxyAuthorization)
		assert.Equal(t, expected, actual)
	})
}
