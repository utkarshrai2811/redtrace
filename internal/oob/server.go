package oob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// maxOOBBody bounds how much of an untrusted HTTP callback body is captured.
const maxOOBBody = 64 << 10

// Server runs the OOB DNS and HTTP listeners and records callbacks.
type Server struct {
	cfg    Config
	store  Store
	notify func(Interaction)
	log    *slog.Logger
}

// New returns an OOB server. It does nothing until Run is called, and Run is a
// no-op unless cfg.Enabled().
func New(cfg Config, store Store, notify func(Interaction), log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{cfg: cfg, store: store, notify: notify, log: log}
}

// Config returns the (read-only) OOB configuration.
func (s *Server) Config() Config { return s.cfg }

// NewPayload generates, persists, and returns a fresh OOB payload.
func (s *Server) NewPayload(ctx context.Context) (Payload, error) {
	if !s.cfg.Enabled() {
		return Payload{}, errors.New("oob is not configured")
	}
	p := Payload{Token: randHex(10), CreatedAt: time.Now()}
	p.Host = p.Token + "." + strings.TrimPrefix(s.cfg.Domain, ".")
	if err := s.store.SavePayload(ctx, p); err != nil {
		return Payload{}, err
	}
	return p, nil
}

// Run starts the configured listeners and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if !s.cfg.Enabled() {
		return nil
	}
	g, gctx := errgroup.WithContext(ctx)
	if s.cfg.HTTPAddr != "" {
		g.Go(func() error { return s.serveHTTP(gctx) })
	}
	if s.cfg.DNSAddr != "" {
		g.Go(func() error { return s.serveDNS(gctx) })
	}
	return g.Wait()
}

// record persists and broadcasts a captured interaction.
func (s *Server) record(protocol, sourceIP, query, detail string, raw []byte) {
	i := Interaction{
		ID: randHex(16), Token: tokenFor(query, s.cfg.Domain), Protocol: protocol,
		SourceIP: sourceIP, Query: query, Detail: detail, Raw: raw, CreatedAt: time.Now(),
	}
	if err := s.store.SaveInteraction(context.Background(), i); err != nil {
		s.log.Error("save oob interaction", "err", err)
		return
	}
	if s.notify != nil {
		s.notify(i)
	}
}

func (s *Server) serveHTTP(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           http.HandlerFunc(s.handleHTTP),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
	}
	go func() { //nolint:gosec // G118: shutdown must use a fresh context, not the cancelled serve context
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sc)
	}()
	s.log.Info("oob http listening", "addr", s.cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("oob http listen: %w", err)
	}
	return nil
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOOBBody)
	raw, _ := httputil.DumpRequest(r, true)
	// Strip the listener port from the Host header so the payload token (the
	// leftmost label under the OOB domain) correlates on non-standard ports.
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	detail := fmt.Sprintf("%s %s", r.Method, r.URL.RequestURI())
	if ua := r.UserAgent(); ua != "" {
		detail += "  UA: " + ua
	}
	s.record("http", clientIP(r.RemoteAddr), host, detail, raw)

	w.Header().Set("Server", "redtrace-oob")
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) serveDNS(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", s.cfg.DNSAddr)
	if err != nil {
		return fmt.Errorf("oob dns listen: %w", err)
	}
	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()
	s.log.Info("oob dns listening", "addr", s.cfg.DNSAddr)
	answer := net.ParseIP(s.cfg.PublicIP)
	buf := make([]byte, 1500)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		s.handleDNS(pc, addr, append([]byte(nil), buf[:n]...), answer)
	}
}

func (s *Server) handleDNS(pc net.PacketConn, addr net.Addr, msg []byte, answer net.IP) {
	// A malformed packet must never crash the read loop.
	defer func() { _ = recover() }()

	_, qtype, qname, qend, ok := parseDNSQuery(msg)
	if !ok {
		return
	}
	s.record("dns", clientIP(addr.String()), qname, dnsTypeName(qtype)+" "+qname, msg)

	// Only answer (with the configured A record) for names under our domain.
	ip := net.IP(nil)
	if answer != nil && underDomain(qname, s.cfg.Domain) {
		ip = answer
	}
	if resp := buildDNSResponse(msg, qend, qtype, ip); resp != nil {
		_, _ = pc.WriteTo(resp, addr)
	}
}

func underDomain(name, domain string) bool {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(domain, "."), "."))
	return name == domain || strings.HasSuffix(name, "."+domain)
}

func clientIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
