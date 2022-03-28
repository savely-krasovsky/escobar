// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/spnego"
	"github.com/stretchr/testify/assert"
	"github.com/undefinedlabs/go-mpatch"
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
			Mode: AutoMode,
		},
		&client.Client{},
	)

	req, _ := http.NewRequest("GET", "https://www.google.com/", nil)

	t.Run("auto", func(t *testing.T) {
		// could not be monkey patched to test
	})

	p.config.Mode = ManualMode

	t.Run("kerberos", func(t *testing.T) {
		expected := "Negotiate a2VyYmVyb3NfdGVzdF90b2tlbg=="

		patch, err := mpatch.PatchMethod(spnego.SetSPNEGOHeader, func(krb5cl *client.Client, req *http.Request, spn string) error {
			req.Header.Set(spnego.HTTPHeaderAuthRequest, expected)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer patch.Unpatch()
		if err := p.setProxyAuthorizationHeader(req); err != nil {
			assert.NoError(t, err)
		}

		actual := req.Header.Get(HeaderProxyAuthorization)
		assert.Equal(t, expected, actual)
	})

	p.config.Mode = BasicMode

	t.Run("basic mode", func(t *testing.T) {
		expected := "Basic dGVzdF91c2VyOnRlc3RfcGFzc3dvcmQ="

		if err := p.setProxyAuthorizationHeader(req); err != nil {
			assert.NoError(t, err)
		}

		actual := req.Header.Get(HeaderProxyAuthorization)
		assert.Equal(t, expected, actual)
	})
}
