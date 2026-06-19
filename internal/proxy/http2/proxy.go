// Package http2 documents RedTrace's HTTP/2 handling strategy.
//
// In Phase 1 the proxy negotiates HTTP/1.1 with both the client (the MITM TLS
// listener advertises only "http/1.1" via ALPN) and the upstream (the shared
// transport disables HTTP/2). This keeps request/response capture simple and
// uniform: every exchange is a classic HTTP/1.1 message regardless of what the
// origin server speaks.
//
// Native end-to-end HTTP/2 (preserving stream multiplexing and the binary
// framing in captures) is planned for a later phase; this package will hold the
// h2-aware proxy implementation.
package http2
