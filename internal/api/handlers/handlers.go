// Package handlers implements the RedTrace REST and WebSocket HTTP handlers.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

// ProxyInfo is read-only proxy configuration surfaced to the UI.
type ProxyInfo struct {
	ProxyAddr     string `json:"proxyAddr"`
	UpstreamProxy string `json:"upstreamProxy"`
}

// API holds the dependencies shared by all handlers.
type API struct {
	Store       *storage.DB
	Scope       *scope.Scope
	Rules       *intercept.RuleSet
	Interceptor *intercept.Interceptor
	Authority   *cert.Authority
	Hub         *Hub
	Version     string
	Proxy       ProxyInfo
	Log         *slog.Logger
}

// errorBody is the consistent error envelope: {"error":{"code","message"}}.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	var b errorBody
	b.Error.Code = errCode
	b.Error.Message = msg
	writeJSON(w, code, b)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
