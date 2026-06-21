CREATE TABLE IF NOT EXISTS oob_payloads (
    token      TEXT PRIMARY KEY,
    host       TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS oob_interactions (
    id         TEXT PRIMARY KEY,
    token      TEXT NOT NULL DEFAULT '',
    protocol   TEXT NOT NULL,
    source_ip  TEXT NOT NULL DEFAULT '',
    query      TEXT NOT NULL DEFAULT '',
    detail     TEXT NOT NULL DEFAULT '',
    raw        BLOB,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oob_interactions_token ON oob_interactions(token, created_at);
CREATE INDEX IF NOT EXISTS idx_oob_interactions_created ON oob_interactions(created_at);
