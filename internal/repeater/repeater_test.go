package repeater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func rawGet(host, path string) []byte {
	return []byte("GET " + path + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n")
}

func TestEngine_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			_, _ = w.Write([]byte("pong"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	resp, err := New().Send(context.Background(), Request{
		Scheme: "http", Host: u.Host, Raw: rawGet(u.Host, "/ping"), HTTPVersion: "HTTP/1.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Raw), "pong") {
		t.Errorf("raw response missing body: %q", resp.Raw)
	}
	if !strings.HasPrefix(string(resp.Raw), "HTTP/") {
		t.Errorf("raw response should start with a status line: %q", resp.Raw)
	}
}

func TestEngine_FollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			_, _ = w.Write([]byte("landed"))
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	e := New()

	noFollow, err := e.Send(context.Background(), Request{
		Scheme: "http", Host: u.Host, Raw: rawGet(u.Host, "/a"), FollowRedirects: false, HTTPVersion: "HTTP/1.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if noFollow.StatusCode != http.StatusFound {
		t.Errorf("no-follow status = %d, want 302", noFollow.StatusCode)
	}

	follow, err := e.Send(context.Background(), Request{
		Scheme: "http", Host: u.Host, Raw: rawGet(u.Host, "/a"), FollowRedirects: true, HTTPVersion: "HTTP/1.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if follow.StatusCode != 200 || !strings.Contains(string(follow.Raw), "landed") {
		t.Errorf("follow result: status=%d raw=%q", follow.StatusCode, follow.Raw)
	}
}
