# Phase 1 — Foundation & Core Proxy

**Goal:** a working MITM proxy that intercepts, logs, and displays HTTP/HTTPS
traffic in real time.

## Scope

- CA certificate generation + per-host leaf certs.
- Core MITM proxy: HTTP, HTTPS (CONNECT + dynamic certs), upstream proxy chaining.
- SQLite storage for projects/requests/responses with FTS5 search.
- Intercept match & replace rules engine.
- REST API + WebSocket live traffic stream.
- Global scope management.
- React + Vite proxy UI: live traffic table, request/response viewer, filters.

## Definition of done

A browser pointed at `127.0.0.1:8080` captures and displays HTTPS traffic in the
UI in real time.

## API surface (Phase 1)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/requests` | Paginated, filterable history |
| `GET` | `/api/requests/{id}` | Single request + response |
| `DELETE` | `/api/requests/{id}` | Delete a captured exchange |
| `POST` | `/api/requests/{id}/send-to-repeater` | Queue for Repeater (Phase 2) |
| `POST` | `/api/requests/{id}/send-to-intruder` | Queue for Intruder (Phase 3) |
| `GET`/`PUT` | `/api/proxy/settings` | Proxy configuration |
| `GET`/`PUT` | `/api/scope` | Scope include/exclude rules |
| `GET`/`POST`/`PUT`/`DELETE` | `/api/intercept/rules` | Match & replace rules |
| `WS` | `/ws/traffic` | Live traffic stream |
