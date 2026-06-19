// Package proxy implements RedTrace's intercepting MITM proxy. It captures
// HTTP and HTTPS traffic (decrypting TLS with on-the-fly certificates), applies
// match & replace rules and manual interception, persists each exchange, and
// streams a summary to subscribers.
package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	ws "github.com/utkarshrai2811/redtrace/internal/proxy/websocket"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// maxCapturedBody bounds how many body bytes are buffered/stored per message.
const maxCapturedBody = 10 * 1024 * 1024

// hopByHop are connection-specific headers that must not be forwarded.
var hopByHop = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// Store is the subset of storage used by the proxy.
type Store interface {
	StoreExchange(ctx context.Context, ex *models.Exchange) error
}

// Config configures the proxy listener.
type Config struct {
	ListenAddr    string // address to listen on, e.g. 127.0.0.1:8080
	UpstreamProxy string // optional upstream proxy URL (corporate/Burp chaining)
}

// Proxy is the MITM proxy engine.
type Proxy struct {
	cfg         Config
	authority   *cert.Authority
	store       Store
	scope       *scope.Scope
	rules       *intercept.RuleSet
	interceptor *intercept.Interceptor
	transport   *http.Transport
	logger      *slog.Logger

	// OnExchange, if set, is called with a summary of every captured exchange.
	OnExchange func(storage.RequestSummary)

	server *http.Server
}

// New constructs a Proxy. scope, rules, and interceptor may be shared with the
// API layer so configuration changes take effect live.
func New(cfg Config, authority *cert.Authority, store Store, sc *scope.Scope, rules *intercept.RuleSet, ic *intercept.Interceptor, logger *slog.Logger) (*Proxy, error) {
	if logger == nil {
		logger = slog.Default()
	}

	transport := &http.Transport{
		// We are intentionally an intercepting proxy: do not verify upstream
		// certificates, since the whole point is to reach arbitrary origins.
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for an intercepting proxy
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}

	if cfg.UpstreamProxy != "" {
		u, err := url.Parse(cfg.UpstreamProxy)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream proxy: %w", err)
		}
		transport.Proxy = http.ProxyURL(u)
	}

	return &Proxy{
		cfg:         cfg,
		authority:   authority,
		store:       store,
		scope:       sc,
		rules:       rules,
		interceptor: ic,
		transport:   transport,
		logger:      logger,
	}, nil
}

// Run binds the configured listen address and serves until ctx is cancelled.
func (p *Proxy) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}
	return p.Serve(ctx, ln)
}

// Serve serves the proxy on ln until ctx is cancelled or the server fails.
func (p *Proxy) Serve(ctx context.Context, ln net.Listener) error {
	p.server = &http.Server{
		Handler:           http.HandlerFunc(p.ServeHTTP),
		ReadHeaderTimeout: 30 * time.Second,
		// Connections are hijacked for MITM, so do not impose a write timeout.
	}

	go func() { //nolint:gosec // G118: the shutdown watcher must use a fresh context, not the serve context
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.server.Shutdown(shutdownCtx)
	}()

	p.logger.Info("proxy listening", "addr", ln.Addr().String())
	if err := p.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ServeHTTP dispatches CONNECT (HTTPS tunnels) and plain-HTTP proxy requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

// handleConnect establishes a MITM TLS tunnel and serves the requests within it.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	authority := r.Host // host:port

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		p.logger.Error("hijack failed", "err", err)
		return
	}
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		return
	}

	// Clients connecting by IP literal send no SNI, so fall back to the host
	// from the CONNECT target when ServerName is empty.
	connectHost, _ := splitAuthority(authority, "https")
	tlsConn := tls.Server(clientConn, &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := hello.ServerName
			if name == "" {
				name = connectHost
			}
			return p.authority.CertForHost(name)
		},
	})
	if err := tlsConn.Handshake(); err != nil {
		_ = tlsConn.Close()
		return
	}
	defer func() { _ = tlsConn.Close() }()

	p.serveTunnel(tlsConn, authority, "https")
}

// serveTunnel reads HTTP requests off an established (decrypted) connection and
// proxies each one, supporting keep-alive and protocol upgrades.
func (p *Proxy) serveTunnel(conn net.Conn, authority, scheme string) {
	br := bufio.NewReader(conn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		body := readBody(req.Body)
		_ = req.Body.Close()

		if isUpgrade(req) {
			p.tunnelUpgrade(conn, req, body, authority, scheme)
			return
		}

		resp, respBody := p.handle(req.Context(), req, body, scheme, authority)
		if err := writeResponse(conn, resp, respBody); err != nil {
			return
		}
		if req.Close || resp.Close {
			return
		}
	}
}

