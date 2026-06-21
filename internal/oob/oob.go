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
	"strings"
	"time"
)

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
	Protocol  string // "dns" | "http" | "https"
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

// tokenFor returns the payload token embedded in a queried name or HTTP Host for
// the configured domain, or "" if the name is not under the domain. For
// "<labels…>.<token>.<domain>" it returns the label immediately left of the
// domain (so DNS-exfil prefixes still correlate to the right payload).
func tokenFor(name, domain string) string {
	name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(domain, "."), "."))
	if domain == "" || name == domain {
		return ""
	}
	prefix, ok := strings.CutSuffix(name, "."+domain)
	if !ok || prefix == "" {
		return ""
	}
	labels := strings.Split(prefix, ".")
	return labels[len(labels)-1]
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
