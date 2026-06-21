package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// CreateOOBPayload records a generated payload.
func (db *DB) CreateOOBPayload(ctx context.Context, p *models.OOBPayload) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO oob_payloads (token, host, created_at) VALUES (?,?,?)`,
		p.Token, p.Host, p.CreatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("create oob payload: %w", err)
	}
	return nil
}

// ListOOBPayloads returns payloads newest first, each with its interaction count.
func (db *DB) ListOOBPayloads(ctx context.Context) ([]*models.OOBPayload, map[string]int, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT token, host, created_at FROM oob_payloads ORDER BY created_at DESC`)
	if err != nil {
		return nil, nil, fmt.Errorf("list oob payloads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.OOBPayload
	for rows.Next() {
		p := &models.OOBPayload{}
		var created string
		if err := rows.Scan(&p.Token, &p.Host, &created); err != nil {
			return nil, nil, fmt.Errorf("scan oob payload: %w", err)
		}
		p.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	counts := map[string]int{}
	crows, err := db.sql.QueryContext(ctx,
		`SELECT token, COUNT(*) FROM oob_interactions WHERE token != '' GROUP BY token`)
	if err != nil {
		return nil, nil, fmt.Errorf("count oob interactions: %w", err)
	}
	defer func() { _ = crows.Close() }()
	for crows.Next() {
		var tok string
		var n int
		if err := crows.Scan(&tok, &n); err != nil {
			return nil, nil, fmt.Errorf("scan oob count: %w", err)
		}
		counts[tok] = n
	}
	return out, counts, crows.Err()
}

// AddOOBInteraction records a captured callback.
func (db *DB) AddOOBInteraction(ctx context.Context, i *models.OOBInteraction) error {
	if i.CreatedAt.IsZero() {
		i.CreatedAt = time.Now()
	}
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO oob_interactions (id, token, protocol, source_ip, query, detail, raw, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		i.ID, i.Token, i.Protocol, i.SourceIP, i.Query, i.Detail, i.Raw, i.CreatedAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("add oob interaction: %w", err)
	}
	return nil
}

// ListOOBInteractions returns interactions newest first (without raw bytes),
// optionally filtered to a single token.
func (db *DB) ListOOBInteractions(ctx context.Context, token string) ([]*models.OOBInteraction, error) {
	q := `SELECT id, token, protocol, source_ip, query, detail, created_at FROM oob_interactions`
	args := []any{}
	if token != "" {
		q += ` WHERE token = ?`
		args = append(args, token)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := db.sql.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list oob interactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*models.OOBInteraction
	for rows.Next() {
		i := &models.OOBInteraction{}
		var created string
		if err := rows.Scan(&i.ID, &i.Token, &i.Protocol, &i.SourceIP, &i.Query, &i.Detail, &created); err != nil {
			return nil, fmt.Errorf("scan oob interaction: %w", err)
		}
		i.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, i)
	}
	return out, rows.Err()
}

// GetOOBInteraction returns one interaction with its raw bytes.
func (db *DB) GetOOBInteraction(ctx context.Context, id string) (*models.OOBInteraction, error) {
	i := &models.OOBInteraction{}
	var created string
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, token, protocol, source_ip, query, detail, raw, created_at FROM oob_interactions WHERE id = ?`, id).
		Scan(&i.ID, &i.Token, &i.Protocol, &i.SourceIP, &i.Query, &i.Detail, &i.Raw, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get oob interaction: %w", err)
	}
	i.CreatedAt, _ = time.Parse(timeLayout, created)
	return i, nil
}

// ClearOOBInteractions removes all captured interactions.
func (db *DB) ClearOOBInteractions(ctx context.Context) error {
	_, err := db.sql.ExecContext(ctx, `DELETE FROM oob_interactions`)
	if err != nil {
		return fmt.Errorf("clear oob interactions: %w", err)
	}
	return nil
}
