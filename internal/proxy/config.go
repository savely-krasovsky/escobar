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

type Mode string

const (
	AutoMode   Mode = "auto"
	ManualMode Mode = "manual"
	BasicMode  Mode = "basic"
)

type Config struct {
	AddrString string       `short:"a" long:"addr" env:"ADDR" description:"Proxy address" default:"localhost:3128" json:"addr"`
	Addr       *net.TCPAddr `no-flag:"yes" json:"-"`

	DownstreamProxyURLString string   `short:"d" long:"downstream-proxy-url" env:"DOWNSTREAM_PROXY_URL" description:"Downstream proxy URL" value-name:"http://proxy.evil.corp:9090" required:"yes" json:"downstreamProxyURL"`
	DownstreamProxyURL       *url.URL `no-flag:"yes" json:"-"`

	DownstreamProxyAuth DownstreamProxyAuth `group:"Downstream Proxy authentication" namespace:"downstream-proxy-auth" env-namespace:"DOWNSTREAM_PROXY_AUTH" json:"downstreamProxyAuth"`

	Kerberos Kerberos `group:"Kerberos options" namespace:"kerberos" env-namespace:"KERBEROS" json:"kerberos"`
	Timeouts Timeouts `group:"Timeouts" namespace:"timeouts" env-namespace:"TIMEOUTS" json:"timeouts"`

	PingURLString string   `long:"ping-url" env:"PING_URL" description:"URL to ping anc check credentials validity" default:"https://www.google.com/" json:"pingURL"`
	PingURL       *url.URL `no-flag:"yes" json:"-"`

	Mode Mode `short:"m" long:"mode" env:"MODE" description:"Escobar mode" default:"auto" json:"mode"`
}

type DownstreamProxyAuth struct {
	User     string `short:"u" long:"user" env:"USER" description:"Downstream Proxy user" json:"user"`
	Password string `short:"p" long:"password" env:"PASSWORD" description:"Downstream Proxy password" json:"password"`
	Keytab   string `short:"k" long:"keytab" env:"KEYTAB" description:"Downstream Proxy path to keytab-file" json:"keytab"`
}

type Kerberos struct {
	Realm string `long:"realm" env:"REALM" description:"Kerberos realm" value-name:"EVIL.CORP" json:"realm"`

	KDCString string       `long:"kdc" env:"KDC" description:"Key Distribution Center (KDC) address" value-name:"kdc.evil.corp:88" json:"kdc"`
	KDC       *net.TCPAddr `no-flag:"yes" json:"-"`
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
	Server          ServerTimeouts          `group:"Server timeouts" namespace:"server" env-namespace:"SERVER" json:"server"`
	Client          ClientTimeouts          `group:"Client timeouts" namespace:"client" env-namespace:"CLIENT" json:"client"`
	DownstreamProxy DownstreamProxyTimeouts `group:"Downstream Proxy timeouts" namespace:"downstream" env-namespace:"DOWNSTREAM" json:"downstreamProxy"`
}

type ServerTimeouts struct {
	ReadTimeout       time.Duration `long:"read" env:"READ" default:"0s" description:"HTTP server read timeout" json:"readTimeout"`
	ReadHeaderTimeout time.Duration `long:"read-header" env:"READ_HEADER" default:"30s" description:"HTTP server read header timeout" json:"readHeaderTimeout"`
	WriteTimeout      time.Duration `long:"write" env:"WRITE" default:"0s" description:"HTTP server write timeout" json:"writeTimeout"`
	IdleTimeout       time.Duration `long:"idle" env:"IDLE" default:"1m" description:"HTTP server idle timeout" json:"idleTimeout"`
}

type ClientTimeouts struct {
	ReadTimeout     time.Duration `long:"read" env:"READ" default:"0s" description:"Client read timeout" json:"readTimeout"`
	WriteTimeout    time.Duration `long:"write" env:"WRITE" default:"0s" description:"Client write timeout" json:"writeTimeout"`
	KeepAlivePeriod time.Duration `long:"keepalive-period" env:"KEEPALIVE_PERIOD" default:"1m" description:"Client keepalive period" json:"keepAlivePeriod"`
}

type DownstreamProxyTimeouts struct {
	DialTimeout     time.Duration `long:"dial" env:"DIAL" default:"10s" description:"Downstream proxy dial timeout" json:"dialTimeout"`
	ReadTimeout     time.Duration `long:"read" env:"READ" default:"0s" description:"Downstream proxy read timeout" json:"readTimeout"`
	WriteTimeout    time.Duration `long:"write" env:"WRITE" default:"0s" description:"Downstream proxy write timeout" json:"writeTimeout"`
	KeepAlivePeriod time.Duration `long:"keepalive-period" env:"KEEPALIVE_PERIOD" default:"1m" description:"Downstream proxy keepalive period" json:"keepAlivePeriod"`
}
