# Phase 5 — Crawler & Sequencer

Two tools: a spider that maps a target, and a token-randomness analyzer.

## Crawler (`internal/crawler`)

A scope-bounded web spider. From a seed URL it fetches pages through the
Repeater transport, extracts links (`href`/`src`/`action`), resolves them
against the page URL, and follows the **same-host, in-scope** ones
breadth-first, level by level, up to a depth and page budget.

- **Bounds:** `maxDepth` (default 3, capped 10) and `maxPages` (default 200,
  capped 2000); links are followed only while in scope (`scope.InScope`) and on
  the seed's host. Redirect `Location` targets are treated as discovered links.
- **Integration:** every fetched page is stored as a normal exchange
  (`source = crawler`), so it appears in the request history and the Target site
  map and can be sent to Repeater/Intruder/Scanner; it is also passively scanned.
  Per crawl, discovered URLs are recorded in `crawl_urls` for the crawler view.
- **Runner:** a bounded, cancellable background job that rejects a double start,
  drains on shutdown, and reconciles a crash-orphaned `running` crawl to
  `stopped` on startup (the hardened lifecycle shared by the other tools).
- **Send to Crawler** seeds a crawl at a captured request's URL.

Link extraction is regex-based (no HTML-parser dependency) — adequate for link
discovery; JS-rendered links and `robots.txt` handling are future work.

## Sequencer (`internal/sequencer`)

Analyzes the randomness of a sample of tokens (e.g. session identifiers).

- **Capture:** replays a base request N times (bounded concurrency), extracting
  a token from each response by **cookie name** or **regex** (first capture
  group), until the target sample size is reached.
- **Analysis:** per-character-position Shannon entropy, summed into an
  *effective bits* estimate, plus bits-per-char, alphabet size, length and
  uniqueness stats, a quality verdict (poor / reasonable / good / excellent),
  and notes (duplicates, constant positions, small sample). It is an indicative
  estimate, not a FIPS-140 battery.
- **Send to Sequencer** seeds a capture from a captured request and pre-fills the
  selector with the response's `Set-Cookie` name when present.

## API surface (Phase 5)

| Method | Path | Description |
|---|---|---|
| `GET/POST` | `/api/crawler/tasks` | List / create crawls |
| `GET/DELETE` | `/api/crawler/tasks/{id}` | Crawl + discovered URLs / delete |
| `POST` | `/api/crawler/tasks/{id}/start` \| `/stop` | Run / cancel |
| `GET/POST` | `/api/sequencer/tasks` | List / create capture tasks |
| `GET/PUT/DELETE` | `/api/sequencer/tasks/{id}` | Task+report+tokens / edit / delete |
| `POST` | `/api/sequencer/tasks/{id}/start` \| `/stop` | Run / cancel |
| `POST` | `/api/requests/{id}/send-to-crawler` | Seed a crawl from history |
| `POST` | `/api/requests/{id}/send-to-sequencer` | Seed a capture from history |

Progress streams over the traffic WebSocket as `crawl` and `sequencer` frames.
Crawls, discovered URLs, and capture tasks (with their token sample and report)
persist (`crawl_tasks`, `crawl_urls`, `sequencer_tasks`; migration 0006).
