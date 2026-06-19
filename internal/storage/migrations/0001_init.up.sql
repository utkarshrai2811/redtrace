CREATE TABLE IF NOT EXISTS projects (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    target     TEXT,
    notes      TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS requests (
    id           TEXT PRIMARY KEY,
    project_id   TEXT REFERENCES projects(id) ON DELETE SET NULL,
    timestamp    TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'proxy',
    method       TEXT NOT NULL,
    scheme       TEXT NOT NULL,
    host         TEXT NOT NULL,
    port         INTEGER NOT NULL,
    path         TEXT NOT NULL,
    query        TEXT,
    url          TEXT NOT NULL,
    http_version TEXT,
    content_type TEXT,
    body_size    INTEGER NOT NULL DEFAULT 0,
    in_scope     INTEGER NOT NULL DEFAULT 0,
    raw          BLOB
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_requests_host ON requests(host);
CREATE INDEX IF NOT EXISTS idx_requests_method ON requests(method);

CREATE TABLE IF NOT EXISTS responses (
    id           TEXT PRIMARY KEY,
    request_id   TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    timestamp    TEXT NOT NULL,
    status_code  INTEGER NOT NULL,
    reason       TEXT,
    http_version TEXT,
    mime_type    TEXT,
    body_size    INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    raw          BLOB
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_responses_request ON responses(request_id);

CREATE TABLE IF NOT EXISTS findings (
    id          TEXT PRIMARY KEY,
    request_id  TEXT REFERENCES requests(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    severity    TEXT NOT NULL,
    confidence  TEXT,
    description TEXT,
    evidence    TEXT,
    remediation TEXT,
    status      TEXT NOT NULL DEFAULT 'new',
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    data       BLOB
);

CREATE TABLE IF NOT EXISTS intercept_rules (
    id         TEXT PRIMARY KEY,
    enabled    INTEGER NOT NULL DEFAULT 1,
    name       TEXT,
    part       TEXT NOT NULL,
    match_type TEXT NOT NULL,
    match      TEXT NOT NULL,
    replace    TEXT,
    priority   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scope_rules (
    id         TEXT PRIMARY KEY,
    enabled    INTEGER NOT NULL DEFAULT 1,
    kind       TEXT NOT NULL,
    matcher    TEXT NOT NULL,
    value      TEXT NOT NULL,
    created_at TEXT NOT NULL
);
