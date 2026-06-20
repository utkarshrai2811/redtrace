package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

type listResponse struct {
	Data    []storage.RequestSummary `json:"data"`
	Page    int                      `json:"page"`
	PerPage int                      `json:"perPage"`
	Total   int                      `json:"total"`
}

type exchangeDetail struct {
	Request     *models.Request  `json:"request"`
	Response    *models.Response `json:"response,omitempty"`
	RequestRaw  []byte           `json:"requestRaw"`
	ResponseRaw []byte           `json:"responseRaw,omitempty"`
}

// ListRequests handles GET /api/requests with filtering and pagination.
func (a *API) ListRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage := atoiDefault(q.Get("per_page"), 50)
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}

	statusMin, statusMax := parseStatus(q.Get("status"))
	filter := storage.RequestFilter{
		Method:      q.Get("method"),
		Host:        q.Get("host"),
		MimeType:    q.Get("mime"),
		Contains:    q.Get("q"),
		StatusMin:   statusMin,
		StatusMax:   statusMax,
		InScopeOnly: q.Get("scope") == "in",
		Limit:       perPage,
		Offset:      (page - 1) * perPage,
	}

	rows, total, err := a.Store.ListRequests(r.Context(), filter)
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	if rows == nil {
		rows = []storage.RequestSummary{}
	}
	writeJSON(w, http.StatusOK, listResponse{Data: rows, Page: page, PerPage: perPage, Total: total})
}

// GetRequest handles GET /api/requests/{id}, returning the full exchange with
// raw request/response bytes (base64-encoded in JSON).
func (a *API) GetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ex, err := a.Store.GetExchange(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such request")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}

	detail := exchangeDetail{Request: ex.Request, Response: ex.Response, RequestRaw: ex.Request.Raw}
	if ex.Response != nil {
		detail.ResponseRaw = ex.Response.Raw
	}
	writeJSON(w, http.StatusOK, detail)
}

// DeleteRequest handles DELETE /api/requests/{id}.
func (a *API) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := a.Store.DeleteRequest(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such request")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ClearRequests handles DELETE /api/requests, clearing the entire history.
func (a *API) ClearRequests(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.ClearRequests(r.Context()); err != nil {
		a.serverError(w, "clear_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Hosts handles GET /api/hosts, returning the distinct observed hosts.
func (a *API) Hosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := a.Store.Hosts(r.Context())
	if err != nil {
		a.serverError(w, "hosts_failed", err)
		return
	}
	if hosts == nil {
		hosts = []string{}
	}
	writeJSON(w, http.StatusOK, hosts)
}

// SendToRepeater handles POST /api/requests/{id}/send-to-repeater, creating a
// Repeater tab seeded from a captured request.
func (a *API) SendToRepeater(w http.ResponseWriter, r *http.Request) {
	ex, err := a.Store.GetExchange(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such request")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	req := ex.Request
	port := req.Port
	if port == 0 {
		if req.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	tab := &models.RepeaterTab{
		ID:          storage.NewID(),
		Name:        req.Method + " " + req.Path,
		Scheme:      req.Scheme,
		Host:        fmt.Sprintf("%s:%d", req.Host, port),
		Raw:         req.Raw,
		HTTPVersion: "HTTP/1.1",
	}
	if err := a.Store.CreateRepeaterTab(r.Context(), tab); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, toTabView(tab))
}

// SendToIntruder is wired but lands in Phase 3.
func (a *API) SendToIntruder(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "Intruder arrives in Phase 3")
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// parseStatus maps "200" to [200,200] and "2xx"/"4xx" to a class range.
func parseStatus(s string) (min, max int) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, 0
	}
	if strings.HasSuffix(s, "xx") && len(s) == 3 {
		if d, err := strconv.Atoi(s[:1]); err == nil {
			return d * 100, d*100 + 99
		}
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, n
	}
	return 0, 0
}
