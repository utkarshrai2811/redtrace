package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ftsFieldLimit bounds how many bytes of each field are indexed for search,
// keeping trigram index growth predictable on large bodies.
const ftsFieldLimit = 64 * 1024

const timeLayout = time.RFC3339Nano

// Query bases for ListRequests. These are constant; the dynamic WHERE clause is
// assembled from a fixed allowlist of predicates and all user-supplied values
// are passed as bound parameters (never concatenated).
const (
	listCountBase  = `SELECT COUNT(*) FROM requests r LEFT JOIN responses resp ON resp.request_id = r.id `
	listSelectBase = `SELECT r.id, r.timestamp, r.source, r.method, r.scheme, r.host, r.port, r.path, r.url, r.in_scope,
	                         COALESCE(resp.status_code, 0), COALESCE(resp.body_size, 0), COALESCE(resp.mime_type, ''),
	                         COALESCE(resp.duration_ms, 0), (resp.id IS NOT NULL)
	                  FROM requests r LEFT JOIN responses resp ON resp.request_id = r.id `
	listOrderTail = ` ORDER BY r.timestamp DESC LIMIT ? OFFSET ?`
)

// RequestFilter constrains a ListRequests query. Zero-valued fields are ignored.
type RequestFilter struct {
	Method      string
	Host        string
	MimeType    string
	Contains    string
	StatusMin   int
	StatusMax   int
	InScopeOnly bool
	Limit       int
	Offset      int
}

// RequestSummary is a row in the proxy history table: request metadata joined
// with the headline fields of its response (zero values when absent).
type RequestSummary struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Source         string    `json:"source"`
	Method         string    `json:"method"`
	Scheme         string    `json:"scheme"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Path           string    `json:"path"`
	URL            string    `json:"url"`
	InScope        bool      `json:"inScope"`
	StatusCode     int       `json:"statusCode"`
	ResponseLength int       `json:"responseLength"`
	MimeType       string    `json:"mimeType"`
	DurationMs     int64     `json:"durationMs"`
	HasResponse    bool      `json:"hasResponse"`
}

// StoreExchange persists a request and (optionally) its response, and indexes
// the exchange for search. IDs on the models must already be set.
func (db *DB) StoreExchange(ctx context.Context, ex *models.Exchange) error {
	if ex == nil || ex.Request == nil {
		return errors.New("storage: nil request")
	}
	r := ex.Request

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO requests
		   (id, project_id, timestamp, source, method, scheme, host, port, path, query, url, http_version, content_type, body_size, in_scope, raw)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, nullString(r.ProjectID), r.Timestamp.Format(timeLayout), string(r.Source),
		r.Method, r.Scheme, r.Host, r.Port, r.Path, r.Query, r.URL, r.HTTPVersion,
		r.ContentType, r.BodySize, boolToInt(r.InScope), r.Raw,
	)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}

	var respRaw []byte
	if resp := ex.Response; resp != nil {
		respRaw = resp.Raw
		_, err = tx.ExecContext(ctx,
			`INSERT INTO responses
			   (id, request_id, timestamp, status_code, reason, http_version, mime_type, body_size, duration_ms, raw)
			 VALUES (?,?,?,?,?,?,?,?,?,?)`,
			resp.ID, r.ID, resp.Timestamp.Format(timeLayout), resp.StatusCode, resp.Reason,
			resp.HTTPVersion, resp.MimeType, resp.BodySize, resp.DurationMs, resp.Raw,
		)
		if err != nil {
			return fmt.Errorf("insert response: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO search_index (request_id, url, request_raw, response_raw) VALUES (?,?,?,?)`,
		r.ID, r.URL, truncate(r.Raw), truncate(respRaw),
	)
	if err != nil {
		return fmt.Errorf("index exchange: %w", err)
	}

	return tx.Commit()
}

