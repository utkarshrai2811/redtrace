package oob

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		name, domain, wantToken string
		wantUnder               bool
	}{
		{"abc123.oob.example.com", "oob.example.com", "abc123", true},
		{"abc123.oob.example.com.", "oob.example.com", "abc123", true},     // trailing dot
		{"data.abc123.oob.example.com", "oob.example.com", "abc123", true}, // exfil prefix
		{"ABC123.OOB.EXAMPLE.COM", "oob.example.com", "abc123", true},      // case-insensitive
		{"oob.example.com", "oob.example.com", "", true},                   // apex (under, no token)
		{"other.com", "oob.example.com", "", false},                        // not under domain
		{"evil-oob.example.com", "oob.example.com", "", false},             // suffix-but-not-label
		{"", "oob.example.com", "", false},                                 // root / empty name
		{"abc.oob.example.com", ".oob.example.com", "abc", true},           // leading-dot domain
		{"abc.oob.example.com", "", "", false},                             // no domain configured
	}
	for _, c := range cases {
		gotToken, gotUnder := match(c.name, c.domain)
		if gotToken != c.wantToken || gotUnder != c.wantUnder {
			t.Errorf("match(%q, %q) = (%q, %v), want (%q, %v)",
				c.name, c.domain, gotToken, gotUnder, c.wantToken, c.wantUnder)
		}
	}
}

// dnsQuery builds a minimal DNS query packet for name/qtype.
func dnsQuery(name string, qtype uint16) []byte {
	b := []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	for _, l := range strings.Split(name, ".") {
		b = append(b, byte(len(l)))
		b = append(b, []byte(l)...)
	}
	b = append(b, 0)
	b = append(b, byte(qtype>>8), byte(qtype), 0x00, 0x01)
	return b
}

func TestParseDNSQuery(t *testing.T) {
	qtype, qname, qend, ok := parseDNSQuery(dnsQuery("abc.oob.test", dnsTypeA))
	if !ok {
		t.Fatal("parse failed on a valid query")
	}
	if qtype != dnsTypeA || qname != "abc.oob.test" {
		t.Errorf("parsed qtype=%d qname=%q", qtype, qname)
	}
	if qend <= 12 {
		t.Errorf("qend = %d, want > 12", qend)
	}
}

func TestParseDNSMalformed(t *testing.T) {
	// None of these may panic; all must return ok=false.
	inputs := [][]byte{
		nil,
		{0x00},
		make([]byte, 11),
		{0x12, 0x34, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0x40},                     // label length claims 64 bytes, truncated
		append([]byte{0x12, 0x34, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}, 0xc0, 0x0c), // pointer in question
	}
	for i, in := range inputs {
		if _, _, _, ok := parseDNSQuery(in); ok {
			t.Errorf("input %d: parse unexpectedly succeeded", i)
		}
	}
}

func TestBuildDNSResponseHasAnswer(t *testing.T) {
	q := dnsQuery("abc.oob.test", dnsTypeA)
	qtype, _, qend, _ := parseDNSQuery(q)
	resp := buildDNSResponse(q, qend, qtype, net.ParseIP("9.9.9.9"))
	if resp[2]&0x80 == 0 {
		t.Error("QR bit not set on response")
	}
	if an := binary.BigEndian.Uint16(resp[6:8]); an != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", an)
	}
	last4 := resp[len(resp)-4:]
	if last4[0] != 9 || last4[1] != 9 || last4[2] != 9 || last4[3] != 9 {
		t.Errorf("answer RDATA = %v, want 9.9.9.9", last4)
	}
}

func TestBuildDNSResponseNonAHasNoAnswer(t *testing.T) {
	q := dnsQuery("abc.oob.test", dnsTypeTXT)
	qtype, _, qend, _ := parseDNSQuery(q)
	resp := buildDNSResponse(q, qend, qtype, net.ParseIP("9.9.9.9"))
	if an := binary.BigEndian.Uint16(resp[6:8]); an != 0 {
		t.Errorf("ANCOUNT = %d for a TXT query, want 0", an)
	}
}

type fakeStore struct {
	mu       sync.Mutex
	payloads []Payload
	inter    []Interaction
}

func (s *fakeStore) SavePayload(_ context.Context, p Payload) error {
	s.mu.Lock()
	s.payloads = append(s.payloads, p)
	s.mu.Unlock()
	return nil
}
func (s *fakeStore) SaveInteraction(_ context.Context, i Interaction) error {
	s.mu.Lock()
	s.inter = append(s.inter, i)
	s.mu.Unlock()
	return nil
}
func (s *fakeStore) last() Interaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inter[len(s.inter)-1]
}

func newTestServer(store Store) *Server {
	return New(Config{Domain: "oob.test", PublicIP: "9.9.9.9", HTTPAddr: "x", DNSAddr: "x"}, store, nil, nil)
}

