package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sampleExchange(id, method, host, path, body string, status int) *models.Exchange {
	now := time.Now()
	return &models.Exchange{
		Request: &models.Request{
			ID:          id,
			Timestamp:   now,
			Source:      models.SourceProxy,
			Method:      method,
			Scheme:      "https",
			Host:        host,
			Port:        443,
			Path:        path,
			URL:         "https://" + host + path,
			HTTPVersion: "HTTP/1.1",
			InScope:     true,
			Raw:         []byte(method + " " + path + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n"),
		},
		Response: &models.Response{
			ID:          id + "-resp",
			RequestID:   id,
			Timestamp:   now,
			StatusCode:  status,
			HTTPVersion: "HTTP/1.1",
			MimeType:    "text/html",
			BodySize:    len(body),
			DurationMs:  12,
			Raw:         []byte("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n" + body),
		},
	}
}

func TestStoreAndGetExchange(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ex := sampleExchange(NewID(), "GET", "example.com", "/login", "<h1>hello secret-token</h1>", 200)
	if err := db.StoreExchange(ctx, ex); err != nil {
		t.Fatalf("StoreExchange: %v", err)
	}

	got, err := db.GetExchange(ctx, ex.Request.ID)
	if err != nil {
		t.Fatalf("GetExchange: %v", err)
	}
	if got.Request.Host != "example.com" || got.Request.Method != "GET" {
		t.Errorf("unexpected request: %+v", got.Request)
	}
	if got.Response == nil || got.Response.StatusCode != 200 {
		t.Errorf("unexpected response: %+v", got.Response)
	}
	if string(got.Request.Raw) != string(ex.Request.Raw) {
		t.Error("raw request not round-tripped")
	}
}

func TestListRequests_Filters(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	exchanges := []*models.Exchange{
		sampleExchange(NewID(), "GET", "a.example.com", "/", "alpha body", 200),
		sampleExchange(NewID(), "POST", "a.example.com", "/login", "secret-token here", 302),
		sampleExchange(NewID(), "GET", "b.example.com", "/admin", "forbidden", 403),
	}
	for _, ex := range exchanges {
		if err := db.StoreExchange(ctx, ex); err != nil {
			t.Fatalf("store: %v", err)
		}
	}

	tests := []struct {
		name   string
		filter RequestFilter
		want   int
	}{
		{"all", RequestFilter{}, 3},
		{"method post", RequestFilter{Method: "POST"}, 1},
		{"host filter", RequestFilter{Host: "a.example.com"}, 2},
		{"status class 4xx", RequestFilter{StatusMin: 400, StatusMax: 499}, 1},
		{"status class 3xx", RequestFilter{StatusMin: 300, StatusMax: 399}, 1},
		{"contains fts", RequestFilter{Contains: "secret-token"}, 1},
		{"contains miss", RequestFilter{Contains: "nonexistent-string"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := db.ListRequests(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListRequests: %v", err)
			}
			if total != tt.want {
				t.Errorf("total = %d, want %d (rows=%d)", total, tt.want, len(rows))
			}
		})
	}
}

func TestDeleteRequest(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	ex := sampleExchange(NewID(), "GET", "example.com", "/", "body", 200)
	if err := db.StoreExchange(ctx, ex); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := db.DeleteRequest(ctx, ex.Request.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetExchange(ctx, ex.Request.ID); err == nil {
		t.Error("expected ErrNotFound after delete")
	}
	if err := db.DeleteRequest(ctx, ex.Request.ID); err != ErrNotFound {
		t.Errorf("second delete: got %v, want ErrNotFound", err)
	}
}

func TestClearRequests(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for range 3 {
		if err := db.StoreExchange(ctx, sampleExchange(NewID(), "GET", "example.com", "/", "needle", 200)); err != nil {
			t.Fatalf("store: %v", err)
		}
	}
	if _, total, _ := db.ListRequests(ctx, RequestFilter{}); total != 3 {
		t.Fatalf("setup total=%d, want 3", total)
	}

	if err := db.ClearRequests(ctx); err != nil {
		t.Fatalf("ClearRequests: %v", err)
	}
	if _, total, err := db.ListRequests(ctx, RequestFilter{}); err != nil || total != 0 {
		t.Errorf("after clear: total=%d err=%v, want 0", total, err)
	}
	// The FTS index must be cleared too.
	if _, n, _ := db.ListRequests(ctx, RequestFilter{Contains: "needle"}); n != 0 {
		t.Errorf("search index not cleared: %d hits remain", n)
	}
}

func TestNewID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := NewID()
		if len(id) != 36 {
			t.Fatalf("bad id length: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = true
	}
}
