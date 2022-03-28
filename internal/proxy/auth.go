// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"encoding/base64"
	"fmt"
	"net/http"

	gospnego "github.com/L11R/go-spnego"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

func (p *Proxy) setProxyAuthorizationHeader(r *http.Request) error {
	switch p.config.Mode {
	case AutoMode:
		provider := gospnego.New()
		header, err := provider.GetSPNEGOHeader(p.config.DownstreamProxyURL.Hostname())
		if err != nil {
			return fmt.Errorf("cannot get SPNEGO header: %w", err)
		}

		r.Header.Set(HeaderProxyAuthorization, header)
	case ManualMode:
		if err := spnego.SetSPNEGOHeader(p.krb5cl, r, "HTTP/"+p.config.DownstreamProxyURL.Hostname()); err != nil {
			return fmt.Errorf("cannot set SPNEGO header: %w", err)
		}

		r.Header.Set(HeaderProxyAuthorization, r.Header.Get(spnego.HTTPHeaderAuthRequest))
		r.Header.Del(spnego.HTTPHeaderAuthRequest)
	case BasicMode:
		r.Header.Set(
			HeaderProxyAuthorization,
			"Basic "+base64.StdEncoding.EncodeToString(
				[]byte(p.config.DownstreamProxyAuth.User+":"+p.config.DownstreamProxyAuth.Password),
			),
		)
	}

	return nil
}
