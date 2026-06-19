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

[Unreleased]: https://github.com/utkarshrai2811/redtrace/commits/develop
