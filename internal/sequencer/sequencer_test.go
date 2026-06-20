package sequencer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnalyzeIdenticalTokens(t *testing.T) {
	tokens := make([]string, 50)
	for i := range tokens {
		tokens[i] = "AAAAAAAA"
	}
	r := Analyze(tokens)
	if r.EffectiveBits != 0 {
		t.Errorf("identical tokens: effective bits = %v, want 0", r.EffectiveBits)
	}
	if r.UniqueCount != 1 {
		t.Errorf("unique = %d, want 1", r.UniqueCount)
	}
	if r.Quality != "poor" {
		t.Errorf("quality = %q, want poor", r.Quality)
	}
}

func TestAnalyzeConstantPrefix(t *testing.T) {
	// A constant "sess-" prefix followed by a varying hex byte.
	tokens := make([]string, 0, 256)
	for i := 0; i < 256; i++ {
		tokens = append(tokens, fmt.Sprintf("sess-%02x", i))
	}
	r := Analyze(tokens)
	for pos := 0; pos < 5; pos++ {
		if r.PositionEntropy[pos] != 0 {
			t.Errorf("prefix position %d entropy = %v, want 0", pos, r.PositionEntropy[pos])
		}
	}
	if r.PositionEntropy[5] <= 0 {
		t.Errorf("varying position entropy = %v, want > 0", r.PositionEntropy[5])
	}
	if r.EffectiveBits <= 0 {
		t.Errorf("effective bits = %v, want > 0", r.EffectiveBits)
	}
}

// fakeSeqStore signals when a capture reaches a terminal status.
type fakeSeqStore struct {
	mu        sync.Mutex
	collected int
	status    string
	done      chan struct{}
}

func (s *fakeSeqStore) PrepareSequencerRun(context.Context, string) error { return nil }
func (s *fakeSeqStore) UpdateSequencerProgress(_ context.Context, _ string, collected int, _ []byte) error {
	s.mu.Lock()
	s.collected = collected
	s.mu.Unlock()
	return nil
}
func (s *fakeSeqStore) SetSequencerResult(_ context.Context, _, status string, _ []byte) error {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
	close(s.done)
	return nil
}

func TestSequencerCapture(t *testing.T) {
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := atomic.AddInt64(&n, 1)
		w.Header().Set("Set-Cookie", fmt.Sprintf("sid=%016x; Path=/; HttpOnly", i))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	store := &fakeSeqStore{done: make(chan struct{})}
	sq := NewSequencer(store, nil)
	cfg := Config{
		Scheme: "http", Host: u.Host, HTTPVersion: "HTTP/1.1", Source: "cookie", Selector: "sid", Target: 30,
		Template: []byte("GET / HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"),
	}
	started, err := sq.Start("t1", cfg)
	if err != nil || !started {
		t.Fatalf("Start = (%v, %v)", started, err)
	}
	select {
	case <-store.done:
	case <-time.After(15 * time.Second):
		t.Fatal("capture did not finish")
	}
	store.mu.Lock()
	collected, status := store.collected, store.status
	store.mu.Unlock()
	if status != StatusCompleted {
		t.Errorf("status = %q, want completed", status)
	}
	if collected < 30 {
		t.Errorf("collected = %d, want >= 30", collected)
	}
}
