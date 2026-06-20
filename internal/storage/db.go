// Package storage is the persistence layer for RedTrace, backed by an embedded
// pure-Go SQLite database (modernc.org/sqlite) with golang-migrate migrations.
package storage

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	migsqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the SQLite connection pool and exposes RedTrace's storage operations.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies all
// pending migrations. Use ":memory:" for an ephemeral database.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dsnFor(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// An in-memory database is private per connection, so the pool must use a
	// single connection — otherwise migrations run on one connection and other
	// queries hit a fresh, schema-less database ("no such table").
	if path == ":memory:" {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrateUp(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return &DB{sql: sqlDB}, nil
}

// dsnFor builds the SQLite DSN, URL-escaping a filesystem path so reserved
// characters (?, #, &) in it cannot leak into the DSN query portion and silently
// drop pragmas or relocate the database file.
func dsnFor(path string) string {
	const params = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	if path == ":memory:" {
		return "file::memory:?" + params
	}
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + params
}

// Close closes the underlying connection pool.
func (db *DB) Close() error {
	return db.sql.Close()
}

func migrateUp(sqlDB *sql.DB) error {
	driver, err := migsqlite.WithInstance(sqlDB, &migsqlite.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrate source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// NewID returns a random RFC 4122 version 4 UUID string.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should not fail; fall back to a zero-prefixed value rather
		// than panicking in library code.
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	var s [36]byte
	hex.Encode(s[0:8], b[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], b[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], b[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], b[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], b[10:16])
	return string(s[:])
}
