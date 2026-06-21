package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/oob"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

type oobConfigView struct {
	Enabled  bool   `json:"enabled"`
	Domain   string `json:"domain"`
	PublicIP string `json:"publicIp"`
	HTTPAddr string `json:"httpAddr"`
	DNSAddr  string `json:"dnsAddr"`
}

type oobPayloadView struct {
	Token        string    `json:"token"`
	Host         string    `json:"host"`
	HTTPURL      string    `json:"httpUrl"`
	Interactions int       `json:"interactions"`
	CreatedAt    time.Time `json:"createdAt"`
}

func payloadView(host string) oobPayloadView {
	return oobPayloadView{Host: host, HTTPURL: "http://" + host + "/"}
}

type oobInteractionView struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"`
	Protocol  string    `json:"protocol"`
	SourceIP  string    `json:"sourceIp"`
	Query     string    `json:"query"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

func toOOBInteractionView(i *models.OOBInteraction) oobInteractionView {
	return oobInteractionView{
		ID: i.ID, Token: i.Token, Protocol: i.Protocol, SourceIP: i.SourceIP,
		Query: i.Query, Detail: i.Detail, CreatedAt: i.CreatedAt,
	}
}

// OOBConfig handles GET /api/oob/config.
func (a *API) OOBConfig(w http.ResponseWriter, _ *http.Request) {
	c := a.OOB.Config()
	writeJSON(w, http.StatusOK, oobConfigView{
		Enabled: c.Enabled(), Domain: c.Domain, PublicIP: c.PublicIP, HTTPAddr: c.HTTPAddr, DNSAddr: c.DNSAddr,
	})
}

// GenerateOOBPayload handles POST /api/oob/payloads.
func (a *API) GenerateOOBPayload(w http.ResponseWriter, r *http.Request) {
	p, err := a.OOB.NewPayload(r.Context())
	if errors.Is(err, oob.ErrDisabled) {
		writeError(w, http.StatusBadRequest, "oob_disabled", "oob is not configured")
		return
	}
	if err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	v := payloadView(p.Host)
	v.Token, v.CreatedAt = p.Token, p.CreatedAt
	writeJSON(w, http.StatusCreated, v)
}

// ListOOBPayloads handles GET /api/oob/payloads.
func (a *API) ListOOBPayloads(w http.ResponseWriter, r *http.Request) {
	payloads, counts, err := a.Store.ListOOBPayloads(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]oobPayloadView, 0, len(payloads))
	for _, p := range payloads {
		v := payloadView(p.Host)
		v.Token, v.CreatedAt, v.Interactions = p.Token, p.CreatedAt, counts[p.Token]
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, views)
}

// ListOOBInteractions handles GET /api/oob/interactions (optional ?token=).
func (a *API) ListOOBInteractions(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.ListOOBInteractions(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	views := make([]oobInteractionView, 0, len(items))
	for _, i := range items {
		views = append(views, toOOBInteractionView(i))
	}
	writeJSON(w, http.StatusOK, views)
}

// GetOOBInteraction handles GET /api/oob/interactions/{id}, with raw bytes.
func (a *API) GetOOBInteraction(w http.ResponseWriter, r *http.Request) {
	i, err := a.Store.GetOOBInteraction(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such interaction")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interaction": toOOBInteractionView(i), "raw": i.Raw})
}

// ClearOOBInteractions handles DELETE /api/oob/interactions.
func (a *API) ClearOOBInteractions(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.ClearOOBInteractions(r.Context()); err != nil {
		a.serverError(w, "clear_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OOBStore adapts the storage layer to the oob.Store interface.
type OOBStore struct {
	DB *storage.DB
}

func (s OOBStore) SavePayload(ctx context.Context, p oob.Payload) error {
	return s.DB.CreateOOBPayload(ctx, &models.OOBPayload{
		Token: p.Token, Host: p.Host, CreatedAt: p.CreatedAt,
	})
}

func (s OOBStore) SaveInteraction(ctx context.Context, i oob.Interaction) error {
	return s.DB.AddOOBInteraction(ctx, &models.OOBInteraction{
		ID: i.ID, Token: i.Token, Protocol: i.Protocol, SourceIP: i.SourceIP,
		Query: i.Query, Detail: i.Detail, Raw: i.Raw, CreatedAt: i.CreatedAt,
	})
}
