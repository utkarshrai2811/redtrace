// Package api exposes RedTrace's REST API, live-traffic WebSocket, and the
// embedded web UI over a single HTTP listener.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/api/handlers"
	"github.com/utkarshrai2811/redtrace/internal/intruder"
	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/repeater"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

// Config configures the API server.
type Config struct {
	ListenAddr string // address for the API + UI, e.g. 127.0.0.1:9090
	Token      string // optional shared auth token ("" disables auth)
	Version    string
	ProxyInfo  handlers.ProxyInfo
}

// Server serves the REST API, WebSocket, and embedded UI.
type Server struct {
	api    *handlers.API
	spa    http.Handler
	log    *slog.Logger
	token  string
	addr   string
	server *http.Server
}

// New constructs the API server, wiring the interception change notifier to the
// WebSocket hub so the UI sees queue updates live.
func New(cfg Config, store *storage.DB, sc *scope.Scope, rules *intercept.RuleSet, ic *intercept.Interceptor, authority *cert.Authority, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	hub := handlers.NewHub()
	runner := intruder.NewRunner(handlers.IntruderStore{DB: store}, hub.BroadcastIntruder)
	a := &handlers.API{
		Store:       store,
		Scope:       sc,
		Rules:       rules,
		Interceptor: ic,
		Authority:   authority,
		Hub:         hub,
		Repeater:    repeater.New(),
		Intruder:    runner,
		Version:     cfg.Version,
		Proxy:       cfg.ProxyInfo,
		Log:         logger,
	}
	ic.SetNotifier(a.PublishIntercept)

	spa, err := spaHandler()
	if err != nil {
		return nil, fmt.Errorf("init web ui: %w", err)
	}

	return &Server{
		api:   a,
		spa:   spa,
		log:   logger,
		token: cfg.Token,
		addr:  cfg.ListenAddr,
	}, nil
}

// PublishExchange streams a captured exchange summary to connected clients. Wire
// this to proxy.Proxy.OnExchange.
func (s *Server) PublishExchange(summary storage.RequestSummary) {
	s.api.Hub.BroadcastTraffic(summary)
}

// Run binds the configured address and serves until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("api listen: %w", err)
	}
	return s.Serve(ctx, ln)
}

// Serve serves on ln until ctx is cancelled or the server fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.server = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 30 * time.Second,
		// Bound a slow request body (slowloris) and idle keep-alive connections.
		// No WriteTimeout: the /ws/traffic stream is long-lived and the hub
		// manages its own read/write deadlines after the upgrade.
		ReadTimeout:    120 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() { //nolint:gosec // G118: the shutdown watcher must use a fresh context, not the serve context
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Cancel in-flight Intruder attacks first so they stop firing requests and
		// record a terminal status while the DB is still open, then drain HTTP.
		s.api.Intruder.Shutdown(shutdownCtx)
		_ = s.server.Shutdown(shutdownCtx)
	}()

	s.log.Info("api + ui listening", "addr", ln.Addr().String())
	if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
