package models

import "time"

// AISettings is the (single-row) persisted AI provider configuration. The API
// key is stored locally in the RedTrace database and is never returned to the
// UI — only whether one is set.
type AISettings struct {
	Provider  string    `json:"provider"` // "anthropic" | "openai"
	Model     string    `json:"model"`
	BaseURL   string    `json:"baseUrl"`
	APIKey    string    `json:"-"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AIConversation is a saved AI conversation. Kind selects the system prompt and
// is set when the conversation is created.
type AIConversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"` // "chat" | "explain" | "triage" | "payloads"
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AIMessage is one turn in a conversation.
type AIMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"` // "user" | "assistant"
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
}
