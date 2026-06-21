package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestExtractLinks(t *testing.T) {
	base, _ := url.Parse("http://ex.com/a/b")
	body := []byte(`<a href="c">x</a> <a href='/d'>y</a> <a href="http://ex.com/e">z</a>
		<form action="/submit"></form> <a href="mailto:x@y.com">m</a> <a href="#top">t</a>
		<a href="http://other.com/f">o</a>`)
	got := extractLinks(base, body)
	want := map[string]bool{
		"http://ex.com/a/c":    true,
		"http://ex.com/d":      true,
		"http://ex.com/e":      true,
		"http://ex.com/submit": true,
		"http://other.com/f":   true,
	}
	if len(got) != len(want) {
		t.Fatalf("extractLinks = %v (%d), want %d links", got, len(got), len(want))
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected link %q", g)
		}
	}
}

// fakeCrawlStore records fetched pages and signals on terminal status.
type fakeCrawlStore struct {
	mu       sync.Mutex
	pages    map[string]bool
	addCalls int
	status   string
	done     chan struct{}
}

func (s *fakeCrawlStore) PrepareCrawlRun(context.Context, string) error { return nil }
func (s *fakeCrawlStore) AddPage(_ context.Context, _ string, p Page) (bool, error) {
	s.mu.Lock()
	s.pages[p.Path] = true
	s.addCalls++
	s.mu.Unlock()
	return true, nil
}
func (s *fakeCrawlStore) UpdateCrawlProgress(context.Context, string, int, int) error { return nil }
func (s *fakeCrawlStore) SetCrawlStatus(_ context.Context, _, status string) error {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
	close(s.done)
	return nil
}

func TestCrawlDiscoversLinkedPages(t *testing.T) {
	mux := http.NewServeMux()
	html := func(s string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(s))
		}
	}
	mux.HandleFunc("/", html(`<a href="/p1">1</a><a href="/p2">2</a>`))
	mux.HandleFunc("/p1", html(`<a href="/p3">3</a>`))
	mux.HandleFunc("/p2", html(`ok`))
	mux.HandleFunc("/p3", html(`ok`))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := &fakeCrawlStore{pages: map[string]bool{}, done: make(chan struct{})}
	c := NewCrawler(store, nil, func(string, string) bool { return true }, nil)
	started, err := c.Start("t1", Config{Seed: srv.URL + "/", MaxDepth: 2, MaxPages: 50, HTTPVersion: "HTTP/1.1"})
	if err != nil || !started {
		t.Fatalf("Start = (%v, %v)", started, err)
	}
	select {
	case <-store.done:
	case <-time.After(15 * time.Second):
		t.Fatal("crawl did not finish")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	var got []string
	for p := range store.pages {
		got = append(got, p)
	}
	sort.Strings(got)
	for _, want := range []string{"/", "/p1", "/p2", "/p3"} {
		if !store.pages[want] {
			t.Errorf("expected to crawl %q; crawled %v", want, got)
		}
	}
}

func TestCrawlSkipsOutOfScopeSeed(t *testing.T) {
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	store := &fakeCrawlStore{pages: map[string]bool{}, done: make(chan struct{})}
	// Nothing is in scope: the seed must not be fetched.
	c := NewCrawler(store, nil, func(string, string) bool { return false }, nil)
	started, err := c.Start("t1", Config{Seed: srv.URL + "/", MaxDepth: 1, MaxPages: 10, HTTPVersion: "HTTP/1.1"})
	if err != nil || !started {
		t.Fatalf("Start = (%v, %v)", started, err)
	}
	select {
	case <-store.done:
	case <-time.After(10 * time.Second):
		t.Fatal("crawl did not finish")
	}
	mu.Lock()
	gotHits := hits
	mu.Unlock()
	if gotHits != 0 {
		t.Errorf("out-of-scope seed was fetched (%d hits)", gotHits)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.pages) != 0 {
		t.Errorf("stored %d pages for an out-of-scope crawl, want 0", len(store.pages))
	}
	if store.status != StatusError {
		t.Errorf("status = %q, want error (no pages fetched)", store.status)
	}
}

func TestSeedTrailingSlashDedup(t *testing.T) {
	// Root links to "/" (a common self/home link). A bare-host seed (no trailing
	// slash) must dedup against the resolved ".../" so the homepage is not
	// fetched twice.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="/">home</a>`))
	}))
	defer srv.Close()

	store := &fakeCrawlStore{pages: map[string]bool{}, done: make(chan struct{})}
	c := NewCrawler(store, nil, func(string, string) bool { return true }, nil)
	// srv.URL has no trailing slash (Path == "").
	started, err := c.Start("t1", Config{Seed: srv.URL, MaxDepth: 2, MaxPages: 50, HTTPVersion: "HTTP/1.1"})
	if err != nil || !started {
		t.Fatalf("Start = (%v, %v)", started, err)
	}
	select {
	case <-store.done:
	case <-time.After(10 * time.Second):
		t.Fatal("crawl did not finish")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.addCalls != 1 {
		t.Errorf("AddPage called %d times, want 1 (homepage must not be fetched twice)", store.addCalls)
	}
}

func TestCrawlRejectsDoubleStart(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	store := &fakeCrawlStore{pages: map[string]bool{}, done: make(chan struct{})}
	c := NewCrawler(store, nil, func(string, string) bool { return true }, nil)
	cfg := Config{Seed: srv.URL + "/", MaxDepth: 1, MaxPages: 10, HTTPVersion: "HTTP/1.1"}

	started, err := c.Start("t1", cfg)
	if err != nil || !started {
		t.Fatalf("first Start = (%v, %v)", started, err)
	}
	started2, err := c.Start("t1", cfg)
	if err != nil {
		t.Fatalf("second Start error: %v", err)
	}
	if started2 {
		t.Error("second Start returned true; want false (already running)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Shutdown(ctx)
}
