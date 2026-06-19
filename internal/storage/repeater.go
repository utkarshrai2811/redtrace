package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateRepeaterTab inserts a new Repeater tab.
func (db *DB) CreateRepeaterTab(ctx context.Context, t *models.RepeaterTab) error {
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO repeater_tabs (id, name, scheme, host, raw, follow_redirects, http_version, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Scheme, t.Host, t.Raw, boolToInt(t.FollowRedirects), t.HTTPVersion,
		t.CreatedAt.Format(timeLayout), t.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create repeater tab: %w", err)
	}
	return nil
}

// ListRepeaterTabs returns all tabs in creation order.
func (db *DB) ListRepeaterTabs(ctx context.Context) ([]*models.RepeaterTab, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, scheme, host, raw, follow_redirects, http_version, created_at, updated_at
		 FROM repeater_tabs ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list repeater tabs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.RepeaterTab
	for rows.Next() {
		t, err := scanRepeaterTab(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetRepeaterTab returns a single tab, or ErrNotFound.
func (db *DB) GetRepeaterTab(ctx context.Context, id string) (*models.RepeaterTab, error) {
	row := db.sql.QueryRowContext(ctx,
		`SELECT id, name, scheme, host, raw, follow_redirects, http_version, created_at, updated_at
		 FROM repeater_tabs WHERE id = ?`, id)
	t, err := scanRepeaterTab(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get repeater tab: %w", err)
	}
	return t, nil
}

// UpdateRepeaterTab updates a tab's editable fields and bumps updated_at.
func (db *DB) UpdateRepeaterTab(ctx context.Context, t *models.RepeaterTab) error {
	t.UpdatedAt = time.Now()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE repeater_tabs SET name=?, scheme=?, host=?, raw=?, follow_redirects=?, http_version=?, updated_at=?
		 WHERE id=?`,
		t.Name, t.Scheme, t.Host, t.Raw, boolToInt(t.FollowRedirects), t.HTTPVersion,
		t.UpdatedAt.Format(timeLayout), t.ID)
	if err != nil {
		return fmt.Errorf("update repeater tab: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRepeaterTab removes a tab and its history (cascade).
func (db *DB) DeleteRepeaterTab(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx, `DELETE FROM repeater_tabs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete repeater tab: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddRepeaterHistory records a send.
func (db *DB) AddRepeaterHistory(ctx context.Context, e *models.RepeaterHistoryEntry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO repeater_history (id, tab_id, request_raw, response_raw, status_code, duration_ms, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		e.ID, e.TabID, e.RequestRaw, e.ResponseRaw, e.StatusCode, e.DurationMs, e.CreatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("add repeater history: %w", err)
	}
	return nil
}

// ListRepeaterHistory returns a tab's sends, oldest first.
func (db *DB) ListRepeaterHistory(ctx context.Context, tabID string) ([]*models.RepeaterHistoryEntry, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, tab_id, request_raw, response_raw, status_code, duration_ms, created_at
		 FROM repeater_history WHERE tab_id = ? ORDER BY created_at`, tabID)
	if err != nil {
		return nil, fmt.Errorf("list repeater history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.RepeaterHistoryEntry
	for rows.Next() {
		e := &models.RepeaterHistoryEntry{}
		var ts string
		if err := rows.Scan(&e.ID, &e.TabID, &e.RequestRaw, &e.ResponseRaw, &e.StatusCode, &e.DurationMs, &ts); err != nil {
			return nil, fmt.Errorf("scan repeater history: %w", err)
		}
		e.CreatedAt, _ = time.Parse(timeLayout, ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRepeaterTab(s rowScanner) (*models.RepeaterTab, error) {
	t := &models.RepeaterTab{}
	var created, updated string
	if err := s.Scan(&t.ID, &t.Name, &t.Scheme, &t.Host, &t.Raw, &t.FollowRedirects,
		&t.HTTPVersion, &created, &updated); err != nil {
		return nil, err
	}
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	t.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return t, nil
}
