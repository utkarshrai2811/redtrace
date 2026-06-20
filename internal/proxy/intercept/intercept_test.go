package intercept

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestInterceptor_DisabledForwardsImmediately(t *testing.T) {
	var n int64
	ic := NewInterceptor(func() string { return strconv.FormatInt(atomic.AddInt64(&n, 1), 10) })

	d := ic.Hold(context.Background(), &Held{Direction: DirRequest})
	if d.Action != ActionForward {
		t.Fatalf("disabled interceptor should forward, got %q", d.Action)
	}
}

func TestInterceptor_HoldAndResolve(t *testing.T) {
	var n int64
	ic := NewInterceptor(func() string { return strconv.FormatInt(atomic.AddInt64(&n, 1), 10) })
	ic.SetEnabled(true)

	decided := make(chan Decision, 1)
	go func() {
		decided <- ic.Hold(context.Background(), &Held{Direction: DirRequest, Method: "GET"})
	}()

	// Wait for the item to appear in the queue.
	var id string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if q := ic.Queue(); len(q) == 1 {
			id = q[0].ID
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("held item never appeared in queue")
	}

	if err := ic.Resolve(id, Decision{Action: ActionDrop}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	select {
	case d := <-decided:
		if d.Action != ActionDrop {
			t.Errorf("got %q, want drop", d.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Hold did not return after Resolve")
	}
	if len(ic.Queue()) != 0 {
		t.Error("queue should be empty after resolve")
	}
}

func TestInterceptor_DisablingForwardsHeld(t *testing.T) {
	ic := NewInterceptor(func() string { return "x" })
	ic.SetEnabled(true)

	decided := make(chan Decision, 1)
	go func() { decided <- ic.Hold(context.Background(), &Held{Direction: DirRequest}) }()
	time.Sleep(50 * time.Millisecond)

	ic.SetEnabled(false) // should release held items as forwards
	select {
	case d := <-decided:
		if d.Action != ActionForward {
			t.Errorf("got %q, want forward", d.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disabling did not release held item")
	}
}

func TestInterceptor_DrainAll(t *testing.T) {
	var n int64
	ic := NewInterceptor(func() string { return strconv.FormatInt(atomic.AddInt64(&n, 1), 10) })
	ic.SetEnabled(true)

	results := make(chan Decision, 3)
	for range 3 {
		go func() { results <- ic.Hold(context.Background(), &Held{Direction: DirRequest}) }()
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(ic.Queue()) < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(ic.Queue()) != 3 {
		t.Fatalf("expected 3 held, got %d", len(ic.Queue()))
	}

	if drained := ic.DrainAll(ActionForward); drained != 3 {
		t.Errorf("DrainAll returned %d, want 3", drained)
	}
	for range 3 {
		select {
		case d := <-results:
			if d.Action != ActionForward {
				t.Errorf("got %q, want forward", d.Action)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Hold did not return after DrainAll")
		}
	}
	if len(ic.Queue()) != 0 {
		t.Errorf("queue not empty after DrainAll: %d", len(ic.Queue()))
	}
}

func TestRuleSet_ApplyRequestHeaderAndBody(t *testing.T) {
	rs := NewRuleSet()
	err := rs.SetRules([]Rule{
		{ID: "1", Enabled: true, Part: PartRequestHeader, MatchType: Literal, Match: "Mozilla", Replace: "RedTrace", Priority: 1},
		{ID: "2", Enabled: true, Part: PartRequestBody, MatchType: Regex, Match: `id=\d+`, Replace: "id=0", Priority: 2},
	})
	if err != nil {
		t.Fatalf("set rules: %v", err)
	}

	req, _ := http.NewRequest("POST", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	body := rs.ApplyRequest(req, []byte("user=admin&id=42"))

	if got := req.Header.Get("User-Agent"); got != "RedTrace/5.0" {
		t.Errorf("User-Agent = %q, want RedTrace/5.0", got)
	}
	if string(body) != "user=admin&id=0" {
		t.Errorf("body = %q, want user=admin&id=0", body)
	}
}

func TestRuleSet_DisabledRuleIgnored(t *testing.T) {
	rs := NewRuleSet()
	_ = rs.SetRules([]Rule{{ID: "1", Enabled: false, Part: PartResponseBody, MatchType: Literal, Match: "a", Replace: "b"}})
	resp := &http.Response{Header: http.Header{}}
	if got := rs.ApplyResponse(resp, []byte("aaa")); string(got) != "aaa" {
		t.Errorf("disabled rule applied: %q", got)
	}
}

func TestInterceptor_TogglingResponsesForwardsHeldResponses(t *testing.T) {
	ic := NewInterceptor(func() string { return "r" })
	ic.SetEnabled(true)
	ic.SetInterceptResponses(true)

	decided := make(chan Decision, 1)
	go func() { decided <- ic.Hold(context.Background(), &Held{Direction: DirResponse}) }()
	time.Sleep(50 * time.Millisecond)
	if len(ic.Queue()) != 1 {
		t.Fatalf("expected the response to be held, queue=%d", len(ic.Queue()))
	}

	ic.SetInterceptResponses(false) // must forward the held response, not strand it
	select {
	case d := <-decided:
		if d.Action != ActionForward {
			t.Errorf("got %q, want forward", d.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("toggling responses off did not release the held response")
	}
	if len(ic.Queue()) != 0 {
		t.Errorf("queue not empty after toggle: %d", len(ic.Queue()))
	}
}

func TestInterceptor_ContextCancelDrops(t *testing.T) {
	ic := NewInterceptor(func() string { return "c" })
	ic.SetEnabled(true)

	ctx, cancel := context.WithCancel(context.Background())
	decided := make(chan Decision, 1)
	go func() { decided <- ic.Hold(ctx, &Held{Direction: DirRequest}) }()
	time.Sleep(50 * time.Millisecond)
	if len(ic.Queue()) != 1 {
		t.Fatalf("expected the request to be held, queue=%d", len(ic.Queue()))
	}

	cancel() // a cancelled context (e.g. client disconnect / shutdown) drops the item
	select {
	case d := <-decided:
		if d.Action != ActionDrop {
			t.Errorf("got %q, want drop", d.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context cancellation did not release the held item")
	}
	if len(ic.Queue()) != 0 {
		t.Errorf("queue not empty after cancel: %d", len(ic.Queue()))
	}
}
