package scanner

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// maxInsertionPoints bounds how many parameters a single active scan probes, so
// a request with an unusually large number of parameters can't blow up the job
// count.
const maxInsertionPoints = 40

// insertion is one place a payload can be injected.
type insertion struct {
	Name string
	Kind string // "query" | "body"
	Orig string // original value (informational)
}

// reqTemplate is a parsed raw request that can be rebuilt with one parameter
// substituted. Headers are preserved verbatim; only the request target (for a
// query insertion) or the body (for a body insertion) is rewritten.
type reqTemplate struct {
	line     string // request line, e.g. "GET /a?b=c HTTP/1.1"
	method   string
	target   string // path[?query]
	proto    string
	path     string
	rawQuery string
	headers  []byte // header lines between the request line and the blank line
	body     []byte
	isForm   bool
}

var contentLengthRe = regexp.MustCompile(`(?im)^content-length:[^\r\n]*`)

// parseRawRequest splits a raw HTTP request into its rewritable parts.
func parseRawRequest(raw []byte) (*reqTemplate, error) {
	lineEnd := bytes.Index(raw, []byte("\r\n"))
	if lineEnd < 0 {
		return nil, fmt.Errorf("malformed request: no request line")
	}
	line := string(raw[:lineEnd])
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, fmt.Errorf("malformed request line %q", line)
	}
	t := &reqTemplate{line: line, method: fields[0], target: fields[1]}
	if len(fields) >= 3 {
		t.proto = fields[2]
	} else {
		t.proto = "HTTP/1.1"
	}
	if i := strings.IndexByte(t.target, '?'); i >= 0 {
		t.path, t.rawQuery = t.target[:i], t.target[i+1:]
	} else {
		t.path = t.target
	}

	rest := raw[lineEnd+2:]
	if sep := bytes.Index(rest, []byte("\r\n\r\n")); sep >= 0 {
		t.headers = rest[:sep]
		t.body = rest[sep+4:]
	} else {
		t.headers = rest
	}
	ct := headerValue(t.headers, "content-type")
	t.isForm = strings.Contains(strings.ToLower(ct), "application/x-www-form-urlencoded")
	return t, nil
}

// headerValue returns the (first) value of a header from a raw header block.
func headerValue(headers []byte, name string) string {
	for _, ln := range strings.Split(string(headers), "\r\n") {
		if i := strings.IndexByte(ln, ':'); i >= 0 && strings.EqualFold(strings.TrimSpace(ln[:i]), name) {
			return strings.TrimSpace(ln[i+1:])
		}
	}
	return ""
}

// parseQueryLenient parses a query/form string into values, falling back to a
// tolerant '&'/';'-splitter when url.ParseQuery rejects the input (e.g. a legacy
// semicolon separator), so such requests still yield insertion points.
func parseQueryLenient(raw string) url.Values {
	if v, err := url.ParseQuery(raw); err == nil {
		return v
	}
	v := url.Values{}
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == '&' || r == ';' }) {
		key, val, _ := strings.Cut(pair, "=")
		if k, err := url.QueryUnescape(key); err == nil {
			key = k
		}
		if u, err := url.QueryUnescape(val); err == nil {
			val = u
		}
		if key != "" {
			v.Add(key, val)
		}
	}
	return v
}

