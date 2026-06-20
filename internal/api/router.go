package api

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/api/middleware"
)

// maxRequestBody caps any single request body so a large POST cannot exhaust
// memory (decode/comparer inputs and Repeater raw requests are buffered fully).
const maxRequestBody = 32 << 20 // 32 MiB

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

	// Site map
	mux.HandleFunc("GET /api/sitemap", a.GetSitemap)
	mux.HandleFunc("PUT /api/sitemap/note", a.PutSitemapNote)

	// Decoder & Comparer
	mux.HandleFunc("POST /api/decoder/run", a.Decode)
	mux.HandleFunc("POST /api/decoder/detect", a.DecodeDetect)
	mux.HandleFunc("POST /api/comparer", a.Compare)

	// Repeater
	mux.HandleFunc("GET /api/repeater/tabs", a.ListRepeaterTabs)
	mux.HandleFunc("POST /api/repeater/tabs", a.CreateRepeaterTab)
	mux.HandleFunc("GET /api/repeater/tabs/{id}", a.GetRepeaterTab)
	mux.HandleFunc("PUT /api/repeater/tabs/{id}", a.UpdateRepeaterTab)
	mux.HandleFunc("DELETE /api/repeater/tabs/{id}", a.DeleteRepeaterTab)
	mux.HandleFunc("POST /api/repeater/tabs/{id}/send", a.SendRepeaterTab)

	// Live traffic stream
	mux.HandleFunc("GET /ws/traffic", a.Hub.ServeWS)

	// Single-page app (everything else)
	mux.Handle("/", s.spa)

	return middleware.Chain(mux,
		middleware.LocalGuard(s.token),
		middleware.CORS,
		middleware.BodyLimit(maxRequestBody),
		middleware.Logger(s.log),
		middleware.Auth(s.token),
	)
}
