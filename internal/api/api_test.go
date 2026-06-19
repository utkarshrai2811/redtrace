package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// Local mirrors of the (unexported) response shapes for assertions.
type apiList struct {
	Data  []storage.RequestSummary `json:"data"`
	Total int                      `json:"total"`
}

type apiDetail struct {
	RequestRaw []byte `json:"requestRaw"`
}

func newTestServer(t *testing.T) (*httptest.Server, *storage.DB) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	authority, err := cert.LoadOrCreateAuthority(t.TempDir())
	if err != nil {
		t.Fatalf("authority: %v", err)
	}

	srv, err := New(Config{Version: "test"}, db, scope.New(), intercept.NewRuleSet(),
		intercept.NewInterceptor(storage.NewID), authority, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts, db
}

func seed(t *testing.T, db *storage.DB) string {
	t.Helper()
	id := storage.NewID()
	ex := &models.Exchange{
		Request: &models.Request{
			ID: id, Timestamp: time.Now(), Source: models.SourceProxy, Method: "GET",
			Scheme: "https", Host: "example.com", Port: 443, Path: "/", URL: "https://example.com/",
			HTTPVersion: "HTTP/1.1", InScope: true, Raw: []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		},
		Response: &models.Response{
			ID: storage.NewID(), RequestID: id, Timestamp: time.Now(), StatusCode: 200,
			HTTPVersion: "HTTP/1.1", MimeType: "text/html", BodySize: 5, DurationMs: 7,
			Raw: []byte("HTTP/1.1 200 OK\r\n\r\nhello"),
		},
	}
	if err := db.StoreExchange(context.Background(), ex); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return id
}

func TestAPI_HealthAndRequests(t *testing.T) {
	ts, db := newTestServer(t)
	id := seed(t, db)

	// Health
	var health map[string]string
	getJSON(t, ts.URL+"/api/health", &health)
	if health["status"] != "ok" {
		t.Errorf("health = %v", health)
	}

	// List
	var list apiList
	getJSON(t, ts.URL+"/api/requests", &list)
	if list.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("list total=%d len=%d", list.Total, len(list.Data))
	}
	if list.Data[0].Method != "GET" || list.Data[0].StatusCode != 200 {
		t.Errorf("unexpected row: %+v", list.Data[0])
	}

	// Detail (raw decoded from base64 by the JSON decoder)
	var detail apiDetail
	getJSON(t, ts.URL+"/api/requests/"+id, &detail)
	if !strings.Contains(string(detail.RequestRaw), "Host: example.com") {
		t.Errorf("requestRaw missing: %q", detail.RequestRaw)
	}

	// 404 for missing
	if code := statusOf(t, http.MethodGet, ts.URL+"/api/requests/does-not-exist", ""); code != http.StatusNotFound {
		t.Errorf("missing detail status = %d", code)
	}

	// Delete then confirm gone
	if code := statusOf(t, http.MethodDelete, ts.URL+"/api/requests/"+id, ""); code != http.StatusNoContent {
		t.Errorf("delete status = %d", code)
	}
	getJSON(t, ts.URL+"/api/requests", &list)
	if list.Total != 0 {
		t.Errorf("expected empty after delete, total=%d", list.Total)
	}
}

func TestAPI_ScopeAndRules(t *testing.T) {
	ts, _ := newTestServer(t)

	body := `[{"id":"1","enabled":true,"kind":"include","matcher":"host","value":"example.com"}]`
	if code := statusOf(t, http.MethodPut, ts.URL+"/api/scope", body); code != http.StatusOK {
		t.Fatalf("put scope status = %d", code)
	}
	var rules []scope.Rule
	getJSON(t, ts.URL+"/api/scope", &rules)
	if len(rules) != 1 || rules[0].Value != "example.com" {
		t.Errorf("scope rules = %+v", rules)
	}

	// Invalid regex should be rejected.
	bad := `[{"id":"1","enabled":true,"kind":"include","matcher":"host_regex","value":"("}]`
	if code := statusOf(t, http.MethodPut, ts.URL+"/api/scope", bad); code != http.StatusBadRequest {
		t.Errorf("bad scope status = %d, want 400", code)
	}

	mr := `[{"id":"1","enabled":true,"part":"request_header","matchType":"literal","match":"a","replace":"b"}]`
	if code := statusOf(t, http.MethodPut, ts.URL+"/api/intercept/rules", mr); code != http.StatusOK {
		t.Errorf("put rules status = %d", code)
	}
}

func TestAPI_InterceptToggle(t *testing.T) {
	ts, _ := newTestServer(t)

	if code := statusOf(t, http.MethodPut, ts.URL+"/api/intercept", `{"enabled":true}`); code != http.StatusOK {
		t.Fatalf("toggle status = %d", code)
	}
	var state map[string]any
	getJSON(t, ts.URL+"/api/intercept", &state)
	if state["enabled"] != true {
		t.Errorf("intercept not enabled: %+v", state)
	}
}

// --- helpers ---

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func statusOf(t *testing.T, method, url, body string) int {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}
