CREATE TABLE IF NOT EXISTS repeater_tabs (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    scheme           TEXT NOT NULL,
    host             TEXT NOT NULL,
    raw              BLOB,
    follow_redirects INTEGER NOT NULL DEFAULT 0,
    http_version     TEXT NOT NULL DEFAULT 'HTTP/1.1',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repeater_history (
    id           TEXT PRIMARY KEY,
    tab_id       TEXT NOT NULL REFERENCES repeater_tabs(id) ON DELETE CASCADE,
    request_raw  BLOB,
    response_raw BLOB,
    status_code  INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_repeater_history_tab ON repeater_history(tab_id, created_at);

CREATE TABLE IF NOT EXISTS sitemap_notes (
    id         TEXT PRIMARY KEY,
    host       TEXT NOT NULL,
    path       TEXT NOT NULL,
    note       TEXT,
    tags       TEXT,
    updated_at TEXT NOT NULL,
    UNIQUE(host, path)
);