// ListRequests returns a filtered, paginated page of history rows together with
// the total number of rows matching the filter (ignoring pagination).
func (db *DB) ListRequests(ctx context.Context, f RequestFilter) ([]RequestSummary, int, error) {
	where, args := buildFilter(f)

	var total int
	countQ := listCountBase + where //nolint:gosec // G202: WHERE is constant fragments; values are bound parameters
	if err := db.sql.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count requests: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	dataQ := listSelectBase + where + listOrderTail //nolint:gosec // G202: WHERE is constant fragments; values are bound parameters
	dataArgs := append(append([]any{}, args...), limit, f.Offset)

	rows, err := db.sql.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []RequestSummary
	for rows.Next() {
		var s RequestSummary
		var ts string
		if err := rows.Scan(&s.ID, &ts, &s.Source, &s.Method, &s.Scheme, &s.Host, &s.Port,
			&s.Path, &s.URL, &s.InScope, &s.StatusCode, &s.ResponseLength, &s.MimeType,
			&s.DurationMs, &s.HasResponse); err != nil {
			return nil, 0, fmt.Errorf("scan request: %w", err)
		}
		s.Timestamp, _ = time.Parse(timeLayout, ts)
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// GetExchange returns a single request with its response (if any).
func (db *DB) GetExchange(ctx context.Context, id string) (*models.Exchange, error) {
	r := &models.Request{}
	var ts string
	var src string
	var projectID sql.NullString
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, project_id, timestamp, source, method, scheme, host, port, path, query, url, http_version, content_type, body_size, in_scope, raw
		 FROM requests WHERE id = ?`, id).
		Scan(&r.ID, &projectID, &ts, &src, &r.Method, &r.Scheme, &r.Host, &r.Port, &r.Path,
			&r.Query, &r.URL, &r.HTTPVersion, &r.ContentType, &r.BodySize, &r.InScope, &r.Raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	r.ProjectID = projectID.String
	r.Source = models.Source(src)
	r.Timestamp, _ = time.Parse(timeLayout, ts)

	ex := &models.Exchange{Request: r}

	resp := &models.Response{}
	var rts string
	err = db.sql.QueryRowContext(ctx,
		`SELECT id, request_id, timestamp, status_code, reason, http_version, mime_type, body_size, duration_ms, raw
		 FROM responses WHERE request_id = ?`, id).
		Scan(&resp.ID, &resp.RequestID, &rts, &resp.StatusCode, &resp.Reason, &resp.HTTPVersion,
			&resp.MimeType, &resp.BodySize, &resp.DurationMs, &resp.Raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// no response captured
	case err != nil:
		return nil, fmt.Errorf("get response: %w", err)
	default:
		resp.Timestamp, _ = time.Parse(timeLayout, rts)
		ex.Response = resp
	}

	return ex, nil
}

// DeleteRequest removes a request, its response (via cascade), and its search
// index entry. It returns ErrNotFound if no such request exists.
func (db *DB) DeleteRequest(ctx context.Context, id string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM requests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_index WHERE request_id = ?`, id); err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	return tx.Commit()
}

// Hosts returns the distinct hosts observed, ordered alphabetically. Useful for
// populating the site map and host filter.
func (db *DB) Hosts(ctx context.Context) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT DISTINCT host FROM requests ORDER BY host`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var hosts []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func buildFilter(f RequestFilter) (string, []any) {
	var conds []string
	var args []any

	if f.Method != "" {
		conds = append(conds, "r.method = ?")
		args = append(args, strings.ToUpper(f.Method))
	}
	if f.Host != "" {
		conds = append(conds, "r.host = ?")
		args = append(args, f.Host)
	}
	if f.MimeType != "" {
		conds = append(conds, "resp.mime_type LIKE '%' || ? || '%'")
		args = append(args, f.MimeType)
	}
	if f.StatusMin > 0 {
		conds = append(conds, "resp.status_code >= ?")
		args = append(args, f.StatusMin)
	}
	if f.StatusMax > 0 {
		conds = append(conds, "resp.status_code <= ?")
		args = append(args, f.StatusMax)
	}
	if f.InScopeOnly {
		conds = append(conds, "r.in_scope = 1")
	}
	if c := strings.TrimSpace(f.Contains); c != "" {
		if len(c) >= 3 {
			// trigram FTS handles arbitrary substring matches >= 3 chars.
			conds = append(conds, "r.id IN (SELECT request_id FROM search_index WHERE search_index MATCH ?)")
			args = append(args, `"`+strings.ReplaceAll(c, `"`, `""`)+`"`)
		} else {
			conds = append(conds, "r.url LIKE '%' || ? || '%'")
			args = append(args, c)
		}
	}

	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func truncate(b []byte) []byte {
	if len(b) > ftsFieldLimit {
		return b[:ftsFieldLimit]
	}
	return b
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
