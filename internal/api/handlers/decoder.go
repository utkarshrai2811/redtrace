package handlers

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/decoder"
)

type decodeRequest struct {
	Input      string       `json:"input"`
	Operations []decoder.Op `json:"operations"`
}

// Decode handles POST /api/decoder/run, applying a chain of operations.
func (a *API) Decode(w http.ResponseWriter, r *http.Request) {
	var body decodeRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	out, err := decoder.Chain(body.Operations, []byte(body.Input))
	if err != nil {
		writeError(w, http.StatusBadRequest, "decode_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": string(out)})
}

// DecodeDetect handles POST /api/decoder/detect, suggesting one decode step.
func (a *API) DecodeDetect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	op, ok := decoder.Detect([]byte(body.Input))
	writeJSON(w, http.StatusOK, map[string]any{"op": op, "ok": ok})
}
