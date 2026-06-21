// Package oob implements RedTrace's out-of-band interaction server — a
// self-hosted Burp Collaborator analog. When enabled it runs catch-all DNS and
// HTTP listeners under a configured domain; any callback to a generated payload
// hostname (e.g. from a blind SSRF/XXE/RCE) is captured and correlated to the
// payload by the token embedded in the hostname.
package oob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ErrDisabled is returned when an OOB operation is attempted while the feature
// is not configured. Callers distinguish it (a user-facing 400) from a genuine
// storage failure (a 500).
var ErrDisabled = errors.New("oob is not configured")

// Config describes the OOB listeners. The feature is enabled only when a domain
// is configured; the listeners then bind to the configured addresses (which the
// operator points a public host/domain at so targets can reach them).
type Config struct {
	Domain   string // e.g. "oob.example.com"
	PublicIP string // A record the DNS listener answers with (and shown to the user)
	HTTPAddr string // e.g. "0.0.0.0:8888"
	DNSAddr  string // e.g. "0.0.0.0:5353"
}

// Enabled reports whether the OOB server should run.
func (c Config) Enabled() bool { return c.Domain != "" }

// Payload is a generated OOB payload.
type Payload struct {
	Token     string
	Host      string
	CreatedAt time.Time
}

// Interaction is one captured callback.
type Interaction struct {
	ID        string
	Token     string
	Protocol  string // "dns" | "http"
	SourceIP  string
	Query     string
	Detail    string
	Raw       []byte
	CreatedAt time.Time
}

// Store persists payloads and interactions.
type Store interface {
	SavePayload(ctx context.Context, p Payload) error
	SaveInteraction(ctx context.Context, i Interaction) error
}

// normalizeDomain lower-cases the configured OOB domain and trims a leading or
// trailing dot so name matching is consistent.
func normalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(domain, "."), "."))
}

// match reports whether a queried name or HTTP Host falls under the configured
// OOB domain and, if so, the payload token embedded in it. For
// "<labels…>.<token>.<domain>" the token is the label immediately left of the
// domain (so DNS-exfil prefixes still correlate to the right payload). The apex
// is under the domain but carries no token. A name outside the domain returns
// ("", false) — callers use this to ignore unrelated callbacks entirely.
func match(name, domain string) (token string, under bool) {
	name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
	domain = normalizeDomain(domain)
	if domain == "" {
		return "", false
	}
	if name == domain {
		return "", true
	}
	prefix, ok := strings.CutSuffix(name, "."+domain)
	if !ok || prefix == "" {
		return "", false
	}
	labels := strings.Split(prefix, ".")
	return labels[len(labels)-1], true
}

// randHex returns n cryptographically random bytes as hex. A crypto/rand failure
// is unrecoverable (and would otherwise yield a colliding empty primary key), so
// it panics rather than returning an empty string.
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("oob: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
