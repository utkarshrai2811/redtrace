// Package intercept implements RedTrace's automatic match & replace rules and
// the manual request/response interception queue.
package intercept

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Part identifies which portion of a request or response a rule operates on.
type Part string

const (
	PartRequestMethod  Part = "request_method"
	PartRequestURL     Part = "request_url"
	PartRequestHeader  Part = "request_header"
	PartRequestBody    Part = "request_body"
	PartResponseHeader Part = "response_header"
	PartResponseBody   Part = "response_body"
)

// MatchType selects literal substring or regular-expression matching.
type MatchType string

const (
	Literal MatchType = "literal"
	Regex   MatchType = "regex"
)

// Rule is a single match & replace directive.
type Rule struct {
	ID        string    `json:"id"`
	Enabled   bool      `json:"enabled"`
	Name      string    `json:"name"`
	Part      Part      `json:"part"`
	MatchType MatchType `json:"matchType"`
	Match     string    `json:"match"`
	Replace   string    `json:"replace"`
	Priority  int       `json:"priority"`
}

type compiledRule struct {
	rule  Rule
	regex *regexp.Regexp
}

// RuleSet is a thread-safe, priority-ordered collection of match & replace
// rules applied automatically to proxied traffic.
type RuleSet struct {
	mu       sync.RWMutex
	rules    []Rule
	compiled []compiledRule
}

// NewRuleSet returns an empty RuleSet.
func NewRuleSet() *RuleSet {
	return &RuleSet{}
}

// SetRules validates and atomically replaces the rules, compiling any regexes.
func (rs *RuleSet) SetRules(rules []Rule) error {
	sorted := append([]Rule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	compiled := make([]compiledRule, 0, len(sorted))
	for _, r := range sorted {
		cr := compiledRule{rule: r}
		if r.MatchType == Regex {
			re, err := regexp.Compile(r.Match)
			if err != nil {
				return fmt.Errorf("rule %s: invalid regex: %w", r.ID, err)
			}
			cr.regex = re
		}
		compiled = append(compiled, cr)
	}

	rs.mu.Lock()
	rs.rules = sorted
	rs.compiled = compiled
	rs.mu.Unlock()
	return nil
}

// Rules returns a copy of the current rules in priority order.
func (rs *RuleSet) Rules() []Rule {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return append([]Rule(nil), rs.rules...)
}

// ApplyRequest applies the enabled request rules in place and returns the
// (possibly rewritten) body.
func (rs *RuleSet) ApplyRequest(req *http.Request, body []byte) []byte {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for _, cr := range rs.compiled {
		if !cr.rule.Enabled {
			continue
		}
		switch cr.rule.Part {
		case PartRequestMethod:
			req.Method = strings.TrimSpace(cr.apply(req.Method))
		case PartRequestURL:
			if u, err := url.Parse(cr.apply(req.URL.String())); err == nil {
				req.URL = u
				if u.Host != "" {
					req.Host = u.Host
				}
			}
		case PartRequestHeader:
			applyHeaderBlock(cr, req.Header, &req.Host, true)
		case PartRequestBody:
			body = []byte(cr.apply(string(body)))
		case PartResponseHeader, PartResponseBody:
			// not applicable to requests
		}
	}
	syncContentLength(req.Header, len(body))
	return body
}

// ApplyResponse applies the enabled response rules in place and returns the
// (possibly rewritten) body.
func (rs *RuleSet) ApplyResponse(resp *http.Response, body []byte) []byte {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for _, cr := range rs.compiled {
		if !cr.rule.Enabled {
			continue
		}
		switch cr.rule.Part {
		case PartResponseHeader:
			applyHeaderBlock(cr, resp.Header, nil, false)
		case PartResponseBody:
			body = []byte(cr.apply(string(body)))
		case PartRequestMethod, PartRequestURL, PartRequestHeader, PartRequestBody:
			// not applicable to responses
		}
	}
	syncContentLength(resp.Header, len(body))
	return body
}

func (cr compiledRule) apply(s string) string {
	switch cr.rule.MatchType {
	case Regex:
		if cr.regex == nil {
			return s
		}
		return cr.regex.ReplaceAllString(s, cr.rule.Replace)
	case Literal:
		if cr.rule.Match == "" {
			return s
		}
		return strings.ReplaceAll(s, cr.rule.Match, cr.rule.Replace)
	default:
		return s
	}
}

// applyHeaderBlock serializes the header set (with Host first for requests) to
// "Key: Value" lines, applies the rule to that block, then parses it back.
func applyHeaderBlock(cr compiledRule, h http.Header, host *string, isRequest bool) {
	var b strings.Builder
	if isRequest && host != nil && *host != "" {
		b.WriteString("Host: " + *host + "\r\n")
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			b.WriteString(k + ": " + v + "\r\n")
		}
	}

	result := cr.apply(b.String())

	for k := range h {
		delete(h, k)
	}
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if isRequest && strings.EqualFold(name, "Host") && host != nil {
			*host = value
			continue
		}
		h.Add(name, value)
	}
}

func syncContentLength(h http.Header, n int) {
	if h.Get("Content-Length") != "" {
		h.Set("Content-Length", fmt.Sprintf("%d", n))
	}
}
