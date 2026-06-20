package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/intruder"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// attackConfig is the JSON persisted in intruder_attacks.config (the parts of an
// attack that aren't first-class columns).
type attackConfig struct {
	PayloadSets     []intruder.PayloadSet `json:"payloadSets"`
	FollowRedirects bool                  `json:"followRedirects"`
	HTTPVersion     string                `json:"httpVersion"`
	Concurrency     int                   `json:"concurrency"`
}

// intruderBody is the create/update request payload.
type intruderBody struct {
	Name            string                `json:"name"`
	Scheme          string                `json:"scheme"`
	Host            string                `json:"host"`
	Template        []byte                `json:"template"` // raw request, base64 in JSON
	Type            string                `json:"type"`
	PayloadSets     []intruder.PayloadSet `json:"payloadSets"`
	FollowRedirects bool                  `json:"followRedirects"`
	HTTPVersion     string                `json:"httpVersion"`
	Concurrency     int                   `json:"concurrency"`
}

func (b intruderBody) normalized() intruderBody {
	if b.Scheme == "" {
		b.Scheme = "https"
	}
	if b.HTTPVersion == "" {
		b.HTTPVersion = "HTTP/1.1"
	}
	if b.Type == "" {
		b.Type = string(intruder.Sniper)
	}
	if b.Name == "" {
		b.Name = "Attack"
	}
	return b
}

