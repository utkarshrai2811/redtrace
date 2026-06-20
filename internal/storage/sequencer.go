package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateSequencerTask inserts a new token-analysis task in the pending state.
func (db *DB) CreateSequencerTask(ctx context.Context, t *models.SequencerTask) error {
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = "pending"
	}
	if t.HTTPVersion == "" {
		t.HTTPVersion = "HTTP/1.1"
	}
	if len(t.Tokens) == 0 {
		t.Tokens = []byte("[]")
	}
	if t.Report == nil {
		t.Report = []byte("") // report is NOT NULL; a fresh task has no analysis yet
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO sequencer_tasks
		   (id, name, scheme, host, template, http_version, source, selector, target, status, collected, tokens, report, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Scheme, t.Host, t.Template, t.HTTPVersion, t.Source, t.Selector, t.Target,
		t.Status, t.Collected, t.Tokens, t.Report, t.CreatedAt.Format(timeLayout), t.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create sequencer task: %w", err)
	}
	return nil
}

// ListSequencerTasks returns tasks newest first (without template/tokens/report).
func (db *DB) ListSequencerTasks(ctx context.Context) ([]*models.SequencerTask, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, scheme, host, http_version, source, selector, target, status, collected, created_at, updated_at
		 FROM sequencer_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list sequencer tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.SequencerTask
	for rows.Next() {
		t := &models.SequencerTask{}
		var created, updated string
		if err := rows.Scan(&t.ID, &t.Name, &t.Scheme, &t.Host, &t.HTTPVersion, &t.Source, &t.Selector,
			&t.Target, &t.Status, &t.Collected, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan sequencer task: %w", err)
		}
		t.CreatedAt, _ = time.Parse(timeLayout, created)
		t.UpdatedAt, _ = time.Parse(timeLayout, updated)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetSequencerTask returns a single task with its template, tokens, and report.
func (db *DB) GetSequencerTask(ctx context.Context, id string) (*models.SequencerTask, error) {
	t := &models.SequencerTask{}
	var created, updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, name, scheme, host, template, http_version, source, selector, target, status, collected, tokens, report, created_at, updated_at
		 FROM sequencer_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Scheme, &t.Host, &t.Template, &t.HTTPVersion, &t.Source, &t.Selector,
			&t.Target, &t.Status, &t.Collected, &t.Tokens, &t.Report, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sequencer task: %w", err)
	}
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	t.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return t, nil
}

// UpdateSequencerTask updates a draft task's editable fields.
func (db *DB) UpdateSequencerTask(ctx context.Context, t *models.SequencerTask) error {
	t.UpdatedAt = time.Now()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE sequencer_tasks SET name=?, scheme=?, host=?, template=?, http_version=?, source=?, selector=?, target=?, updated_at=?
		 WHERE id=?`,
		t.Name, t.Scheme, t.Host, t.Template, t.HTTPVersion, t.Source, t.Selector, t.Target,
		t.UpdatedAt.Format(timeLayout), t.ID)
	if err != nil {
		return fmt.Errorf("update sequencer task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSequencerTask removes a task.
func (db *DB) DeleteSequencerTask(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx, `DELETE FROM sequencer_tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete sequencer task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PrepareSequencerRun resets a task for a (re-)run.
func (db *DB) PrepareSequencerRun(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx,
		`UPDATE sequencer_tasks SET collected = 0, tokens = '[]', report = '', status = 'running', updated_at = ? WHERE id = ?`,
		time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("prepare sequencer run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateSequencerProgress persists the collected count and the token snapshot.
func (db *DB) UpdateSequencerProgress(ctx context.Context, id string, collected int, tokens []byte) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE sequencer_tasks SET collected = ?, tokens = ?, updated_at = ? WHERE id = ?`,
		collected, tokens, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("update sequencer progress: %w", err)
	}
	return nil
}

// SetSequencerResult records the terminal status and the analysis report.
func (db *DB) SetSequencerResult(ctx context.Context, id, status string, report []byte) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE sequencer_tasks SET status = ?, report = ?, updated_at = ? WHERE id = ?`,
		status, report, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set sequencer result: %w", err)
	}
	return nil
}

// reconcileRunningSequencers marks any task left 'running' (e.g. by a crash) as
// 'stopped'. Called once on startup.
func (db *DB) reconcileRunningSequencers() error {
	_, err := db.sql.ExecContext(context.Background(),
		`UPDATE sequencer_tasks SET status = ?, updated_at = ? WHERE status = ?`,
		"stopped", time.Now().Format(timeLayout), "running")
	if err != nil {
		return fmt.Errorf("reconcile running sequencers: %w", err)
	}
	return nil
}
