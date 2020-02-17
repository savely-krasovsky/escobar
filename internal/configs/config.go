// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package configs

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/L11R/escobar/internal/proxy"
	"github.com/L11R/escobar/internal/static"
	"github.com/L11R/escobar/internal/version"

	"github.com/jessevdk/go-flags"
)

type Config struct {
	Proxy  *proxy.Config  `group:"Proxy args" namespace:"proxy" env-namespace:"ESCOBAR_PROXY"`
	Static *static.Config `group:"Static args" namespace:"static" env-namespace:"ESCOBAR_STATIC"`

	Verbose []bool `short:"v" long:"verbose" env:"ESCOBAR_VERBOSE" description:"Verbose logs"`
	Version func() `short:"V" long:"version" description:"Escobar version"`
}

// Parse returns *Config parsed from command line arguments.
func Parse() (*Config, error) {
	var config Config

	// Print Escobar version if -V is passed
	config.Version = func() {
		fmt.Printf("escobar version: %s-%s\n", version.Version, version.Commit)
		os.Exit(0)
	}

	p := flags.NewParser(&config, flags.HelpFlag|flags.PassDoubleDash)
	// Windows uses SSPI by default, so there is no need to required username
	if runtime.GOOS == "windows" {
		switchRequired := func() {
			if user := p.FindOptionByLongName("proxy.downstream-proxy-auth.user"); user != nil {
				user.Required = !user.Required
			}
			if kdc := p.FindOptionByLongName("proxy.kerberos.kdc"); kdc != nil {
				kdc.Required = !kdc.Required
			}
			if realm := p.FindOptionByLongName("proxy.kerberos.realm"); realm != nil {
				realm.Required = !realm.Required
			}
		}

		switchRequired()

		// If manual mode is on, turn required fields again;
		// This is bad, but go-flags library hasn't p.AddOption() method
		// ref: https://github.com/jessevdk/go-flags/issues/195
		for _, arg := range os.Args {
			if arg == "-m" || arg == "/m" || strings.Contains(arg, "proxy.manual-mode") {
				switchRequired()
			}
		}
	}

	_, err := p.ParseArgs(os.Args)
	if err != nil {
		return nil, err
	}

	// Parse address as *net.TCPAddr
	config.Proxy.Addr, err = net.ResolveTCPAddr("tcp", config.Proxy.AddrString)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve proxy address: %w", err)
	}

	// Parse Downstream Proxy URL as *url.URL
	config.Proxy.DownstreamProxyURL, err = url.Parse(config.Proxy.DownstreamProxyURLString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse downstream proxy URL: %w", err)
	}
	// Otherwise user could provide proxy.server.local:3128, it will be parsed incorrectly by url package
	if config.Proxy.DownstreamProxyURL.Hostname() == "" {
		return nil, fmt.Errorf("incorrect URL format, you are probably passing it without http://")
	}

	// Windows has different right management model
	if runtime.GOOS == "windows" {
		if config.Proxy.ManualMode {
			if config.Proxy.DownstreamProxyAuth.Password == "" && config.Proxy.DownstreamProxyAuth.Keytab == "" {
				return nil, fmt.Errorf("you should pass path keytab-file or at least password")
			}
		}
	} else {
		if config.Proxy.DownstreamProxyAuth.Password == "" && config.Proxy.DownstreamProxyAuth.Keytab == "" {
			return nil, fmt.Errorf("you should pass path keytab-file or at least password")
		}

		// Check keytab directory and file rights, it MUST NOT be too permissive
		if config.Proxy.DownstreamProxyAuth.Keytab != "" {
			dirInfo, err := os.Stat(filepath.Dir(config.Proxy.DownstreamProxyAuth.Keytab))
			if err != nil {
				return nil, fmt.Errorf("cannot get dir stats: %w", err)
			}

			if m := dirInfo.Mode(); m.Perm() != os.FileMode(0700) {
				return nil, fmt.Errorf("keytab directory rights are too permissive")
			}

			fileInfo, err := os.Stat(config.Proxy.DownstreamProxyAuth.Keytab)
			if err != nil {
				return nil, fmt.Errorf("cannot get file stats: %w", err)
			}

			if m := fileInfo.Mode(); m.Perm() != os.FileMode(0600) {
				return nil, fmt.Errorf("keytab file rights are too permissive")
			}
		}
	}

	if !config.Proxy.BasicMode {
		config.Proxy.Kerberos.KDC, err = net.ResolveTCPAddr("tcp", config.Proxy.Kerberos.KDCString)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve KDC address: %w", err)
		}
	}

	// Parse Ping URL as *url.URL
	config.Proxy.PingURL, err = url.Parse(config.Proxy.PingURLString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse ping URL: %w", err)
	}

	config.Static.Addr, err = net.ResolveTCPAddr("tcp", config.Static.AddrString)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve static server address: %w", err)
	}

	return &config, nil
}
