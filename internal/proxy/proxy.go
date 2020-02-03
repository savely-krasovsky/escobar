// Copyright (c) 2020 Savely Krasovsky. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/L11R/httputil"

	"github.com/jcmturner/gokrb5/v8/client"
	"go.uber.org/zap"
)

const (
	LogEntryCtx        = "log_entry"
	ProxyAuthorization = "Proxy-Authorization"
)

type Proxy struct {
	logger    *zap.Logger
	config    *Config
	krb5cl    *client.Client
	server    *http.Server
	httpProxy *httputil.ReverseProxy
}

// NewProxy returns Proxy instance
func NewProxy(logger *zap.Logger, config *Config, krb5cl *client.Client) *Proxy {
	fp := httputil.NewForwardingProxy()
	fp.ErrorHandler = httpErrorHandler

	p := &Proxy{
		logger:    logger,
		config:    config,
		krb5cl:    krb5cl,
		httpProxy: fp,
	}

	p.httpProxy.ErrorLog = zap.NewStdLog(logger)
	p.server = &http.Server{
		Addr:    config.Addr.String(),
		Handler: p,
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		// Timeouts
		ReadTimeout:       config.Timeouts.Server.ReadTimeout,
		ReadHeaderTimeout: config.Timeouts.Server.ReadHeaderTimeout,
		WriteTimeout:      config.Timeouts.Server.WriteTimeout,
		IdleTimeout:       config.Timeouts.Server.IdleTimeout,
	}

	return p
}

// CheckAuth checks auth against Ping URL; should be called after starting proxy server itself
func (p *Proxy) CheckAuth() (bool, error) {
	u, err := url.Parse("http://" + p.config.Addr.String())
	if err != nil {
		return false, fmt.Errorf("invalid proxy url: %w", err)
	}

	// We need http client with custom transport
	httpClient := http.DefaultClient

	tr := http.DefaultTransport
	// Pass our newly deployed local proxy
	tr.(*http.Transport).Proxy = http.ProxyURL(u)
	// We check it against corporate proxy, so it usually use MITM
	tr.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient.Transport = tr

	req, err := http.NewRequest("GET", p.config.PingURL.String(), nil)
	if err != nil {
		return false, fmt.Errorf("cannot create request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req = req.WithContext(ctx)

	if err := p.setProxyAuthorizationHeader(req); err != nil {
		return false, fmt.Errorf("cannot set authorization header: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("cannot do request: %w", err)
	}
	//noinspection ALL
	defer resp.Body.Close()

	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		return false, fmt.Errorf("cannot read body: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, nil
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	logger := p.logger.With(
		zap.String("http_proto", req.Proto),
		zap.String("http_method", req.Method),
		zap.String("user_agent", req.UserAgent()),
		zap.String("uri", req.RequestURI),
	)
	req = req.WithContext(context.WithValue(context.Background(), LogEntryCtx, logger))

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				logger.Error("Panic recovered", zap.Error(err))
			}

			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	}()

	logger.Debug("Request started")

	if req.URL.Scheme == "http" {
		p.http(rw, req)
	} else {
		p.https(rw, req)
	}

	logger.Debug("Request completed")
}

func httpErrorHandler(rw http.ResponseWriter, req *http.Request, err error) {
	logger := req.Context().Value(LogEntryCtx).(*zap.Logger)

	if errors.Is(err, context.Canceled) {
		logger.Debug("http: proxy client disconnected")
		return
	}

	logger.Error("http: proxy error", zap.Error(err))
	rw.WriteHeader(http.StatusBadGateway)
}

func (p *Proxy) http(rw http.ResponseWriter, req *http.Request) {
	tr := http.DefaultTransport
	tr.(*http.Transport).Proxy = http.ProxyURL(p.config.DownstreamProxyURL)

	if err := p.setProxyAuthorizationHeader(req); err != nil {
		httpErrorHandler(rw, req, fmt.Errorf("cannot set authorization header: %w", err))
		return
	}

	p.httpProxy.ServeHTTP(rw, req)
}

func httpsErrorHandler(rw http.ResponseWriter, req *http.Request, err error) {
	logger := req.Context().Value(LogEntryCtx).(*zap.Logger)

	logger.Error("https: proxy error", zap.Error(err))
	rw.WriteHeader(http.StatusBadGateway)
}

func httpsErrorHijackedHandler(brw *bufio.ReadWriter, req *http.Request, err error) {
	logger := req.Context().Value(LogEntryCtx).(*zap.Logger)

	logger.Error("https: proxy error", zap.Error(err))

	resp := newResponse(http.StatusBadGateway, nil, req)
	if err := resp.Write(brw); err != nil {
		logger.Error("Cannot write response", zap.Error(err))
	}
	if err := brw.Flush(); err != nil {
		logger.Error("Cannot flush writer", zap.Error(err))
	}
}

func (p *Proxy) https(rw http.ResponseWriter, req *http.Request) {
	logger := req.Context().Value(LogEntryCtx).(*zap.Logger)

	// Check that we are handling CONNECT
	if req.Method != http.MethodConnect {
		http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Take control of the connection
	hj, ok := rw.(http.Hijacker)
	if !ok {
		httpsErrorHandler(rw, req, fmt.Errorf("hijacking is not supported"))
		return
	}

	conn, brw, err := hj.Hijack()
	if err != nil {
		httpsErrorHandler(rw, req, fmt.Errorf("hijack failed: %w", err))
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("Cannot close connection", zap.Error(err))
		}
	}()

	// Set Keep-Alive
	if tconn, ok := conn.(*net.TCPConn); ok {
		if err := tconn.SetKeepAlive(true); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot turn on keep-alive: %w", err))
			return
		}
		if err := tconn.SetKeepAlivePeriod(p.config.Timeouts.Client.KeepAlivePeriod); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set keep-alive period: %w", err))
			return
		}
	}

	// Set client connection timeouts
	now := time.Now()
	if p.config.Timeouts.Client.ReadTimeout.Nanoseconds() != 0 {
		if err := conn.SetReadDeadline(now.Add(p.config.Timeouts.Client.ReadTimeout)); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set read timeout for connection with client: %w", err))
			return
		}
	}
	if p.config.Timeouts.Client.WriteTimeout.Nanoseconds() != 0 {
		if err := conn.SetWriteDeadline(now.Add(p.config.Timeouts.Client.WriteTimeout)); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set write timeout for connection with client: %w", err))
			return
		}
	}

	p.connectAndCopy(conn, brw, req, false)
}