// handleHTTP proxies a plain-HTTP request received in absolute form.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Host == "" {
		http.Error(w, "redtrace: non-proxy request", http.StatusBadRequest)
		return
	}
	authority := r.Host
	body := readBody(r.Body)
	_ = r.Body.Close()

	if isUpgrade(r) {
		p.upgradeOverResponseWriter(w, r, body, authority)
		return
	}

	resp, respBody := p.handle(r.Context(), r, body, "http", authority)

	for k := range hopByHop {
		resp.Header.Del(hopByHop[k])
	}
	copyHeader(w.Header(), resp.Header)
	w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// handle runs the capture pipeline for one request and returns the response to
// send back to the client along with its (possibly rewritten) body.
func (p *Proxy) handle(ctx context.Context, req *http.Request, reqBody []byte, scheme, authority string) (*http.Response, []byte) {
	host, port := splitAuthority(authority, scheme)
	req.URL.Scheme = scheme
	req.URL.Host = authority
	if req.Host == "" {
		req.Host = authority
	}
	inScope := p.scope.InScope(host, req.URL.Path)

	reqBody = p.rules.ApplyRequest(req, reqBody)

	// Manual interception of the request.
	rawReq := rawRequest(req, reqBody)
	switch d := p.interceptor.Hold(ctx, &intercept.Held{
		Direction: intercept.DirRequest, Method: req.Method, URL: req.URL.String(), Host: host, Raw: rawReq,
	}); d.Action {
	case intercept.ActionDrop:
		return synthResponse(http.StatusGatewayTimeout, "Request dropped by RedTrace"), nil
	case intercept.ActionForward:
		if d.Raw != nil {
			if nr, nb, err := reparseRequest(d.Raw, scheme, authority); err == nil {
				req, reqBody = nr, nb
				rawReq = d.Raw
			}
		}
	}

	prepareOutbound(req, reqBody)
	start := time.Now()
	resp, err := p.transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		p.logger.Debug("upstream error", "url", req.URL.String(), "err", err)
		return synthResponse(http.StatusBadGateway, "RedTrace upstream error: "+err.Error()), nil
	}

	respBody := readBody(resp.Body)
	_ = resp.Body.Close()
	respBody = decompress(resp, respBody)
	respBody = p.rules.ApplyResponse(resp, respBody)

	// Manual interception of the response.
	rawResp := rawResponse(resp, respBody)
	switch d := p.interceptor.Hold(ctx, &intercept.Held{
		Direction: intercept.DirResponse, StatusCode: resp.StatusCode, URL: req.URL.String(), Host: host, Raw: rawResp,
	}); d.Action {
	case intercept.ActionDrop:
		return synthResponse(http.StatusGatewayTimeout, "Response dropped by RedTrace"), nil
	case intercept.ActionForward:
		if d.Raw != nil {
			if nr, nb, err := reparseResponse(d.Raw, req); err == nil {
				resp, respBody = nr, nb
				rawResp = d.Raw
			}
		}
	}

	p.persist(req, reqBody, rawReq, resp, respBody, rawResp, scheme, host, port, inScope, duration)

	// Normalize the response for writing back to the client.
	resp.Header.Del("Transfer-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(respBody)))
	return resp, respBody
}

func (p *Proxy) persist(req *http.Request, reqBody, rawReq []byte, resp *http.Response, respBody, rawResp []byte, scheme, host string, port int, inScope bool, dur time.Duration) {
	now := time.Now()
	reqID := storage.NewID()

	reqModel := &models.Request{
		ID:          reqID,
		Timestamp:   now,
		Source:      models.SourceProxy,
		Method:      req.Method,
		Scheme:      scheme,
		Host:        host,
		Port:        port,
		Path:        req.URL.Path,
		Query:       req.URL.RawQuery,
		URL:         req.URL.String(),
		HTTPVersion: req.Proto,
		ContentType: req.Header.Get("Content-Type"),
		BodySize:    len(reqBody),
		InScope:     inScope,
		Raw:         rawReq,
	}
	respModel := &models.Response{
		ID:          storage.NewID(),
		RequestID:   reqID,
		Timestamp:   now.Add(dur),
		StatusCode:  resp.StatusCode,
		Reason:      statusReason(resp.Status),
		HTTPVersion: resp.Proto,
		MimeType:    mimeType(resp.Header.Get("Content-Type")),
		BodySize:    len(respBody),
		DurationMs:  dur.Milliseconds(),
		Raw:         rawResp,
	}

	ex := &models.Exchange{Request: reqModel, Response: respModel}
	go func() {
		if err := p.store.StoreExchange(context.Background(), ex); err != nil {
			p.logger.Error("store exchange", "err", err)
			return
		}
		if p.OnExchange != nil {
			p.OnExchange(summaryOf(reqModel, respModel))
		}
	}()
}

