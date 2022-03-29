// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"bytes"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_newResponse(t *testing.T) {
	req, err := http.NewRequest("GET", "http://www.example.com", nil)
	if err != nil {
		t.Fatalf("http.NewRequest(): got %v, want no error", err)
	}
	req.Close = true

	res := newResponse(200, nil, req)
	if got, want := res.StatusCode, 200; got != want {
		t.Errorf("res.StatusCode: got %d, want %d", got, want)
	}
	if got, want := res.Status, "200 OK"; got != want {
		t.Errorf("res.Status: got %q, want %q", got, want)
	}
	if !res.Close {
		t.Error("res.Close: got false, want true")
	}
	if got, want := res.Proto, "HTTP/1.1"; got != want {
		t.Errorf("res.Proto: got %q, want %q", got, want)
	}
	if got, want := res.ProtoMajor, 1; got != want {
		t.Errorf("res.ProtoMajor: got %d, want %d", got, want)
	}
	if got, want := res.ProtoMinor, 1; got != want {
		t.Errorf("res.ProtoMinor: got %d, want %d", got, want)
	}
	if res.Header == nil {
		t.Error("res.Header: got nil, want header")
	}
	if got, want := res.Request, req; got != want {
		t.Errorf("res.Request: got %v, want %v", got, want)
	}
}

func Test_connectCopier(t *testing.T) {
	backend, err := net.Listen("tcp", "localhost:4430")
	require.NoError(t, err)

	proxy, err := net.Listen("tcp", "localhost:31280")
	require.NoError(t, err)

	handleProxy := func(clientProxyConn net.Conn) {
		defer clientProxyConn.Close()

		proxyBackendConn, err := net.DialTimeout("tcp", "localhost:4430", 10*time.Second)
		require.NoError(t, err)
		defer proxyBackendConn.Close()

		cc := connectCopier{
			logger:  zap.NewNop(),
			client:  clientProxyConn,
			backend: proxyBackendConn,
		}

		errc := make(chan error, 1)
		go cc.copyToBackend(errc)
		go cc.copyFromBackend(errc)
		require.NoError(t, <-errc)
	}

	go func() {
		for {
			conn, err := proxy.Accept()
			require.NoError(t, err)

			go handleProxy(conn)
		}
	}()

	handleBackend := func(clientBackendConn net.Conn) {
		defer clientBackendConn.Close()

		b := make([]byte, 1<<10)
		_, err := clientBackendConn.Read(b)
		require.NoError(t, err)

		if bytes.HasPrefix(b, []byte("ping")) {
			_, err = clientBackendConn.Write([]byte("pong"))
			require.NoError(t, err)

			return
		}

		_, err = clientBackendConn.Write([]byte("unknown command"))
		require.NoError(t, err)
	}

	go func() {
		for {
			conn, err := backend.Accept()
			require.NoError(t, err)

			go handleBackend(conn)
		}
	}()

	proxyClientConn, err := net.DialTimeout("tcp", "localhost:31280", 10*time.Second)
	require.NoError(t, err)
	defer proxyClientConn.Close()

	_, err = proxyClientConn.Write([]byte("ping"))
	require.NoError(t, err)

	b := make([]byte, 1<<10)
	_, err = proxyClientConn.Read(b)
	require.NoError(t, err)

	require.True(t, bytes.HasPrefix(b, []byte("pong")))
}
