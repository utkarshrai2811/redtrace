package scope

import "testing"

func TestInScope(t *testing.T) {
	tests := []struct {
		name  string
		rules []Rule
		host  string
		path  string
		want  bool
	}{
		{
			name: "no rules means in scope",
			host: "anything.com", path: "/", want: true,
		},
		{
			name:  "include host match",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHost, Value: "example.com"}},
			host:  "example.com", path: "/", want: true,
		},
		{
			name:  "include host miss",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHost, Value: "example.com"}},
			host:  "other.com", path: "/", want: false,
		},
		{
			name: "exclude path prefix",
			rules: []Rule{
				{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHost, Value: "example.com"},
				{ID: "2", Enabled: true, Kind: Exclude, Matcher: MatchPathPrefix, Value: "/logout"},
			},
			host: "example.com", path: "/logout/now", want: false,
		},
		{
			name:  "host regex",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHostRegex, Value: `^.*\.example\.com$`}},
			host:  "api.example.com", path: "/", want: true,
		},
		{
			name:  "cidr match",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchCIDR, Value: "10.0.0.0/8"}},
			host:  "10.1.2.3", path: "/", want: true,
		},
		{
			name:  "cidr miss",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchCIDR, Value: "10.0.0.0/8"}},
			host:  "192.168.0.1", path: "/", want: false,
		},
		{
			name:  "host with port is stripped",
			rules: []Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHost, Value: "example.com"}},
			host:  "example.com:443", path: "/", want: true,
		},
		{
			name: "disabled include rule is ignored",
			rules: []Rule{
				{ID: "1", Enabled: false, Kind: Include, Matcher: MatchHost, Value: "example.com"},
			},
			host: "other.com", path: "/", want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			if err := s.SetRules(tt.rules); err != nil {
				t.Fatalf("SetRules: %v", err)
			}
			if got := s.InScope(tt.host, tt.path); got != tt.want {
				t.Errorf("InScope(%q, %q) = %v, want %v", tt.host, tt.path, got, tt.want)
			}
		})
	}
}

func TestSetRules_InvalidValues(t *testing.T) {
	s := New()
	if err := s.SetRules([]Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchHostRegex, Value: "("}}); err == nil {
		t.Error("expected error for invalid regex")
	}
	if err := s.SetRules([]Rule{{ID: "1", Enabled: true, Kind: Include, Matcher: MatchCIDR, Value: "not-a-cidr"}}); err == nil {
		t.Error("expected error for invalid CIDR")
	}
}
