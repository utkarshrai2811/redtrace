package intruder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

func TestEngineRun_Sniper(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		_, _ = w.Write([]byte("id=" + r.URL.Query().Get("id")))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	cfg := Config{
		Scheme:      "http",
		Host:        u.Host,
		Template:    []byte("GET /?id=§1§ HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
		Type:        Sniper,
		PayloadSets: []PayloadSet{{Payloads: []string{"a", "b", "c"}}},
		Concurrency: 4,
	}

	var mu sync.Mutex
	var got []Result
	n, err := NewEngine().Run(context.Background(), cfg, func(r Result) {
		mu.Lock()
		got = append(got, r)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 3 || len(got) != 3 {
		t.Fatalf("dispatched=%d results=%d, want 3", n, len(got))
	}
	if atomic.LoadInt64(&hits) != 3 {
		t.Errorf("server saw %d requests, want 3", hits)
	}
	// Every request should have succeeded with a 200 and a distinct index.
	sort.Slice(got, func(i, j int) bool { return got[i].Index < got[j].Index })
	for i, r := range got {
		if r.Index != i {
			t.Errorf("result %d has index %d", i, r.Index)
		}
		if r.StatusCode != 200 {
			t.Errorf("result %d status = %d, want 200 (err=%q)", i, r.StatusCode, r.Error)
		}
		if r.Length == 0 {
			t.Errorf("result %d has zero length", i)
		}
	}
}

func TestEngineRun_Cancel(t *testing.T) {
	// The server blocks until the test releases it, so the attack is in-flight
	// when we cancel.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)
	u, _ := url.Parse(srv.URL)

	cfg := Config{
		Scheme:      "http",
		Host:        u.Host,
		Template:    []byte("GET /?id=§1§ HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
		Type:        Sniper,
		PayloadSets: []PayloadSet{{Payloads: []string{"a", "b", "c", "d", "e", "f", "g", "h"}}},
		Concurrency: 2,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before running: Run must return promptly, not hang

	_, err := NewEngine().Run(ctx, cfg, func(Result) {})
	if err == nil {
		t.Fatal("expected a cancellation error")
	}
}
