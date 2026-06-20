// Package proxy implements RedTrace's intercepting MITM proxy. It captures
// HTTP and HTTPS traffic (decrypting TLS with on-the-fly certificates), applies
// match & replace rules and manual interception, persists each exchange, and
// streams a summary to subscribers.
package proxy

import (
	"bufio"
	"bytes"
	"compress/flate"
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
// Bodies larger than this are streamed to the client verbatim (see readCapped).
// It is a var only so tests can lower it.
var maxCapturedBody = 10 * 1024 * 1024

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

	// OnExchangeStored, if set, is called with the fully stored exchange (raw
	// bytes included) after capture — used by the passive scanner.
	OnExchangeStored func(*models.Exchange)

	server  *http.Server
	baseCtx context.Context // cancelled on shutdown; parents held-item contexts
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
	p.baseCtx = ctx
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
	// Decrypted tunnel requests carry no context of their own (http.ReadRequest
	// leaves it nil), so derive one from the connection. It is cancelled when
	// the tunnel closes and is parented on the proxy's base context, so held
	// items are released on shutdown instead of blocking their goroutines.
	connCtx, cancel := context.WithCancel(p.connContext())
	defer cancel()

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

		reqCtx, reqCancel := context.WithCancel(connCtx)
		// When interception is on, a request/response may be held; watch the
		// connection so a held item is released (dropped) if the client
		// disconnects mid-hold instead of blocking forever.
		var stop func()
		if p.interceptor.Enabled() {
			stop = p.watchClientClose(conn, br, reqCancel)
		}
		resp, respBody, stream := p.handle(reqCtx, req, body, scheme, authority)
		if stop != nil {
			stop()
		}

		ok := p.writeTunnelResponse(conn, resp, respBody, stream)
		reqCancel()
		if !ok || req.Close || resp.Close {
			return
		}
	}
}

// writeTunnelResponse writes resp to the raw (decrypted) tunnel connection. It
// returns false when the connection must be closed: on a write error, or after
// a streamed body whose framing is not keep-alive-safe.
func (p *Proxy) writeTunnelResponse(conn net.Conn, resp *http.Response, respBody []byte, stream io.Reader) bool {
	if _, err := conn.Write(rawResponseHead(resp)); err != nil {
		if stream != nil {
			_ = resp.Body.Close()
		}
		return false
	}
	if stream != nil {
		_, _ = io.Copy(conn, stream)
		_ = resp.Body.Close()
		return false
	}
	if len(respBody) > 0 {
		if _, err := conn.Write(respBody); err != nil {
			return false
		}
	}
	return true
}

// connContext returns the proxy's base context (cancelled on shutdown), or the
// background context if the proxy was not started via Serve.
func (p *Proxy) connContext() context.Context {
	if p.baseCtx != nil {
		return p.baseCtx
	}
	return context.Background()
}

// watchClientClose cancels via cancel() if the client closes the tunnel while a
// request/response is held. It peeks the connection without consuming any
// buffered (pipelined) bytes; the returned stop must be called before the read
// loop touches br again. A stop-induced read deadline only triggers a now-moot
// cancel, so it is harmless.
func (p *Proxy) watchClientClose(conn net.Conn, br *bufio.Reader, cancel context.CancelFunc) (stop func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := br.Peek(1); err != nil {
			cancel()
		}
	}()
	return func() {
		_ = conn.SetReadDeadline(time.Now())
		<-done
		_ = conn.SetReadDeadline(time.Time{})
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

	resp, respBody, stream := p.handle(r.Context(), r, body, "http", authority)
	if stream != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	// handle() already strips hop-by-hop response headers.
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if stream != nil {
		_, _ = io.Copy(w, stream)
	} else if len(respBody) > 0 {
		_, _ = w.Write(respBody)
	}
}

