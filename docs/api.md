# RedTrace API (Phase 1)

The API server listens on `127.0.0.1:9090` by default and serves the REST API,
the live-traffic WebSocket, and the embedded web UI from the same origin.

All error responses share the envelope:

```json
{ "error": { "code": "string", "message": "string" } }
```

Timestamps are RFC 3339. Raw request/response bytes are base64-encoded in JSON.

## Health & info

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | `{ "status": "ok", "version": "…" }` |
| `GET` | `/api/proxy/settings` | `{ "proxyAddr", "upstreamProxy" }` |
| `GET` | `/api/ca/cert` | Downloads the CA certificate (PEM) |

## Proxy history

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/requests` | List captured exchanges (see filters) |
| `GET` | `/api/requests/{id}` | Full exchange with `requestRaw`/`responseRaw` (base64) |
| `DELETE` | `/api/requests/{id}` | Delete an exchange |
| `POST` | `/api/requests/{id}/send-to-repeater` | Phase 2 (501 for now) |
| `POST` | `/api/requests/{id}/send-to-intruder` | Phase 3 (501 for now) |
| `GET` | `/api/hosts` | Distinct observed hosts |

`GET /api/requests` query parameters: `page`, `per_page`, `method`, `host`,
`status` (`200` or `2xx`/`3xx`/`4xx`/`5xx`), `mime`, `q` (substring/FTS),
`scope=in`.

## Scope

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/scope` | List scope rules |
| `PUT` | `/api/scope` | Replace scope rules (`400` on invalid regex/CIDR) |

Rule: `{ id, enabled, kind: include|exclude, matcher: host|host_regex|path_prefix|cidr, value }`.

## Interception

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/intercept` | `{ enabled, interceptResponses, queue, count }` |
| `PUT` | `/api/intercept` | Toggle `{ enabled?, interceptResponses? }` |
| `POST` | `/api/intercept/{id}/forward` | Forward held item; optional `{ raw }` (base64) to edit |
| `POST` | `/api/intercept/{id}/drop` | Drop held item |
| `GET` | `/api/intercept/rules` | List match & replace rules |
| `PUT` | `/api/intercept/rules` | Replace match & replace rules |

Match & replace rule: `{ id, enabled, name, part, matchType: literal|regex, match, replace, priority }`
where `part` ∈ `request_method`, `request_url`, `request_header`, `request_body`,
`response_header`, `response_body`.

## WebSocket

`GET /ws/traffic` streams JSON frames:

- `{ "type": "traffic", "data": RequestSummary }` — a newly captured exchange.
- `{ "type": "intercept", "data": InterceptState }` — interception state changed.
