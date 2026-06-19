package handlers

import "net/http"

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Health handles GET /api/health.
func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: a.Version})
}

// ProxySettings handles GET /api/proxy/settings, returning read-only proxy
// configuration. Mutable behavior (intercept, scope, rules) has its own routes.
func (a *API) ProxySettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.Proxy)
}

// CACert handles GET /api/ca/cert, serving the RedTrace CA certificate for
// import into a trust store.
func (a *API) CACert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="redtrace-ca.pem"`)
	_, _ = w.Write(a.Authority.CACertPEM())
}
