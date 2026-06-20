package intruder

import (
	"reflect"
	"strings"
	"testing"
)

func tmpl(s string) []byte { return []byte(s) }

func collect(cfg Config) [][]string {
	var jobs [][]string
	forEachJob(cfg, func(p []string) bool {
		jobs = append(jobs, append([]string(nil), p...))
		return true
	})
	return jobs
}

func TestPositions(t *testing.T) {
	cases := map[string]int{
		"GET /?a=§1§ HTTP/1.1":       1,
		"GET /?a=§1§&b=§2§ HTTP/1.1": 2,
		"GET / HTTP/1.1":             0,
		"GET /?a=§1§&b=§2§&c=§3§":    3,
	}
	for in, want := range cases {
		if got := Positions(tmpl(in)); got != want {
			t.Errorf("Positions(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSniper(t *testing.T) {
	cfg := Config{
		Template:    tmpl("a=§X§&b=§Y§"),
		Type:        Sniper,
		PayloadSets: []PayloadSet{{Payloads: []string{"1", "2"}}},
	}
	if got := Count(cfg); got != 4 { // 2 positions × 2 payloads
		t.Fatalf("Count = %d, want 4", got)
	}
	want := [][]string{
		{"1", "Y"}, {"2", "Y"}, // position 0 varies, position 1 at base
		{"X", "1"}, {"X", "2"}, // position 1 varies, position 0 at base
	}
	if got := collect(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("sniper vectors = %v, want %v", got, want)
	}
}

func TestBatteringRam(t *testing.T) {
	cfg := Config{
		Template:    tmpl("a=§X§&b=§Y§"),
		Type:        BatteringRam,
		PayloadSets: []PayloadSet{{Payloads: []string{"p", "q"}}},
	}
	if got := Count(cfg); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
	want := [][]string{{"p", "p"}, {"q", "q"}}
	if got := collect(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("battering ram vectors = %v, want %v", got, want)
	}
}

func TestPitchfork(t *testing.T) {
	cfg := Config{
		Template: tmpl("a=§X§&b=§Y§"),
		Type:     Pitchfork,
		PayloadSets: []PayloadSet{
			{Payloads: []string{"a1", "a2", "a3"}},
			{Payloads: []string{"b1", "b2"}}, // shorter set bounds the run
		},
	}
	if got := Count(cfg); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
	want := [][]string{{"a1", "b1"}, {"a2", "b2"}}
	if got := collect(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("pitchfork vectors = %v, want %v", got, want)
	}
}

func TestClusterBomb(t *testing.T) {
	cfg := Config{
		Template: tmpl("a=§X§&b=§Y§"),
		Type:     ClusterBomb,
		PayloadSets: []PayloadSet{
			{Payloads: []string{"a1", "a2"}},
			{Payloads: []string{"b1", "b2", "b3"}},
		},
	}
	if got := Count(cfg); got != 6 { // 2 × 3
		t.Fatalf("Count = %d, want 6", got)
	}
	want := [][]string{
		{"a1", "b1"}, {"a1", "b2"}, {"a1", "b3"},
		{"a2", "b1"}, {"a2", "b2"}, {"a2", "b3"},
	}
	if got := collect(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("cluster bomb vectors = %v, want %v", got, want)
	}
}

func TestApplyProcessors(t *testing.T) {
	cases := []struct {
		procs []Processor
		in    string
		want  string
	}{
		{[]Processor{{Kind: "prefix", Value: "x-"}}, "p", "x-p"},
		{[]Processor{{Kind: "suffix", Value: "!"}}, "p", "p!"},
		{[]Processor{{Kind: "upper"}}, "abc", "ABC"},
		{[]Processor{{Kind: "base64"}}, "hi", "aGk="},
		{[]Processor{{Kind: "url"}}, "a b", "a+b"},
		{[]Processor{{Kind: "prefix", Value: "a"}, {Kind: "suffix", Value: "z"}}, "X", "aXz"},
	}
	for _, c := range cases {
		if got := applyProcessors(c.in, c.procs); got != c.want {
			t.Errorf("applyProcessors(%q, %v) = %q, want %q", c.in, c.procs, got, c.want)
		}
	}
}

func TestBuildUpdatesContentLength(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 1\r\n\r\nid=§v§"
	tp := parseTemplate([]byte(raw))
	out := string(tp.build([]string{"123456"}))
	// Body is "id=123456" (9 bytes); the header must be rewritten to match.
	if !strings.Contains(out, "Content-Length: 9\r\n") {
		t.Errorf("Content-Length not updated: %q", out)
	}
	if !strings.HasSuffix(out, "\r\n\r\nid=123456") {
		t.Errorf("body not substituted: %q", out)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(Config{Template: tmpl("GET / HTTP/1.1"), Type: Sniper}); err == nil {
		t.Error("expected error for a template with no positions")
	}
	if err := Validate(Config{Template: tmpl("a=§1§"), Type: Sniper}); err == nil {
		t.Error("expected error for no payload sets")
	}
	ok := Config{Template: tmpl("a=§1§"), Type: Sniper, PayloadSets: []PayloadSet{{Payloads: []string{"x"}}}}
	if err := Validate(ok); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

func TestValidateUnknownType(t *testing.T) {
	cfg := Config{Template: tmpl("a=§1§"), Type: "bogus", PayloadSets: []PayloadSet{{Payloads: []string{"x"}}}}
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown attack type") {
		t.Errorf("Validate(unknown type) = %v, want an 'unknown attack type' error", err)
	}
}

func TestClusterBombCountSaturates(t *testing.T) {
	big := make([]string, 8000)
	for i := range big {
		big[i] = "x"
	}
	cfg := Config{
		Template:    tmpl("a=§X§&b=§Y§"),
		Type:        ClusterBomb,
		PayloadSets: []PayloadSet{{Payloads: big}, {Payloads: big}},
	}
	// 8000 × 8000 = 64,000,000 exceeds maxJobs: Count must saturate (stay
	// positive, never overflow to a negative/garbage total) and Validate reject.
	if n := Count(cfg); n <= maxJobs {
		t.Fatalf("Count = %d, want > maxJobs (%d)", n, maxJobs)
	}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Errorf("Validate(over-large cluster bomb) = %v, want a 'maximum' error", err)
	}
}
