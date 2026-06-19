package intercept

import (
	"context"
	"errors"
	"sync"
)

// Action is the resolution of a held request or response.
type Action string

const (
	ActionForward Action = "forward"
	ActionDrop    Action = "drop"
)

// Direction distinguishes a held request from a held response.
type Direction string

const (
	DirRequest  Direction = "request"
	DirResponse Direction = "response"
)

// Decision resolves a held item. Raw, if non-nil, replaces the original bytes
// (an edited request or response) before forwarding.
type Decision struct {
	Action Action
	Raw    []byte
}

// Held is an intercepted item awaiting an operator decision.
type Held struct {
	ID         string    `json:"id"`
	Direction  Direction `json:"direction"`
	Method     string    `json:"method,omitempty"`
	URL        string    `json:"url,omitempty"`
	Host       string    `json:"host,omitempty"`
	StatusCode int       `json:"statusCode,omitempty"`
	Raw        []byte    `json:"raw"`

	decision chan Decision
}

// ErrUnknownItem is returned when resolving an item not present in the queue.
var ErrUnknownItem = errors.New("intercept: unknown item")

// Interceptor coordinates manual interception: when enabled it holds in-scope
// requests (and optionally responses) until an operator forwards or drops them.
// It is safe for concurrent use.
type Interceptor struct {
	mu                 sync.Mutex
	enabled            bool
	interceptResponses bool
	queue              map[string]*Held
	order              []string
	notify             func()
	idFn               func() string
}

// NewInterceptor returns an Interceptor that mints item IDs with idFn.
func NewInterceptor(idFn func() string) *Interceptor {
	return &Interceptor{
		queue: make(map[string]*Held),
		idFn:  idFn,
	}
}

// SetNotifier registers a callback invoked whenever the queue changes, so the
// API layer can push updates to connected clients.
func (i *Interceptor) SetNotifier(fn func()) {
	i.mu.Lock()
	i.notify = fn
	i.mu.Unlock()
}

// Enabled reports whether interception is on.
func (i *Interceptor) Enabled() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.enabled
}

// SetEnabled toggles interception. Disabling forwards everything currently held.
func (i *Interceptor) SetEnabled(on bool) {
	i.mu.Lock()
	i.enabled = on
	var release []*Held
	if !on {
		for _, id := range i.order {
			release = append(release, i.queue[id])
		}
		i.queue = make(map[string]*Held)
		i.order = nil
	}
	notify := i.notify
	i.mu.Unlock()

	for _, h := range release {
		h.decision <- Decision{Action: ActionForward}
	}
	if notify != nil {
		notify()
	}
}

// InterceptResponses reports whether responses are also held.
func (i *Interceptor) InterceptResponses() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.interceptResponses
}

// SetInterceptResponses controls whether responses are held when interception
// is enabled.
func (i *Interceptor) SetInterceptResponses(on bool) {
	i.mu.Lock()
	i.interceptResponses = on
	i.mu.Unlock()
}

// Hold blocks until the operator resolves the item, returning the Decision. If
// interception is disabled (or responses are not being intercepted) it returns
// a forward decision immediately. If ctx is cancelled while held, the item is
// dropped.
func (i *Interceptor) Hold(ctx context.Context, h *Held) Decision {
	i.mu.Lock()
	active := i.enabled && (h.Direction == DirRequest || i.interceptResponses)
	if !active {
		i.mu.Unlock()
		return Decision{Action: ActionForward}
	}
	h.ID = i.idFn()
	h.decision = make(chan Decision, 1)
	i.queue[h.ID] = h
	i.order = append(i.order, h.ID)
	notify := i.notify
	i.mu.Unlock()

	if notify != nil {
		notify()
	}

	select {
	case d := <-h.decision:
		return d
	case <-ctx.Done():
		i.remove(h.ID)
		return Decision{Action: ActionDrop}
	}
}

// Queue returns a snapshot of held items in arrival order.
func (i *Interceptor) Queue() []*Held {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]*Held, 0, len(i.order))
	for _, id := range i.order {
		out = append(out, i.queue[id])
	}
	return out
}

// Resolve delivers a decision to a held item.
func (i *Interceptor) Resolve(id string, d Decision) error {
	i.mu.Lock()
	h, ok := i.queue[id]
	if !ok {
		i.mu.Unlock()
		return ErrUnknownItem
	}
	i.deleteLocked(id)
	notify := i.notify
	i.mu.Unlock()

	h.decision <- d
	if notify != nil {
		notify()
	}
	return nil
}

func (i *Interceptor) remove(id string) {
	i.mu.Lock()
	i.deleteLocked(id)
	notify := i.notify
	i.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (i *Interceptor) deleteLocked(id string) {
	delete(i.queue, id)
	for idx, v := range i.order {
		if v == id {
			i.order = append(i.order[:idx], i.order[idx+1:]...)
			break
		}
	}
}
