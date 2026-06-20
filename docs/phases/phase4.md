# Phase 4 — Scanner

**Goal:** find vulnerabilities — passively, by analysing the traffic RedTrace
already captures, and actively, by probing a request's insertion points with
attack payloads and inspecting the responses.

## Delivered

### Passive scanner (`internal/scanner`, `passive.go`)

Runs automatically on every **in-scope** exchange captured by the proxy (wired
through `proxy.OnExchangeStored`). It sends no traffic of its own and dedupes
findings by a stable fingerprint, so an issue is recorded once rather than on
every request to an endpoint. Checks:

| Check | Severity | Notes |
|---|---|---|
| `missing_hsts` | low | HTTPS response without Strict-Transport-Security |
| `missing_xcto` | info | no `X-Content-Type-Options: nosniff` |
| `missing_csp` | low | HTML response without a Content-Security-Policy |
| `clickjacking` | low | HTML with neither X-Frame-Options nor CSP frame-ancestors |
| `insecure_cookie` | medium/low | Set-Cookie missing Secure (HTTPS) / HttpOnly / SameSite |
| `info_disclosure` | info | Server / X-Powered-By / version banners |
| `cors_misconfig` | high | Origin reflected with `Access-Control-Allow-Credentials: true` |
| `cors_wildcard` | low | `Access-Control-Allow-Origin: *` |
| `directory_listing` | medium | automatic index page |
| `error_disclosure` | low | stack trace / framework error in the body |
| `password_over_http` | medium | password input served over cleartext HTTP |

### Active scanner (`active.go`, `runner.go`)

Launched explicitly against a captured request (**Send to Scanner** seeds a
task). It derives insertion points from the request's query string and
form-urlencoded body, sends a baseline, then probes each point with each
check's payloads through the Repeater transport (always without following
redirects, so `Location` is observable and reflection is detected on the
immediate response). Checks use distinctive, false-positive-resistant markers:

| Check | Detection |
|---|---|
| `reflected_xss` | a unique `<rtx…>` marker reflected unencoded |
| `sql_injection` | a SQL-error signature that appears only after a quote is injected |
| `path_traversal` | an `/etc/passwd` `root:…:0:0:` line in the response |
| `open_redirect` | a 3xx whose `Location` points at the injected external host |
| `ssti` | the product `1337*1337 => 1787569` rendered (expression evaluated) |
| `command_injection` | the shell arithmetic `$((31337*31337)) => 981990569` evaluated |

The runner is a bounded (default 8, max 32), cancellable worker pool that
persists findings, streams progress, and — like the Intruder runner — rejects a
double start, drains on shutdown, and reconciles a crash-orphaned `running` task
to `stopped` on startup.

## API surface (Phase 4)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/scanner/issues` | List all findings |
| `GET` | `/api/scanner/issues/{id}` | One finding with raw request/response |
| `DELETE` | `/api/scanner/issues` | Clear all findings |
| `GET` | `/api/scanner/tasks` | List active-scan tasks |
| `POST` | `/api/scanner/tasks` | Create an active-scan draft |
| `GET` | `/api/scanner/tasks/{id}` | Task + its findings |
| `PUT` | `/api/scanner/tasks/{id}` | Update a draft |
| `DELETE` | `/api/scanner/tasks/{id}` | Delete (stops it first if running) |
| `POST` | `/api/scanner/tasks/{id}/start` | Validate and run |
| `POST` | `/api/scanner/tasks/{id}/stop` | Cancel a running scan |
| `POST` | `/api/requests/{id}/send-to-scanner` | Seed a scan from history |

Findings and active-scan progress are pushed over the existing traffic
WebSocket as `scanner` frames. Tasks and findings persist (`scan_tasks`,
`scan_issues`; migration 0005); findings are kept even when their task is
deleted, since a finding is evidence.

## Notes

- Crawling/site-spidering is intentionally **not** part of the active scanner —
  it is Phase 5. The active scanner probes one request's parameters, not a site.
- The active scanner reuses the Repeater transport, so it inherits faithful
  response handling and the per-host connection pooling.
