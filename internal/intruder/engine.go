package intruder

import (
	"context"
	"sync"

	"github.com/utkarshrai2811/redtrace/internal/repeater"
)

const (
	defaultConcurrency = 10
	maxConcurrency     = 64
)

// Engine builds and sends the requests for an attack, reusing the Repeater
// request-replay engine as its transport.
type Engine struct {
	sender *repeater.Engine
}

// NewEngine returns a ready Engine.
func NewEngine() *Engine {
	return &Engine{sender: repeater.New()}
}

// Run executes cfg, invoking onResult for each completed request. onResult is
// called from a single goroutine (never concurrently), so callers need no
// locking. It returns the number of requests dispatched, or ctx.Err() if the
// attack was cancelled.
func (e *Engine) Run(ctx context.Context, cfg Config, onResult func(Result)) (int, error) {
	t := parseTemplate(cfg.Template)

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if concurrency > maxConcurrency {
		concurrency = maxConcurrency
	}

	type job struct {
		index    int
		payloads []string
	}
	jobs := make(chan job)
	results := make(chan Result)

	// Producer: generate payload vectors and feed the workers; stop on cancel.
	go func() {
		defer close(jobs)
		i := 0
		forEachJob(cfg, func(payloads []string) bool {
			select {
			case jobs <- job{index: i, payloads: payloads}:
				i++
				return true
			case <-ctx.Done():
				return false
			}
		})
	}()

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- e.send(ctx, t, cfg, j.index, j.payloads)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	dispatched := 0
	for r := range results {
		dispatched++
		onResult(r)
	}
	if err := ctx.Err(); err != nil {
		return dispatched, err
	}
	return dispatched, nil
}

func (e *Engine) send(ctx context.Context, t template, cfg Config, index int, payloads []string) Result {
	raw := t.build(payloads)
	res := Result{Index: index, Payloads: payloads, RequestRaw: raw}
	resp, err := e.sender.Send(ctx, repeater.Request{
		Scheme:          cfg.Scheme,
		Host:            cfg.Host,
		Raw:             raw,
		FollowRedirects: cfg.FollowRedirects,
		HTTPVersion:     cfg.HTTPVersion,
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.StatusCode = resp.StatusCode
	res.DurationMs = resp.DurationMs
	res.ResponseRaw = resp.Raw
	res.Length = len(resp.Raw)
	return res
}

// Status values for an attack's lifecycle.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusStopped   = "stopped"
	StatusError     = "error"
)

// Store is the persistence the Runner needs to record an attack's progress.
type Store interface {
	AddIntruderResult(ctx context.Context, attackID string, r Result) error
	UpdateIntruderProgress(ctx context.Context, attackID string, completed int) error
	SetIntruderStatus(ctx context.Context, attackID, status string) error
}

// Update is broadcast as an attack progresses. Result is set on per-request
// updates and nil on lifecycle (status) transitions.
type Update struct {
	AttackID  string  `json:"attackId"`
	Status    string  `json:"status"`
	Completed int     `json:"completed"`
	Total     int     `json:"total"`
	Result    *Result `json:"result,omitempty"`
}

// Runner manages running attacks: it executes each in the background, persists
// every result, and reports progress through a notifier. It is safe for
// concurrent use.
type Runner struct {
	engine *Engine
	store  Store
	notify func(Update)

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// NewRunner wires a Runner to its store and a progress notifier (e.g. the WS hub).
func NewRunner(store Store, notify func(Update)) *Runner {
	return &Runner{
		engine:  NewEngine(),
		store:   store,
		notify:  notify,
		running: make(map[string]context.CancelFunc),
	}
}

// Start launches cfg in the background and returns immediately. total is the
// precomputed request count (Count(cfg)).
func (r *Runner) Start(attackID string, cfg Config, total int) {
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.running[attackID] = cancel
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.running, attackID)
			r.mu.Unlock()
			cancel()
		}()

		bg := context.Background()
		_ = r.store.SetIntruderStatus(bg, attackID, StatusRunning)
		r.emit(Update{AttackID: attackID, Status: StatusRunning, Total: total})

		completed := 0
		_, runErr := r.engine.Run(ctx, cfg, func(res Result) {
			completed++
			// Persist with a background context so results already produced are
			// not lost when the attack is cancelled mid-flight.
			_ = r.store.AddIntruderResult(bg, attackID, res)
			_ = r.store.UpdateIntruderProgress(bg, attackID, completed)
			rc := res
			r.emit(Update{AttackID: attackID, Status: StatusRunning, Completed: completed, Total: total, Result: &rc})
		})

		status := StatusCompleted
		switch {
		case ctx.Err() != nil:
			status = StatusStopped
		case runErr != nil:
			status = StatusError
		}
		_ = r.store.SetIntruderStatus(bg, attackID, status)
		r.emit(Update{AttackID: attackID, Status: status, Completed: completed, Total: total})
	}()
}

// Stop cancels a running attack, reporting whether it was running.
func (r *Runner) Stop(attackID string) bool {
	r.mu.Lock()
	cancel, ok := r.running[attackID]
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (r *Runner) emit(u Update) {
	if r.notify != nil {
		r.notify(u)
	}
}
