package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// streamAnthropic calls the Anthropic Messages API with streaming enabled.
func (s *Service) streamAnthropic(ctx context.Context, cfg Config, system string, msgs []Message, onDelta func(string) error) (string, error) {
	am := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		am = append(am, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{
		"model":      cfg.Model,
		"max_tokens": cfg.MaxTokens,
		"system":     system,
		"stream":     true,
		"messages":   am,
	})
	if err != nil {
		return "", fmt.Errorf("encode anthropic request: %w", err)
	}

	base := cfg.BaseURL
	if base == "" {
		base = anthropicBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", providerError("anthropic", resp)
	}

	var full strings.Builder
	err = readSSE(resp.Body, func(data []byte) (bool, error) {
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if e := json.Unmarshal(data, &ev); e != nil {
			return false, nil // ignore an unparseable keep-alive frame
		}
		switch ev.Type {
		case "content_block_delta":
			if ev.Delta.Text != "" {
				full.WriteString(ev.Delta.Text)
				if e := onDelta(ev.Delta.Text); e != nil {
					return true, e
				}
			}
		case "error":
			return true, fmt.Errorf("anthropic: %s", ev.Error.Message)
		case "message_stop":
			return true, nil
		}
		return false, nil
	})
	return full.String(), err
}
