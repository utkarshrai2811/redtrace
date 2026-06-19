# Architecture

RedTrace is a single Go binary that runs two listeners and embeds the web UI:

- **Proxy listener** (default `127.0.0.1:8080`) — the intercepting MITM proxy
  that browsers and tools point at.
- **API/UI listener** (default `127.0.0.1:9090`) — the REST API, the WebSocket
  traffic stream, and the embedded React single-page app.

```
                 ┌─────────────────────────────────────────────┐
   browser ─────▶│  Proxy (:8080)                               │
   / tools       │   • CONNECT + dynamic TLS (per-host certs)   │
                 │   • request/response capture                 │
                 │   • match & replace, scope                   │
                 └───────────────┬─────────────────────────────┘
                                 │ persist + publish
                                 ▼
                 ┌─────────────────────────────────────────────┐
                 │  Storage (SQLite, modernc, FTS5)             │
                 └───────────────┬─────────────────────────────┘
                                 │ query
                                 ▼
                 ┌─────────────────────────────────────────────┐
   web UI ──────▶│  API (:9090)  REST + ws://.../ws/traffic     │
                 └─────────────────────────────────────────────┘
```

## Packages

| Package | Responsibility |
|---|---|
| `cmd/redtrace` | CLI entrypoint (cobra), config (viper) |
| `internal/proxy` | MITM engine, certs, intercept rules, ws/http2 |
| `internal/storage` | SQLite connection, migrations, models |
| `internal/api` | HTTP server, router, handlers, middleware, WS hub |
| `internal/scope` | Global in/out-of-scope matching |

Later phases add `internal/{repeater,decoder,comparer,intruder,scanner,...}`.

## Principles

- **Local-first & private.** Binds to loopback by default; no telemetry.
- **No CGO.** Pure-Go SQLite (`modernc.org/sqlite`) keeps cross-compilation and
  single-binary distribution trivial.
- **Dependency injection.** No global mutable state; components receive their
  collaborators via struct fields.
