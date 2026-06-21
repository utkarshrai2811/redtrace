package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/sequencer"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// seqBody is the create payload for a token-capture task.
type seqBody struct {
	Name        string `json:"name"`
	Scheme      string `json:"scheme"`
	Host        string `json:"host"`
	Template    []byte `json:"template"` // raw request, base64 in JSON
	HTTPVersion string `json:"httpVersion"`
	Source      string `json:"source"`   // "cookie" | "regex"
	Selector    string `json:"selector"` // cookie name or regex
	Target      int    `json:"target"`
}

func (b seqBody) normalized() seqBody {
	if b.Name == "" {
		b.Name = "Sequencer"
	}
	if b.Scheme == "" {
		b.Scheme = "https"
	}
	if b.HTTPVersion == "" {
		b.HTTPVersion = "HTTP/1.1"
	}
	if b.Source != "regex" {
		b.Source = "cookie"
	}
	if b.Target <= 0 {
		b.Target = 200
	}
	if b.Target > 20000 {
		b.Target = 20000
	}
	return b
}

type seqTaskView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Scheme      string    `json:"scheme"`
	Host        string    `json:"host"`
	Template    []byte    `json:"template"`
	HTTPVersion string    `json:"httpVersion"`
	Source      string    `json:"source"`
	Selector    string    `json:"selector"`
	Target      int       `json:"target"`
	Status      string    `json:"status"`
	Collected   int       `json:"collected"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toSeqTaskView(t *models.SequencerTask) seqTaskView {
	return seqTaskView{
		ID: t.ID, Name: t.Name, Scheme: t.Scheme, Host: t.Host, Template: t.Template,
		HTTPVersion: t.HTTPVersion, Source: t.Source, Selector: t.Selector, Target: t.Target,
		Status: t.Status, Collected: t.Collected, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// ListSequencerTasks handles GET /api/sequencer/tasks.
func (a *API) ListSequencerTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.Store.ListSequencerTasks(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]seqTaskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, toSeqTaskView(t))
	}
	writeJSON(w, http.StatusOK, views)
}

// CreateSequencerTask handles POST /api/sequencer/tasks.
func (a *API) CreateSequencerTask(w http.ResponseWriter, r *http.Request) {
	var body seqBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	task := &models.SequencerTask{
		ID: storage.NewID(), Name: body.Name, Scheme: body.Scheme, Host: body.Host,
		Template: body.Template, HTTPVersion: body.HTTPVersion, Source: body.Source,
		Selector: body.Selector, Target: body.Target, Status: sequencer.StatusPending,
	}
	if err := a.Store.CreateSequencerTask(r.Context(), task); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, toSeqTaskView(task))
}

// GetSequencerTask handles GET /api/sequencer/tasks/{id}, returning the task,
// the analysis report, and a sample of collected tokens.
func (a *API) GetSequencerTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetSequencerTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such task")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	var tokens []string
	_ = json.Unmarshal(task.Tokens, &tokens)
	if tokens == nil {
		tokens = []string{}
	}
	var report any
	if len(task.Report) > 0 {
		report = json.RawMessage(task.Report)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task": toSeqTaskView(task), "report": report, "tokens": tokens,
	})
}

// UpdateSequencerTask handles PUT /api/sequencer/tasks/{id}, editing a draft.
func (a *API) UpdateSequencerTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetSequencerTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such task")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	var body seqBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	task.Name, task.Scheme, task.Host = body.Name, body.Scheme, body.Host
	task.Template, task.HTTPVersion = body.Template, body.HTTPVersion
	task.Source, task.Selector, task.Target = body.Source, body.Selector, body.Target
	if err := a.Store.UpdateSequencerTask(r.Context(), task); err != nil {
		a.serverError(w, "update_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, toSeqTaskView(task))
}

// DeleteSequencerTask handles DELETE /api/sequencer/tasks/{id}.
func (a *API) DeleteSequencerTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Sequencer.Stop(id)
	err := a.Store.DeleteSequencerTask(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such task")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartSequencerTask handles POST /api/sequencer/tasks/{id}/start.
func (a *API) StartSequencerTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetSequencerTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such task")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	if len(task.Template) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_task", "no request template to replay")
		return
	}
	if task.Selector == "" {
		writeError(w, http.StatusBadRequest, "invalid_task", fmt.Sprintf("no %s selector for token extraction", task.Source))
		return
	}
	if task.Source == "regex" {
		if _, err := regexp.Compile(task.Selector); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_task", "selector is not a valid regular expression: "+err.Error())
			return
		}
	}
	cfg := sequencer.Config{
		Scheme: task.Scheme, Host: task.Host, Template: task.Template, HTTPVersion: task.HTTPVersion,
		Source: task.Source, Selector: task.Selector, Target: task.Target,
	}
	started, err := a.Sequencer.Start(task.ID, cfg)
	if err != nil {
		a.serverError(w, "start_failed", err)
		return
	}
	if !started {
		writeError(w, http.StatusConflict, "already_running", "capture is already running")
		return
	}
	task.Status, task.Collected = sequencer.StatusRunning, 0
	writeJSON(w, http.StatusOK, toSeqTaskView(task))
}

// StopSequencerTask handles POST /api/sequencer/tasks/{id}/stop.
func (a *API) StopSequencerTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a.Sequencer.Stop(id) {
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	if task, err := a.Store.GetSequencerTask(r.Context(), id); err == nil && task.Status == sequencer.StatusRunning {
		_ = a.Store.SetSequencerResult(r.Context(), id, sequencer.StatusStopped, task.Report)
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	writeError(w, http.StatusConflict, "not_running", "capture is not running")
}

// SequencerStore adapts the storage layer to the sequencer.Store interface.
type SequencerStore struct {
	DB *storage.DB
}

func (s SequencerStore) PrepareSequencerRun(ctx context.Context, taskID string) error {
	return s.DB.PrepareSequencerRun(ctx, taskID)
}

func (s SequencerStore) UpdateSequencerProgress(ctx context.Context, taskID string, collected int, tokens []byte) error {
	return s.DB.UpdateSequencerProgress(ctx, taskID, collected, tokens)
}

func (s SequencerStore) SetSequencerResult(ctx context.Context, taskID, status string, report []byte) error {
	return s.DB.SetSequencerResult(ctx, taskID, status, report)
}
