package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/scanner"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// scanBody is the create/update payload for an active scan task.
type scanBody struct {
	Name            string `json:"name"`
	Scheme          string `json:"scheme"`
	Host            string `json:"host"`
	Template        []byte `json:"template"` // raw request, base64 in JSON
	HTTPVersion     string `json:"httpVersion"`
	FollowRedirects bool   `json:"followRedirects"`
}

func (b scanBody) normalized() scanBody {
	if b.Scheme == "" {
		b.Scheme = "https"
	}
	if b.HTTPVersion == "" {
		b.HTTPVersion = "HTTP/1.1"
	}
	if b.Name == "" {
		b.Name = "Scan"
	}
	return b
}

type scanTaskView struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Scheme          string    `json:"scheme"`
	Host            string    `json:"host"`
	Template        []byte    `json:"template"`
	HTTPVersion     string    `json:"httpVersion"`
	FollowRedirects bool      `json:"followRedirects"`
	Status          string    `json:"status"`
	Total           int       `json:"total"`
	Completed       int       `json:"completed"`
	Issues          int       `json:"issues"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func toScanTaskView(t *models.ScanTask) scanTaskView {
	return scanTaskView{
		ID: t.ID, Name: t.Name, Scheme: t.Scheme, Host: t.Host, Template: t.Template,
		HTTPVersion: t.HTTPVersion, FollowRedirects: t.FollowRedirects, Status: t.Status,
		Total: t.Total, Completed: t.Completed, Issues: t.Issues,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

type scanIssueView struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"taskId,omitempty"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Severity    string    `json:"severity"`
	Confidence  string    `json:"confidence"`
	Scheme      string    `json:"scheme"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	Param       string    `json:"param,omitempty"`
	Payload     string    `json:"payload,omitempty"`
	Detail      string    `json:"detail"`
	Evidence    string    `json:"evidence"`
	Remediation string    `json:"remediation"`
	Origin      string    `json:"origin"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toScanIssueView(is *models.ScanIssue) scanIssueView {
	return scanIssueView{
		ID: is.ID, TaskID: is.TaskID, Type: is.Type, Name: is.Name, Severity: is.Severity,
		Confidence: is.Confidence, Scheme: is.Scheme, Host: is.Host, Port: is.Port, Path: is.Path,
		Method: is.Method, Param: is.Param, Payload: is.Payload, Detail: is.Detail,
		Evidence: is.Evidence, Remediation: is.Remediation, Origin: is.Origin, CreatedAt: is.CreatedAt,
	}
}

// ListScanIssues handles GET /api/scanner/issues.
func (a *API) ListScanIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := a.Store.ListScanIssues(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]scanIssueView, 0, len(issues))
	for _, is := range issues {
		views = append(views, toScanIssueView(is))
	}
	writeJSON(w, http.StatusOK, views)
}

// GetScanIssue handles GET /api/scanner/issues/{id}, with raw request/response.
func (a *API) GetScanIssue(w http.ResponseWriter, r *http.Request) {
	is, err := a.Store.GetScanIssue(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such issue")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issue":       toScanIssueView(is),
		"requestRaw":  is.RequestRaw,
		"responseRaw": is.ResponseRaw,
	})
}

// ClearScanIssues handles DELETE /api/scanner/issues.
func (a *API) ClearScanIssues(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.ClearScanIssues(r.Context()); err != nil {
		a.serverError(w, "clear_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListScanTasks handles GET /api/scanner/tasks.
func (a *API) ListScanTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.Store.ListScanTasks(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]scanTaskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, toScanTaskView(t))
	}
	writeJSON(w, http.StatusOK, views)
}

// CreateScanTask handles POST /api/scanner/tasks, saving an active-scan draft.
func (a *API) CreateScanTask(w http.ResponseWriter, r *http.Request) {
	var body scanBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	task := &models.ScanTask{
		ID: storage.NewID(), Name: body.Name, Scheme: body.Scheme, Host: body.Host,
		Template: body.Template, HTTPVersion: body.HTTPVersion, FollowRedirects: body.FollowRedirects,
		Status: scanner.StatusPending,
	}
	if err := a.Store.CreateScanTask(r.Context(), task); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, toScanTaskView(task))
}

// GetScanTask handles GET /api/scanner/tasks/{id}, returning the task and the
// issues it has found.
func (a *API) GetScanTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetScanTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such scan")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	issues, err := a.Store.ListScanIssuesByTask(r.Context(), task.ID)
	if err != nil {
		a.serverError(w, "issues_failed", err)
		return
	}
	views := make([]scanIssueView, 0, len(issues))
	for _, is := range issues {
		views = append(views, toScanIssueView(is))
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": toScanTaskView(task), "issues": views})
}

// UpdateScanTask handles PUT /api/scanner/tasks/{id}, editing a draft.
func (a *API) UpdateScanTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetScanTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such scan")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	var body scanBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	task.Name, task.Scheme, task.Host = body.Name, body.Scheme, body.Host
	task.Template, task.HTTPVersion, task.FollowRedirects = body.Template, body.HTTPVersion, body.FollowRedirects
	if err := a.Store.UpdateScanTask(r.Context(), task); err != nil {
		a.serverError(w, "update_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, toScanTaskView(task))
}

// DeleteScanTask handles DELETE /api/scanner/tasks/{id}.
func (a *API) DeleteScanTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Scanner.Stop(id) // stop first if it is running
	err := a.Store.DeleteScanTask(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such scan")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartScanTask handles POST /api/scanner/tasks/{id}/start.
func (a *API) StartScanTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetScanTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such scan")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}

	total, err := scanner.PlanCount(task.Template)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scan", err.Error())
		return
	}
	cfg := scanner.ActiveConfig{
		Scheme: task.Scheme, Host: task.Host, Template: task.Template, HTTPVersion: task.HTTPVersion,
	}
	started, err := a.Scanner.StartActive(task.ID, cfg, total)
	if err != nil {
		a.serverError(w, "start_failed", err)
		return
	}
	if !started {
		writeError(w, http.StatusConflict, "already_running", "scan is already running")
		return
	}

	task.Status, task.Total, task.Completed, task.Issues = scanner.StatusRunning, total, 0, 0
	writeJSON(w, http.StatusOK, toScanTaskView(task))
}

// StopScanTask handles POST /api/scanner/tasks/{id}/stop.
func (a *API) StopScanTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a.Scanner.Stop(id) {
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	if task, err := a.Store.GetScanTask(r.Context(), id); err == nil && task.Status == scanner.StatusRunning {
		_ = a.Store.SetScanStatus(r.Context(), id, scanner.StatusStopped)
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	writeError(w, http.StatusConflict, "not_running", "scan is not running")
}

// ScanExchange runs the passive scanner over a captured in-scope exchange. It is
// wired to the proxy's post-capture hook.
func (a *API) ScanExchange(ex *models.Exchange) {
	if a.Scanner == nil || ex == nil || ex.Request == nil || ex.Response == nil || !ex.Request.InScope {
		return
	}
	a.Scanner.ScanPassive(context.Background(), scanner.Target{
		Scheme: ex.Request.Scheme, Host: ex.Request.Host, Port: ex.Request.Port,
		Method: ex.Request.Method, Path: ex.Request.Path, Query: ex.Request.Query,
		RequestRaw: ex.Request.Raw, ResponseRaw: ex.Response.Raw,
	})
}

// ScannerStore adapts the storage layer to the scanner.Store interface.
type ScannerStore struct {
	DB *storage.DB
}

func (s ScannerStore) AddIssue(ctx context.Context, taskID string, is scanner.Issue) (bool, error) {
	id := is.ID
	if id == "" {
		id = storage.NewID()
	}
	return s.DB.AddScanIssue(ctx, &models.ScanIssue{
		ID: id, TaskID: taskID, Type: is.Type, Name: is.Name, Severity: string(is.Severity),
		Confidence: string(is.Confidence), Scheme: is.Scheme, Host: is.Host, Port: is.Port,
		Path: is.Path, Method: is.Method, Param: is.Param, Payload: is.Payload, Detail: is.Detail,
		Evidence: is.Evidence, Remediation: is.Remediation, Origin: is.Origin, Fingerprint: is.Fingerprint,
		RequestRaw: is.RequestRaw, ResponseRaw: is.ResponseRaw,
	})
}

func (s ScannerStore) PrepareScanRun(ctx context.Context, taskID string, total int) error {
	return s.DB.PrepareScanRun(ctx, taskID, total)
}

func (s ScannerStore) UpdateScanProgress(ctx context.Context, taskID string, completed, issues int) error {
	return s.DB.UpdateScanProgress(ctx, taskID, completed, issues)
}

func (s ScannerStore) SetScanStatus(ctx context.Context, taskID, status string) error {
	return s.DB.SetScanStatus(ctx, taskID, status)
}
