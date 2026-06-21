package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// streamOpenAI calls an OpenAI-compatible /chat/completions endpoint with
// streaming enabled. This covers OpenAI, OpenRouter, and local runtimes
// (Ollama, LM Studio, vLLM, …) via the configured base URL.
func (s *Service) streamOpenAI(ctx context.Context, cfg Config, system string, msgs []Message, onDelta func(string) error) (string, error) {
	om := make([]map[string]string, 0, len(msgs)+1)
	om = append(om, map[string]string{"role": "system", "content": system})
	for _, m := range msgs {
		om = append(om, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{
		"model":      cfg.Model,
		"stream":     true,
		"max_tokens": cfg.MaxTokens,
		"messages":   om,
	})
	if err != nil {
		return "", fmt.Errorf("encode openai request: %w", err)
	}

	base := cfg.BaseURL
	if base == "" {
		base = openAIBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", providerError("openai", resp)
	}

	var full strings.Builder
	err = readSSE(resp.Body, func(data []byte) (bool, error) {
		if string(data) == "[DONE]" {
			return true, nil
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if e := json.Unmarshal(data, &ev); e != nil {
			return false, nil
		}
		for _, c := range ev.Choices {
			if c.Delta.Content != "" {
				full.WriteString(c.Delta.Content)
				if e := onDelta(c.Delta.Content); e != nil {
					return true, e
				}
			}
		}
		return false, nil
	})
	return full.String(), err
}
