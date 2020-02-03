// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build windows

package proxy

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/alexbrainman/sspi/negotiate"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

func (p *Proxy) setProxyAuthorizationHeader(r *http.Request) error {
	if !p.config.ManualMode {
		credentials, err := negotiate.AcquireCurrentUserCredentials()
		if err != nil {
			return fmt.Errorf("cannot acquire current user credentials: %w", err)
		}
		defer credentials.Release()

		ctx, token, err := negotiate.NewClientContext(credentials, "HTTP/"+p.config.DownstreamProxyURL.Hostname())
		if err != nil {
			return fmt.Errorf("cannot create new client context: %w", err)
		}
		defer ctx.Release()

		r.Header.Set(ProxyAuthorization, "Negotiate "+base64.StdEncoding.EncodeToString(token))
	} else if !p.config.BasicMode {
		if err := spnego.SetSPNEGOHeader(p.krb5cl, r, "HTTP/"+p.config.DownstreamProxyURL.Hostname()); err != nil {
			return fmt.Errorf("cannot set SPNEGO header: %w", err)
		}

		r.Header.Set(ProxyAuthorization, r.Header.Get(spnego.HTTPHeaderAuthRequest))
		r.Header.Del(spnego.HTTPHeaderAuthRequest)
	} else {
		r.Header.Set(
			ProxyAuthorization,
			"Basic "+base64.StdEncoding.EncodeToString(
				[]byte(p.config.DownstreamProxyAuth.User+":"+p.config.DownstreamProxyAuth.Password),
			),
		)
	}

	return nil
}
