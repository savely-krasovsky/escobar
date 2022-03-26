//go:build !windows
// +build !windows

package configs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/L11R/escobar/internal/proxy"
)

func (c *Config) CheckCredentials() error {
	// Check keytab directory and file rights, it MUST NOT be too permissive
	if c.Proxy.DownstreamProxyAuth.Keytab != "" {
		dirInfo, err := os.Stat(filepath.Dir(c.Proxy.DownstreamProxyAuth.Keytab))
		if err != nil {
			return fmt.Errorf("cannot get dir stats: %w", err)
		}

		if m := dirInfo.Mode(); m.Perm() != os.FileMode(0700) {
			return fmt.Errorf("keytab directory rights are too permissive")
		}

		fileInfo, err := os.Stat(c.Proxy.DownstreamProxyAuth.Keytab)
		if err != nil {
			return fmt.Errorf("cannot get file stats: %w", err)
		}

		if m := fileInfo.Mode(); m.Perm() != os.FileMode(0600) {
			return fmt.Errorf("keytab file rights are too permissive")
		}
	}

	if c.Proxy.Mode != proxy.ManualMode {
		return nil
	}

	if c.Proxy.DownstreamProxyAuth.Password != "" || c.Proxy.DownstreamProxyAuth.Keytab != "" {
		return nil
	}

	return fmt.Errorf("you should pass path keytab-file or at least password")
}
