CREATE TABLE IF NOT EXISTS crawl_tasks (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    seed        TEXT NOT NULL,
    scheme      TEXT NOT NULL,
    host        TEXT NOT NULL,
    max_depth   INTEGER NOT NULL DEFAULT 3,
    max_pages   INTEGER NOT NULL DEFAULT 200,
    status      TEXT NOT NULL DEFAULT 'pending',
    pages       INTEGER NOT NULL DEFAULT 0,
    found       INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS crawl_urls (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL,
    url          TEXT NOT NULL,
    method       TEXT NOT NULL DEFAULT 'GET',
    status_code  INTEGER NOT NULL DEFAULT 0,
    length       INTEGER NOT NULL DEFAULT 0,
    content_type TEXT NOT NULL DEFAULT '',
    depth        INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_crawl_urls_task ON crawl_urls(task_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crawl_urls_task_url ON crawl_urls(task_id, url);

CREATE TABLE IF NOT EXISTS sequencer_tasks (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    scheme       TEXT NOT NULL DEFAULT '',
    host         TEXT NOT NULL DEFAULT '',
    template     BLOB,
    http_version TEXT NOT NULL DEFAULT 'HTTP/1.1',
    source       TEXT NOT NULL DEFAULT 'cookie',
    selector     TEXT NOT NULL DEFAULT '',
    target       INTEGER NOT NULL DEFAULT 200,
    status       TEXT NOT NULL DEFAULT 'pending',
    collected    INTEGER NOT NULL DEFAULT 0,
    tokens       TEXT NOT NULL DEFAULT '[]',
    report       TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