// handle runs the capture pipeline for one request. It returns the response to
// send to the client and either a body to write (respBody) or, for bodies too
// large to buffer, a stream to copy verbatim (stream != nil). respBody is nil
// for bodyless responses (HEAD/204/304/1xx).
func (p *Proxy) handle(ctx context.Context, req *http.Request, reqBody []byte, scheme, authority string) (*http.Response, []byte, io.Reader) {
	req = req.WithContext(ctx)
	req.URL.Scheme = scheme
	req.URL.Host = authority
	if req.Host == "" {
		req.Host = authority
	}

	reqBody = p.rules.ApplyRequest(req, reqBody)

	// Resolve the destination after match & replace: a URL/Host-rewriting rule
	// can retarget the request, so scope-gating and the stored metadata must
	// reflect the host/path actually contacted, not the original.
	host, port := splitAuthority(req.URL.Host, scheme)
	inScope := p.scope.InScope(host, req.URL.Path)

	// Manual interception of the request — only for in-scope traffic, so
	// background and out-of-scope requests are never held in the queue.
	rawReq := rawRequest(req, reqBody)
	if inScope {
		switch d := p.interceptor.Hold(ctx, &intercept.Held{
			Direction: intercept.DirRequest, Method: req.Method, URL: req.URL.String(), Host: host, Raw: rawReq,
		}); d.Action {
		case intercept.ActionDrop:
			return synthResponse(http.StatusGatewayTimeout, "Request dropped by RedTrace")
		case intercept.ActionForward:
			if d.Raw != nil {
				if nr, nb, err := reparseRequest(d.Raw, scheme, authority); err == nil {
					req, reqBody = nr.WithContext(ctx), nb
					rawReq = d.Raw
				}
			}
		}
	}

	prepareOutbound(req, reqBody)
	start := time.Now()
	resp, err := p.transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		p.logger.Debug("upstream error", "url", req.URL.String(), "err", err)
		return synthResponse(http.StatusBadGateway, "RedTrace upstream error: "+err.Error())
	}

	// Strip hop-by-hop response headers here so both the plain-HTTP and the
	// HTTPS-tunnel paths behave identically (the tunnel serializes headers
	// verbatim, so handleHTTP stripping alone would leave them on HTTPS traffic).
	for _, h := range hopByHop {
		resp.Header.Del(h)
	}

	// Bodyless responses must carry neither a body nor a fabricated length.
	if bodyless(req.Method, resp.StatusCode) {
		_ = resp.Body.Close()
		// 204/304/1xx forbid Content-Length; HEAD keeps the origin's value.
		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified ||
			(resp.StatusCode >= 100 && resp.StatusCode < 200) {
			resp.Header.Del("Content-Length")
		}
		resp.Header.Del("Transfer-Encoding")
		p.persist(req, reqBody, rawReq, resp, nil, rawResponse(resp, nil), scheme, host, port, inScope, duration)
		return resp, nil, nil
	}

	prefix, full, truncated := readCapped(resp.Body, maxCapturedBody)
	if truncated {
		// Too large to buffer: forward verbatim by streaming (origin framing
		// intact), store only the captured prefix, and close after — match &
		// replace and response interception don't apply to a streamed body.
		resp.Close = true
		resp.Header.Set("Connection", "close")
		p.persist(req, reqBody, rawReq, resp, prefix, rawResponse(resp, prefix), scheme, host, port, inScope, duration)
		return resp, nil, full
	}
	_ = resp.Body.Close()

	respBody := decompress(resp, prefix)
	respBody = p.rules.ApplyResponse(resp, respBody)

	// Manual interception of the response (in-scope traffic only).
	rawResp := rawResponse(resp, respBody)
	if inScope {
		switch d := p.interceptor.Hold(ctx, &intercept.Held{
			Direction: intercept.DirResponse, StatusCode: resp.StatusCode, URL: req.URL.String(), Host: host, Raw: rawResp,
		}); d.Action {
		case intercept.ActionDrop:
			return synthResponse(http.StatusGatewayTimeout, "Response dropped by RedTrace")
		case intercept.ActionForward:
			if d.Raw != nil {
				if nr, nb, err := reparseResponse(d.Raw, req); err == nil {
					resp, respBody = nr, nb
					rawResp = d.Raw
				}
			}
		}
	}

	p.persist(req, reqBody, rawReq, resp, respBody, rawResp, scheme, host, port, inScope, duration)

	// Normalize the response for writing back to the client.
	resp.Header.Del("Transfer-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(respBody)))
	return resp, respBody, nil
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
		if p.OnExchangeStored != nil {
			p.OnExchangeStored(ex)
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
	// Drop any per-connection headers the client named in Connection, then the
	// hop-by-hop set itself.
	for _, tok := range strings.Split(req.Header.Get("Connection"), ",") {
		if name := strings.TrimSpace(tok); name != "" {
			req.Header.Del(name)
		}
	}
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
	b, _ := io.ReadAll(io.LimitReader(r, int64(maxCapturedBody)))
	return b
}

// bodyless reports whether the response must not carry a message body, per
// RFC 9110 (HEAD requests, and 1xx/204/304 status codes).
func bodyless(method string, status int) bool {
	return method == http.MethodHead ||
		status == http.StatusNoContent ||
		status == http.StatusNotModified ||
		(status >= 100 && status < 200)
}

// readCapped reads up to limit bytes from r for capture/storage. If r holds
// more than limit bytes it returns truncated=true together with a reader that
// replays the captured prefix followed by the untouched remainder, so the body
// can be forwarded verbatim without buffering all of it.
func readCapped(r io.Reader, limit int) (prefix []byte, full io.Reader, truncated bool) {
	if r == nil {
		return nil, nil, false
	}
	prefix, _ = io.ReadAll(io.LimitReader(r, int64(limit)))
	if len(prefix) < limit {
		return prefix, nil, false
	}
	var probe [1]byte
	n, _ := io.ReadFull(r, probe[:])
	if n == 0 {
		return prefix, nil, false
	}
	return prefix, io.MultiReader(bytes.NewReader(prefix), bytes.NewReader(probe[:n]), r), true
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
		// Prefer zlib-wrapped DEFLATE; fall back to raw DEFLATE which some
		// servers send despite the spec.
		if zr, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			reader = zr
		} else {
			reader = flate.NewReader(bytes.NewReader(body))
		}
	default:
		return body
	}
	defer func() { _ = reader.Close() }()

	// Read one byte past the cap to detect a body whose decompressed size exceeds
	// what we buffer. A highly compressible body (small on the wire, huge decoded)
	// would otherwise be silently truncated and re-framed with a wrong length, so
	// in that case we forward the original compressed bytes verbatim instead —
	// Content-Encoding stays intact and the client decodes them itself.
	out, err := io.ReadAll(io.LimitReader(reader, int64(maxCapturedBody)+1))
	if err != nil || len(out) > maxCapturedBody {
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

// rawResponseHead serializes the status line and headers (no body), for writing
// a response whose body is sent separately (buffered bytes or a stream).
func rawResponseHead(resp *http.Response) []byte {
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
	return b.Bytes()
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

// synthResponse builds a RedTrace-generated response (drops, upstream errors).
// It returns the (resp, body, stream) tuple handle() uses and marks the
// connection to close so the client never mis-frames a synthetic reply.
func synthResponse(code int, msg string) (*http.Response, []byte, io.Reader) {
	body := []byte(msg + "\n")
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Close:      true,
		Header: http.Header{
			"Content-Type":   {"text/plain; charset=utf-8"},
			"Content-Length": {strconv.Itoa(len(body))},
			"Connection":     {"close"},
		},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}, body, nil
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
