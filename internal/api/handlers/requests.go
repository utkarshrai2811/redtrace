package handlers

import (
	"errors"
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

	min, max := parseStatus(q.Get("status"))
	filter := storage.RequestFilter{
		Method:      q.Get("method"),
		Host:        q.Get("host"),
		MimeType:    q.Get("mime"),
		Contains:    q.Get("q"),
		StatusMin:   min,
		StatusMax:   max,
		InScopeOnly: q.Get("scope") == "in",
		Limit:       perPage,
		Offset:      (page - 1) * perPage,
	}

	rows, total, err := a.Store.ListRequests(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
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
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
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
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Hosts handles GET /api/hosts, returning the distinct observed hosts.
func (a *API) Hosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := a.Store.Hosts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hosts_failed", err.Error())
		return
	}
	if hosts == nil {
		hosts = []string{}
	}
	writeJSON(w, http.StatusOK, hosts)
}

// SendToRepeater and SendToIntruder are wired now but land in later phases.
func (a *API) SendToRepeater(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "Repeater arrives in Phase 2")
}

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