// connectAndCopy connects to downstream proxy, authenticates if there is a need and copies traffic between connections
func (p *Proxy) connectAndCopy(conn net.Conn, brw *bufio.ReadWriter, req *http.Request, reconnected bool) {
	logger := req.Context().Value(LogEntryCtx).(*zap.Logger)

	// Open connection with downstream proxy
	pconn, err := net.DialTimeout("tcp", p.config.DownstreamProxyURL.Host, p.config.Timeouts.DownstreamProxy.DialTimeout)
	if err != nil {
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot connect to downstream proxy: %w", err))
		return
	}
	defer func() {
		if err := pconn.Close(); err != nil {
			logger.Error("Cannot close connection", zap.Error(err))
		}
	}()

	// Set Keep-Alive
	if tconn, ok := pconn.(*net.TCPConn); ok {
		if err := tconn.SetKeepAlive(true); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot turn on keep-alive: %w", err))
			return
		}
		if err := tconn.SetKeepAlivePeriod(p.config.Timeouts.DownstreamProxy.KeepAlivePeriod); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set keep-alive period: %w", err))
			return
		}
	}

	// Set Downstream Proxy connection timeouts
	now := time.Now()
	if p.config.Timeouts.DownstreamProxy.ReadTimeout.Nanoseconds() != 0 {
		if err := pconn.SetReadDeadline(now.Add(p.config.Timeouts.DownstreamProxy.ReadTimeout)); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set read timeout for connection with downstream proxy: %w", err))
			return
		}
	}
	if p.config.Timeouts.DownstreamProxy.WriteTimeout.Nanoseconds() != 0 {
		if err := pconn.SetWriteDeadline(now.Add(p.config.Timeouts.DownstreamProxy.WriteTimeout)); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set write timeout for connection with downstream proxy: %w", err))
			return
		}
	}

	pbw := bufio.NewWriter(pconn)
	pbr := bufio.NewReader(pconn)

	// Write client's request into proxy connection
	if err := req.Write(pbw); err != nil {
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot write request into proxy connection: %w", err))
		return
	}
	if err := pbw.Flush(); err != nil {
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot flush writer to commit request into proxy connection: %w", err))
		return
	}

	// Read response from body, usually it's just 407 Proxy Authentication Required
	resp, err := http.ReadResponse(pbr, req)
	if err != nil {
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot read response from proxy connection: %w", err))
		return
	}

	switch resp.StatusCode {
	// Proxy authentication required, we have to pass Proxy-Authorization header
	case http.StatusProxyAuthRequired:
		// Close the body, no need to read it, there is only HTML template with Proxy warning
		//noinspection ALL
		resp.Body.Close()

		// Set Proxy-Authorization header
		if err := p.setProxyAuthorizationHeader(req); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot set authrorization header: %w", err))
			return
		}

		// Write request into proxy connection again, now with proper auth
		if err := req.Write(pbw); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot write request into proxy connection: %w", err))
			return
		}
		if err := pbw.Flush(); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot flush writer to commit request into proxy connection: %w", err))
			return
		}

		// Read proxy response again, hope user credentials are valid and proxy returned 200
		resp, err = http.ReadResponse(pbr, req)
		if err != nil {
			// Some proxies drop connection after responding 407
			var target *net.OpError
			if (errors.As(err, &target) || errors.Is(err, io.ErrUnexpectedEOF)) && !reconnected {
				// Reconnection could be tried only once to prevent infinity loop;
				// Proxy-Authorization header is already set, so we can try again;
				p.connectAndCopy(conn, brw, req, true)
				return
			}

			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot read response from proxy connection: %w", err))
			return
		}
		if resp.StatusCode == http.StatusOK {
			resp.Body = nil
		} else {
			logger.Warn("Proxy did NOT return 200 Connection established", zap.Int("proxy_resp_code", resp.StatusCode))
		}

		// Return this response to client
		if err := resp.Write(brw); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot write response from proxy into client connection: %w", err))
			return
		}
		if err := brw.Flush(); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot flush writer to commit response from proxy into client connection: %w", err))
			return
		}
	// 200 Connection established, we can immediately start data transfer
	case http.StatusOK:
		// Set body to nil, otherwise we will get deadlock
		resp.Body = nil

		// Return this response to client
		if err := resp.Write(brw); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot write response from proxy into client connection: %w", err))
			return
		}
		if err := brw.Flush(); err != nil {
			httpsErrorHijackedHandler(brw, req, fmt.Errorf("cannot flush writer to commit response from proxy into client connection: %w", err))
			return
		}
	default:
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("unknown code recieved: %d", resp.StatusCode))
		return
	}

	// Start traffic copying inside newly created tunnel
	errc := make(chan error, 1)
	cc := connectCopier{
		logger:  logger,
		client:  conn,
		backend: pconn,
	}
	go cc.copyToBackend(errc)
	go cc.copyFromBackend(errc)

	logger.Debug("CONNECT tunnel opened")
	defer logger.Debug("CONNECT tunnel closed")

	err = <-errc
	if err == nil {
		err = <-errc
	}

	if err != nil {
		httpsErrorHijackedHandler(brw, req, fmt.Errorf("traffic copying inside tunnel failed: %w", err))
		return
	}

	logger.Debug("Traffic copied successfully")
}

func (p *Proxy) Listen() (net.Listener, error) {
	p.logger.Info("Listening socket", zap.String("address", p.config.Addr.String()))

	l, err := net.Listen("tcp", p.config.Addr.String())
	if err != nil {
		p.logger.Error("Error while listening!", zap.Error(err))
		return nil, err
	}

	return l, nil
}

// Serve serves HTTP requests.
func (p *Proxy) Serve(l net.Listener) error {
	p.logger.Info("Serving HTTP requests", zap.String("address", p.config.Addr.String()))

	if err := p.server.Serve(l); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			p.logger.Error("Error while serving HTTP requests!", zap.Error(err))
			return err
		}
	}

	return nil
}

// Shutdown shuts down the HTTP server.
func (p *Proxy) Shutdown(ctx context.Context) error {
	if err := p.server.Shutdown(ctx); err != nil {
		p.logger.Error("Error shutting down HTTP server!", zap.Error(err))
		return err
	}

	return nil
}
