package sequencer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"
	"sync"

	"github.com/utkarshrai2811/redtrace/internal/repeater"
)

const (
	defaultConcurrency = 6
	progressEvery      = 25
	maxStoredTokens    = 500
)

// Status values for a capture's lifecycle.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusStopped   = "stopped"
	StatusError     = "error"
)

// Config describes a token-capture run.
type Config struct {
	Scheme      string
	Host        string
	Template    []byte
	HTTPVersion string
	Source      string // "cookie" | "regex"
	Selector    string // cookie name or regex pattern
	Target      int
}

// Store is the persistence the Sequencer needs.
type Store interface {
	PrepareSequencerRun(ctx context.Context, taskID string) error
	UpdateSequencerProgress(ctx context.Context, taskID string, collected int, tokens []byte) error
	SetSequencerResult(ctx context.Context, taskID, status string, report []byte) error
}

// Update is broadcast as a capture progresses.
type Update struct {
	Kind      string `json:"kind"` // "progress" | "status"
	TaskID    string `json:"taskId,omitempty"`
	Status    string `json:"status,omitempty"`
	Collected int    `json:"collected"`
	Target    int    `json:"target"`
}

type activeRun struct {
	cancel context.CancelFunc
}

// Sequencer runs token captures in the background. It is safe for concurrent use.
type Sequencer struct {
	sender *repeater.Engine
	store  Store
	notify func(Update)

	mu      sync.Mutex
	running map[string]*activeRun
	wg      sync.WaitGroup
}

// NewSequencer wires a Sequencer to its store and notifier.
func NewSequencer(store Store, notify func(Update)) *Sequencer {
	return &Sequencer{
		sender:  repeater.New(),
		store:   store,
		notify:  notify,
		running: make(map[string]*activeRun),
	}
}

// Start prepares and launches a capture in the background. It reports false (no
// error) if a capture for this task is already running.
func (s *Sequencer) Start(taskID string, cfg Config) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	if _, exists := s.running[taskID]; exists {
		s.mu.Unlock()
		cancel()
		return false, nil
	}
	run := &activeRun{cancel: cancel}
	s.running[taskID] = run
	s.mu.Unlock()

	bg := context.Background()
	if err := s.store.PrepareSequencerRun(bg, taskID); err != nil {
		s.release(taskID, run)
		cancel()
		return false, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.release(taskID, run)
			cancel()
		}()

		s.emit(Update{Kind: "status", TaskID: taskID, Status: StatusRunning, Target: cfg.Target})
		tokens := s.runCapture(ctx, bg, taskID, cfg)

		status := StatusCompleted
		switch {
		case ctx.Err() != nil:
			status = StatusStopped
		case len(tokens) == 0:
			// Nothing collected (dead target, wrong cookie name, or bad regex).
			status = StatusError
		}
		report, _ := json.Marshal(Analyze(tokens))
		_ = s.store.SetSequencerResult(bg, taskID, status, report)
		s.emit(Update{Kind: "status", TaskID: taskID, Status: status, Collected: len(tokens), Target: cfg.Target})
	}()
	return true, nil
}

func (s *Sequencer) runCapture(ctx, bg context.Context, taskID string, cfg Config) []string {
	target := cfg.Target
	if target <= 0 {
		target = 200
	}
	var re *regexp.Regexp
	if cfg.Source == "regex" {
		var err error
		if re, err = regexp.Compile(cfg.Selector); err != nil {
			return nil // an invalid selector yields no tokens; Analyze notes the empty sample
		}
	}

	var mu sync.Mutex
	tokens := make([]string, 0, target)
	fails := 0
	maxFails := target + 20 // give up if extraction keeps failing

	persist := func() {
		mu.Lock()
		n := len(tokens)
		snap := tokens
		if len(snap) > maxStoredTokens {
			snap = snap[:maxStoredTokens]
		}
		blob, _ := json.Marshal(snap)
		mu.Unlock()
		_ = s.store.UpdateSequencerProgress(bg, taskID, n, blob)
		s.emit(Update{Kind: "progress", TaskID: taskID, Status: StatusRunning, Collected: n, Target: target})
	}

	var wg sync.WaitGroup
	for w := 0; w < defaultConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				done := len(tokens) >= target || fails >= maxFails
				mu.Unlock()
				if done {
					return
				}
				resp, err := s.sender.Send(ctx, repeater.Request{
					Scheme: cfg.Scheme, Host: cfg.Host, Raw: cfg.Template,
					FollowRedirects: false, HTTPVersion: cfg.HTTPVersion,
				})
				if err != nil {
					mu.Lock()
					fails++
					mu.Unlock()
					continue
				}
				tok, ok := extractToken(cfg.Source, cfg.Selector, re, resp.Raw)
				mu.Lock()
				switch {
				case ok && tok != "" && len(tokens) < target:
					// Re-check the cap inside the same critical section as the append
					// so collected never overshoots target.
					tokens = append(tokens, tok)
					n := len(tokens)
					mu.Unlock()
					if n%progressEvery == 0 {
						persist()
					}
				case ok && tok != "":
					// Already at target; this extra sample is dropped, worker stops.
					mu.Unlock()
					return
				default:
					fails++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	if len(tokens) > target {
		tokens = tokens[:target]
	}
	out := append([]string(nil), tokens...)
	mu.Unlock()
	persist()
	return out
}

// extractToken pulls a token from a raw response by cookie name or regex.
func extractToken(source, selector string, re *regexp.Regexp, rawResp []byte) (string, bool) {
	switch source {
	case "cookie":
		headers := parseHeaders(rawResp)
		for _, sc := range headers.Values("Set-Cookie") {
			name, val := cookieNameValue(sc)
			if strings.EqualFold(name, selector) {
				return val, true
			}
		}
		return "", false
	case "regex":
		if re == nil {
			return "", false
		}
		m := re.FindSubmatch(rawResp)
		if m == nil {
			return "", false
		}
		if len(m) >= 2 {
			return string(m[1]), true
		}
		return string(m[0]), true
	}
	return "", false
}

func parseHeaders(raw []byte) http.Header {
	r := bufio.NewReader(bytes.NewReader(raw))
	if _, err := r.ReadString('\n'); err != nil {
		return http.Header{}
	}
	mh, _ := textproto.NewReader(r).ReadMIMEHeader()
	_, _ = io.Copy(io.Discard, r)
	return http.Header(mh)
}

func cookieNameValue(sc string) (string, string) {
	if i := strings.IndexByte(sc, ';'); i >= 0 {
		sc = sc[:i]
	}
	name, val, _ := strings.Cut(sc, "=")
	return strings.TrimSpace(name), strings.TrimSpace(val)
}

// Stop cancels a running capture, reporting whether it was running.
func (s *Sequencer) Stop(taskID string) bool {
	s.mu.Lock()
	run, ok := s.running[taskID]
	s.mu.Unlock()
	if ok {
		run.cancel()
	}
	return ok
}

// Shutdown cancels every running capture and waits for them to drain.
func (s *Sequencer) Shutdown(ctx context.Context) {
	s.mu.Lock()
	for _, run := range s.running {
		run.cancel()
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (s *Sequencer) release(taskID string, run *activeRun) {
	s.mu.Lock()
	if s.running[taskID] == run {
		delete(s.running, taskID)
	}
	s.mu.Unlock()
}

func (s *Sequencer) emit(u Update) {
	if s.notify != nil {
		s.notify(u)
	}
}
