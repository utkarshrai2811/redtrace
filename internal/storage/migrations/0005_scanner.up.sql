CREATE TABLE IF NOT EXISTS scan_tasks (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    scheme          TEXT NOT NULL,
    host            TEXT NOT NULL,
    template        BLOB,
    http_version    TEXT NOT NULL DEFAULT 'HTTP/1.1',
    follow_redirects INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending',
    total           INTEGER NOT NULL DEFAULT 0,
    completed       INTEGER NOT NULL DEFAULT 0,
    issues          INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_issues (
    id           TEXT PRIMARY KEY,
    -- The active scan that found it (NULL for passive findings). Not a foreign
    -- key: a finding is evidence and outlives the task that produced it.
    task_id      TEXT,
    type         TEXT NOT NULL,
    name         TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT 'info',
    confidence   TEXT NOT NULL DEFAULT 'tentative',
    scheme       TEXT NOT NULL DEFAULT '',
    host         TEXT NOT NULL DEFAULT '',
    port         INTEGER NOT NULL DEFAULT 0,
    path         TEXT NOT NULL DEFAULT '',
    method       TEXT NOT NULL DEFAULT '',
    param        TEXT NOT NULL DEFAULT '',
    payload      TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    evidence     TEXT NOT NULL DEFAULT '',
    remediation  TEXT NOT NULL DEFAULT '',
    origin       TEXT NOT NULL DEFAULT 'passive',
    -- Stable per-issue identity; a unique index dedupes so the same finding is
    -- not recorded on every request to an endpoint.
    fingerprint  TEXT NOT NULL,
    request_raw  BLOB,
    response_raw BLOB,
    created_at   TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_issues_fingerprint ON scan_issues(fingerprint);
CREATE INDEX IF NOT EXISTS idx_scan_issues_host ON scan_issues(host);
CREATE INDEX IF NOT EXISTS idx_scan_issues_task ON scan_issues(task_id);
CREATE INDEX IF NOT EXISTS idx_scan_issues_created ON scan_issues(created_at);
