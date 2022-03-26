// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package configs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/L11R/escobar/internal/proxy"
	"github.com/L11R/escobar/internal/static"
	"github.com/L11R/escobar/internal/version"
	"github.com/shibukawa/configdir"

	"github.com/jessevdk/go-flags"
)

type Config struct {
	Proxy  *proxy.Config  `group:"Proxy args" namespace:"proxy" env-namespace:"ESCOBAR_PROXY" json:"proxy"`
	Static *static.Config `group:"Static args" namespace:"static" env-namespace:"ESCOBAR_STATIC" json:"static"`

	Install   bool `long:"install" description:"Install service" json:"-"`
	Uninstall bool `long:"uninstall" description:"Uninstall service" json:"-"`

	Verbose []bool `short:"v" long:"verbose" env:"ESCOBAR_VERBOSE" description:"Verbose logs" json:"verbose"`
	Version func() `short:"V" long:"version" description:"Escobar version" json:"-"`
}

// Parse returns *Config parsed from command line arguments.
func Parse() (*Config, error) {
	var (
		config Config
		err    error
	)

	// Print Escobar version if -V is passed
	config.Version = func() {
		fmt.Printf("escobar version: %s-%s\n", version.Version, version.Commit)
		os.Exit(0)
	}

	p := flags.NewParser(&config, flags.HelpFlag|flags.PassDoubleDash)
	if _, err = p.Parse(); err != nil {
		err, ok := err.(*flags.Error)
		if !ok {
			return nil, err
		}

		if !errors.Is(err.Type, flags.ErrRequired) {
			return nil, err
		}
	}

	// Various modes require various options
	user := p.FindOptionByLongName("proxy.downstream-proxy-auth.user")
	password := p.FindOptionByLongName("proxy.downstream-proxy-auth.password")
	kdc := p.FindOptionByLongName("proxy.kerberos.kdc")
	realm := p.FindOptionByLongName("proxy.kerberos.realm")

	switch config.Proxy.Mode {
	case proxy.SSPIMode:
		// does not require anything
	case proxy.ManualMode:
		user.Required = true
		kdc.Required = true
		realm.Required = true
	case proxy.BasicMode:
		user.Required = true
		password.Required = true
	}

	// Try to read config, otherwise try to parse again
	configDirs := configdir.New("Escobar", "Escobar")
	configDirs.LocalPath, _ = filepath.Abs(".")
	folder := configDirs.QueryFolderContainsFile("settings.json")
	if folder != nil && len(os.Args) == 1 {
		data, _ := folder.ReadFile("settings.json")
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Printf("Invalid config file: %v\n", err)
			os.Exit(1)
		}
	} else if _, err := p.Parse(); err != nil {
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
	if err := config.CheckCredentials(); err != nil {
		return nil, err
	}

	if config.Proxy.Mode == proxy.ManualMode {
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
