// Copyright (c) 2019 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package static

import "net"

type Config struct {
	AddrString string       `long:"addr" env:"ADDR" description:"Static server address" default:"localhost:3129" json:"addrString"`
	Addr       *net.TCPAddr `no-flag:"yes" json:"-"`
}
