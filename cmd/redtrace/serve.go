package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/utkarshrai2811/redtrace/internal/api"
	"github.com/utkarshrai2811/redtrace/internal/api/handlers"
	"github.com/utkarshrai2811/redtrace/internal/proxy"
	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
	"github.com/utkarshrai2811/redtrace/internal/scope"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the intercepting proxy and web UI",
	RunE:  runServe,
}

func init() {
	f := serveCmd.Flags()
	f.String("proxy-listen", "", "proxy listen address (default: 127.0.0.1:8080)")
	f.String("api-listen", "", "API + UI listen address (default: 127.0.0.1:9090)")
	f.String("upstream-proxy", "", "upstream proxy URL for chaining (e.g. http://127.0.0.1:8081)")
	f.String("token", "", "shared auth token for the API/UI (empty disables auth)")
	_ = viper.BindPFlag("proxy.listen", f.Lookup("proxy-listen"))
	_ = viper.BindPFlag("api.listen", f.Lookup("api-listen"))
	_ = viper.BindPFlag("upstream.proxy", f.Lookup("upstream-proxy"))
	_ = viper.BindPFlag("auth.token", f.Lookup("token"))
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg := loadConfig()
	logger := newLogger(cfg.LogLevel)

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	authority, err := cert.LoadOrCreateAuthority(cfg.CADir)
	if err != nil {
		return fmt.Errorf("certificate authority: %w", err)
	}

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	sc := scope.New()
	rules := intercept.NewRuleSet()
	interceptor := intercept.NewInterceptor(storage.NewID)

	px, err := proxy.New(proxy.Config{
		ListenAddr:    cfg.ProxyListen,
		UpstreamProxy: cfg.UpstreamProxy,
	}, authority, db, sc, rules, interceptor, logger)
	if err != nil {
		return fmt.Errorf("init proxy: %w", err)
	}

	srv, err := api.New(api.Config{
		ListenAddr: cfg.APIListen,
		Token:      cfg.Token,
		Version:    version,
		ProxyInfo:  handlers.ProxyInfo{ProxyAddr: cfg.ProxyListen, UpstreamProxy: cfg.UpstreamProxy},
	}, db, sc, rules, interceptor, authority, logger)
	if err != nil {
		return fmt.Errorf("init api: %w", err)
	}
	px.OnExchange = srv.PublishExchange
	px.OnExchangeStored = srv.ScanExchange

	printBanner(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return px.Run(ctx) })
	g.Go(func() error { return srv.Run(ctx) })
	return g.Wait()
}

func printBanner(cfg Config) {
	caPath := filepath.Join(cfg.CADir, "redtrace-ca.pem")
	fmt.Printf(`
  RedTrace %s

    Proxy    http://%s   (set as your browser HTTP/HTTPS proxy)
    Web UI   http://%s
    CA cert  %s
             install it to intercept HTTPS — see 'redtrace ca' and scripts/install-ca.sh

  Press Ctrl-C to stop.

`, version, cfg.ProxyListen, cfg.APIListen, caPath)
}
