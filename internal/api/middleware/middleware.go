// Package middleware provides HTTP middleware for the RedTrace API server.
package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Chain applies middleware in order, so Chain(h, a, b) yields a(b(h)).
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// CORS reflects the Origin only for loopback origins (the local web UI / dev
// server). A page on any other site receives no Access-Control-Allow-Origin, so
// the browser blocks it from reading the API — preventing a visited website
// from driving the proxy.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isLoopbackOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-RedTrace-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LocalGuard blocks requests whose Host header is not a loopback address when no
// auth token is configured. This defeats DNS-rebinding against the default
// loopback bind; when a token is set (an intentional remote bind), the token is
// the gate and any Host is allowed.
func LocalGuard(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" && !isLoopbackHost(r.Host) {
				http.Error(w, `{"error":{"code":"forbidden","message":"non-loopback Host blocked; set an auth token to allow remote access"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isLoopbackHost reports whether host (optionally host:port) is localhost or a
// loopback IP.
func isLoopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return isLoopbackHost(u.Host)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Logger logs each request's method, path, status, and duration.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Debug("api request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start).String(),
			)
		})
	}
}

// Auth optionally requires a shared token (Authorization: Bearer <t> or the
// X-RedTrace-Token header). When token is empty, authentication is disabled.
func Auth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-RedTrace-Token")
			if provided == "" {
				const prefix = "Bearer "
				if h := r.Header.Get("Authorization"); len(h) > len(prefix) && h[:len(prefix)] == prefix {
					provided = h[len(prefix):]
				}
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				http.Error(w, `{"error":{"code":"unauthorized","message":"invalid or missing token"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
