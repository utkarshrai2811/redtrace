package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateScanTask inserts a new active-scan task in the pending state.
func (db *DB) CreateScanTask(ctx context.Context, t *models.ScanTask) error {
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
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO scan_tasks
		   (id, name, scheme, host, template, http_version, follow_redirects, status, total, completed, issues, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Scheme, t.Host, t.Template, t.HTTPVersion, t.FollowRedirects,
		t.Status, t.Total, t.Completed, t.Issues, t.CreatedAt.Format(timeLayout), t.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create scan task: %w", err)
	}
	return nil
}

// ListScanTasks returns scan tasks newest first (without their template blob).
func (db *DB) ListScanTasks(ctx context.Context) ([]*models.ScanTask, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, scheme, host, http_version, follow_redirects, status, total, completed, issues, created_at, updated_at
		 FROM scan_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list scan tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.ScanTask
	for rows.Next() {
		t := &models.ScanTask{}
		var created, updated string
		if err := rows.Scan(&t.ID, &t.Name, &t.Scheme, &t.Host, &t.HTTPVersion, &t.FollowRedirects,
			&t.Status, &t.Total, &t.Completed, &t.Issues, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan scan task: %w", err)
		}
		t.CreatedAt, _ = time.Parse(timeLayout, created)
		t.UpdatedAt, _ = time.Parse(timeLayout, updated)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetScanTask returns a single task with its template, or ErrNotFound.
func (db *DB) GetScanTask(ctx context.Context, id string) (*models.ScanTask, error) {
	t := &models.ScanTask{}
	var created, updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, name, scheme, host, template, http_version, follow_redirects, status, total, completed, issues, created_at, updated_at
		 FROM scan_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Scheme, &t.Host, &t.Template, &t.HTTPVersion, &t.FollowRedirects,
			&t.Status, &t.Total, &t.Completed, &t.Issues, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get scan task: %w", err)
	}
	t.CreatedAt, _ = time.Parse(timeLayout, created)
	t.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return t, nil
}

// UpdateScanTask updates a draft task's editable fields.
func (db *DB) UpdateScanTask(ctx context.Context, t *models.ScanTask) error {
	t.UpdatedAt = time.Now()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE scan_tasks SET name=?, scheme=?, host=?, template=?, http_version=?, follow_redirects=?, updated_at=?
		 WHERE id=?`,
		t.Name, t.Scheme, t.Host, t.Template, t.HTTPVersion, t.FollowRedirects,
		t.UpdatedAt.Format(timeLayout), t.ID)
	if err != nil {
		return fmt.Errorf("update scan task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteScanTask removes a scan task. Its findings (evidence) are kept.
func (db *DB) DeleteScanTask(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx, `DELETE FROM scan_tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete scan task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetScanStatus updates a task's lifecycle status.
func (db *DB) SetScanStatus(ctx context.Context, id, status string) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE scan_tasks SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set scan status: %w", err)
	}
	return nil
}

