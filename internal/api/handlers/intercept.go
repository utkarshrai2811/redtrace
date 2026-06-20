package handlers

import (
	"errors"
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/proxy/intercept"
)

// HeldItem is the JSON view of an intercepted request/response. Raw is
// base64-encoded by the JSON encoder.
type HeldItem struct {
	ID         string `json:"id"`
	Direction  string `json:"direction"`
	Method     string `json:"method,omitempty"`
	URL        string `json:"url,omitempty"`
	Host       string `json:"host,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Raw        []byte `json:"raw"`
}

// InterceptStatus handles GET /api/intercept.
func (a *API) InterceptStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.interceptState())
}

type updateInterceptBody struct {
	Enabled            *bool `json:"enabled"`
	InterceptResponses *bool `json:"interceptResponses"`
}

// UpdateIntercept handles PUT /api/intercept to toggle interception.
func (a *API) UpdateIntercept(w http.ResponseWriter, r *http.Request) {
	var body updateInterceptBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.InterceptResponses != nil {
		a.Interceptor.SetInterceptResponses(*body.InterceptResponses)
	}
	if body.Enabled != nil {
		a.Interceptor.SetEnabled(*body.Enabled)
	}
	a.PublishIntercept()
	writeJSON(w, http.StatusOK, a.interceptState())
}

type resolveBody struct {
	Raw []byte `json:"raw"`
}

// ForwardItem handles POST /api/intercept/{id}/forward, optionally with edited
// raw bytes in the body.
func (a *API) ForwardItem(w http.ResponseWriter, r *http.Request) {
	var body resolveBody
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}
	a.resolve(w, r.PathValue("id"), intercept.Decision{Action: intercept.ActionForward, Raw: body.Raw})
}

// DropItem handles POST /api/intercept/{id}/drop.
func (a *API) DropItem(w http.ResponseWriter, r *http.Request) {
	a.resolve(w, r.PathValue("id"), intercept.Decision{Action: intercept.ActionDrop})
}

func (a *API) resolve(w http.ResponseWriter, id string, d intercept.Decision) {
	err := a.Interceptor.Resolve(id, d)
	if errors.Is(err, intercept.ErrUnknownItem) {
		writeError(w, http.StatusNotFound, "not_found", "no such intercepted item")
		return
	}
	if err != nil {
		a.serverError(w, "resolve_failed", err)
		return
	}
	a.PublishIntercept()
	writeJSON(w, http.StatusOK, a.interceptState())
}

// ForwardAll handles POST /api/intercept/forward-all, draining the queue.
func (a *API) ForwardAll(w http.ResponseWriter, r *http.Request) {
	a.Interceptor.DrainAll(intercept.ActionForward)
	a.PublishIntercept()
	writeJSON(w, http.StatusOK, a.interceptState())
}

// DropAll handles POST /api/intercept/drop-all, dropping the whole queue.
func (a *API) DropAll(w http.ResponseWriter, r *http.Request) {
	a.Interceptor.DrainAll(intercept.ActionDrop)
	a.PublishIntercept()
	writeJSON(w, http.StatusOK, a.interceptState())
}

// GetRules handles GET /api/intercept/rules.
func (a *API) GetRules(w http.ResponseWriter, r *http.Request) {
	rules := a.Rules.Rules()
	if rules == nil {
		rules = []intercept.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// PutRules handles PUT /api/intercept/rules, replacing the whole rule set.
func (a *API) PutRules(w http.ResponseWriter, r *http.Request) {
	var rules []intercept.Rule
	if err := decodeJSON(r, &rules); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := a.Rules.SetRules(rules); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a.Rules.Rules())
}

// PublishIntercept broadcasts the current interception state to WS clients. It
// is also registered as the interceptor's change notifier.
func (a *API) PublishIntercept() {
	if a.Hub != nil {
		a.Hub.BroadcastIntercept(a.interceptState())
	}
}

func (a *API) interceptState() InterceptState {
	queue := a.Interceptor.Queue()
	items := make([]HeldItem, 0, len(queue))
	for _, h := range queue {
		items = append(items, HeldItem{
			ID: h.ID, Direction: string(h.Direction), Method: h.Method,
			URL: h.URL, Host: h.Host, StatusCode: h.StatusCode, Raw: h.Raw,
		})
	}
	return InterceptState{
		Enabled:            a.Interceptor.Enabled(),
		InterceptResponses: a.Interceptor.InterceptResponses(),
		Queue:              items,
		Count:              len(items),
	}
}