// tunnelUpgrade relays an upgraded (e.g. WebSocket) connection to the origin.
func (p *Proxy) tunnelUpgrade(client net.Conn, req *http.Request, body []byte, authority, scheme string) {
	var upstream net.Conn
	var err error
	if scheme == "https" {
		host, _ := splitAuthority(authority, scheme)
		upstream, err = tls.Dial("tcp", authority, &tls.Config{InsecureSkipVerify: true, ServerName: host}) //nolint:gosec // intercepting proxy
	} else {
		upstream, err = net.DialTimeout("tcp", authority, 15*time.Second)
	}
	if err != nil {
		p.logger.Debug("upgrade dial failed", "authority", authority, "err", err)
		return
	}
	if _, err := upstream.Write(rawRequest(req, body)); err != nil {
		_ = upstream.Close()
		return
	}
	ws.Tunnel(client, upstream)
}

func (p *Proxy) upgradeOverResponseWriter(w http.ResponseWriter, r *http.Request, body []byte, authority string) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		return
	}
	p.tunnelUpgrade(client, r, body, authority, "http")
}

// ---- helpers ----

func prepareOutbound(req *http.Request, body []byte) {
	req.RequestURI = ""
	for _, h := range hopByHop {
		req.Header.Del(h)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
}

func readBody(r io.Reader) []byte {
	if r == nil {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(r, maxCapturedBody))
	return b
}

func decompress(resp *http.Response, body []byte) []byte {
	enc := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	var reader io.ReadCloser
	switch enc {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		reader = zr
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		reader = zr
	default:
		return body
	}
	defer func() { _ = reader.Close() }()

	out, err := io.ReadAll(io.LimitReader(reader, maxCapturedBody))
	if err != nil {
		return body
	}
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return out
}

func isUpgrade(req *http.Request) bool {
	if req.Header.Get("Upgrade") == "" {
		return false
	}
	for _, tok := range strings.Split(req.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(tok), "upgrade") {
			return true
		}
	}
	return false
}

func rawRequest(req *http.Request, body []byte) []byte {
	var b bytes.Buffer
	proto := req.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	uri := req.URL.RequestURI()
	fmt.Fprintf(&b, "%s %s %s\r\n", req.Method, uri, proto)
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	fmt.Fprintf(&b, "Host: %s\r\n", host)
	writeHeaders(&b, req.Header, "Host")
	b.WriteString("\r\n")
	b.Write(body)
	return b.Bytes()
}

func rawResponse(resp *http.Response, body []byte) []byte {
	var b bytes.Buffer
	proto := resp.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	status := resp.Status
	if status == "" {
		status = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	fmt.Fprintf(&b, "%s %s\r\n", proto, status)
	writeHeaders(&b, resp.Header, "")
	b.WriteString("\r\n")
	b.Write(body)
	return b.Bytes()
}

func writeHeaders(b *bytes.Buffer, h http.Header, skip string) {
	keys := make([]string, 0, len(h))
	for k := range h {
		if skip != "" && strings.EqualFold(k, skip) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			fmt.Fprintf(b, "%s: %s\r\n", k, v)
		}
	}
}

func writeResponse(conn net.Conn, resp *http.Response, body []byte) error {
	resp.Header.Del("Transfer-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	_, err := conn.Write(rawResponse(resp, body))
	return err
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func reparseRequest(raw []byte, scheme, authority string) (*http.Request, []byte, error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(raw)))
	if err != nil {
		return nil, nil, err
	}
	body := readBody(req.Body)
	_ = req.Body.Close()
	req.RequestURI = ""
	req.URL.Scheme = scheme
	if req.URL.Host == "" {
		req.URL.Host = authority
	}
	return req, body, nil
}

func reparseResponse(raw []byte, req *http.Request) (*http.Response, []byte, error) {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), req)
	if err != nil {
		return nil, nil, err
	}
	body := readBody(resp.Body)
	_ = resp.Body.Close()
	return resp, body, nil
}

func synthResponse(code int, msg string) *http.Response {
	body := []byte(msg)
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type": {"text/plain; charset=utf-8"},
		},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func splitAuthority(authority, scheme string) (host string, port int) {
	if h, p, err := net.SplitHostPort(authority); err == nil {
		host = h
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
		return host, port
	}
	if scheme == "https" {
		return authority, 443
	}
	return authority, 80
}

func statusReason(status string) string {
	if _, reason, ok := strings.Cut(status, " "); ok {
		return reason
	}
	return status
}

func mimeType(contentType string) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		return strings.TrimSpace(contentType[:i])
	}
	return strings.TrimSpace(contentType)
}

func summaryOf(req *models.Request, resp *models.Response) storage.RequestSummary {
	s := storage.RequestSummary{
		ID:        req.ID,
		Timestamp: req.Timestamp,
		Source:    string(req.Source),
		Method:    req.Method,
		Scheme:    req.Scheme,
		Host:      req.Host,
		Port:      req.Port,
		Path:      req.Path,
		URL:       req.URL,
		InScope:   req.InScope,
	}
	if resp != nil {
		s.StatusCode = resp.StatusCode
		s.ResponseLength = resp.BodySize
		s.MimeType = resp.MimeType
		s.DurationMs = resp.DurationMs
		s.HasResponse = true
	}
	return s
}