type attackView struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Scheme          string                `json:"scheme"`
	Host            string                `json:"host"`
	Template        []byte                `json:"template"`
	Type            string                `json:"type"`
	PayloadSets     []intruder.PayloadSet `json:"payloadSets"`
	FollowRedirects bool                  `json:"followRedirects"`
	HTTPVersion     string                `json:"httpVersion"`
	Concurrency     int                   `json:"concurrency"`
	Status          string                `json:"status"`
	Total           int                   `json:"total"`
	Completed       int                   `json:"completed"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

func toAttackView(a *models.IntruderAttack) attackView {
	var cfg attackConfig
	_ = json.Unmarshal(a.Config, &cfg)
	if cfg.PayloadSets == nil {
		cfg.PayloadSets = []intruder.PayloadSet{}
	}
	return attackView{
		ID: a.ID, Name: a.Name, Scheme: a.Scheme, Host: a.Host, Template: a.Template,
		Type: a.AttackType, PayloadSets: cfg.PayloadSets, FollowRedirects: cfg.FollowRedirects,
		HTTPVersion: cfg.HTTPVersion, Concurrency: cfg.Concurrency,
		Status: a.Status, Total: a.Total, Completed: a.Completed,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

type resultView struct {
	ID         string   `json:"id"`
	Index      int      `json:"index"`
	Payloads   []string `json:"payloads"`
	StatusCode int      `json:"statusCode"`
	Length     int      `json:"length"`
	DurationMs int64    `json:"durationMs"`
	Error      string   `json:"error,omitempty"`
}

func toResultView(r *models.IntruderResult) resultView {
	var payloads []string
	_ = json.Unmarshal(r.Payloads, &payloads)
	return resultView{
		ID: r.ID, Index: r.Index, Payloads: payloads, StatusCode: r.StatusCode,
		Length: r.Length, DurationMs: r.DurationMs, Error: r.Error,
	}
}

// configJSON marshals the non-column attack fields for storage.
func (b intruderBody) configJSON() []byte {
	out, _ := json.Marshal(attackConfig{
		PayloadSets: b.PayloadSets, FollowRedirects: b.FollowRedirects,
		HTTPVersion: b.HTTPVersion, Concurrency: b.Concurrency,
	})
	return out
}

// ListIntruderAttacks handles GET /api/intruder/attacks.
func (a *API) ListIntruderAttacks(w http.ResponseWriter, r *http.Request) {
	attacks, err := a.Store.ListIntruderAttacks(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]attackView, 0, len(attacks))
	for _, at := range attacks {
		views = append(views, toAttackView(at))
	}
	writeJSON(w, http.StatusOK, views)
}

// CreateIntruderAttack handles POST /api/intruder/attacks, saving a draft.
func (a *API) CreateIntruderAttack(w http.ResponseWriter, r *http.Request) {
	var body intruderBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	attack := &models.IntruderAttack{
		ID: storage.NewID(), Name: body.Name, Scheme: body.Scheme, Host: body.Host,
		Template: body.Template, AttackType: body.Type, Config: body.configJSON(),
		Status: intruder.StatusPending,
	}
	if err := a.Store.CreateIntruderAttack(r.Context(), attack); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, toAttackView(attack))
}

// GetIntruderAttack handles GET /api/intruder/attacks/{id}, returning the attack
// and its results.
func (a *API) GetIntruderAttack(w http.ResponseWriter, r *http.Request) {
	attack, err := a.Store.GetIntruderAttack(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such attack")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	results, err := a.Store.ListIntruderResults(r.Context(), attack.ID)
	if err != nil {
		a.serverError(w, "results_failed", err)
		return
	}
	views := make([]resultView, 0, len(results))
	for _, res := range results {
		views = append(views, toResultView(res))
	}
	writeJSON(w, http.StatusOK, map[string]any{"attack": toAttackView(attack), "results": views})
}

// UpdateIntruderAttack handles PUT /api/intruder/attacks/{id}, editing a draft.
func (a *API) UpdateIntruderAttack(w http.ResponseWriter, r *http.Request) {
	attack, err := a.Store.GetIntruderAttack(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such attack")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	var body intruderBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	attack.Name, attack.Scheme, attack.Host = body.Name, body.Scheme, body.Host
	attack.Template, attack.AttackType, attack.Config = body.Template, body.Type, body.configJSON()
	if err := a.Store.UpdateIntruderAttack(r.Context(), attack); err != nil {
		a.serverError(w, "update_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, toAttackView(attack))
}

// DeleteIntruderAttack handles DELETE /api/intruder/attacks/{id}.
func (a *API) DeleteIntruderAttack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Intruder.Stop(id) // stop first if it is running
	err := a.Store.DeleteIntruderAttack(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such attack")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartIntruderAttack handles POST /api/intruder/attacks/{id}/start.
func (a *API) StartIntruderAttack(w http.ResponseWriter, r *http.Request) {
	attack, err := a.Store.GetIntruderAttack(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such attack")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}

	var cfg attackConfig
	_ = json.Unmarshal(attack.Config, &cfg)
	icfg := intruder.Config{
		Scheme: attack.Scheme, Host: attack.Host, Template: attack.Template,
		Type: intruder.AttackType(attack.AttackType), PayloadSets: cfg.PayloadSets,
		FollowRedirects: cfg.FollowRedirects, HTTPVersion: cfg.HTTPVersion, Concurrency: cfg.Concurrency,
	}
	if err := intruder.Validate(icfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_attack", err.Error())
		return
	}

	total := intruder.Count(icfg)
	if err := a.Store.PrepareIntruderRun(r.Context(), attack.ID, total); err != nil {
		a.serverError(w, "start_failed", err)
		return
	}
	a.Intruder.Start(attack.ID, icfg, total)

	// Progress and the final status flow to clients over the intruder WS frames.
	attack.Status, attack.Total, attack.Completed = intruder.StatusRunning, total, 0
	writeJSON(w, http.StatusOK, toAttackView(attack))
}

// StopIntruderAttack handles POST /api/intruder/attacks/{id}/stop.
func (a *API) StopIntruderAttack(w http.ResponseWriter, r *http.Request) {
	if !a.Intruder.Stop(r.PathValue("id")) {
		writeError(w, http.StatusConflict, "not_running", "attack is not running")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
}

// GetIntruderResult handles GET /api/intruder/results/{id}, returning one result
// with its raw request and response bytes.
func (a *API) GetIntruderResult(w http.ResponseWriter, r *http.Request) {
	res, err := a.Store.GetIntruderResult(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such result")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"result":      toResultView(res),
		"requestRaw":  res.RequestRaw,
		"responseRaw": res.ResponseRaw,
	})
}

// IntruderStore adapts the storage layer to the intruder.Store interface the
// runner uses, translating an intruder.Result into a persisted row.
type IntruderStore struct {
	DB *storage.DB
}

func (s IntruderStore) AddIntruderResult(ctx context.Context, attackID string, r intruder.Result) error {
	payloads, _ := json.Marshal(r.Payloads)
	return s.DB.AddIntruderResult(ctx, &models.IntruderResult{
		ID: storage.NewID(), AttackID: attackID, Index: r.Index, Payloads: payloads,
		StatusCode: r.StatusCode, Length: r.Length, DurationMs: r.DurationMs,
		RequestRaw: r.RequestRaw, ResponseRaw: r.ResponseRaw, Error: r.Error,
	})
}

func (s IntruderStore) UpdateIntruderProgress(ctx context.Context, attackID string, completed int) error {
	return s.DB.UpdateIntruderProgress(ctx, attackID, completed)
}

func (s IntruderStore) SetIntruderStatus(ctx context.Context, attackID, status string) error {
	return s.DB.SetIntruderStatus(ctx, attackID, status)
}
