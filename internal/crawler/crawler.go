// Package crawler implements RedTrace's scope-bounded web spider: starting from
// a seed URL it fetches pages, extracts links, and follows in-scope, same-host
// links breadth-first up to a depth/page budget. Every fetched page is stored as
// an exchange (so it appears in the history and site map) and recorded per crawl.
package crawler

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Config describes a crawl.
type Config struct {
	Seed        string // full seed URL
	MaxDepth    int
	MaxPages    int
	HTTPVersion string
}

// Page is one fetched page handed to the store / notifier.
type Page struct {
	URL         string
	Method      string
	Scheme      string
	Host        string // hostname (no port)
	Port        int
	Path        string
	Query       string
	StatusCode  int
	ContentType string
	Length      int
	Depth       int
	DurationMs  int64
	RequestRaw  []byte
	ResponseRaw []byte
}

// PageSummary is the compact view pushed over the WebSocket.
type PageSummary struct {
	URL         string `json:"url"`
	StatusCode  int    `json:"statusCode"`
	ContentType string `json:"contentType"`
	Depth       int    `json:"depth"`
}

// Update is broadcast as a crawl progresses.
type Update struct {
	Kind   string       `json:"kind"` // "page" | "progress" | "status"
	TaskID string       `json:"taskId,omitempty"`
	Status string       `json:"status,omitempty"`
	Pages  int          `json:"pages"`
	Found  int          `json:"found"`
	Page   *PageSummary `json:"page,omitempty"`
}

// Store is the persistence the Crawler needs.
type Store interface {
	PrepareCrawlRun(ctx context.Context, taskID string) error
	AddPage(ctx context.Context, taskID string, p Page) (bool, error)
	UpdateCrawlProgress(ctx context.Context, taskID string, pages, found int) error
	SetCrawlStatus(ctx context.Context, taskID, status string) error
}

// linkRe matches href/src/action attribute values (double, single, or unquoted).
var linkRe = regexp.MustCompile(`(?i)(?:href|src|action)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s">]+))`)

// extractLinks resolves every link in body against base, returning absolute
// http(s) URLs with the fragment stripped.
func extractLinks(base *url.URL, body []byte) []string {
	var out []string
	for _, m := range linkRe.FindAllSubmatch(body, -1) {
		raw := strings.TrimSpace(string(firstNonEmpty(m[1], m[2], m[3])))
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		switch low := strings.ToLower(raw); {
		case strings.HasPrefix(low, "javascript:"),
			strings.HasPrefix(low, "mailto:"),
			strings.HasPrefix(low, "data:"),
			strings.HasPrefix(low, "tel:"):
			continue
		}
		ref, err := url.Parse(raw)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ref)
		abs.Fragment = ""
		if abs.Scheme != "http" && abs.Scheme != "https" {
			continue
		}
		out = append(out, abs.String())
	}
	return out
}

func firstNonEmpty(bs ...[]byte) []byte {
	for _, b := range bs {
		if len(b) > 0 {
			return b
		}
	}
	return nil
}

// parseResponse extracts the status line headers and body from raw response bytes.
func parseResponse(raw []byte) (headers http.Header, body []byte) {
	r := bufio.NewReader(bytes.NewReader(raw))
	if _, err := r.ReadString('\n'); err != nil {
		return http.Header{}, nil
	}
	mh, _ := textproto.NewReader(r).ReadMIMEHeader()
	body, _ = io.ReadAll(r)
	return http.Header(mh), body
}

// hostPort splits an authority into a hostname and port, defaulting by scheme.
func hostPort(u *url.URL) (string, int) {
	host := u.Hostname()
	if p := u.Port(); p != "" {
		port, _ := strconv.Atoi(p)
		return host, port
	}
	if u.Scheme == "https" {
		return host, 443
	}
	return host, 80
}

func mimeOf(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}
