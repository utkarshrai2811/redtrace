package scanner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseAndInsertions(t *testing.T) {
	raw := []byte("POST /search?q=hi&page=1 HTTP/1.1\r\n" +
		"Host: ex.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 7\r\n\r\nname=bob")
	tpl, err := parseRawRequest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tpl.method != "POST" || tpl.path != "/search" {
		t.Fatalf("method/path = %q %q", tpl.method, tpl.path)
	}
	kinds := map[string]string{}
	for _, ip := range tpl.insertions() {
		kinds[ip.Name] = ip.Kind
	}
	if kinds["q"] != "query" || kinds["page"] != "query" || kinds["name"] != "body" {
		t.Errorf("insertions = %v, want q/page query and name body", kinds)
	}

	// Rebuilding a body insertion must update Content-Length to the new body.
	out := tpl.build(insertion{Name: "name", Kind: "body"}, "INJECTED")
	if !strings.Contains(string(out), "name=INJECTED") {
		t.Errorf("body not substituted: %q", out)
	}
	if !strings.Contains(string(out), "Content-Length: 13") { // len("name=INJECTED")
		t.Errorf("Content-Length not updated: %q", out)
	}
}

func TestPlanCountRejectsNoParams(t *testing.T) {
	if _, err := PlanCount([]byte("GET / HTTP/1.1\r\nHost: ex.com\r\n\r\n")); err == nil {
		t.Error("expected error for a request with no insertion points")
	}
}

func TestBuildPreservesEncodedPayload(t *testing.T) {
	tpl, err := parseRawRequest([]byte("GET /?file=x HTTP/1.1\r\nHost: ex.com\r\n\r\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := string(tpl.build(insertion{Name: "file", Kind: "query"}, "..%2f..%2fetc%2fpasswd"))
	// The already-percent-encoded traversal payload must reach the wire single-
	// encoded, not double-encoded into %252f.
	if !strings.Contains(out, "..%2f..%2fetc%2fpasswd") {
		t.Errorf("encoded payload was mangled: %q", out)
	}
	if strings.Contains(out, "%252f") {
		t.Errorf("payload was double-encoded: %q", out)
	}
}

func TestInsertionsDeterministicOverCap(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("GET /?")
	for i := 0; i < 60; i++ {
		if i > 0 {
			sb.WriteByte('&')
		}
		fmt.Fprintf(&sb, "p%02d=v", i)
	}
	sb.WriteString(" HTTP/1.1\r\nHost: ex.com\r\n\r\n")
	tpl, err := parseRawRequest([]byte(sb.String()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, b := tpl.insertions(), tpl.insertions()
	if len(a) != maxInsertionPoints {
		t.Fatalf("got %d insertions, want cap %d", len(a), maxInsertionPoints)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("insertions not deterministic at %d: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestInsertionsSemicolonQuery(t *testing.T) {
	tpl, err := parseRawRequest([]byte("GET /a?x=1;y=2 HTTP/1.1\r\nHost: ex.com\r\n\r\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	names := map[string]bool{}
	for _, ip := range tpl.insertions() {
		names[ip.Name] = true
	}
	if !names["x"] || !names["y"] {
		t.Errorf("semicolon-separated query not parsed into insertions: %v", names)
	}
}

// collectStore captures findings and signals when the scan reaches a terminal
// status, deduping by fingerprint like the real store.
type collectStore struct {
	mu     sync.Mutex
	seen   map[string]bool
	issues []Issue
	done   chan struct{}
}

func newCollectStore() *collectStore {
	return &collectStore{seen: map[string]bool{}, done: make(chan struct{})}
}

func (s *collectStore) AddIssue(_ context.Context, _ string, is Issue) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[is.Fingerprint] {
		return false, nil
	}
	s.seen[is.Fingerprint] = true
	s.issues = append(s.issues, is)
	return true, nil
}
func (s *collectStore) PrepareScanRun(context.Context, string, int) error          { return nil }
func (s *collectStore) UpdateScanProgress(context.Context, string, int, int) error { return nil }
func (s *collectStore) SetScanStatus(_ context.Context, _, status string) error {
	if status == StatusCompleted || status == StatusStopped || status == StatusError {
		close(s.done)
	}
	return nil
}
func (s *collectStore) types() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[string]bool{}
	for _, is := range s.issues {
		m[is.Type] = true
	}
	return m
}

func TestActiveScanDetectsVulns(t *testing.T) {
	// A deliberately vulnerable endpoint: it reflects the q parameter unencoded
	// (XSS) and leaks a SQL error when q contains a quote (SQLi). Other payloads
	// are reflected literally and must NOT trigger their checks.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/html")
		if strings.ContainsAny(q, "'\"") {
			fmt.Fprintf(w, "<html>You have an error in your SQL syntax near '%s'</html>", q)
			return
		}
		fmt.Fprintf(w, "<html>results for %s</html>", q)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	store := newCollectStore()
	sc := NewScanner(store, nil)
	cfg := ActiveConfig{
		Scheme: "http", Host: u.Host, HTTPVersion: "HTTP/1.1",
		Template: []byte("GET /?q=hello HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
	}
	total, err := PlanCount(cfg.Template)
	if err != nil {
		t.Fatalf("PlanCount: %v", err)
	}
	started, err := sc.StartActive("t1", cfg, total)
	if err != nil || !started {
		t.Fatalf("StartActive = (%v, %v)", started, err)
	}

	select {
	case <-store.done:
	case <-time.After(15 * time.Second):
		t.Fatal("scan did not finish")
	}

	got := store.types()
	if !got["reflected_xss"] {
		t.Error("expected reflected_xss finding")
	}
	if !got["sql_injection"] {
		t.Error("expected sql_injection finding")
	}
	for _, falsePos := range []string{"path_traversal", "ssti", "command_injection", "open_redirect"} {
		if got[falsePos] {
			t.Errorf("unexpected (false positive) finding %q", falsePos)
		}
	}
}

func TestScannerRejectsDoubleStart(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)
	u, _ := url.Parse(srv.URL)

	store := newCollectStore()
	sc := NewScanner(store, nil)
	cfg := ActiveConfig{
		Scheme: "http", Host: u.Host, HTTPVersion: "HTTP/1.1",
		Template: []byte("GET /?q=hello HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
	}
	total, _ := PlanCount(cfg.Template)

	started, err := sc.StartActive("t1", cfg, total)
	if err != nil || !started {
		t.Fatalf("first StartActive = (%v, %v)", started, err)
	}
	started2, err := sc.StartActive("t1", cfg, total)
	if err != nil {
		t.Fatalf("second StartActive error: %v", err)
	}
	if started2 {
		t.Error("second StartActive returned true; want false (already running)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sc.Shutdown(ctx)
}
