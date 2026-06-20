// Package scanner implements RedTrace's vulnerability scanner: a passive
// analyzer that flags issues in captured traffic, and an active scanner that
// probes a request's insertion points for injection-class vulnerabilities.
package scanner

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/textproto"
	"strings"
)

// Severity ranks a finding's impact.
type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// Confidence is how sure the scanner is that a finding is real.
type Confidence string

const (
	ConfidenceTentative Confidence = "tentative"
	ConfidenceFirm      Confidence = "firm"
	ConfidenceCertain   Confidence = "certain"
)

// Origin distinguishes passively-observed findings from active-probe findings.
const (
	OriginPassive = "passive"
	OriginActive  = "active"
)

// Issue is one finding. Fingerprint is its stable identity for deduplication so
// the same issue is recorded once rather than on every matching request. ID is
// assigned when the issue is produced and reused as the persisted row's id, so a
// finding streamed over the WebSocket can be opened by id.
type Issue struct {
	ID          string
	Type        string
	Name        string
	Severity    Severity
	Confidence  Confidence
	Scheme      string
	Host        string
	Port        int
	Path        string
	Method      string
	Param       string
	Payload     string
	Detail      string
	Evidence    string
	Remediation string
	Origin      string
	Fingerprint string
	RequestRaw  []byte
	ResponseRaw []byte
}

// Target is a captured exchange handed to the passive scanner.
type Target struct {
	Scheme      string
	Host        string
	Port        int
	Method      string
	Path        string
	Query       string
	RequestRaw  []byte
	ResponseRaw []byte
}

// message is a parsed HTTP message: its headers and body, regardless of whether
// it is a request or a response (the first line is skipped).
type message struct {
	headers http.Header
	body    []byte
}

// parseMessage splits raw HTTP bytes into headers and body. It is tolerant of
// malformed input, returning whatever it could parse.
func parseMessage(raw []byte) message {
	r := bufio.NewReader(bytes.NewReader(raw))
	if _, err := r.ReadString('\n'); err != nil {
		return message{headers: http.Header{}}
	}
	mh, _ := textproto.NewReader(r).ReadMIMEHeader()
	body, _ := io.ReadAll(r)
	return message{headers: http.Header(mh), body: body}
}

// fingerprint builds a stable dedup key from the parts that make a finding
// distinct.
func fingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// truncate bounds an evidence string so a finding never stores a huge blob.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
