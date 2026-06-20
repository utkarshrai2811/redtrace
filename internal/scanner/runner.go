package scanner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/utkarshrai2811/redtrace/internal/repeater"
)

const (
	defaultConcurrency = 8
	maxConcurrency     = 32
	progressEvery      = 32
)

// Status values for an active scan's lifecycle.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusStopped   = "stopped"
	StatusError     = "error"
)

// ActiveConfig describes an active scan: the request to probe and how to send
// it. Active probes are always sent without following redirects so Location can
// be inspected and reflection is detected on the immediate response.
type ActiveConfig struct {
	Scheme      string
	Host        string // host:port
	Template    []byte
	HTTPVersion string
}

// Store is the persistence the Scanner needs.
type Store interface {
	// AddIssue records a finding (taskID empty for passive), deduped by
	// fingerprint; it reports whether the issue was newly inserted.
	AddIssue(ctx context.Context, taskID string, is Issue) (bool, error)
	PrepareScanRun(ctx context.Context, taskID string, total int) error
	UpdateScanProgress(ctx context.Context, taskID string, completed, issues int) error
	SetScanStatus(ctx context.Context, taskID, status string) error
}

// IssueSummary is the compact, raw-byte-free view of a finding pushed over the
// WebSocket so the UI can show it live (and open it by id).
type IssueSummary struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Severity   Severity   `json:"severity"`
	Confidence Confidence `json:"confidence"`
	Scheme     string     `json:"scheme"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Path       string     `json:"path"`
	Method     string     `json:"method"`
	Param      string     `json:"param,omitempty"`
	Origin     string     `json:"origin"`
}

// Update is broadcast as scanning progresses: a new issue, or an active scan's
// progress/status transition.
type Update struct {
	Kind      string        `json:"kind"` // "issue" | "progress" | "status"
	Issue     *IssueSummary `json:"issue,omitempty"`
	TaskID    string        `json:"taskId,omitempty"`
	Status    string        `json:"status,omitempty"`
	Completed int           `json:"completed"`
	Total     int           `json:"total"`
	Issues    int           `json:"issues"`
}

// Scanner runs passive analysis on captured traffic and active scans in the
// background. It is safe for concurrent use.
type Scanner struct {
	sender *repeater.Engine
	store  Store
	notify func(Update)

	mu      sync.Mutex
	running map[string]*activeRun
	wg      sync.WaitGroup
}

type activeRun struct {
	cancel context.CancelFunc
}

// NewScanner wires a Scanner to its store and a progress notifier (e.g. the WS hub).
func NewScanner(store Store, notify func(Update)) *Scanner {
	return &Scanner{
		sender:  repeater.New(),
		store:   store,
		notify:  notify,
		running: make(map[string]*activeRun),
	}
}

// ScanPassive runs the passive checks against a captured exchange and records
// any new findings. It is safe to call from the proxy's persist goroutine.
func (s *Scanner) ScanPassive(ctx context.Context, t Target) {
	for _, is := range Passive(t) {
		is.ID = newID()
		inserted, err := s.store.AddIssue(ctx, "", is)
		if err != nil || !inserted {
			continue
		}
		s.emitIssue("", is)
	}
}

// PlanCount validates an active-scan template and returns how many requests it
// will send (one baseline plus a probe per insertion point per payload).
func PlanCount(template []byte) (int, error) {
	t, err := parseRawRequest(template)
	if err != nil {
		return 0, err
	}
	ips := t.insertions()
	if len(ips) == 0 {
		return 0, fmt.Errorf("no insertion points: the request has no query or form parameters to probe")
	}
	per := 0
	for _, c := range activeChecks {
		per += len(c.payloads(""))
	}
	return 1 + len(ips)*per, nil
}

// StartActive prepares and launches an active scan in the background, returning
// immediately. It reports false (no error) if a scan for this task is already
// running, so a double start cannot corrupt an in-flight run.
func (s *Scanner) StartActive(taskID string, cfg ActiveConfig, total int) (bool, error) {
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
	if err := s.store.PrepareScanRun(bg, taskID, total); err != nil {
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

		s.emit(Update{Kind: "status", TaskID: taskID, Status: StatusRunning, Total: total})
		completed, issues := s.runActive(ctx, bg, taskID, cfg, total)

		status := StatusCompleted
		if ctx.Err() != nil {
			status = StatusStopped
		}
		_ = s.store.SetScanStatus(bg, taskID, status)
		// Carry the final counts so the live UI shows the real result instead of
		// resetting to 0 on completion.
		s.emit(Update{Kind: "status", TaskID: taskID, Status: status, Completed: completed, Total: total, Issues: issues})
	}()
	return true, nil
}

func (s *Scanner) runActive(ctx, bg context.Context, taskID string, cfg ActiveConfig, total int) (int, int) {
	t, err := parseRawRequest(cfg.Template)
	if err != nil {
		return 0, 0
	}
	ips := t.insertions()
	token := randToken()

	completed := 0
	issues := 0
	flush := func(force bool) {
		if force || completed%progressEvery == 0 {
			_ = s.store.UpdateScanProgress(bg, taskID, completed, issues)
			s.emit(Update{Kind: "progress", TaskID: taskID, Status: StatusRunning, Completed: completed, Total: total, Issues: issues})
		}
	}

	// Baseline: the original request, used to suppress findings already present
	// before any payload was injected. If it fails, the diff-based detectors are
	// skipped (an empty baseline would defeat their suppression).
	base, _, baseErr := s.sendProbe(ctx, cfg, cfg.Template)
	baselineOK := baseErr == nil
	completed++
	flush(false)

	type job struct {
		ip      insertion
		check   int
		payload string
	}
	var jobs []job
	for _, ip := range ips {
		for ci, c := range activeChecks {
			for _, p := range c.payloads(token) {
				jobs = append(jobs, job{ip: ip, check: ci, payload: p})
			}
		}
	}

	jobCh := make(chan job)
	resCh := make(chan *Issue)
	var wg sync.WaitGroup
	for w := 0; w < defaultConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				c := activeChecks[j.check]
				// A failed baseline disables the diff-based suppression, so skip
				// those checks rather than emit false positives.
				if c.needsBaseline && !baselineOK {
					resCh <- nil
					continue
				}
				raw := t.build(j.ip, j.payload)
				resp, rawResp, err := s.sendProbe(ctx, cfg, raw)
				if err != nil {
					resCh <- nil
					continue
				}
				ev, ok := c.detect(j.payload, token, resp, base)
				if !ok {
					resCh <- nil
					continue
				}
				is := buildActiveIssue(taskID, cfg, t, j.ip, c, j.payload, ev, raw, rawResp)
				resCh <- &is
			}
		}()
	}
	go func() {
		defer close(jobCh)
		for _, j := range jobs {
			select {
			case jobCh <- j:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resCh)
	}()

	for r := range resCh {
		completed++
		if r != nil {
			r.ID = newID()
			if inserted, err := s.store.AddIssue(bg, taskID, *r); err == nil && inserted {
				issues++
				s.emitIssue(taskID, *r)
			}
		}
		flush(false)
	}
	flush(true)
	return completed, issues
}

func (s *Scanner) sendProbe(ctx context.Context, cfg ActiveConfig, raw []byte) (probeResp, []byte, error) {
	resp, err := s.sender.Send(ctx, repeater.Request{
		Scheme:          cfg.Scheme,
		Host:            cfg.Host,
		Raw:             raw,
		FollowRedirects: false,
		HTTPVersion:     cfg.HTTPVersion,
	})
	if err != nil {
		return probeResp{}, nil, err
	}
	return toProbeResp(resp.Raw, resp.StatusCode), resp.Raw, nil
}

func buildActiveIssue(taskID string, cfg ActiveConfig, t *reqTemplate, ip insertion, c activeCheck, payload, evidence string, rawReq, rawResp []byte) Issue {
	host, port := hostPort(cfg.Host, cfg.Scheme)
	return Issue{
		Type: c.id, Name: c.name, Severity: c.severity, Confidence: c.confidence,
		Scheme: cfg.Scheme, Host: host, Port: port, Path: t.path, Method: t.method,
		Param: ip.Name, Payload: payload,
		Detail:      fmt.Sprintf("The %s parameter %q is vulnerable to %s.", ip.Kind, ip.Name, c.name),
		Evidence:    truncate(evidence, 512),
		Remediation: c.remediation,
		Origin:      OriginActive,
		// Scope the active fingerprint to the task so two scans of the same
		// endpoint each record (and count) their own findings; the task-scoped
		// PrepareScanRun reset then stays consistent with the dedup scope.
		Fingerprint: fingerprint("active", taskID, c.id, cfg.Host, t.path, ip.Kind, ip.Name),
		RequestRaw:  rawReq, ResponseRaw: rawResp,
	}
}

// Stop cancels a running active scan, reporting whether it was running.
func (s *Scanner) Stop(taskID string) bool {
	s.mu.Lock()
	run, ok := s.running[taskID]
	s.mu.Unlock()
	if ok {
		run.cancel()
	}
	return ok
}

// Shutdown cancels every running active scan and waits for the goroutines to
// finish persisting, bounded by ctx.
func (s *Scanner) Shutdown(ctx context.Context) {
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

// release removes the run from the map only if it is still the active one.
func (s *Scanner) release(taskID string, run *activeRun) {
	s.mu.Lock()
	if s.running[taskID] == run {
		delete(s.running, taskID)
	}
	s.mu.Unlock()
}

func (s *Scanner) emitIssue(taskID string, is Issue) {
	s.emit(Update{Kind: "issue", TaskID: taskID, Issue: summaryOf(is)})
}

// emit delivers an update to the notifier, if one is configured.
func (s *Scanner) emit(u Update) {
	if s.notify != nil {
		s.notify(u)
	}
}

func summaryOf(is Issue) *IssueSummary {
	return &IssueSummary{
		ID: is.ID, Type: is.Type, Name: is.Name, Severity: is.Severity, Confidence: is.Confidence,
		Scheme: is.Scheme, Host: is.Host, Port: is.Port, Path: is.Path, Method: is.Method,
		Param: is.Param, Origin: is.Origin,
	}
}

func hostPort(authority, scheme string) (string, int) {
	host, portStr, err := net.SplitHostPort(authority)
	if err != nil {
		if scheme == "https" {
			return authority, 443
		}
		return authority, 80
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func randToken() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
