// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"text/template"
	"time"
)

const krb5conf = `[libdefaults]
  default_realm = {{.Realm}}

[realms]
  {{.Realm}} = {
    kdc = {{.KDC.String}}
  }`

type Config struct {
	AddrString string       `short:"a" long:"addr" env:"ADDR" description:"Proxy address" default:"localhost:3128"`
	Addr       *net.TCPAddr `no-flag:"yes"`

	DownstreamProxyURLString string   `short:"d" long:"downstream-proxy-url" env:"DOWNSTREAM_PROXY_URL" description:"Downstream proxy URL" value-name:"http://proxy.evil.corp:9090" required:"yes"`
	DownstreamProxyURL       *url.URL `no-flag:"yes"`

	DownstreamProxyAuth DownstreamProxyAuth `group:"Downstream Proxy authentication" namespace:"downstream-proxy-auth" env-namespace:"DOWNSTREAM_PROXY_AUTH"`

	Kerberos Kerberos `group:"Kerberos options" namespace:"kerberos" env-namespace:"KERBEROS"`
	Timeouts Timeouts `group:"Timeouts" namespace:"timeouts" env-namespace:"TIMEOUTS"`

	PingURLString string   `long:"ping-url" env:"PING_URL" description:"URL to ping anc check credentials validity" default:"https://www.google.com/"`
	PingURL       *url.URL `no-flag:"yes"`

	BasicMode  bool `short:"b" long:"basic-mode" env:"BASIC_MODE" description:"Basic authorization mode (do not use if Kerberos works for you)"`
	ManualMode bool `short:"m" long:"manual-mode" env:"MANUAL_MODE" description:"Turns off Windows SSPI (which is enabled by default)"`
}

type DownstreamProxyAuth struct {
	User     string `short:"u" long:"user" env:"USER" description:"Downstream Proxy user" required:"yes"`
	Password string `short:"p" long:"password" env:"PASSWORD" description:"Downstream Proxy password"`
	Keytab   string `short:"k" long:"keytab" env:"KEYTAB" description:"Downstream Proxy path to keytab-file"`
}

type Kerberos struct {
	Realm string `long:"realm" env:"REALM" description:"Kerberos realm" required:"yes" value-name:"EVIL.CORP"`

	KDCString string       `long:"kdc" env:"KDC_ADDR" description:"Key Distribution Center (KDC) address" required:"yes" value-name:"kdc.evil.corp:88"`
	KDC       *net.TCPAddr `no-flag:"yes"`
}

func (k *Kerberos) Reader() (io.Reader, error) {
	tmpl, err := template.New("krb5.conf").Parse(krb5conf)
	if err != nil {
		return nil, fmt.Errorf("cannot parse template: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, k); err != nil {
		return nil, fmt.Errorf("cannot execute template: %w", err)
	}

	return buf, nil
}

type Timeouts struct {
	Server          ServerTimeouts          `group:"Server timeouts" namespace:"server" env-namespace:"SERVER"`
	Client          ClientTimeouts          `group:"Client timeouts" namespace:"client" env-namespace:"CLIENT"`
	DownstreamProxy DownstreamProxyTimeouts `group:"Downstream Proxy timeouts" namespace:"downstream" env-namespace:"DOWNSTREAM"`
}

type ServerTimeouts struct {
	ReadTimeout       time.Duration `long:"read" env:"READ" default:"0s" description:"HTTP server read timeout"`
	ReadHeaderTimeout time.Duration `long:"read-header" env:"READ_HEADER" default:"30s" description:"HTTP server read header timeout"`
	WriteTimeout      time.Duration `long:"write" env:"WRITE" default:"0s" description:"HTTP server write timeout"`
	IdleTimeout       time.Duration `long:"idle" env:"IDLE" default:"1m" description:"HTTP server idle timeout"`
}

type ClientTimeouts struct {
	ReadTimeout     time.Duration `long:"read" env:"READ" default:"0s" description:"Client read timeout"`
	WriteTimeout    time.Duration `long:"write" env:"WRITE" default:"0s" description:"Client write timeout"`
	KeepAlivePeriod time.Duration `long:"keepalive-period" env:"KEEPALIVE_PERIOD" default:"1m" description:"Client keepalive period"`
}

type DownstreamProxyTimeouts struct {
	DialTimeout     time.Duration `long:"dial" env:"DIAL" default:"10s" description:"Downstream proxy dial timeout"`
	ReadTimeout     time.Duration `long:"read" env:"READ" default:"0s" description:"Downstream proxy read timeout"`
	WriteTimeout    time.Duration `long:"write" env:"WRITE" default:"0s" description:"Downstream proxy write timeout"`
	KeepAlivePeriod time.Duration `long:"keepalive-period" env:"KEEPALIVE_PERIOD" default:"1m" description:"Downstream proxy keepalive period"`
}
