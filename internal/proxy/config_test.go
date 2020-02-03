// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestKerberos_Reader(t *testing.T) {
	kdc := &net.TCPAddr{
		IP:   net.IPv4(10, 0, 0, 1),
		Port: 88,
		Zone: "",
	}

	p := NewProxy(
		zap.NewNop(),
		&Config{
			Kerberos: Kerberos{
				Realm: "EVIL.CORP",
				KDC:   kdc,
			},
		},
		nil,
	)

	expected := `[libdefaults]
  default_realm = EVIL.CORP

[realms]
  EVIL.CORP = {
    kdc = 10.0.0.1:88
  }`

	r, err := p.config.Kerberos.Reader()
	require.NoError(t, err)

	raw, err := ioutil.ReadAll(r)
	require.NoError(t, err)

	actual := string(raw)
	assert.Equal(t, expected, actual)
}
