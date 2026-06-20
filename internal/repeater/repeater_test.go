package repeater

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
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

func TestEngine_PreservesContentEncoding(t *testing.T) {
	// A raw request without Accept-Encoding must not cause the transport to inject
	// gzip and silently decompress: the response is shown exactly as sent.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte("compressed-payload"))
	_ = zw.Close()
	gz := buf.Bytes()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(gz)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	resp, err := New().Send(context.Background(), Request{
		Scheme: "http", Host: u.Host, Raw: rawGet(u.Host, "/"), HTTPVersion: "HTTP/1.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	raw := string(resp.Raw)
	if !strings.Contains(strings.ToLower(raw), "content-encoding: gzip") {
		t.Errorf("Content-Encoding stripped from raw response: %q", raw)
	}
	if !bytes.Contains(resp.Raw, gz) {
		t.Error("raw response body was decompressed instead of forwarded verbatim")
	}
}

func TestEngine_FollowsRedirectWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/dest", http.StatusTemporaryRedirect) // 307
		case "/dest":
			b, _ := io.ReadAll(r.Body)
			_, _ = w.Write([]byte("dest:" + r.Method + ":" + string(b)))
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	raw := []byte("POST /start HTTP/1.1\r\nHost: " + u.Host + "\r\nContent-Type: text/plain\r\nContent-Length: 5\r\n\r\nhello")
	resp, err := New().Send(context.Background(), Request{
		Scheme: "http", Host: u.Host, Raw: raw, FollowRedirects: true, HTTPVersion: "HTTP/1.1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (307 with body should be followed)", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Raw), "dest:POST:hello") {
		t.Errorf("redirect did not replay method+body: %q", resp.Raw)
	}
}
