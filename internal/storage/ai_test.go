package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

func aiTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAISettingsUpsertRoundTrip(t *testing.T) {
	db := aiTestDB(t)
	ctx := context.Background()

	if _, ok, err := db.LoadAISettings(ctx); err != nil || ok {
		t.Fatalf("fresh db: ok=%v err=%v, want ok=false", ok, err)
	}
	if err := db.SaveAISettings(ctx, models.AISettings{Provider: "openai", Model: "m1", BaseURL: "http://x", APIKey: "k1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := db.LoadAISettings(ctx)
	if err != nil || !ok || got.Provider != "openai" || got.Model != "m1" || got.APIKey != "k1" {
		t.Fatalf("load = %+v ok=%v err=%v", got, ok, err)
	}
	// A second save must update the single row, not insert a duplicate.
	if err := db.SaveAISettings(ctx, models.AISettings{Provider: "anthropic", Model: "m2", APIKey: "k2"}); err != nil {
		t.Fatalf("resave: %v", err)
	}
	got, _, _ = db.LoadAISettings(ctx)
	if got.Provider != "anthropic" || got.Model != "m2" || got.BaseURL != "" {
		t.Errorf("after upsert = %+v", got)
	}
}

func TestAppendAIMessageTitlesOnlyUntitled(t *testing.T) {
	db := aiTestDB(t)
	ctx := context.Background()
	conv := &models.AIConversation{ID: "c1", Kind: "chat"}
	if err := db.CreateAIConversation(ctx, conv); err != nil {
		t.Fatal(err)
	}
	// First turn names the untitled conversation.
	if err := db.AppendAIMessage(ctx, &models.AIMessage{ID: "m1", ConversationID: "c1", Role: "user", Content: "hi"}, "First title"); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetAIConversation(ctx, "c1")
	if got.Title != "First title" {
		t.Fatalf("title = %q, want First title", got.Title)
	}
	// A later turn must NOT overwrite an existing title.
	if err := db.AppendAIMessage(ctx, &models.AIMessage{ID: "m2", ConversationID: "c1", Role: "assistant", Content: "yo"}, "Second title"); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetAIConversation(ctx, "c1")
	if got.Title != "First title" {
		t.Errorf("title overwritten to %q", got.Title)
	}
}

func TestListAIMessagesOrderingAndDeleteCascade(t *testing.T) {
	db := aiTestDB(t)
	ctx := context.Background()
	if err := db.CreateAIConversation(ctx, &models.AIConversation{ID: "c1", Kind: "chat"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"m1", "m2", "m3"} {
		if err := db.AppendAIMessage(ctx, &models.AIMessage{ID: id, ConversationID: "c1", Role: "user", Content: id}, ""); err != nil {
			t.Fatal(err)
		}
	}
	msgs, err := db.ListAIMessages(ctx, "c1")
	if err != nil || len(msgs) != 3 || msgs[0].ID != "m1" || msgs[2].ID != "m3" {
		t.Fatalf("messages = %+v err=%v (want m1,m2,m3 oldest-first)", msgs, err)
	}

	// Deleting the conversation removes its messages.
	if err := db.DeleteAIConversation(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if msgs, _ := db.ListAIMessages(ctx, "c1"); len(msgs) != 0 {
		t.Errorf("messages survived conversation delete: %d", len(msgs))
	}
	// Deleting a missing conversation is ErrNotFound.
	if err := db.DeleteAIConversation(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing = %v, want ErrNotFound", err)
	}
}
