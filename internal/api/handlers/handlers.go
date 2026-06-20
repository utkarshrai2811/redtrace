// Package handlers implements the RedTrace REST and WebSocket HTTP handlers.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/crawler"
	"github.com/utkarshrai2811/redtrace/internal/intruder"
	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/repeater"
	"github.com/utkarshrai2811/redtrace/internal/scanner"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/sequencer"
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
	Repeater    *repeater.Engine
	Intruder    *intruder.Runner
	Scanner     *scanner.Scanner
	Crawler     *crawler.Crawler
	Sequencer   *sequencer.Sequencer
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

// serverError logs an internal (5xx) error server-side and returns a generic
// message to the client, so storage/driver internals (SQL, file paths) are not
// disclosed over the wire. Reserve verbatim err.Error() for user-actionable 4xx
// validation and live-network (502) errors.
func (a *API) serverError(w http.ResponseWriter, errCode string, err error) {
	if a.Log != nil {
		a.Log.Error("request failed", "code", errCode, "err", err)
	}
	writeError(w, http.StatusInternalServerError, errCode, "internal error")
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