func TestHandleHTTPRecords(t *testing.T) {
	store := &fakeStore{}
	s := newTestServer(store)
	req := httptest.NewRequest(http.MethodGet, "http://abc123.oob.test:18888/path?x=1", nil)
	req.Host = "abc123.oob.test:18888" // Host includes the listener port
	req.RemoteAddr = "203.0.113.7:44321"
	rec := httptest.NewRecorder()
	s.handleHTTP(rec, req)

	if len(store.inter) != 1 {
		t.Fatalf("recorded %d interactions, want 1", len(store.inter))
	}
	i := store.last()
	// The port must be stripped so the token correlates and the query is clean.
	if i.Protocol != "http" || i.Token != "abc123" || i.SourceIP != "203.0.113.7" || i.Query != "abc123.oob.test" {
		t.Errorf("interaction = %+v", i)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// capConn is a minimal net.PacketConn that captures the written response.
type capConn struct {
	net.PacketConn
	written []byte
}

func (c *capConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	c.written = append([]byte(nil), p...)
	return len(p), nil
}

type stubAddr string

func (a stubAddr) Network() string { return "udp" }
func (a stubAddr) String() string  { return string(a) }

func TestHandleDNSRecordsAndAnswers(t *testing.T) {
	store := &fakeStore{}
	s := newTestServer(store)
	q := dnsQuery("data.abc123.oob.test", dnsTypeA)
	conn := &capConn{}
	s.handleDNS(conn, stubAddr("198.51.100.9:53"), q, net.ParseIP("9.9.9.9"))

	if len(store.inter) != 1 {
		t.Fatalf("recorded %d interactions, want 1", len(store.inter))
	}
	i := store.last()
	if i.Protocol != "dns" || i.Token != "abc123" || i.SourceIP != "198.51.100.9" {
		t.Errorf("interaction = %+v", i)
	}
	if len(conn.written) == 0 {
		t.Fatal("no DNS response written")
	}
	if an := binary.BigEndian.Uint16(conn.written[6:8]); an != 1 {
		t.Errorf("response ANCOUNT = %d, want 1", an)
	}
}

func TestHandleDNSDropsOffDomain(t *testing.T) {
	store := &fakeStore{}
	s := newTestServer(store)
	conn := &capConn{}
	// A query for an unrelated name (and the DNS root) must not be recorded and
	// must not be answered — so we never act as an open resolver/reflector.
	for _, name := range []string{"victim.example.org", ""} {
		s.handleDNS(conn, stubAddr("198.51.100.9:53"), dnsQuery(name, dnsTypeA), net.ParseIP("9.9.9.9"))
	}
	if len(store.inter) != 0 {
		t.Errorf("recorded %d off-domain interactions, want 0", len(store.inter))
	}
	if len(conn.written) != 0 {
		t.Errorf("wrote %d bytes for an off-domain query, want 0", len(conn.written))
	}
}

func TestHandleHTTPDropsOffDomain(t *testing.T) {
	store := &fakeStore{}
	s := newTestServer(store)
	req := httptest.NewRequest(http.MethodGet, "http://example.org/", nil)
	req.Host = "scanner.example.org" // not under oob.test
	rec := httptest.NewRecorder()
	s.handleHTTP(rec, req)

	if len(store.inter) != 0 {
		t.Errorf("recorded %d off-domain HTTP interactions, want 0", len(store.inter))
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (still a catch-all responder)", rec.Code)
	}
}

func TestHandleHTTPLargeBodyCapturesHeaders(t *testing.T) {
	store := &fakeStore{}
	s := newTestServer(store)
	body := strings.NewReader(strings.Repeat("A", maxOOBBody+5000)) // exceeds the cap
	req := httptest.NewRequest(http.MethodPost, "http://abc123.oob.test/exfil", body)
	req.Host = "abc123.oob.test"
	rec := httptest.NewRecorder()
	s.handleHTTP(rec, req)

	if len(store.inter) != 1 {
		t.Fatalf("recorded %d interactions, want 1", len(store.inter))
	}
	raw := store.last().Raw
	if len(raw) == 0 {
		t.Fatal("raw capture is empty for an oversized body; headers should survive")
	}
	if !strings.HasPrefix(string(raw), "POST ") || !strings.Contains(string(raw), "/exfil") {
		t.Errorf("raw capture missing the request line: %q", raw[:min(64, len(raw))])
	}
}

func TestNewPayloadDisabledWhenNoDomain(t *testing.T) {
	s := New(Config{}, &fakeStore{}, nil, nil)
	_, err := s.NewPayload(context.Background())
	if !errors.Is(err, ErrDisabled) {
		t.Errorf("NewPayload with OOB disabled = %v, want ErrDisabled", err)
	}
	// Run is a no-op when disabled.
	if err := s.Run(context.Background()); err != nil {
		t.Errorf("Run() with OOB disabled = %v, want nil", err)
	}
}