// UpdateScanProgress records how many requests have completed and issues found.
func (db *DB) UpdateScanProgress(ctx context.Context, id string, completed, issues int) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE scan_tasks SET completed = ?, issues = ?, updated_at = ? WHERE id = ?`,
		completed, issues, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("update scan progress: %w", err)
	}
	return nil
}

// PrepareScanRun resets a task for a (re-)run: it clears that task's prior
// findings and sets the total, zeroes counters, and marks it running.
func (db *DB) PrepareScanRun(ctx context.Context, id string, total int) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scan_issues WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("clear scan issues: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE scan_tasks SET total = ?, completed = 0, issues = 0, status = 'running', updated_at = ? WHERE id = ?`,
		total, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("prepare scan run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// AddScanIssue records a finding, deduped by fingerprint. It reports whether the
// issue was newly inserted (false means an identical finding already existed).
func (db *DB) AddScanIssue(ctx context.Context, is *models.ScanIssue) (bool, error) {
	if is.CreatedAt.IsZero() {
		is.CreatedAt = time.Now()
	}
	res, err := db.sql.ExecContext(ctx,
		`INSERT OR IGNORE INTO scan_issues
		   (id, task_id, type, name, severity, confidence, scheme, host, port, path, method,
		    param, payload, detail, evidence, remediation, origin, fingerprint, request_raw, response_raw, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		is.ID, nullString(is.TaskID), is.Type, is.Name, is.Severity, is.Confidence, is.Scheme, is.Host,
		is.Port, is.Path, is.Method, is.Param, is.Payload, is.Detail, is.Evidence, is.Remediation,
		is.Origin, is.Fingerprint, is.RequestRaw, is.ResponseRaw, is.CreatedAt.Format(timeLayout))
	if err != nil {
		return false, fmt.Errorf("add scan issue: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListScanIssues returns findings newest first (without raw bytes).
func (db *DB) ListScanIssues(ctx context.Context) ([]*models.ScanIssue, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, task_id, type, name, severity, confidence, scheme, host, port, path, method,
		        param, payload, detail, evidence, remediation, origin, created_at
		 FROM scan_issues ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list scan issues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.ScanIssue
	for rows.Next() {
		is := &models.ScanIssue{}
		var taskID sql.NullString
		var created string
		if err := rows.Scan(&is.ID, &taskID, &is.Type, &is.Name, &is.Severity, &is.Confidence,
			&is.Scheme, &is.Host, &is.Port, &is.Path, &is.Method, &is.Param, &is.Payload,
			&is.Detail, &is.Evidence, &is.Remediation, &is.Origin, &created); err != nil {
			return nil, fmt.Errorf("scan scan issue: %w", err)
		}
		is.TaskID = taskID.String
		is.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, is)
	}
	return out, rows.Err()
}

// ListScanIssuesByTask returns one task's findings (without raw bytes), using
// the task_id index instead of scanning the whole table.
func (db *DB) ListScanIssuesByTask(ctx context.Context, taskID string) ([]*models.ScanIssue, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, task_id, type, name, severity, confidence, scheme, host, port, path, method,
		        param, payload, detail, evidence, remediation, origin, created_at
		 FROM scan_issues WHERE task_id = ? ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list scan issues by task: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.ScanIssue
	for rows.Next() {
		is := &models.ScanIssue{}
		var tID sql.NullString
		var created string
		if err := rows.Scan(&is.ID, &tID, &is.Type, &is.Name, &is.Severity, &is.Confidence,
			&is.Scheme, &is.Host, &is.Port, &is.Path, &is.Method, &is.Param, &is.Payload,
			&is.Detail, &is.Evidence, &is.Remediation, &is.Origin, &created); err != nil {
			return nil, fmt.Errorf("scan scan issue: %w", err)
		}
		is.TaskID = tID.String
		is.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, is)
	}
	return out, rows.Err()
}

// GetScanIssue returns one finding with its raw request/response bytes.
func (db *DB) GetScanIssue(ctx context.Context, id string) (*models.ScanIssue, error) {
	is := &models.ScanIssue{}
	var taskID sql.NullString
	var created string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, task_id, type, name, severity, confidence, scheme, host, port, path, method,
		        param, payload, detail, evidence, remediation, origin, request_raw, response_raw, created_at
		 FROM scan_issues WHERE id = ?`, id).
		Scan(&is.ID, &taskID, &is.Type, &is.Name, &is.Severity, &is.Confidence, &is.Scheme, &is.Host,
			&is.Port, &is.Path, &is.Method, &is.Param, &is.Payload, &is.Detail, &is.Evidence,
			&is.Remediation, &is.Origin, &is.RequestRaw, &is.ResponseRaw, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get scan issue: %w", err)
	}
	is.TaskID = taskID.String
	is.CreatedAt, _ = time.Parse(timeLayout, created)
	return is, nil
}

// ClearScanIssues removes all findings and resets every task's issue counter so
// the persisted counts do not outlive the findings they counted.
func (db *DB) ClearScanIssues(ctx context.Context) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scan_issues`); err != nil {
		return fmt.Errorf("clear scan issues: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE scan_tasks SET issues = 0`); err != nil {
		return fmt.Errorf("reset scan issue counts: %w", err)
	}
	return tx.Commit()
}

// reconcileRunningScans marks any scan task left 'running' (e.g. by a crash with
// no live runner) as 'stopped'. Called once on startup.
func (db *DB) reconcileRunningScans() error {
	_, err := db.sql.ExecContext(context.Background(),
		`UPDATE scan_tasks SET status = ?, updated_at = ? WHERE status = ?`,
		"stopped", time.Now().Format(timeLayout), "running")
	if err != nil {
		return fmt.Errorf("reconcile running scans: %w", err)
	}
	return nil
}
