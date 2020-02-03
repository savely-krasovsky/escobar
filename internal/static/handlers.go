// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package static

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi"
)

const pacFile = `function FindProxyForURL(url, host) {
    if (isInNet(host, "127.0.0.0", "255.0.0.0")) return "DIRECT";
    else if (isInNet(host, "10.0.0.0", "255.0.0.0")) return "DIRECT";
    else if (isInNet(host, "172.16.0.0", "255.240.0.0")) return "DIRECT";
    else if (isInNet(host, "192.168.0.0", "255.255.0.0")) return "DIRECT";

    return "PROXY %s; DIRECT";
}`

func (s *Static) newRouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/proxy.pac", s.pac)
	r.Get("/ca.crt", s.ca)

	return r
}

func (s *Static) pac(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
	if _, err := w.Write(
		[]byte(
			fmt.Sprintf(
				pacFile,
				s.proxyConfig.Addr.String(),
			),
		),
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ca returns actual root CA certificate by doing request through our proxy
func (s *Static) ca(w http.ResponseWriter, _ *http.Request) {
	u, err := url.Parse("http://" + s.proxyConfig.Addr.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// We need http client with custom transport
	httpClient := http.DefaultClient

	tr := http.DefaultTransport
	// Pass our newly deployed local proxy
	tr.(*http.Transport).Proxy = http.ProxyURL(u)
	// We check it against corporate proxy, so it usually use MITM
	tr.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient.Transport = tr

	req, err := http.NewRequest("GET", "https://www.google.com", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	resp.Body.Close()

	buf := bytes.NewBuffer(nil)
	if err := pem.Encode(buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: resp.TLS.PeerCertificates[len(resp.TLS.PeerCertificates)-1].Raw,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	if _, err := w.Write(buf.Bytes()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
