// Package repeater replays an edited raw HTTP request against a target and
// returns the raw response, for the RedTrace Repeater tool.
package repeater

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// Request is a single Repeater send.
type Request struct {
	Scheme          string // "http" | "https"
	Host            string // host:port to dial
	Raw             []byte // raw HTTP request (request-line + headers + body)
	FollowRedirects bool
	HTTPVersion     string // "HTTP/1.1" | "HTTP/2"
}

// Response is the raw result of a send.
type Response struct {
	Raw        []byte
	StatusCode int
	DurationMs int64
}

// Engine sends Repeater requests. It is safe for concurrent use.
type Engine struct {
	h1 *http.Transport
	h2 *http.Transport
}

// New returns a Repeater engine with HTTP/1.1 and HTTP/2-capable transports.
func New() *Engine {
	mk := func(forceH2 bool) *http.Transport {
		t := &http.Transport{
			// The operator chose the target explicitly; upstream certs are not
			// verified (the same posture as the intercepting proxy).
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: deliberate for a security tool
			ForceAttemptHTTP2:     forceH2,
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		}
		if !forceH2 {
			t.TLSClientConfig.NextProtos = []string{"http/1.1"}
		}
		return t
	}
	return &Engine{h1: mk(false), h2: mk(true)}
}

// Send replays req against its target and returns the raw response.
func (e *Engine) Send(ctx context.Context, req Request) (*Response, error) {
	parsed, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req.Raw)))
	if err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}
	body, _ := io.ReadAll(parsed.Body)
	_ = parsed.Body.Close()

	scheme := req.Scheme
	if scheme == "" {
		scheme = "https"
	}
	parsed.RequestURI = ""
	parsed.URL.Scheme = scheme
	parsed.URL.Host = req.Host // dial target; the Host header keeps parsed.Host
	parsed.Body = io.NopCloser(bytes.NewReader(body))
	parsed.ContentLength = int64(len(body))
	parsed = parsed.WithContext(ctx)

	transport := e.h1
	if strings.EqualFold(req.HTTPVersion, "HTTP/2") {
		transport = e.h2
	}

	client := &http.Client{Transport: transport, Timeout: 60 * time.Second}
	if !req.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	start := time.Now()
	resp, err := client.Do(parsed)
	duration := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return nil, fmt.Errorf("dump response: %w", err)
	}

	return &Response{Raw: raw, StatusCode: resp.StatusCode, DurationMs: duration.Milliseconds()}, nil
}
