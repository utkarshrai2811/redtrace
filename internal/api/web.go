package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// distFS holds the built web UI. A placeholder index.html is committed so the
// Go binary always builds; `make web-build` overwrites this directory with the
// real Vite output before release builds.
//
//go:embed all:dist
var distFS embed.FS

// spaHandler serves the embedded single-page app, falling back to index.html
// for client-side routes so deep links work.
func spaHandler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w)
			return
		}
		if _, err := fs.Stat(sub, p); err != nil {
			serveIndex(w) // SPA fallback
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
