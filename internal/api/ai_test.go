package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/utkarshrai2811/redtrace/internal/ai"
	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

func newAIServer(t *testing.T, aiCfg ai.Config) (*httptest.Server, *storage.DB) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	authority, err := cert.LoadOrCreateAuthority(t.TempDir())
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	srv, err := New(Config{Version: "test", AI: aiCfg}, db, scope.New(), intercept.NewRuleSet(),
		intercept.NewInterceptor(storage.NewID), authority, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts, db
}

func mockOpenAIServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(s.Close)
	return s
}

func createConversation(t *testing.T, base, body string) string {
	t.Helper()
	resp, err := http.Post(base+"/api/ai/conversations", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var out struct {
		Conversation struct {
			ID string `json:"id"`
		} `json:"conversation"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Conversation.ID
}

// readSSE collects delta texts, whether a done frame arrived, and any error msg.
func readSSE(t *testing.T, body io.Reader) (deltas []string, done bool, errMsg string) {
	t.Helper()
	raw, _ := io.ReadAll(body)
	for _, frame := range strings.Split(string(raw), "\n\n") {
		var ev, data string
		for _, line := range strings.Split(frame, "\n") {
			if v, ok := strings.CutPrefix(line, "event:"); ok {
				ev = strings.TrimSpace(v)
			} else if v, ok := strings.CutPrefix(line, "data:"); ok {
				data = strings.TrimSpace(v)
			}
		}
		switch ev {
		case "delta":
			var d struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal([]byte(data), &d)
			deltas = append(deltas, d.Text)
		case "done":
			done = true
		case "error":
			var e struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal([]byte(data), &e)
			errMsg = e.Message
		}
	}
	return deltas, done, errMsg
}

func TestAIDisabledByDefault(t *testing.T) {
	ts, _ := newAIServer(t, ai.Config{})
	resp, err := http.Get(ts.URL + "/api/ai/config")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var cfg struct {
		Enabled bool `json:"enabled"`
		KeySet  bool `json:"keySet"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&cfg)
	if cfg.Enabled || cfg.KeySet {
		t.Errorf("default config = %+v, want disabled with no key", cfg)
	}

	// Streaming on a disabled assistant is a clean 400, not an SSE stream.
	id := createConversation(t, ts.URL, `{"kind":"chat","context":"hi"}`)
	resp2, err := http.Post(ts.URL+"/api/ai/conversations/"+id+"/messages", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("disabled stream status = %d, want 400", resp2.StatusCode)
	}
}

func TestAIKeepDoesNotPersistEnvKey(t *testing.T) {
	// Simulate a flag/env-supplied key: it is live in the service but not on disk.
	ts, db := newAIServer(t, ai.Config{Provider: ai.ProviderAnthropic, Model: "m", APIKey: "env-secret"})

	// A "keep" save (no apiKey field) must not write the env key to disk.
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/ai/config",
		strings.NewReader(`{"provider":"anthropic","model":"m2","baseUrl":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put status = %d", resp.StatusCode)
	}

	s, ok, err := db.LoadAISettings(context.Background())
	if err != nil || !ok {
		t.Fatalf("load settings: ok=%v err=%v", ok, err)
	}
	if s.APIKey != "" {
		t.Errorf("env key was persisted to disk on a keep-save: %q", s.APIKey)
	}
	if s.Model != "m2" {
		t.Errorf("model not persisted: %q", s.Model)
	}
}

func TestAIConversationStreamAndDelete(t *testing.T) {
	mock := mockOpenAIServer(t)
	ts, _ := newAIServer(t, ai.Config{Provider: ai.ProviderOpenAI, Model: "m", BaseURL: mock.URL, APIKey: "k"})

	id := createConversation(t, ts.URL, `{"kind":"explain","context":"GET /x"}`)
	resp, err := http.Post(ts.URL+"/api/ai/conversations/"+id+"/messages", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	deltas, done, errMsg := readSSE(t, resp.Body)
	if errMsg != "" {
		t.Fatalf("stream error: %s", errMsg)
	}
	if !done {
		t.Error("no done frame")
	}
	if strings.Join(deltas, "") != "hello world" {
		t.Errorf("deltas = %q", deltas)
	}

	// Conversation now has the seeded user turn + the persisted assistant reply.
	gresp, _ := http.Get(ts.URL + "/api/ai/conversations/" + id)
	defer func() { _ = gresp.Body.Close() }()
	var got struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(gresp.Body).Decode(&got)
	if len(got.Messages) != 2 || got.Messages[1].Role != "assistant" || got.Messages[1].Content != "hello world" {
		t.Errorf("messages = %+v", got.Messages)
	}

	// Delete is 204; deleting again (missing) is 404.
	for i, want := range []int{http.StatusNoContent, http.StatusNotFound} {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/ai/conversations/"+id, nil)
		dresp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = dresp.Body.Close()
		if dresp.StatusCode != want {
			t.Errorf("delete #%d status = %d, want %d", i, dresp.StatusCode, want)
		}
	}
}
