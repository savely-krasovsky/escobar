// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"syscall"

	"go.uber.org/zap"
)

// newResponse builds new HTTP responses.
// If body is nil, an empty byte.Buffer will be provided to be consistent with
// the guarantees provided by http.Transport and http.Client.
func newResponse(code int, body io.Reader, req *http.Request) *http.Response {
	if body == nil {
		body = &bytes.Buffer{}
	}

	rc, ok := body.(io.ReadCloser)
	if !ok {
		rc = ioutil.NopCloser(body)
	}

	res := &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
		Body:       rc,
		Request:    req,
	}

	if req != nil {
		res.Close = req.Close
		res.Proto = req.Proto
		res.ProtoMajor = req.ProtoMajor
		res.ProtoMinor = req.ProtoMinor
	}

	return res
}

type connectCopier struct {
	logger          *zap.Logger
	client, backend io.ReadWriter
}

func (c connectCopier) copyFromBackend(errc chan<- error) {
	_, err := io.Copy(c.client, c.backend)

	if _, ok := c.client.(*net.TCPConn); ok {
		if err := c.client.(*net.TCPConn).CloseWrite(); err != nil {
			if err, ok := err.(*net.OpError).Err.(*os.SyscallError); ok {
				if err.Err != syscall.ENOTCONN {
					c.logger.Error("cannot close write", zap.Error(err))
				} else {
					c.logger.Debug("cannot close write", zap.Error(err))
				}
			}
		}
	}
	if _, ok := c.backend.(*net.TCPConn); ok {
		if err := c.backend.(*net.TCPConn).CloseRead(); err != nil {
			if err, ok := err.(*net.OpError).Err.(*os.SyscallError); ok {
				if err.Err != syscall.ENOTCONN {
					c.logger.Error("cannot close read", zap.Error(err))
				} else {
					c.logger.Debug("cannot close write", zap.Error(err))
				}
			}
		}
	}

	errc <- err
}

func (c connectCopier) copyToBackend(errc chan<- error) {
	_, err := io.Copy(c.backend, c.client)

	if _, ok := c.client.(*net.TCPConn); ok {
		if err := c.client.(*net.TCPConn).CloseRead(); err != nil {
			if err, ok := err.(*net.OpError).Err.(*os.SyscallError); ok {
				if err.Err != syscall.ENOTCONN {
					c.logger.Error("cannot close read", zap.Error(err))
				} else {
					c.logger.Debug("cannot close write", zap.Error(err))
				}
			}
		}
	}
	if _, ok := c.backend.(*net.TCPConn); ok {
		if err := c.backend.(*net.TCPConn).CloseWrite(); err != nil {
			if err, ok := err.(*net.OpError).Err.(*os.SyscallError); ok {
				if err.Err != syscall.ENOTCONN {
					c.logger.Error("cannot close write", zap.Error(err))
				} else {
					c.logger.Debug("cannot close write", zap.Error(err))
				}
			}
		}
	}

	errc <- err
}
