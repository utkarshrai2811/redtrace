# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Phase 1: project scaffold, CA certificate generation, core MITM proxy,
  SQLite storage layer, REST API + WebSocket traffic stream, and the proxy
  traffic UI.
- Phase 2: Repeater (persistent tabs, history, follow-redirects, HTTP version),
  Decoder (URL/Base64/Hex/HTML/Gzip/JWT, chained ops, smart detect), Comparer
  (line/word LCS diff), the Target site map (tree + notes + JSON export), and a
  Send-to-Repeater action from proxy history.
- Phase 3: Intruder — automated request fuzzing with Sniper, Battering ram,
  Pitchfork, and Cluster bomb attack types; §-marked payload positions; payload
  sets with processors (prefix/suffix, base64, URL, case, hashes); a concurrent
  (unthrottled) runner with live results streamed over the WebSocket; a results
  table with anomaly highlighting; and a Send-to-Intruder action from proxy
  history. Attacks and results persist (migration 0004).
- Phase 4: Scanner — a passive analyzer that flags issues in captured in-scope
  traffic (missing security headers, insecure cookies, CORS misconfiguration,
  technology/version disclosure, directory listing, error/stack-trace leakage,
  cleartext password forms) and an active scanner that probes a request's query
  and form insertion points for reflected XSS, error-based SQL injection, path
  traversal, open redirect, server-side template injection, and OS command
  injection. Findings are deduplicated, severity-ranked, and streamed live over
  the WebSocket; active scans run as a bounded, cancellable background job. Adds
  a Send-to-Scanner action from proxy history. Tasks and findings persist
  (migration 0005).
- Phase 5: Crawler & Sequencer — a scope-bounded spider that starts from a seed
  URL, follows same-host in-scope links breadth-first within a depth/page budget,
  and records every fetched page in the history and site map (passively scanning
  each); and a Sequencer that replays a request to collect a sample of tokens (by
  cookie name or regex) and reports their randomness (per-position Shannon
  entropy, effective bits, and a quality verdict). Both run as bounded,
  cancellable background jobs with live progress, add Send-to-Crawler and
  Send-to-Sequencer actions from proxy history, and persist (migration 0006).

### Fixed
- Post-Phase-4 (Scanner) review pass:
  - **Passive checks:** cookie Secure/HttpOnly/SameSite detection now parses the
    attribute list instead of substring-matching the whole Set-Cookie line (a
    `__Secure-`-prefixed cookie missing the Secure attribute, or a value
    containing those words, was wrongly passed); the password-over-HTTP check
    matches single-quoted and spaced `type=password` markup; HSTS and
    X-Content-Type-Options findings are scoped per path; evidence truncation no
    longer slices multibyte UTF-8 mid-rune.
  - **Active scanner:** the request builder no longer double-encodes an
    already-percent-encoded payload (the `%2f` path-traversal variant reached the
    wire as `%252f`); insertion-point selection is deterministic (sorted) so a
    scan probes the same parameters every run and within the cap; a failed
    baseline request now skips the diff-based detectors instead of suppressing
    nothing (which caused false positives); legacy semicolon-separated query
    strings still yield insertion points.
  - **Scan lifecycle/UI:** the terminal status frame carries the final
    completed/issue counts so a finished scan no longer flashes `0/total`; active
    findings now stream into the open scan's Findings table live; live findings
    sort to the top of their severity tier; clearing findings also resets the
    per-task counts; deleting a scan asks for confirmation; issue rows are
    keyboard-activatable.
  - **Active findings dedup** is scoped per task, so two scans of the same
    endpoint each record and count their own findings; the per-task issues query
    uses its index instead of scanning the whole table.
- Post-Phase-3 (Intruder) review pass:
  - **Lifecycle:** starting an attack that is already running is rejected
    (409) instead of launching a second engine that wiped the first run's
    results and orphaned an uncancellable goroutine; in-flight attacks are now
    cancelled and drained on server shutdown, and attacks left `running` by a
    crash are reconciled to `stopped` on startup.
  - **Live results:** a result streamed over the WebSocket now carries the same
    id as its stored row, so it can be opened (and keys/selection work) while
    the attack is still running.
  - **Payload generation:** a Cluster bomb's request count is overflow-safe and
    bounded, and an unknown attack type is reported as such rather than as
    "generates no requests".
  - **Performance:** the persisted progress counter is throttled (not written
    per request); the request transport keeps per-host keep-alive connections so
    a concurrent attack stops re-handshaking TLS every request; the results
    table inserts in order and memoizes rows so a large live run no longer
    re-sorts and re-renders everything on every update.
  - **UI:** Start is gated on the projected request count (so pitchfork/cluster
    bomb with an empty per-position set can't be started), the count is shown
    before launch, the concurrency input is clamped to the real 64-worker cap,
    the anomaly highlight needs a genuine majority baseline (no more flagging
    every row), and the attack delete control is visible to keyboard focus.
- Interception now only holds in-scope traffic; bodyless (HEAD/204/304)
  responses and >10 MB bodies are handled correctly; CORS is restricted to
  loopback origins with a DNS-rebinding guard; assorted UI/UX and a11y fixes.
- Post-Phase-2 review pass:
  - **Proxy:** a highly compressible response whose decoded size exceeds the
    buffer cap is now forwarded compressed-verbatim instead of being silently
    truncated; hop-by-hop response headers are stripped on the HTTPS tunnel path
    too; scope is evaluated after match & replace so URL-rewriting rules gate
    correctly; held HTTPS requests/responses are released on client disconnect
    or shutdown instead of leaking their goroutine.
  - **Live traffic:** the `/ws/traffic` WebSocket upgrade no longer fails behind
    the logging middleware (the wrapper now supports hijacking).
  - **Interception:** turning off response interception forwards any responses
    already held.
  - **Repeater:** responses are shown exactly as received (no transparent gzip
    decompression / `Content-Encoding` stripping); 307/308 redirects with a body
    are followed.
  - **Decoder/Comparer:** gzip decompression and the diff table are bounded to
    prevent zip-bomb / large-input memory exhaustion.
  - **API/auth:** a configured `--token` no longer breaks the bundled UI — the
    SPA and assets load without a token while the API and WebSocket stay gated,
    the UI sends the token (captured from `?token=`) on every request and the WS
    handshake; added a request-body size limit, read/idle timeouts, and generic
    5xx error messages (internals are logged, not returned).
  - **Storage:** in-memory databases work under concurrent access; `LIKE`
    filters treat `%`/`_` literally; the DB path is URL-escaped in the DSN.
  - **UI/a11y:** dashboard table no longer overflows on long URLs; readable
    empty-state text meets contrast; keyboard focus is always visible; glyph-only
    buttons have accessible names; editing a binary intercepted message no longer
    corrupts it; the toolbar count reads "matching" when a filter is active.

[Unreleased]: https://github.com/utkarshrai2811/redtrace/commits/develop
