package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

type harness struct {
	proxyURL *url.URL
	client   *http.Client
	db       *storage.DB
	rules    *intercept.RuleSet
	scope    *scope.Scope
	ic       *intercept.Interceptor
	captured chan storage.RequestSummary
}

func startProxy(t *testing.T) *harness {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "proxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	authority, err := cert.LoadOrCreateAuthority(t.TempDir())
	if err != nil {
		t.Fatalf("authority: %v", err)
	}

	sc := scope.New()
	rules := intercept.NewRuleSet()
	ic := intercept.NewInterceptor(storage.NewID)

	p, err := New(Config{}, authority, db, sc, rules, ic, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}
	captured := make(chan storage.RequestSummary, 16)
	p.OnExchange = func(s storage.RequestSummary) { captured <- s }

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = p.Serve(ctx, ln) }()

	proxyURL, _ := url.Parse("http://" + ln.Addr().String())
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(authority.CACertPEM())

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{RootCAs: caPool, MinVersion: tls.VersionTLS12},
		},
	}
	t.Cleanup(client.CloseIdleConnections)

	return &harness{proxyURL: proxyURL, client: client, db: db, rules: rules, scope: sc, ic: ic, captured: captured}
}

func (h *harness) wait(t *testing.T) storage.RequestSummary {
	t.Helper()
	select {
	case s := <-h.captured:
		return s
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for captured exchange")
		return storage.RequestSummary{}
	}
}

func TestProxy_HTTPSCapture(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "hello %s", r.URL.Query().Get("q"))
	}))
	defer origin.Close()

	h := startProxy(t)

	resp, err := h.client.Get(origin.URL + "/greet?q=world")
	if err != nil {
		t.Fatalf("GET via proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "hello world" {
		t.Fatalf("body = %q, want %q", body, "hello world")
	}

	s := h.wait(t)
	if s.Method != "GET" || s.StatusCode != 200 || s.Path != "/greet" || s.Scheme != "https" {
		t.Errorf("unexpected summary: %+v", s)
	}

	// The exchange must be searchable by response content via FTS.
	rows, total, err := h.db.ListRequests(context.Background(), storage.RequestFilter{Contains: "hello world"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 indexed exchange, got total=%d rows=%d", total, len(rows))
	}
}

func TestProxy_PlainHTTPCapture(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain-ok")
	}))
	defer origin.Close()

	h := startProxy(t)

	resp, err := h.client.Get(origin.URL + "/x")
	if err != nil {
		t.Fatalf("GET via proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "plain-ok" {
		t.Fatalf("body = %q", body)
	}

	s := h.wait(t)
	if s.Scheme != "http" || s.StatusCode != 200 {
		t.Errorf("unexpected summary: %+v", s)
	}
}

func TestProxy_MatchReplaceResponse(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "the secret is alpha")
	}))
	defer origin.Close()

	h := startProxy(t)
	if err := h.rules.SetRules([]intercept.Rule{{
		ID: "1", Enabled: true, Part: intercept.PartResponseBody,
		MatchType: intercept.Literal, Match: "alpha", Replace: "REDACTED",
	}}); err != nil {
		t.Fatalf("set rules: %v", err)
	}

	resp, err := h.client.Get(origin.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "the secret is REDACTED" {
		t.Fatalf("match&replace not applied: %q", body)
	}
	h.wait(t)
}

func TestProxy_InterceptOnlyHoldsInScope(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	h := startProxy(t)
	// Only example.com is in scope; the loopback origin is out of scope.
	if err := h.scope.SetRules([]scope.Rule{{
		ID: "1", Enabled: true, Kind: scope.Include, Matcher: scope.MatchHost, Value: "example.com",
	}}); err != nil {
		t.Fatalf("scope: %v", err)
	}
	h.ic.SetEnabled(true)

	// With interception on, an out-of-scope request must pass straight through
	// (never held) rather than flooding the queue.
	resp, err := h.client.Get(origin.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
	if n := len(h.ic.Queue()); n != 0 {
		t.Errorf("out-of-scope request was held (queue=%d); intercept must respect scope", n)
	}
	h.wait(t)
}

func TestProxy_ScopeTagging(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer origin.Close()

	h := startProxy(t)
	// Only example.com is in scope; the loopback origin should be out of scope.
	if err := h.scope.SetRules([]scope.Rule{{
		ID: "1", Enabled: true, Kind: scope.Include, Matcher: scope.MatchHost, Value: "example.com",
	}}); err != nil {
		t.Fatalf("scope: %v", err)
	}

	resp, err := h.client.Get(origin.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()

	if s := h.wait(t); s.InScope {
		t.Errorf("expected out-of-scope, got inScope=true: %+v", s)
	}
}
