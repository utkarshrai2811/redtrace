package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/utkarshrai2811/redtrace/internal/ai"
	"github.com/utkarshrai2811/redtrace/internal/api"
	"github.com/utkarshrai2811/redtrace/internal/api/handlers"
	"github.com/utkarshrai2811/redtrace/internal/oob"
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
	f.String("oob-domain", "", "OOB/Collaborator domain; setting it starts the OOB listeners (e.g. oob.example.com)")
	f.String("oob-public-ip", "", "IPv4 the OOB DNS listener answers with (your public IP)")
	f.String("oob-http-listen", "", "OOB HTTP listener address (default 0.0.0.0:8888)")
	f.String("oob-dns-listen", "", "OOB DNS listener address (default 0.0.0.0:5353)")
	f.String("ai-provider", "", "AI provider: anthropic or openai (default anthropic)")
	f.String("ai-model", "", "AI model id (e.g. claude-sonnet-4-6, gpt-4o-mini)")
	f.String("ai-api-key", "", "AI provider API key (enables the AI assistant)")
	f.String("ai-base-url", "", "OpenAI-compatible base URL (e.g. http://127.0.0.1:11434/v1 for a local model)")
	_ = viper.BindPFlag("proxy.listen", f.Lookup("proxy-listen"))
	_ = viper.BindPFlag("api.listen", f.Lookup("api-listen"))
	_ = viper.BindPFlag("upstream.proxy", f.Lookup("upstream-proxy"))
	_ = viper.BindPFlag("auth.token", f.Lookup("token"))
	_ = viper.BindPFlag("oob.domain", f.Lookup("oob-domain"))
	_ = viper.BindPFlag("oob.public-ip", f.Lookup("oob-public-ip"))
	_ = viper.BindPFlag("oob.http-listen", f.Lookup("oob-http-listen"))
	_ = viper.BindPFlag("oob.dns-listen", f.Lookup("oob-dns-listen"))
	_ = viper.BindPFlag("ai.provider", f.Lookup("ai-provider"))
	_ = viper.BindPFlag("ai.model", f.Lookup("ai-model"))
	_ = viper.BindPFlag("ai.api-key", f.Lookup("ai-api-key"))
	_ = viper.BindPFlag("ai.base-url", f.Lookup("ai-base-url"))
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg := loadConfig()
	logger := newLogger(cfg.LogLevel)

	if cfg.OOBDomain != "" {
		if _, _, err := net.SplitHostPort(cfg.OOBHTTPListen); err != nil {
			return fmt.Errorf("invalid --oob-http-listen %q: %w", cfg.OOBHTTPListen, err)
		}
		if _, _, err := net.SplitHostPort(cfg.OOBDNSListen); err != nil {
			return fmt.Errorf("invalid --oob-dns-listen %q: %w", cfg.OOBDNSListen, err)
		}
		if net.ParseIP(cfg.OOBPublicIP) == nil {
			logger.Warn("oob: --oob-public-ip is unset or not a valid IP; DNS A answers are disabled",
				"value", cfg.OOBPublicIP)
		}
	}

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
		OOB: oob.Config{
			Domain: cfg.OOBDomain, PublicIP: cfg.OOBPublicIP,
			HTTPAddr: cfg.OOBHTTPListen, DNSAddr: cfg.OOBDNSListen,
		},
		AI: resolveAIConfig(db, cfg),
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
	g.Go(func() error { return srv.RunOOB(ctx) })
	return g.Wait()
}

// resolveAIConfig assembles the startup AI configuration. Flags/env take
// precedence; any field they leave empty falls back to the settings persisted in
// the local database (set via the Settings UI). The key is not written back to
// disk here — only an explicit Settings save persists it.
func resolveAIConfig(db *storage.DB, cfg Config) ai.Config {
	out := ai.Config{Provider: cfg.AIProvider, Model: cfg.AIModel, APIKey: cfg.AIAPIKey, BaseURL: cfg.AIBaseURL}
	s, ok, err := db.LoadAISettings(context.Background())
	if err != nil || !ok {
		return out
	}
	// Inherit persisted fields only when they belong to the provider that will be
	// in effect. Otherwise a flag like `--ai-provider anthropic` could inherit a
	// previously-saved OpenAI base URL/model and send the Anthropic key there.
	effective := out.Provider
	if effective == "" {
		effective = s.Provider
	}
	if !strings.EqualFold(effective, s.Provider) {
		return out
	}
	if out.Provider == "" {
		out.Provider = s.Provider
	}
	if out.Model == "" {
		out.Model = s.Model
	}
	if out.BaseURL == "" {
		out.BaseURL = s.BaseURL
	}
	if out.APIKey == "" {
		out.APIKey = s.APIKey
	}
	return out
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
