package handlers

import (
	"net/http"

	"github.com/utkarshrai2811/redtrace/internal/storage"
)

// GetSitemap handles GET /api/sitemap, returning the host → path tree.
func (a *API) GetSitemap(w http.ResponseWriter, r *http.Request) {
	tree, err := a.Store.Sitemap(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sitemap_failed", err.Error())
		return
	}
	if tree == nil {
		tree = []storage.SitemapHost{}
	}
	writeJSON(w, http.StatusOK, tree)
}

type sitemapNoteBody struct {
	Host string `json:"host"`
	Path string `json:"path"`
	Note string `json:"note"`
	Tags string `json:"tags"`
}

// PutSitemapNote handles PUT /api/sitemap/note.
func (a *API) PutSitemapNote(w http.ResponseWriter, r *http.Request) {
	var body sitemapNoteBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.Host == "" || body.Path == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "host and path are required")
		return
	}
	if err := a.Store.UpsertSitemapNote(r.Context(), body.Host, body.Path, body.Note, body.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, "note_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
