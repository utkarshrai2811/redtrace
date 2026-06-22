package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// LoadAISettings returns the persisted AI configuration. A fresh database has no
// row yet; that is reported as (zero-value settings, false, nil).
func (db *DB) LoadAISettings(ctx context.Context) (models.AISettings, bool, error) {
	var s models.AISettings
	var updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT provider, model, base_url, api_key, updated_at FROM ai_settings WHERE id = 1`).
		Scan(&s.Provider, &s.Model, &s.BaseURL, &s.APIKey, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AISettings{}, false, nil
	}
	if err != nil {
		return models.AISettings{}, false, fmt.Errorf("load ai settings: %w", err)
	}
	s.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return s, true, nil
}

// SaveAISettings upserts the single AI settings row.
func (db *DB) SaveAISettings(ctx context.Context, s models.AISettings) error {
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO ai_settings (id, provider, model, base_url, api_key, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   provider = excluded.provider, model = excluded.model,
		   base_url = excluded.base_url, api_key = excluded.api_key,
		   updated_at = excluded.updated_at`,
		s.Provider, s.Model, s.BaseURL, s.APIKey, s.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("save ai settings: %w", err)
	}
	return nil
}

// CreateAIConversation inserts a new conversation.
func (db *DB) CreateAIConversation(ctx context.Context, c *models.AIConversation) error {
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO ai_conversations (id, title, kind, created_at, updated_at) VALUES (?,?,?,?,?)`,
		c.ID, c.Title, c.Kind, c.CreatedAt.Format(timeLayout), c.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create ai conversation: %w", err)
	}
	return nil
}

// ListAIConversations returns conversations most-recently-updated first.
func (db *DB) ListAIConversations(ctx context.Context) ([]*models.AIConversation, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, title, kind, created_at, updated_at FROM ai_conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list ai conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.AIConversation
	for rows.Next() {
		c := &models.AIConversation{}
		var created, updated string
		if err := rows.Scan(&c.ID, &c.Title, &c.Kind, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan ai conversation: %w", err)
		}
		c.CreatedAt, _ = time.Parse(timeLayout, created)
		c.UpdatedAt, _ = time.Parse(timeLayout, updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetAIConversation returns one conversation.
func (db *DB) GetAIConversation(ctx context.Context, id string) (*models.AIConversation, error) {
	c := &models.AIConversation{}
	var created, updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, title, kind, created_at, updated_at FROM ai_conversations WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &c.Kind, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ai conversation: %w", err)
	}
	c.CreatedAt, _ = time.Parse(timeLayout, created)
	c.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return c, nil
}

// DeleteAIConversation removes a conversation and its messages atomically,
// reporting ErrNotFound when the conversation does not exist (matching the other
// delete endpoints). The message delete is explicit (rather than relying solely
// on the ON DELETE CASCADE) so it also cleans up databases migrated before the
// foreign key existed.
func (db *DB) DeleteAIConversation(ctx context.Context, id string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete ai conversation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM ai_messages WHERE conversation_id = ?`, id); err != nil {
		return fmt.Errorf("delete ai messages: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM ai_conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete ai conversation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// ClearAIConversations removes every conversation and message atomically.
func (db *DB) ClearAIConversations(ctx context.Context) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clear ai conversations: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM ai_messages`); err != nil {
		return fmt.Errorf("clear ai messages: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ai_conversations`); err != nil {
		return fmt.Errorf("clear ai conversations: %w", err)
	}
	return tx.Commit()
}

// AppendAIMessage stores a message and bumps its conversation's updated_at. The
// optional title is applied only when the conversation has none yet (so the
// first user turn names an untitled conversation).
func (db *DB) AppendAIMessage(ctx context.Context, m *models.AIMessage, title string) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("append ai message: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO ai_messages (id, conversation_id, role, content, created_at) VALUES (?,?,?,?,?)`,
		m.ID, m.ConversationID, m.Role, m.Content, m.CreatedAt.Format(timeLayout)); err != nil {
		return fmt.Errorf("append ai message: %w", err)
	}
	if title != "" {
		_, err = tx.ExecContext(ctx,
			`UPDATE ai_conversations SET updated_at = ?, title = ? WHERE id = ? AND title = ''`,
			m.CreatedAt.Format(timeLayout), title, m.ConversationID)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE ai_conversations SET updated_at = ? WHERE id = ?`,
			m.CreatedAt.Format(timeLayout), m.ConversationID)
	}
	if err != nil {
		return fmt.Errorf("touch ai conversation: %w", err)
	}
	return tx.Commit()
}

// ListAIMessages returns a conversation's messages oldest-first.
func (db *DB) ListAIMessages(ctx context.Context, conversationID string) ([]*models.AIMessage, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, created_at FROM ai_messages
		 WHERE conversation_id = ? ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list ai messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.AIMessage
	for rows.Next() {
		m := &models.AIMessage{}
		var created string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &created); err != nil {
			return nil, fmt.Errorf("scan ai message: %w", err)
		}
		m.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, m)
	}
	return out, rows.Err()
}
