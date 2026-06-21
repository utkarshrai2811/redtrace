package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func collect(t *testing.T, s *Service, kind string) (full string, deltas []string, err error) {
	t.Helper()
	full, err = s.Stream(context.Background(), kind, []Message{{Role: RoleUser, Content: "hi"}},
		func(d string) error { deltas = append(deltas, d); return nil })
	return full, deltas, err
}

func TestStreamOpenAIMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("auth header = %q, want Bearer k", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w,
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n"+
				"data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n"+
				"data: [DONE]\n\n")
	}))
	defer srv.Close()

	s := NewService(Config{Provider: ProviderOpenAI, Model: "gpt-x", APIKey: "k", BaseURL: srv.URL}, nil)
	full, deltas, err := collect(t, s, KindChat)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if full != "Hello world" {
		t.Errorf("full = %q, want %q", full, "Hello world")
	}
	if len(deltas) != 2 || deltas[0] != "Hello " || deltas[1] != "world" {
		t.Errorf("deltas = %v", deltas)
	}
}

func TestStreamAnthropicMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic headers")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w,
			"event: content_block_delta\n"+
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \"}}\n\n"+
				"event: content_block_delta\n"+
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"world\"}}\n\n"+
				"event: message_stop\n"+
				"data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer srv.Close()

	s := NewService(Config{Provider: ProviderAnthropic, Model: "claude-x", APIKey: "k", BaseURL: srv.URL}, nil)
	full, deltas, err := collect(t, s, KindExplain)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if full != "Hello world" {
		t.Errorf("full = %q, want %q", full, "Hello world")
	}
	if len(deltas) != 2 {
		t.Errorf("deltas = %v", deltas)
	}
}

func TestStreamProviderErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	s := NewService(Config{Provider: ProviderOpenAI, Model: "gpt-x", APIKey: "bad", BaseURL: srv.URL}, nil)
	_, _, err := collect(t, s, KindChat)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v, want a 401 provider error", err)
	}
}

func TestStreamDisabled(t *testing.T) {
	s := NewService(Config{}, nil) // no key
	if _, _, err := collect(t, s, KindChat); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

func TestEnabledAndStatus(t *testing.T) {
	cases := []struct {
		cfg  Config
		want bool
	}{
		{Config{Provider: ProviderAnthropic, APIKey: "k"}, true},
		{Config{Provider: ProviderAnthropic}, false},                  // no key
		{Config{Provider: ProviderOpenAI, BaseURL: "http://x"}, true}, // local, no key needed
		{Config{Provider: ProviderOpenAI, APIKey: "k"}, true},
		{Config{Provider: ProviderOpenAI}, false}, // neither key nor base url
	}
	for i, c := range cases {
		s := NewService(c.cfg, nil)
		if got := s.Enabled(); got != c.want {
			t.Errorf("case %d: Enabled() = %v, want %v", i, got, c.want)
		}
		if st := s.Status(); st.KeySet != (strings.TrimSpace(c.cfg.APIKey) != "") {
			t.Errorf("case %d: Status().KeySet = %v", i, st.KeySet)
		}
	}
	// Status must never expose the key.
	s := NewService(Config{Provider: ProviderAnthropic, Model: "m", APIKey: "supersecret"}, nil)
	if st := s.Status(); st.Model != "m" || !st.Enabled || !st.KeySet {
		t.Errorf("status = %+v", st)
	}
}

func TestSystemPromptKinds(t *testing.T) {
	for _, k := range []string{KindChat, KindExplain, KindTriage, KindPayloads, "unknown"} {
		p := systemPrompt(k)
		if !strings.HasPrefix(p, sharedPreamble) || len(p) <= len(sharedPreamble) {
			t.Errorf("systemPrompt(%q) missing preamble or kind text", k)
		}
	}
}

func TestTruncateMessages(t *testing.T) {
	long := strings.Repeat("a", maxMessageChars+100)
	out := truncateMessages([]Message{{Role: RoleUser, Content: long}})
	if len([]rune(out[0].Content)) > maxMessageChars+len([]rune("\n…[truncated]")) {
		t.Errorf("content not truncated: %d runes", len([]rune(out[0].Content)))
	}
	if !strings.HasSuffix(out[0].Content, "[truncated]") {
		t.Error("truncation marker missing")
	}
}

func TestNormalizeDefaults(t *testing.T) {
	c := normalize(Config{Provider: "ANTHROPIC"})
	if c.Provider != ProviderAnthropic || c.Model != defaultAnthropicModel || c.MaxTokens != defaultMaxTokens {
		t.Errorf("normalize = %+v", c)
	}
	if c2 := normalize(Config{Provider: "weird"}); c2.Provider != ProviderAnthropic {
		t.Errorf("unknown provider should fall back to anthropic, got %q", c2.Provider)
	}
	if c3 := normalize(Config{Provider: ProviderOpenAI, BaseURL: "http://x/"}); c3.BaseURL != "http://x" || c3.Model != defaultOpenAIModel {
		t.Errorf("openai normalize = %+v", c3)
	}
}
