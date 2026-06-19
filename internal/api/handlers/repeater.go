package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/repeater"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

type tabView struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Scheme          string    `json:"scheme"`
	Host            string    `json:"host"`
	FollowRedirects bool      `json:"followRedirects"`
	HTTPVersion     string    `json:"httpVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Raw             []byte    `json:"raw"`
}

type historyView struct {
	ID          string    `json:"id"`
	StatusCode  int       `json:"statusCode"`
	DurationMs  int64     `json:"durationMs"`
	CreatedAt   time.Time `json:"createdAt"`
	RequestRaw  []byte    `json:"requestRaw"`
	ResponseRaw []byte    `json:"responseRaw"`
}

func toTabView(t *models.RepeaterTab) tabView {
	return tabView{
		ID: t.ID, Name: t.Name, Scheme: t.Scheme, Host: t.Host,
		FollowRedirects: t.FollowRedirects, HTTPVersion: t.HTTPVersion,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Raw: t.Raw,
	}
}

type tabBody struct {
	Name            string `json:"name"`
	Scheme          string `json:"scheme"`
	Host            string `json:"host"`
	FollowRedirects bool   `json:"followRedirects"`
	HTTPVersion     string `json:"httpVersion"`
	Raw             []byte `json:"raw"`
}

func (b tabBody) normalized() tabBody {
	if b.Scheme == "" {
		b.Scheme = "https"
	}
	if b.HTTPVersion == "" {
		b.HTTPVersion = "HTTP/1.1"
	}
	if b.Name == "" {
		b.Name = "Tab"
	}
	return b
}

// ListRepeaterTabs handles GET /api/repeater/tabs.
func (a *API) ListRepeaterTabs(w http.ResponseWriter, r *http.Request) {
	tabs, err := a.Store.ListRepeaterTabs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	views := make([]tabView, 0, len(tabs))
	for _, t := range tabs {
		views = append(views, toTabView(t))
	}
	writeJSON(w, http.StatusOK, views)
}

// CreateRepeaterTab handles POST /api/repeater/tabs.
func (a *API) CreateRepeaterTab(w http.ResponseWriter, r *http.Request) {
	var body tabBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	tab := &models.RepeaterTab{
		ID: storage.NewID(), Name: body.Name, Scheme: body.Scheme, Host: body.Host,
		Raw: body.Raw, FollowRedirects: body.FollowRedirects, HTTPVersion: body.HTTPVersion,
	}
	if err := a.Store.CreateRepeaterTab(r.Context(), tab); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toTabView(tab))
}

// GetRepeaterTab handles GET /api/repeater/tabs/{id}, returning the tab + history.
func (a *API) GetRepeaterTab(w http.ResponseWriter, r *http.Request) {
	tab, err := a.Store.GetRepeaterTab(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such tab")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	entries, err := a.Store.ListRepeaterHistory(r.Context(), tab.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
		return
	}
	history := make([]historyView, 0, len(entries))
	for _, e := range entries {
		history = append(history, historyView{
			ID: e.ID, StatusCode: e.StatusCode, DurationMs: e.DurationMs,
			CreatedAt: e.CreatedAt, RequestRaw: e.RequestRaw, ResponseRaw: e.ResponseRaw,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tab": toTabView(tab), "history": history})
}

// UpdateRepeaterTab handles PUT /api/repeater/tabs/{id}.
func (a *API) UpdateRepeaterTab(w http.ResponseWriter, r *http.Request) {
	tab, err := a.Store.GetRepeaterTab(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such tab")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	var body tabBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	tab.Name, tab.Scheme, tab.Host = body.Name, body.Scheme, body.Host
	tab.FollowRedirects, tab.HTTPVersion, tab.Raw = body.FollowRedirects, body.HTTPVersion, body.Raw
	if err := a.Store.UpdateRepeaterTab(r.Context(), tab); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toTabView(tab))
}

// DeleteRepeaterTab handles DELETE /api/repeater/tabs/{id}.
func (a *API) DeleteRepeaterTab(w http.ResponseWriter, r *http.Request) {
	err := a.Store.DeleteRepeaterTab(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such tab")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SendRepeaterTab handles POST /api/repeater/tabs/{id}/send, replaying the saved
// request and recording the result in history.
func (a *API) SendRepeaterTab(w http.ResponseWriter, r *http.Request) {
	tab, err := a.Store.GetRepeaterTab(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such tab")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}

	resp, err := a.Repeater.Send(r.Context(), repeater.Request{
		Scheme: tab.Scheme, Host: tab.Host, Raw: tab.Raw,
		FollowRedirects: tab.FollowRedirects, HTTPVersion: tab.HTTPVersion,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "send_failed", err.Error())
		return
	}

	entry := &models.RepeaterHistoryEntry{
		ID: storage.NewID(), TabID: tab.ID, RequestRaw: tab.Raw, ResponseRaw: resp.Raw,
		StatusCode: resp.StatusCode, DurationMs: resp.DurationMs,
	}
	if err := a.Store.AddRepeaterHistory(r.Context(), entry); err != nil {
		a.Log.Error("store repeater history", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"response": map[string]any{
			"raw":        resp.Raw,
			"statusCode": resp.StatusCode,
			"durationMs": resp.DurationMs,
		},
		"history": historyView{
			ID: entry.ID, StatusCode: entry.StatusCode, DurationMs: entry.DurationMs,
			CreatedAt: time.Now(), RequestRaw: entry.RequestRaw, ResponseRaw: entry.ResponseRaw,
		},
	})
}
