CREATE TABLE IF NOT EXISTS intruder_attacks (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    scheme      TEXT NOT NULL,
    host        TEXT NOT NULL,
    template    BLOB,
    attack_type TEXT NOT NULL,
    config      TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    total       INTEGER NOT NULL DEFAULT 0,
    completed   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS intruder_results (
    id           TEXT PRIMARY KEY,
    attack_id    TEXT NOT NULL REFERENCES intruder_attacks(id) ON DELETE CASCADE,
    idx          INTEGER NOT NULL,
    payloads     TEXT NOT NULL,
    status_code  INTEGER NOT NULL DEFAULT 0,
    length       INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    request_raw  BLOB,
    response_raw BLOB,
    error        TEXT,
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_intruder_results_attack ON intruder_results(attack_id, idx);
