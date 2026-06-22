package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/ai"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

type aiConversationView struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type aiMessageView struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func toConversationView(c *models.AIConversation) aiConversationView {
	return aiConversationView{ID: c.ID, Title: c.Title, Kind: c.Kind, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func toMessageView(m *models.AIMessage) aiMessageView {
	return aiMessageView{ID: m.ID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt}
}

func validKind(kind string) bool {
	switch kind {
	case ai.KindChat, ai.KindExplain, ai.KindTriage, ai.KindPayloads:
		return true
	}
	return false
}

// titleFromText derives a short conversation title from its first user turn.
func titleFromText(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := []rune(line)
		if len(r) > 60 {
			return string(r[:60]) + "…"
		}
		return line
	}
	return "Conversation"
}

// AIConfig handles GET /api/ai/config — non-secret AI status (never the key).
func (a *API) AIConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.AI.Status())
}

type aiConfigRequest struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	BaseURL  string  `json:"baseUrl"`
	APIKey   *string `json:"apiKey"` // nil = keep the current key; "" = clear it
}

// UpdateAIConfig handles PUT /api/ai/config: reconfigure and persist the AI
// provider settings (the key is stored only in the local database).
func (a *API) UpdateAIConfig(w http.ResponseWriter, r *http.Request) {
	var req aiConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not parse request body")
		return
	}
	cur := a.AI.Config()
	next := ai.Config{Provider: req.Provider, Model: req.Model, BaseURL: req.BaseURL, APIKey: cur.APIKey}
	if req.APIKey != nil {
		next.APIKey = *req.APIKey
	}
	a.AI.Configure(next)

	// Persist the key only when the request actually carried one. On a "keep"
	// save (no apiKey field) we must NOT write the live in-memory key to disk —
	// it may be a flag/env key that the operator deliberately kept off disk — so
	// we re-persist whatever key was already stored (if any).
	resolved := a.AI.Config()
	persistKey := resolved.APIKey
	if req.APIKey == nil {
		persistKey = ""
		if s, ok, _ := a.Store.LoadAISettings(r.Context()); ok {
			persistKey = s.APIKey
		}
	}
	if err := a.Store.SaveAISettings(r.Context(), models.AISettings{
		Provider: resolved.Provider, Model: resolved.Model, BaseURL: resolved.BaseURL, APIKey: persistKey,
	}); err != nil {
		a.serverError(w, "save_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, a.AI.Status())
}

// ListAIConversations handles GET /api/ai/conversations.
func (a *API) ListAIConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := a.Store.ListAIConversations(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]aiConversationView, 0, len(convs))
	for _, c := range convs {
		views = append(views, toConversationView(c))
	}
	writeJSON(w, http.StatusOK, views)
}

type createConversationRequest struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Context string `json:"context"`
}

// CreateAIConversation handles POST /api/ai/conversations. When context is
// provided it becomes the first user message (the frontend assembles it from the
// selected exchange/finding); the kind selects the system prompt at generation.
func (a *API) CreateAIConversation(w http.ResponseWriter, r *http.Request) {
	var req createConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not parse request body")
		return
	}
	if req.Kind == "" {
		req.Kind = ai.KindChat
	}
	if !validKind(req.Kind) {
		writeError(w, http.StatusBadRequest, "invalid_kind", "unknown conversation kind")
		return
	}

	conv := &models.AIConversation{ID: storage.NewID(), Title: strings.TrimSpace(req.Title), Kind: req.Kind}
	if err := a.Store.CreateAIConversation(r.Context(), conv); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}

	var messages []aiMessageView
	if ctxText := strings.TrimSpace(req.Context); ctxText != "" {
		title := conv.Title
		if title == "" {
			title = titleFromText(ctxText)
		}
		m := &models.AIMessage{ID: storage.NewID(), ConversationID: conv.ID, Role: ai.RoleUser, Content: req.Context}
		if err := a.Store.AppendAIMessage(r.Context(), m, title); err != nil {
			a.serverError(w, "create_failed", err)
			return
		}
		conv.Title = title
		messages = append(messages, toMessageView(m))
	}

	writeJSON(w, http.StatusCreated, map[string]any{"conversation": toConversationView(conv), "messages": messages})
}

