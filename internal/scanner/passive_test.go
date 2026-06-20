package scanner

import (
	"strings"
	"testing"
)

func passiveTypes(issues []Issue) map[string]Issue {
	m := make(map[string]Issue, len(issues))
	for _, is := range issues {
		m[is.Type] = is
	}
	return m
}

func TestPassiveFlagsInsecureResponse(t *testing.T) {
	resp := strings.Join([]string{
		"HTTP/1.1 200 OK",
		"Content-Type: text/html",
		"Server: Apache/2.4.49",
		"Set-Cookie: sid=abc; Path=/",
		"Access-Control-Allow-Origin: *",
		"",
		"<html><body>hello</body></html>",
	}, "\r\n")
	req := "GET /home HTTP/1.1\r\nHost: ex.com\r\n\r\n"

	target := Target{
		Scheme: "https", Host: "ex.com", Port: 443, Method: "GET", Path: "/home",
		RequestRaw: []byte(req), ResponseRaw: []byte(resp),
	}
	got := passiveTypes(Passive(target))

	for _, want := range []string{
		"missing_hsts", "missing_xcto", "missing_csp", "clickjacking",
		"insecure_cookie", "info_disclosure", "cors_wildcard",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected passive finding %q, missing", want)
		}
	}
	// The insecure cookie over HTTPS without Secure is at least medium.
	if is := got["insecure_cookie"]; is.Severity != SeverityMedium {
		t.Errorf("insecure_cookie severity = %q, want medium", is.Severity)
	}
}

func TestPassiveCleanResponseIsQuiet(t *testing.T) {
	resp := strings.Join([]string{
		"HTTP/1.1 200 OK",
		"Content-Type: application/json",
		"Strict-Transport-Security: max-age=31536000",
		"X-Content-Type-Options: nosniff",
		"",
		`{"ok":true}`,
	}, "\r\n")
	target := Target{
		Scheme: "https", Host: "ex.com", Port: 443, Method: "GET", Path: "/api",
		RequestRaw: []byte("GET /api HTTP/1.1\r\nHost: ex.com\r\n\r\n"), ResponseRaw: []byte(resp),
	}
	// A JSON response with HSTS + nosniff and no cookies/banners should not be
	// flagged for the HTML-only or header checks.
	if got := Passive(target); len(got) != 0 {
		t.Errorf("expected no findings on a clean JSON response, got %d: %+v", len(got), got)
	}
}

func TestPassiveCORSReflectionWithCredentials(t *testing.T) {
	resp := strings.Join([]string{
		"HTTP/1.1 200 OK",
		"Content-Type: application/json",
		"Strict-Transport-Security: max-age=1",
		"X-Content-Type-Options: nosniff",
		"Access-Control-Allow-Origin: https://evil.example",
		"Access-Control-Allow-Credentials: true",
		"",
		"{}",
	}, "\r\n")
	req := "GET /api HTTP/1.1\r\nHost: ex.com\r\nOrigin: https://evil.example\r\n\r\n"
	target := Target{
		Scheme: "https", Host: "ex.com", Port: 443, Method: "GET", Path: "/api",
		RequestRaw: []byte(req), ResponseRaw: []byte(resp),
	}
	got := passiveTypes(Passive(target))
	is, ok := got["cors_misconfig"]
	if !ok {
		t.Fatalf("expected cors_misconfig, got types %v", keys(got))
	}
	if is.Severity != SeverityHigh {
		t.Errorf("cors_misconfig severity = %q, want high", is.Severity)
	}
}

func keys(m map[string]Issue) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
