// Package scope implements RedTrace's global include/exclude target scope, which
// is applied uniformly across the proxy and every analysis tool.
package scope

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
)

// Kind is whether a rule includes or excludes matching traffic.
type Kind string

const (
	Include Kind = "include"
	Exclude Kind = "exclude"
)

// Matcher selects how a rule's value is compared against a request.
type Matcher string

const (
	MatchHost       Matcher = "host"        // exact host (case-insensitive)
	MatchHostRegex  Matcher = "host_regex"  // regular expression over the host
	MatchPathPrefix Matcher = "path_prefix" // path has this prefix
	MatchCIDR       Matcher = "cidr"        // host is an IP within this CIDR
)

// Rule is a single scope entry.
type Rule struct {
	ID      string  `json:"id"`
	Enabled bool    `json:"enabled"`
	Kind    Kind    `json:"kind"`
	Matcher Matcher `json:"matcher"`
	Value   string  `json:"value"`
}

type compiledRule struct {
	rule  Rule
	regex *regexp.Regexp
	cidr  *net.IPNet
}

// Scope holds an ordered set of rules and evaluates whether a request is in
// scope. It is safe for concurrent use.
type Scope struct {
	mu       sync.RWMutex
	rules    []Rule
	compiled []compiledRule
}

// New returns an empty Scope. With no include rules, all traffic is in scope
// (minus anything matched by an exclude rule).
func New() *Scope {
	return &Scope{}
}

// SetRules validates and atomically replaces the rule set. It returns an error
// (leaving the existing rules untouched) if any regex or CIDR value is invalid.
func (s *Scope) SetRules(rules []Rule) error {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		cr := compiledRule{rule: r}
		switch r.Matcher {
		case MatchHostRegex:
			re, err := regexp.Compile(r.Value)
			if err != nil {
				return fmt.Errorf("scope rule %s: invalid regex: %w", r.ID, err)
			}
			cr.regex = re
		case MatchCIDR:
			_, network, err := net.ParseCIDR(r.Value)
			if err != nil {
				return fmt.Errorf("scope rule %s: invalid CIDR: %w", r.ID, err)
			}
			cr.cidr = network
		case MatchHost, MatchPathPrefix:
			// no precompilation needed
		default:
			return fmt.Errorf("scope rule %s: unknown matcher %q", r.ID, r.Matcher)
		}
		compiled = append(compiled, cr)
	}

	s.mu.Lock()
	s.rules = append([]Rule(nil), rules...)
	s.compiled = compiled
	s.mu.Unlock()
	return nil
}

// Rules returns a copy of the current rule set.
func (s *Scope) Rules() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Rule(nil), s.rules...)
}

// InScope reports whether the given host and path are in scope.
func (s *Scope) InScope(host, path string) bool {
	host = stripPort(strings.ToLower(host))

	s.mu.RLock()
	defer s.mu.RUnlock()

	hasInclude := false
	included := false
	for _, cr := range s.compiled {
		if !cr.rule.Enabled {
			continue
		}
		if cr.rule.Kind == Include {
			hasInclude = true
			if cr.matches(host, path) {
				included = true
			}
		}
	}
	// With no include rules, everything is implicitly included.
	if hasInclude && !included {
		return false
	}

	for _, cr := range s.compiled {
		if cr.rule.Enabled && cr.rule.Kind == Exclude && cr.matches(host, path) {
			return false
		}
	}
	return true
}

func (cr compiledRule) matches(host, path string) bool {
	switch cr.rule.Matcher {
	case MatchHost:
		return strings.EqualFold(stripPort(cr.rule.Value), host)
	case MatchHostRegex:
		return cr.regex != nil && cr.regex.MatchString(host)
	case MatchPathPrefix:
		return strings.HasPrefix(path, cr.rule.Value)
	case MatchCIDR:
		ip := net.ParseIP(host)
		return cr.cidr != nil && ip != nil && cr.cidr.Contains(ip)
	default:
		return false
	}
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