// GetAIConversation handles GET /api/ai/conversations/{id}.
func (a *API) GetAIConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := a.Store.GetAIConversation(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such conversation")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	msgs, err := a.Store.ListAIMessages(r.Context(), conv.ID)
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	views := make([]aiMessageView, 0, len(msgs))
	for _, m := range msgs {
		views = append(views, toMessageView(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": toConversationView(conv), "messages": views})
}

// DeleteAIConversation handles DELETE /api/ai/conversations/{id}.
func (a *API) DeleteAIConversation(w http.ResponseWriter, r *http.Request) {
	err := a.Store.DeleteAIConversation(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such conversation")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ClearAIConversations handles DELETE /api/ai/conversations.
func (a *API) ClearAIConversations(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.ClearAIConversations(r.Context()); err != nil {
		a.serverError(w, "clear_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type streamMessageRequest struct {
	Content string `json:"content"`
}

// StreamAIMessage handles POST /api/ai/conversations/{id}/messages. It appends
// an optional user message, then streams the assistant reply as Server-Sent
// Events (delta/done/error) and persists it. With an empty body it just
// generates a reply for the conversation's current messages (used right after
// a "Send to AI" action seeds the first user turn).
func (a *API) StreamAIMessage(w http.ResponseWriter, r *http.Request) {
	// Preflight before switching to SSE so the disabled case is a clean 400.
	if !a.AI.Enabled() {
		writeError(w, http.StatusBadRequest, "ai_disabled", "AI is not configured — set a provider and key in Settings")
		return
	}
	id := r.PathValue("id")
	conv, err := a.Store.GetAIConversation(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such conversation")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}

	// Serialize streaming per conversation so two concurrent requests (e.g. two
	// browser tabs) cannot interleave reads/appends and duplicate the reply.
	if _, busy := a.aiStreams.LoadOrStore(id, struct{}{}); busy {
		writeError(w, http.StatusConflict, "stream_in_progress", "a reply is already streaming for this conversation")
		return
	}
	defer a.aiStreams.Delete(id)

	// An empty body (Content-Length 0, "{}", or a chunked empty body) means
	// "generate a reply for the current messages"; tolerate the resulting EOF.
	var req streamMessageRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not parse request body")
		return
	}
	if content := strings.TrimSpace(req.Content); content != "" {
		title := ""
		if conv.Title == "" {
			title = titleFromText(req.Content)
		}
		um := &models.AIMessage{ID: storage.NewID(), ConversationID: id, Role: ai.RoleUser, Content: req.Content}
		if err := a.Store.AppendAIMessage(r.Context(), um, title); err != nil {
			a.serverError(w, "append_failed", err)
			return
		}
	}

	stored, err := a.Store.ListAIMessages(r.Context(), id)
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	if len(stored) == 0 {
		writeError(w, http.StatusBadRequest, "empty_conversation", "nothing to generate a reply to")
		return
	}
	msgs := make([]ai.Message, 0, len(stored))
	for _, m := range stored {
		msgs = append(msgs, ai.Message{Role: m.Role, Content: m.Content})
	}

	// Switch to Server-Sent Events.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	if fl != nil {
		fl.Flush()
	}

	full, genErr := a.AI.Stream(r.Context(), conv.Kind, msgs, func(delta string) error {
		return sseEvent(w, fl, "delta", map[string]string{"text": delta})
	})

	// A disconnected client cancels the context; just persist any partial reply.
	if r.Context().Err() != nil {
		a.persistAssistant(context.Background(), id, full)
		return
	}
	if genErr != nil {
		a.persistAssistant(r.Context(), id, full)
		_ = sseEvent(w, fl, "error", map[string]string{"message": cleanProviderError(genErr)})
		return
	}
	saved, ok := a.persistAssistant(r.Context(), id, full)
	if strings.TrimSpace(full) != "" && !ok {
		// The reply generated but could not be saved; report it rather than
		// emitting a 'done' that claims a message the next reload won't have.
		_ = sseEvent(w, fl, "error", map[string]string{"message": "reply generated but could not be saved"})
		return
	}
	_ = sseEvent(w, fl, "done", saved)
}

// persistAssistant stores a (possibly partial) assistant reply when non-empty
// and returns its view and whether it was actually saved.
func (a *API) persistAssistant(ctx context.Context, conversationID, content string) (aiMessageView, bool) {
	if strings.TrimSpace(content) == "" {
		return aiMessageView{}, true
	}
	m := &models.AIMessage{ID: storage.NewID(), ConversationID: conversationID, Role: ai.RoleAssistant, Content: content}
	if err := a.Store.AppendAIMessage(ctx, m, ""); err != nil {
		a.Log.Error("persist ai reply", "err", err)
		return aiMessageView{}, false
	}
	return toMessageView(m), true
}

func sseEvent(w http.ResponseWriter, fl http.Flusher, event string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	if fl != nil {
		fl.Flush()
	}
	return nil
}

// cleanProviderError returns a bounded, single-line error message for the UI.
// AI provider errors concern the operator's own provider/credentials, so they
// are surfaced (not hidden behind a generic 500 like storage errors).
func cleanProviderError(err error) string {
	msg := strings.ReplaceAll(err.Error(), "\n", " ")
	if r := []rune(msg); len(r) > 400 {
		msg = string(r[:400]) + "…"
	}
	return msg
}
