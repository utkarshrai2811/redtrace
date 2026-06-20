package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateIntruderAttack inserts a new attack in the pending state.
func (db *DB) CreateIntruderAttack(ctx context.Context, a *models.IntruderAttack) error {
	now := time.Now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "pending"
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO intruder_attacks
		   (id, name, scheme, host, template, attack_type, config, status, total, completed, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.Scheme, a.Host, a.Template, a.AttackType, a.Config, a.Status,
		a.Total, a.Completed, a.CreatedAt.Format(timeLayout), a.UpdatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create intruder attack: %w", err)
	}
	return nil
}

// ListIntruderAttacks returns attacks newest first (without their template/config
// blobs, which the detail endpoint loads).
func (db *DB) ListIntruderAttacks(ctx context.Context) ([]*models.IntruderAttack, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, name, scheme, host, attack_type, status, total, completed, created_at, updated_at
		 FROM intruder_attacks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list intruder attacks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.IntruderAttack
	for rows.Next() {
		a := &models.IntruderAttack{}
		var created, updated string
		if err := rows.Scan(&a.ID, &a.Name, &a.Scheme, &a.Host, &a.AttackType, &a.Status,
			&a.Total, &a.Completed, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan intruder attack: %w", err)
		}
		a.CreatedAt, _ = time.Parse(timeLayout, created)
		a.UpdatedAt, _ = time.Parse(timeLayout, updated)
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetIntruderAttack returns a single attack with its template and config, or
// ErrNotFound.
func (db *DB) GetIntruderAttack(ctx context.Context, id string) (*models.IntruderAttack, error) {
	a := &models.IntruderAttack{}
	var created, updated string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, name, scheme, host, template, attack_type, config, status, total, completed, created_at, updated_at
		 FROM intruder_attacks WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.Scheme, &a.Host, &a.Template, &a.AttackType, &a.Config,
			&a.Status, &a.Total, &a.Completed, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get intruder attack: %w", err)
	}
	a.CreatedAt, _ = time.Parse(timeLayout, created)
	a.UpdatedAt, _ = time.Parse(timeLayout, updated)
	return a, nil
}

// UpdateIntruderAttack updates a draft attack's editable fields.
func (db *DB) UpdateIntruderAttack(ctx context.Context, a *models.IntruderAttack) error {
	a.UpdatedAt = time.Now()
	res, err := db.sql.ExecContext(ctx,
		`UPDATE intruder_attacks SET name=?, scheme=?, host=?, template=?, attack_type=?, config=?, updated_at=?
		 WHERE id=?`,
		a.Name, a.Scheme, a.Host, a.Template, a.AttackType, a.Config,
		a.UpdatedAt.Format(timeLayout), a.ID)
	if err != nil {
		return fmt.Errorf("update intruder attack: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteIntruderAttack removes an attack and its results (cascade).
func (db *DB) DeleteIntruderAttack(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx, `DELETE FROM intruder_attacks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete intruder attack: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetIntruderStatus updates an attack's lifecycle status.
func (db *DB) SetIntruderStatus(ctx context.Context, id, status string) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE intruder_attacks SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set intruder status: %w", err)
	}
	return nil
}

// UpdateIntruderProgress records how many requests have completed.
func (db *DB) UpdateIntruderProgress(ctx context.Context, id string, completed int) error {
	_, err := db.sql.ExecContext(ctx,
		`UPDATE intruder_attacks SET completed = ?, updated_at = ? WHERE id = ?`,
		completed, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("update intruder progress: %w", err)
	}
	return nil
}

// reconcileRunningAttacks marks any attack left 'running' (e.g. by a crash or
// hard kill with no live runner) as 'stopped'. Called once on startup.
func (db *DB) reconcileRunningAttacks() error {
	_, err := db.sql.ExecContext(context.Background(),
		`UPDATE intruder_attacks SET status = ?, updated_at = ? WHERE status = ?`,
		"stopped", time.Now().Format(timeLayout), "running")
	if err != nil {
		return fmt.Errorf("reconcile running attacks: %w", err)
	}
	return nil
}

// PrepareIntruderRun resets an attack for a (re-)run: it clears any prior
// results and sets the total, zeroes completed, and marks it running.
func (db *DB) PrepareIntruderRun(ctx context.Context, id string, total int) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM intruder_results WHERE attack_id = ?`, id); err != nil {
		return fmt.Errorf("clear intruder results: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE intruder_attacks SET total = ?, completed = 0, status = 'running', updated_at = ? WHERE id = ?`,
		total, time.Now().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("prepare intruder run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// AddIntruderResult records the outcome of one generated request.
func (db *DB) AddIntruderResult(ctx context.Context, r *models.IntruderResult) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO intruder_results
		   (id, attack_id, idx, payloads, status_code, length, duration_ms, request_raw, response_raw, error, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.AttackID, r.Index, r.Payloads, r.StatusCode, r.Length, r.DurationMs,
		r.RequestRaw, r.ResponseRaw, nullString(r.Error), r.CreatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("add intruder result: %w", err)
	}
	return nil
}

// ListIntruderResults returns an attack's results ordered by request index.
func (db *DB) ListIntruderResults(ctx context.Context, attackID string) ([]*models.IntruderResult, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, attack_id, idx, payloads, status_code, length, duration_ms, error, created_at
		 FROM intruder_results WHERE attack_id = ? ORDER BY idx`, attackID)
	if err != nil {
		return nil, fmt.Errorf("list intruder results: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.IntruderResult
	for rows.Next() {
		r := &models.IntruderResult{}
		var errStr sql.NullString
		var created string
		if err := rows.Scan(&r.ID, &r.AttackID, &r.Index, &r.Payloads, &r.StatusCode,
			&r.Length, &r.DurationMs, &errStr, &created); err != nil {
			return nil, fmt.Errorf("scan intruder result: %w", err)
		}
		r.Error = errStr.String
		r.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetIntruderResult returns one result with its raw request/response bytes.
func (db *DB) GetIntruderResult(ctx context.Context, id string) (*models.IntruderResult, error) {
	r := &models.IntruderResult{}
	var errStr sql.NullString
	var created string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, attack_id, idx, payloads, status_code, length, duration_ms, request_raw, response_raw, error, created_at
		 FROM intruder_results WHERE id = ?`, id).
		Scan(&r.ID, &r.AttackID, &r.Index, &r.Payloads, &r.StatusCode, &r.Length,
			&r.DurationMs, &r.RequestRaw, &r.ResponseRaw, &errStr, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get intruder result: %w", err)
	}
	r.Error = errStr.String
	r.CreatedAt, _ = time.Parse(timeLayout, created)
	return r, nil
}
