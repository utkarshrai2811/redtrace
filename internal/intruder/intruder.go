// Package intruder implements RedTrace's automated request-fuzzing tool. It
// substitutes payloads into marked positions of a request template across four
// attack types (Sniper, Battering ram, Pitchfork, Cluster bomb) and replays the
// generated requests against a target.
package intruder

import (
	"bytes"
	"crypto/md5" //nolint:gosec // G501: md5 is offered only as a payload-processing option, not for security
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Marker delimits a payload position in a request template (Burp's "§").
const Marker = "§"

// maxJobs caps how many requests a single attack may generate. It guards the
// user's own machine against an accidental runaway attack (e.g. a Cluster bomb
// of several large payload sets filling the disk) and keeps Count from silently
// overflowing — product saturates here rather than wrapping to a bogus total.
const maxJobs = 50_000_000

// AttackType selects how payloads are distributed across positions.
type AttackType string

const (
	// Sniper places each payload in one position at a time (others stay at their
	// base value), using a single payload set. Jobs = positions × payloads.
	Sniper AttackType = "sniper"
	// BatteringRam places the same payload in every position at once, using a
	// single payload set. Jobs = payloads.
	BatteringRam AttackType = "battering_ram"
	// Pitchfork iterates one payload set per position in lockstep. Jobs = the
	// shortest set's length.
	Pitchfork AttackType = "pitchfork"
	// ClusterBomb tries every combination of one payload set per position. Jobs =
	// the product of the set lengths.
	ClusterBomb AttackType = "cluster_bomb"
)

// Processor is one transformation applied to a payload before substitution.
type Processor struct {
	Kind  string `json:"kind"`            // prefix|suffix|base64|base64url|url|upper|lower|sha256|md5
	Value string `json:"value,omitempty"` // operand for prefix/suffix
}

// PayloadSet is a list of payloads with an optional processing chain.
type PayloadSet struct {
	Payloads   []string    `json:"payloads"`
	Processors []Processor `json:"processors,omitempty"`
}

// Config fully describes an attack.
type Config struct {
	Scheme          string       `json:"scheme"`
	Host            string       `json:"host"`
	Template        []byte       `json:"-"` // raw request with §marked§ positions
	Type            AttackType   `json:"type"`
	PayloadSets     []PayloadSet `json:"payloadSets"`
	FollowRedirects bool         `json:"followRedirects"`
	HTTPVersion     string       `json:"httpVersion"`
	Concurrency     int          `json:"concurrency"`
}

// Result is the outcome of one generated request.
type Result struct {
	// ID is assigned when the result is produced and is reused as the persisted
	// row's primary key, so a result streamed over the WebSocket carries the same
	// id the detail endpoint serves — the UI can open it while the attack runs.
	ID          string   `json:"id"`
	Index       int      `json:"index"`
	Payloads    []string `json:"payloads"`
	StatusCode  int      `json:"statusCode"`
	Length      int      `json:"length"`
	DurationMs  int64    `json:"durationMs"`
	RequestRaw  []byte   `json:"-"`
	ResponseRaw []byte   `json:"-"`
	Error       string   `json:"error,omitempty"`
}

// template is a request split into literal chunks and the base values at each
// marked position, such that build(payloads) reconstructs the request.
type template struct {
	literals []string // len == len(base)+1
	base     []string // base value of each position
}

// parseTemplate splits raw on the position marker. Positions are the odd-index
// chunks; literals are the even-index chunks. An unbalanced trailing marker is
// tolerated by padding the literals.
func parseTemplate(raw []byte) template {
	parts := strings.Split(string(raw), Marker)
	var t template
	for i, p := range parts {
		if i%2 == 0 {
			t.literals = append(t.literals, p)
		} else {
			t.base = append(t.base, p)
		}
	}
	if len(t.literals) == len(t.base) {
		t.literals = append(t.literals, "")
	}
	return t
}

// Positions returns the number of payload positions in a template.
func Positions(raw []byte) int {
	return len(parseTemplate(raw).base)
}

// build reconstructs the request with payloads substituted at each position and
// recomputes Content-Length when that header is present.
func (t template) build(payloads []string) []byte {
	var b strings.Builder
	for i := 0; i < len(t.literals); i++ {
		b.WriteString(t.literals[i])
		if i < len(payloads) {
			b.WriteString(payloads[i])
		}
	}
	return fixContentLength([]byte(b.String()))
}

var contentLengthRe = regexp.MustCompile(`(?im)^content-length:[^\r\n]*`)

// fixContentLength rewrites an existing Content-Length header to match the body
// size, so payload substitution that changes the body length yields a valid
// request. It does not add the header when absent (a bodyless request must not
// gain one).
func fixContentLength(raw []byte) []byte {
	i := bytes.Index(raw, []byte("\r\n\r\n"))
	if i < 0 {
		return raw
	}
	headers, body := raw[:i], raw[i+4:]
	if !contentLengthRe.Match(headers) {
		return raw
	}
	headers = contentLengthRe.ReplaceAll(headers, []byte("Content-Length: "+strconv.Itoa(len(body))))
	out := make([]byte, 0, len(headers)+4+len(body))
	out = append(out, headers...)
	out = append(out, "\r\n\r\n"...)
	out = append(out, body...)
	return out
}

// applyProcessors runs payload through a chain of processors left to right.
func applyProcessors(payload string, procs []Processor) string {
	for _, p := range procs {
		switch p.Kind {
		case "prefix":
			payload = p.Value + payload
		case "suffix":
			payload += p.Value
		case "base64":
			payload = base64.StdEncoding.EncodeToString([]byte(payload))
		case "base64url":
			payload = base64.RawURLEncoding.EncodeToString([]byte(payload))
		case "url":
			payload = url.QueryEscape(payload)
		case "upper":
			payload = strings.ToUpper(payload)
		case "lower":
			payload = strings.ToLower(payload)
		case "sha256":
			sum := sha256.Sum256([]byte(payload))
			payload = hex.EncodeToString(sum[:])
		case "md5":
			sum := md5.Sum([]byte(payload)) //nolint:gosec // G401: offered as a payload option, not for security
			payload = hex.EncodeToString(sum[:])
		}
	}
	return payload
}

// processedSets applies each set's processors to its payloads up front.
func processedSets(cfg Config) [][]string {
	out := make([][]string, len(cfg.PayloadSets))
	for i, ps := range cfg.PayloadSets {
		out[i] = make([]string, 0, len(ps.Payloads))
		for _, p := range ps.Payloads {
			out[i] = append(out[i], applyProcessors(p, ps.Processors))
		}
	}
	return out
}

// Count returns how many requests an attack will generate, without building any.
func Count(cfg Config) int {
	n := Positions(cfg.Template)
	if n == 0 {
		return 0
	}
	sets := processedSets(cfg)
	switch cfg.Type {
	case Sniper:
		if len(sets) == 0 {
			return 0
		}
		return n * len(sets[0])
	case BatteringRam:
		if len(sets) == 0 {
			return 0
		}
		return len(sets[0])
	case Pitchfork:
		return lockstepLen(sets, n)
	case ClusterBomb:
		return product(sets, n)
	default:
		return 0
	}
}

// forEachJob invokes fn with the payload vector (one entry per position) for
// every generated request, in deterministic order. fn receives a fresh slice it
// may retain and returns false to stop early. It returns the number of jobs
// emitted (which equals Count unless fn stopped it).
func forEachJob(cfg Config, fn func(payloads []string) bool) int {
	t := parseTemplate(cfg.Template)
	n := len(t.base)
	if n == 0 {
		return 0
	}
	sets := processedSets(cfg)
	count := 0
	emit := func(vec []string) bool {
		count++
		return fn(vec)
	}

	switch cfg.Type {
	case Sniper:
		if len(sets) == 0 {
			return 0
		}
		for pos := 0; pos < n; pos++ {
			for _, pay := range sets[0] {
				vec := append([]string(nil), t.base...)
				vec[pos] = pay
				if !emit(vec) {
					return count
				}
			}
		}
	case BatteringRam:
		if len(sets) == 0 {
			return 0
		}
		for _, pay := range sets[0] {
			vec := make([]string, n)
			for i := range vec {
				vec[i] = pay
			}
			if !emit(vec) {
				return count
			}
		}
	case Pitchfork:
		s := min(n, len(sets))
		length := lockstepLen(sets, n)
		for k := 0; k < length; k++ {
			vec := append([]string(nil), t.base...)
			for i := 0; i < s; i++ {
				vec[i] = sets[i][k]
			}
			if !emit(vec) {
				return count
			}
		}
	case ClusterBomb:
		s := min(n, len(sets))
		if product(sets, n) == 0 {
			return 0
		}
		idx := make([]int, s)
		for {
			vec := append([]string(nil), t.base...)
			for i := 0; i < s; i++ {
				vec[i] = sets[i][idx[i]]
			}
			if !emit(vec) {
				return count
			}
			// Odometer increment over the set indices.
			pos := s - 1
			for pos >= 0 {
				idx[pos]++
				if idx[pos] < len(sets[pos]) {
					break
				}
				idx[pos] = 0
				pos--
			}
			if pos < 0 {
				break
			}
		}
	}
	return count
}

// lockstepLen is the shortest payload-set length across the positions that have
// a set (used by Pitchfork). Zero if any contributing set is empty.
func lockstepLen(sets [][]string, positions int) int {
	s := min(positions, len(sets))
	if s == 0 {
		return 0
	}
	length := len(sets[0])
	for i := 1; i < s; i++ {
		if len(sets[i]) < length {
			length = len(sets[i])
		}
	}
	return length
}

// product is the number of combinations across the positions that have a set
// (used by Cluster bomb). Zero if any contributing set is empty. It saturates at
// maxJobs+1 instead of overflowing, so a combination space beyond what Validate
// allows is reported as "too large" rather than wrapping to a bogus total.
func product(sets [][]string, positions int) int {
	s := min(positions, len(sets))
	if s == 0 {
		return 0
	}
	total := 1
	for i := 0; i < s; i++ {
		if len(sets[i]) == 0 {
			return 0
		}
		if total > maxJobs/len(sets[i]) {
			return maxJobs + 1
		}
		total *= len(sets[i])
	}
	return total
}

// newID returns a random RFC 4122 v4 identifier for a generated result.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Validate reports a problem with cfg before an attack is started.
func Validate(cfg Config) error {
	switch cfg.Type {
	case Sniper, BatteringRam, Pitchfork, ClusterBomb:
	default:
		return fmt.Errorf("unknown attack type %q", cfg.Type)
	}
	if Positions(cfg.Template) == 0 {
		return fmt.Errorf("no payload positions: mark at least one with %s…%s", Marker, Marker)
	}
	if len(cfg.PayloadSets) == 0 {
		return fmt.Errorf("no payload sets")
	}
	nonEmpty := false
	for _, ps := range cfg.PayloadSets {
		if len(ps.Payloads) > 0 {
			nonEmpty = true
			break
		}
	}
	if !nonEmpty {
		return fmt.Errorf("payload sets are empty")
	}
	n := Count(cfg)
	if n <= 0 {
		return fmt.Errorf("attack generates no requests for type %q", cfg.Type)
	}
	if n > maxJobs {
		return fmt.Errorf("attack would generate %d requests; the maximum is %d — narrow the payloads or positions", n, maxJobs)
	}
	return nil
}
