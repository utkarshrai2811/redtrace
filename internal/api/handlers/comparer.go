package handlers

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/comparer"
)

type compareRequest struct {
	A    string `json:"a"`
	B    string `json:"b"`
	Mode string `json:"mode"` // "lines" (default) | "words"
}

// Compare handles POST /api/comparer, diffing two texts.
func (a *API) Compare(w http.ResponseWriter, r *http.Request) {
	var body compareRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var segments []comparer.Segment
	if body.Mode == "words" {
		segments = comparer.DiffWords(body.A, body.B)
	} else {
		segments = comparer.DiffLines(body.A, body.B)
	}
	if segments == nil {
		segments = []comparer.Segment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"segments": segments})
}
