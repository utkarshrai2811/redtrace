package handlers

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/scope"
)

// GetScope handles GET /api/scope.
func (a *API) GetScope(w http.ResponseWriter, r *http.Request) {
	rules := a.Scope.Rules()
	if rules == nil {
		rules = []scope.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// PutScope handles PUT /api/scope, replacing the whole scope rule set.
func (a *API) PutScope(w http.ResponseWriter, r *http.Request) {
	var rules []scope.Rule
	if err := decodeJSON(r, &rules); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := a.Scope.SetRules(rules); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a.Scope.Rules())
}
