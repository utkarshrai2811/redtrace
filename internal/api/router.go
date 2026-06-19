package api

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/api/middleware"
)

// routes registers all REST and WebSocket routes and wraps them in middleware.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	a := s.api

	// Health & info
	mux.HandleFunc("GET /api/health", a.Health)
	mux.HandleFunc("GET /api/proxy/settings", a.ProxySettings)
	mux.HandleFunc("GET /api/ca/cert", a.CACert)

	// Proxy history
	mux.HandleFunc("GET /api/requests", a.ListRequests)
	mux.HandleFunc("DELETE /api/requests", a.ClearRequests)
	mux.HandleFunc("GET /api/requests/{id}", a.GetRequest)
	mux.HandleFunc("DELETE /api/requests/{id}", a.DeleteRequest)
	mux.HandleFunc("POST /api/requests/{id}/send-to-repeater", a.SendToRepeater)
	mux.HandleFunc("POST /api/requests/{id}/send-to-intruder", a.SendToIntruder)
	mux.HandleFunc("GET /api/hosts", a.Hosts)

	// Scope
	mux.HandleFunc("GET /api/scope", a.GetScope)
	mux.HandleFunc("PUT /api/scope", a.PutScope)

	// Interception
	mux.HandleFunc("GET /api/intercept", a.InterceptStatus)
	mux.HandleFunc("PUT /api/intercept", a.UpdateIntercept)
	mux.HandleFunc("POST /api/intercept/forward-all", a.ForwardAll)
	mux.HandleFunc("POST /api/intercept/drop-all", a.DropAll)
	mux.HandleFunc("POST /api/intercept/{id}/forward", a.ForwardItem)
	mux.HandleFunc("POST /api/intercept/{id}/drop", a.DropItem)
	mux.HandleFunc("GET /api/intercept/rules", a.GetRules)
	mux.HandleFunc("PUT /api/intercept/rules", a.PutRules)

	// Live traffic stream
	mux.HandleFunc("GET /ws/traffic", a.Hub.ServeWS)

	// Single-page app (everything else)
	mux.Handle("/", s.spa)

	return middleware.Chain(mux,
		middleware.LocalGuard(s.token),
		middleware.CORS,
		middleware.Logger(s.log),
		middleware.Auth(s.token),
	)
}