// insertions returns the parameters the active scanner will probe (query
// parameters, plus form-body parameters when the body is form-urlencoded). The
// selection is deterministic (sorted by name) so the same template always probes
// the same parameters, including which survive the maxInsertionPoints cap.
func (t *reqTemplate) insertions() []insertion {
	var out []insertion
	seen := map[string]bool{}
	add := func(name, value, kind string) {
		key := kind + ":" + name
		if seen[key] || len(out) >= maxInsertionPoints {
			return
		}
		seen[key] = true
		out = append(out, insertion{Name: name, Kind: kind, Orig: value})
	}
	addAll := func(v url.Values, kind string) {
		names := make([]string, 0, len(v))
		for name := range v {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			add(name, first(v[name]), kind)
		}
	}
	addAll(parseQueryLenient(t.rawQuery), "query")
	if t.isForm {
		addAll(parseQueryLenient(string(t.body)), "body")
	}
	return out
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// build rebuilds the raw request with ip set to payload. The injected value is
// escaped only enough to keep the request line / body well-formed (spaces and
// query delimiters), NOT via url.Values.Encode, so a payload already in
// percent-encoded wire form (e.g. the ..%2f.. traversal variant) is preserved
// rather than double-encoded into ..%252f..
func (t *reqTemplate) build(ip insertion, payload string) []byte {
	method, proto, headers, body := t.method, t.proto, t.headers, t.body
	target := t.target

	switch ip.Kind {
	case "query":
		if q := encodeWithRawParam(parseQueryLenient(t.rawQuery), ip.Name, payload); q != "" {
			target = t.path + "?" + q
		} else {
			target = t.path
		}
	case "body":
		body = []byte(encodeWithRawParam(parseQueryLenient(string(t.body)), ip.Name, payload))
		headers = setContentLength(headers, len(body))
	}

	var b bytes.Buffer
	b.WriteString(method + " " + target + " " + proto + "\r\n")
	b.Write(headers)
	b.WriteString("\r\n\r\n")
	b.Write(body)
	return b.Bytes()
}

// encodeWithRawParam re-encodes every parameter except name normally, then
// appends name=value with value escaped only for structurally-dangerous bytes
// (so existing percent-encoding in the payload survives).
func encodeWithRawParam(v url.Values, name, value string) string {
	rest := url.Values{}
	for k, vals := range v {
		if k == name {
			continue
		}
		rest[k] = vals
	}
	parts := make([]string, 0, 2)
	if enc := rest.Encode(); enc != "" {
		parts = append(parts, enc)
	}
	parts = append(parts, url.QueryEscape(name)+"="+escapeParamValue(value))
	return strings.Join(parts, "&")
}

// escapeParamValue percent-encodes only the bytes that would break the request
// line or the query/body structure, leaving everything else (including '%', '/',
// '<', quotes) intact so attack payloads reach the target as intended.
func escapeParamValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 0x20 || c >= 0x7f || c == '&' || c == '#' || c == '+' {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// setContentLength rewrites (or appends) a Content-Length header in a raw header
// block to match a new body length.
func setContentLength(headers []byte, n int) []byte {
	repl := []byte("Content-Length: " + strconv.Itoa(n))
	if contentLengthRe.Match(headers) {
		return contentLengthRe.ReplaceAll(headers, repl)
	}
	out := append([]byte{}, headers...)
	return append(out, append([]byte("\r\n"), repl...)...)
}

// probeResp is the parsed view of a probe's response used by active detectors.
type probeResp struct {
	status   int
	location string
	body     string
	bodyLow  string
}

func toProbeResp(rawResp []byte, status int) probeResp {
	m := parseMessage(rawResp)
	body := string(m.body)
	return probeResp{
		status:   status,
		location: m.headers.Get("Location"),
		body:     body,
		bodyLow:  strings.ToLower(body),
	}
}

// activeCheck is one injection probe: payloads to try at an insertion point and
// a detector for evidence of the vulnerability in the response.
type activeCheck struct {
	id          string
	name        string
	severity    Severity
	confidence  Confidence
	remediation string
	// needsBaseline marks checks that suppress findings by diffing against the
	// baseline response; they are skipped when the baseline request failed (an
	// empty baseline would defeat the suppression and cause false positives).
	needsBaseline bool
	payloads      func(token string) []string
	detect        func(payload, token string, resp, base probeResp) (evidence string, ok bool)
}

var (
	etcPasswdRe = regexp.MustCompile(`root:.*?:0:0:`)
	sqlErrors   = []string{
		"you have an error in your sql syntax",
		"warning: mysql",
		"unclosed quotation mark after the character string",
		"quoted string not properly terminated",
		"pg::syntaxerror",
		"psqlexception",
		"sqlite3::",
		"sqlite_error",
		"ora-00933",
		"ora-01756",
		"odbc sql server driver",
		"microsoft ole db provider for sql server",
		"sqlstate",
	}
)

// activeChecks is the registry of active probes.
var activeChecks = []activeCheck{
	{
		id: "reflected_xss", name: "Reflected cross-site scripting", severity: SeverityHigh, confidence: ConfidenceFirm,
		remediation: "Context-aware output-encode all user input and apply a restrictive Content-Security-Policy.",
		payloads: func(token string) []string {
			m := "rtx" + token
			return []string{`"><` + m + `>`, `'><` + m + `>`}
		},
		detect: func(_, token string, resp, _ probeResp) (string, bool) {
			marker := "<rtx" + token + ">"
			if strings.Contains(resp.body, marker) {
				return marker, true
			}
			return "", false
		},
	},
	{
		id: "sql_injection", name: "SQL injection (error-based)", severity: SeverityHigh, confidence: ConfidenceFirm,
		remediation:   "Use parameterised queries / prepared statements; never build SQL from concatenated input.",
		needsBaseline: true,
		payloads:      func(string) []string { return []string{"'", "\"", "')", "'-- -"} },
		detect: func(_, _ string, resp, base probeResp) (string, bool) {
			for _, sig := range sqlErrors {
				if strings.Contains(resp.bodyLow, sig) && !strings.Contains(base.bodyLow, sig) {
					return sig, true
				}
			}
			return "", false
		},
	},
	{
		id: "path_traversal", name: "Path traversal / local file inclusion", severity: SeverityHigh, confidence: ConfidenceFirm,
		remediation:   "Resolve and validate paths against an allowlist; never pass user input to filesystem APIs.",
		needsBaseline: true,
		payloads: func(string) []string {
			return []string{
				"../../../../../../../../etc/passwd",
				"....//....//....//....//....//etc/passwd",
				"..%2f..%2f..%2f..%2f..%2f..%2fetc%2fpasswd",
			}
		},
		detect: func(_, _ string, resp, base probeResp) (string, bool) {
			if m := etcPasswdRe.FindString(resp.body); m != "" && !etcPasswdRe.MatchString(base.body) {
				return m, true
			}
			return "", false
		},
	},
	{
		id: "open_redirect", name: "Open redirect", severity: SeverityMedium, confidence: ConfidenceFirm,
		remediation: "Redirect only to a server-side allowlist of paths/hosts; never to a raw user-supplied URL.",
		payloads: func(string) []string {
			return []string{"https://redtrace.invalid/", "//redtrace.invalid/"}
		},
		detect: func(_, _ string, resp, _ probeResp) (string, bool) {
			if resp.status >= 300 && resp.status < 400 {
				loc := strings.ToLower(resp.location)
				if strings.HasPrefix(loc, "https://redtrace.invalid") || strings.HasPrefix(loc, "//redtrace.invalid") {
					return "Location: " + resp.location, true
				}
			}
			return "", false
		},
	},
	{
		id: "ssti", name: "Server-side template injection", severity: SeverityHigh, confidence: ConfidenceFirm,
		remediation:   "Do not render user input as a template; use sandboxed, logic-less templates with escaping.",
		needsBaseline: true,
		payloads: func(string) []string {
			return []string{"{{1337*1337}}", "${1337*1337}", "#{1337*1337}", "<%= 1337*1337 %>"}
		},
		detect: func(_, _ string, resp, base probeResp) (string, bool) {
			// The product appearing while the literal expression does not means the
			// expression was evaluated server-side.
			if strings.Contains(resp.body, "1787569") && !strings.Contains(resp.body, "1337*1337") &&
				!strings.Contains(base.body, "1787569") {
				return "1337*1337 => 1787569", true
			}
			return "", false
		},
	},
	{
		id: "command_injection", name: "OS command injection", severity: SeverityHigh, confidence: ConfidenceFirm,
		remediation:   "Avoid shelling out with user input; use argument arrays / safe APIs and strict allowlists.",
		needsBaseline: true,
		payloads: func(string) []string {
			a := "$((31337*31337))" // 981990569 when evaluated by a shell
			return []string{";echo " + a, "|echo " + a, "`echo " + a + "`", "$(echo " + a + ")"}
		},
		detect: func(_, _ string, resp, base probeResp) (string, bool) {
			if strings.Contains(resp.body, "981990569") && !strings.Contains(resp.body, "31337*31337") &&
				!strings.Contains(base.body, "981990569") {
				return "31337*31337 => 981990569", true
			}
			return "", false
		},
	},
}
