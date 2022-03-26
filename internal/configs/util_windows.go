//go:build windows
// +build windows

package configs

import (
	"fmt"

	"github.com/L11R/escobar/internal/proxy"
)

func (c *Config) CheckCredentials() error {
	if c.Proxy.Mode != proxy.ManualMode {
		return nil
	}

	if c.Proxy.DownstreamProxyAuth.Password != "" || c.Proxy.DownstreamProxyAuth.Keytab != "" {
		return nil
	}

	return fmt.Errorf("you should pass path keytab-file or at least password")
}
