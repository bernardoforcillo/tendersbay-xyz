// Package server provides the HTTP handler that serves the embedded
// single-page frontend with client-side routing fallback.
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// New returns an http.Handler that serves static files from fsys. Requests
// for existing files are served as-is. A request for a path that does not
// exist falls back to index.html when it has no file extension (so client-side
// SPA routes resolve); a missing path that looks like an asset returns 404.
func New(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if fileExists(fsys, name) {
			fileServer.ServeHTTP(w, r)
			return
		}

		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}

		serveIndex(w, r, fsys)
	})
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
