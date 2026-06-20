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
	"time"
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

// fakeStore is an in-memory intruder.Store for Runner tests.
type fakeStore struct {
	mu       sync.Mutex
	prepared int
	statuses []string
}

func (s *fakeStore) PrepareIntruderRun(context.Context, string, int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepared++
	return nil
}
func (s *fakeStore) AddIntruderResult(context.Context, string, Result) error   { return nil }
func (s *fakeStore) UpdateIntruderProgress(context.Context, string, int) error { return nil }
func (s *fakeStore) SetIntruderStatus(_ context.Context, _, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses = append(s.statuses, status)
	return nil
}
func (s *fakeStore) lastStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.statuses) == 0 {
		return ""
	}
	return s.statuses[len(s.statuses)-1]
}
func (s *fakeStore) prepareCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepared
}

// blockingRunner returns a Runner whose attacks block on an unreleased server,
// so a run is reliably in-flight; the caller defers the returned cleanup.
func blockingRunner(t *testing.T) (*Runner, *fakeStore, Config, func()) {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	u, _ := url.Parse(srv.URL)
	cfg := Config{
		Scheme:      "http",
		Host:        u.Host,
		Template:    []byte("GET /?id=§1§ HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
		Type:        Sniper,
		PayloadSets: []PayloadSet{{Payloads: []string{"a", "b", "c"}}},
		Concurrency: 1,
	}
	store := &fakeStore{}
	cleanup := func() {
		close(release) // unblock the handler so srv.Close can return
		srv.Close()
	}
	return NewRunner(store, nil), store, cfg, cleanup
}

func TestRunnerRejectsDoubleStart(t *testing.T) {
	r, store, cfg, cleanup := blockingRunner(t)
	defer cleanup()

	started, err := r.Start("attack-1", cfg, 3)
	if err != nil || !started {
		t.Fatalf("first Start = (%v, %v), want (true, nil)", started, err)
	}
	// A second start while the first is in-flight must be rejected without
	// re-preparing — re-preparing would wipe the live run's results.
	started2, err := r.Start("attack-1", cfg, 3)
	if err != nil {
		t.Fatalf("second Start error: %v", err)
	}
	if started2 {
		t.Error("second Start returned true; want false (already running)")
	}

	r.Shutdown(context.Background())
	if n := store.prepareCount(); n != 1 {
		t.Errorf("PrepareIntruderRun called %d times, want 1", n)
	}
}

func TestRunnerShutdownStopsRunningAttack(t *testing.T) {
	r, store, cfg, cleanup := blockingRunner(t)
	defer cleanup()

	started, err := r.Start("attack-1", cfg, 3)
	if err != nil || !started {
		t.Fatalf("Start = (%v, %v), want (true, nil)", started, err)
	}
	// Shutdown must cancel the in-flight run and record a terminal 'stopped'
	// status (not leave it stuck at 'running'), within the deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.Shutdown(ctx)
	if got := store.lastStatus(); got != StatusStopped {
		t.Errorf("status after shutdown = %q, want %q", got, StatusStopped)
	}
}
