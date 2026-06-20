package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateCrawlTask inserts a new crawl in the pending state.
func (db *DB) CreateCrawlTask(ctx context.Context, t *models.CrawlTask) error {
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = "pending"
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO crawl_tasks
		   (id, name, seed, scheme, host, max_depth, max_pages, status, pages, found, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Seed, t.Scheme, t.Host, t.MaxDepth, t.MaxPages, t.Status, t.Pages, t.Found,
		t.CreatedAt.Format(timeLayout), t.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create crawl task: %w", err)
	}
	return nil
}

// ListCrawlTasks returns crawls newest first.
func (db *DB) ListCrawlTasks(ctx context.Context) ([]*models.CrawlTask, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, seed, scheme, host, max_depth, max_pages, status, pages, found, created_at, updated_at
		 FROM crawl_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list crawl tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.CrawlTask
	for rows.Next() {
		t := &models.CrawlTask{}
		var created, updated string
		if err := rows.Scan(&t.ID, &t.Name, &t.Seed, &t.Scheme, &t.Host, &t.MaxDepth, &t.MaxPages,
			&t.Status, &t.Pages, &t.Found, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan crawl task: %w", err)
		}
		t.CreatedAt, _ = time.Parse(timeLayout, created)
		t.UpdatedAt, _ = time.Parse(timeLayout, updated)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetCrawlTask returns a single crawl, or ErrNotFound.
func (db *DB) GetCrawlTask(ctx context.Context, id string) (*models.CrawlTask, error) {
	t := &models.CrawlTask{}
	var created, updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, name, seed, scheme, host, max_depth, max_pages, status, pages, found, created_at, updated_at
		 FROM crawl_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Seed, &t.Scheme, &t.Host, &t.MaxDepth, &t.MaxPages,
			&t.Status, &t.Pages, &t.Found, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get crawl task: %w", err)
	}
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	t.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return t, nil
}

// DeleteCrawlTask removes a crawl and its discovered-URL rows.
func (db *DB) DeleteCrawlTask(ctx context.Context, id string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM crawl_urls WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete crawl urls: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM crawl_tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete crawl task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// SetCrawlStatus updates a crawl's lifecycle status.
func (db *DB) SetCrawlStatus(ctx context.Context, id, status string) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE crawl_tasks SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set crawl status: %w", err)
	}
	return nil
}

// UpdateCrawlProgress records how many pages were fetched and URLs found.
func (db *DB) UpdateCrawlProgress(ctx context.Context, id string, pages, found int) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE crawl_tasks SET pages = ?, found = ?, updated_at = ? WHERE id = ?`,
		pages, found, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("update crawl progress: %w", err)
	}
	return nil
}

// PrepareCrawlRun resets a crawl for a (re-)run: clears its discovered URLs and
// zeroes counters, marking it running.
func (db *DB) PrepareCrawlRun(ctx context.Context, id string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM crawl_urls WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("clear crawl urls: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE crawl_tasks SET pages = 0, found = 0, status = 'running', updated_at = ? WHERE id = ?`,
		time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("prepare crawl run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// AddCrawlURL records a discovered URL, deduped per task by url. It reports
// whether the URL was newly inserted.
func (db *DB) AddCrawlURL(ctx context.Context, u *models.CrawlURL) (bool, error) {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	res, err := db.sql.ExecContext(ctx,
		`INSERT OR IGNORE INTO crawl_urls
		   (id, task_id, url, method, status_code, length, content_type, depth, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		u.ID, u.TaskID, u.URL, u.Method, u.StatusCode, u.Length, u.ContentType, u.Depth,
		u.CreatedAt.Format(timeLayout))
	if err != nil {
		return false, fmt.Errorf("add crawl url: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListCrawlURLs returns a crawl's discovered URLs ordered by discovery.
func (db *DB) ListCrawlURLs(ctx context.Context, taskID string) ([]*models.CrawlURL, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, task_id, url, method, status_code, length, content_type, depth, created_at
		 FROM crawl_urls WHERE task_id = ? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list crawl urls: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.CrawlURL
	for rows.Next() {
		u := &models.CrawlURL{}
		var created string
		if err := rows.Scan(&u.ID, &u.TaskID, &u.URL, &u.Method, &u.StatusCode, &u.Length,
			&u.ContentType, &u.Depth, &created); err != nil {
			return nil, fmt.Errorf("scan crawl url: %w", err)
		}
		u.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, u)
	}
	return out, rows.Err()
}

// reconcileRunningCrawls marks any crawl left 'running' (e.g. by a crash) as
// 'stopped'. Called once on startup.
func (db *DB) reconcileRunningCrawls() error {
	_, err := db.sql.ExecContext(context.Background(),
		`UPDATE crawl_tasks SET status = ?, updated_at = ? WHERE status = ?`,
		"stopped", time.Now().Format(timeLayout), "running")
	if err != nil {
		return fmt.Errorf("reconcile running crawls: %w", err)
	}
	return nil
}
