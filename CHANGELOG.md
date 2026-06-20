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

### Fixed
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
