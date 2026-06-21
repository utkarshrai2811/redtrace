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
	mux.HandleFunc("POST /api/requests/{id}/send-to-scanner", a.SendToScanner)
	mux.HandleFunc("POST /api/requests/{id}/send-to-crawler", a.SendToCrawler)
	mux.HandleFunc("POST /api/requests/{id}/send-to-sequencer", a.SendToSequencer)
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

	// Intruder
	mux.HandleFunc("GET /api/intruder/attacks", a.ListIntruderAttacks)
	mux.HandleFunc("POST /api/intruder/attacks", a.CreateIntruderAttack)
	mux.HandleFunc("GET /api/intruder/attacks/{id}", a.GetIntruderAttack)
	mux.HandleFunc("PUT /api/intruder/attacks/{id}", a.UpdateIntruderAttack)
	mux.HandleFunc("DELETE /api/intruder/attacks/{id}", a.DeleteIntruderAttack)
	mux.HandleFunc("POST /api/intruder/attacks/{id}/start", a.StartIntruderAttack)
	mux.HandleFunc("POST /api/intruder/attacks/{id}/stop", a.StopIntruderAttack)
	mux.HandleFunc("GET /api/intruder/results/{id}", a.GetIntruderResult)

	// Scanner
	mux.HandleFunc("GET /api/scanner/issues", a.ListScanIssues)
	mux.HandleFunc("DELETE /api/scanner/issues", a.ClearScanIssues)
	mux.HandleFunc("GET /api/scanner/issues/{id}", a.GetScanIssue)
	mux.HandleFunc("GET /api/scanner/tasks", a.ListScanTasks)
	mux.HandleFunc("POST /api/scanner/tasks", a.CreateScanTask)
	mux.HandleFunc("GET /api/scanner/tasks/{id}", a.GetScanTask)
	mux.HandleFunc("PUT /api/scanner/tasks/{id}", a.UpdateScanTask)
	mux.HandleFunc("DELETE /api/scanner/tasks/{id}", a.DeleteScanTask)
	mux.HandleFunc("POST /api/scanner/tasks/{id}/start", a.StartScanTask)
	mux.HandleFunc("POST /api/scanner/tasks/{id}/stop", a.StopScanTask)

	// Crawler
	mux.HandleFunc("GET /api/crawler/tasks", a.ListCrawlTasks)
	mux.HandleFunc("POST /api/crawler/tasks", a.CreateCrawlTask)
	mux.HandleFunc("GET /api/crawler/tasks/{id}", a.GetCrawlTask)
	mux.HandleFunc("DELETE /api/crawler/tasks/{id}", a.DeleteCrawlTask)
	mux.HandleFunc("POST /api/crawler/tasks/{id}/start", a.StartCrawlTask)
	mux.HandleFunc("POST /api/crawler/tasks/{id}/stop", a.StopCrawlTask)

	// Sequencer
	mux.HandleFunc("GET /api/sequencer/tasks", a.ListSequencerTasks)
	mux.HandleFunc("POST /api/sequencer/tasks", a.CreateSequencerTask)
	mux.HandleFunc("GET /api/sequencer/tasks/{id}", a.GetSequencerTask)
	mux.HandleFunc("PUT /api/sequencer/tasks/{id}", a.UpdateSequencerTask)
	mux.HandleFunc("DELETE /api/sequencer/tasks/{id}", a.DeleteSequencerTask)
	mux.HandleFunc("POST /api/sequencer/tasks/{id}/start", a.StartSequencerTask)
	mux.HandleFunc("POST /api/sequencer/tasks/{id}/stop", a.StopSequencerTask)

	// Repeater
	mux.HandleFunc("GET /api/repeater/tabs", a.ListRepeaterTabs)
	mux.HandleFunc("POST /api/repeater/tabs", a.CreateRepeaterTab)
	mux.HandleFunc("GET /api/repeater/tabs/{id}", a.GetRepeaterTab)
	mux.HandleFunc("PUT /api/repeater/tabs/{id}", a.UpdateRepeaterTab)
	mux.HandleFunc("DELETE /api/repeater/tabs/{id}", a.DeleteRepeaterTab)
	mux.HandleFunc("POST /api/repeater/tabs/{id}/send", a.SendRepeaterTab)

	// Out-of-band (Collaborator)
	mux.HandleFunc("GET /api/oob/config", a.OOBConfig)
	mux.HandleFunc("GET /api/oob/payloads", a.ListOOBPayloads)
	mux.HandleFunc("POST /api/oob/payloads", a.GenerateOOBPayload)
	mux.HandleFunc("GET /api/oob/interactions", a.ListOOBInteractions)
	mux.HandleFunc("DELETE /api/oob/interactions", a.ClearOOBInteractions)
	mux.HandleFunc("GET /api/oob/interactions/{id}", a.GetOOBInteraction)

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
