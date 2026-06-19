# Phase 2 — Core Analysis Tools

**Goal:** the everyday tools — Repeater, Decoder, Comparer, and the Target site
map — plus the Match & Replace UI.

## Delivered

- **Repeater** (`internal/repeater`): replay an edited raw request against a
  target with a follow-redirects toggle and HTTP/1.1 or HTTP/2. Tabs and their
  send history persist (`repeater_tabs`, `repeater_history`). "Send to Repeater"
  from proxy history seeds a tab.
- **Decoder** (`internal/decoder`): URL, Base64 (standard + URL-safe), Hex, HTML
  entities, Gzip, and JWT decode; operations chain; smart-detect a likely decode.
- **Comparer** (`internal/comparer`): LCS-based line and word diff, returning
  equal/insert/delete segments.
- **Target site map**: a host → path tree built from captured traffic with
  per-endpoint notes/tags (`sitemap_notes`) and JSON export.

## API surface (Phase 2)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/decoder/run` | Apply a chain of decode/encode operations |
| `POST` | `/api/decoder/detect` | Suggest a likely decode step |
| `POST` | `/api/comparer` | Diff two texts (`mode`: `lines`\|`words`) |
| `GET` | `/api/repeater/tabs` | List Repeater tabs |
| `POST` | `/api/repeater/tabs` | Create a tab |
| `GET` | `/api/repeater/tabs/{id}` | Tab + send history |
| `PUT` | `/api/repeater/tabs/{id}` | Update a tab |
| `DELETE` | `/api/repeater/tabs/{id}` | Delete a tab |
| `POST` | `/api/repeater/tabs/{id}/send` | Replay the tab's request |
| `GET` | `/api/sitemap` | Host → path tree |
| `PUT` | `/api/sitemap/note` | Annotate an endpoint |
| `POST` | `/api/requests/{id}/send-to-repeater` | Seed a tab from history |

## Follow-ups (not blocking)

- Match & Replace UI exists in Settings (CRUD + priority); "test a rule against
  a captured request" is a future enhancement.
- Request-body (large upload) streaming for Repeater/proxy.
