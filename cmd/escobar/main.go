// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"github.com/L11R/escobar/internal/daemon"
	"github.com/kardianos/service"
)

func main() {
	d := daemon.New()

	svc, err := service.New(d, &service.Config{
		Name:        "escobar",
		DisplayName: "Escobar Proxy",
		Description: "Local forward proxy server that helps to remove authentication. It's a Kerberos alternative to cntlm utility.",
	})
	if err != nil {
		log.Fatalln(err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalln(err)
	}
}
