// Package ai implements RedTrace's optional AI assistant: a thin streaming
// client over a configurable LLM provider (Anthropic or any OpenAI-compatible
// endpoint, including local models). It is opt-in — disabled until a model and
// credentials are configured — and only ever talks to the provider the operator
// configures. Captured traffic is sent to that provider only when the operator
// explicitly invokes an AI action.
package ai

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Provider identifiers.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
)

// Conversation kinds; each selects a system prompt.
const (
	KindChat     = "chat"
	KindExplain  = "explain"
	KindTriage   = "triage"
	KindPayloads = "payloads"
)

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

const (
	anthropicBaseURL      = "https://api.anthropic.com"
	anthropicVersion      = "2023-06-01"
	openAIBaseURL         = "https://api.openai.com/v1"
	defaultAnthropicModel = "claude-sonnet-4-6"
	defaultOpenAIModel    = "gpt-4o-mini"
	defaultMaxTokens      = 2048
	maxMessageChars       = 24000 // per-message cap so a huge paste can't blow the context
)

// ErrNotConfigured is returned when an AI action is attempted before the
// assistant is configured (no model / no credentials).
var ErrNotConfigured = errors.New("ai is not configured")

// Config is the AI provider configuration.
type Config struct {
	Provider  string
	Model     string
	APIKey    string
	BaseURL   string
	MaxTokens int
}

// Status is the non-secret configuration surfaced to the UI — it never includes
// the API key, only whether one is set.
type Status struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"baseUrl"`
	KeySet   bool   `json:"keySet"`
}

// Message is one conversation turn handed to a provider.
type Message struct {
	Role    string
	Content string
}

// Service holds the live (mutable) AI configuration and an HTTP client tuned for
// streaming. It is safe for concurrent use.
type Service struct {
	mu     sync.RWMutex
	cfg    Config
	client *http.Client
	log    *slog.Logger
}

// NewService returns a Service seeded with cfg.
func NewService(cfg Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		cfg: normalize(cfg),
		log: log,
		// No overall client timeout: a stream is long-lived and cancellation is
		// driven by the request context. Bound the connection setup instead.
		client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
	}
}

// normalize fills in defaults so a partially-specified config is usable.
func normalize(cfg Config) Config {
	cfg.Provider = strings.TrimSpace(strings.ToLower(cfg.Provider))
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.Provider == "" {
		cfg.Provider = ProviderAnthropic
	}
	if cfg.Provider != ProviderOpenAI {
		cfg.Provider = ProviderAnthropic
	}
	if cfg.Model == "" {
		if cfg.Provider == ProviderOpenAI {
			cfg.Model = defaultOpenAIModel
		} else {
			cfg.Model = defaultAnthropicModel
		}
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	return cfg
}

// enabled reports whether the assistant has enough configuration to run. An
// OpenAI-compatible endpoint needs a base URL (the key is optional for local
// models); Anthropic needs an API key.
func (c Config) enabled() bool {
	if c.Model == "" {
		return false
	}
	if c.Provider == ProviderOpenAI {
		return c.APIKey != "" || c.BaseURL != ""
	}
	return c.APIKey != ""
}

// Configure replaces the live configuration.
func (s *Service) Configure(cfg Config) {
	s.mu.Lock()
	s.cfg = normalize(cfg)
	s.mu.Unlock()
}

// Config returns a copy of the live configuration, including the API key. It is
// for persistence only — never serialize it to a client; use Status instead.
func (s *Service) Config() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Status returns the non-secret configuration for the UI.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Status{
		Enabled:  s.cfg.enabled(),
		Provider: s.cfg.Provider,
		Model:    s.cfg.Model,
		BaseURL:  s.cfg.BaseURL,
		KeySet:   s.cfg.APIKey != "",
	}
}

// Enabled reports whether an AI action can currently run.
func (s *Service) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.enabled()
}

// Stream sends the conversation to the configured provider and invokes onDelta
// for each text chunk as it arrives. It returns the full assistant text. The
// kind selects the system prompt. ctx cancellation aborts the upstream request.
func (s *Service) Stream(ctx context.Context, kind string, msgs []Message, onDelta func(string) error) (string, error) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if !cfg.enabled() {
		return "", ErrNotConfigured
	}
	system := systemPrompt(kind)
	msgs = truncateMessages(msgs)
	if cfg.Provider == ProviderOpenAI {
		return s.streamOpenAI(ctx, cfg, system, msgs, onDelta)
	}
	return s.streamAnthropic(ctx, cfg, system, msgs, onDelta)
}

// truncateMessages caps each message so a large paste cannot blow up the request.
func truncateMessages(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		m.Content = truncate(m.Content, maxMessageChars)
		out[i] = m
	}
	return out
}

// truncate shortens s to at most n runes, appending a marker when it cuts.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n…[truncated]"
}
