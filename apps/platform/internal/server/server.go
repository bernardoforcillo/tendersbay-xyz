// Package server provides the HTTP handler that serves the embedded
// single-page frontend with client-side routing fallback.
package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// New returns an http.Handler that serves static files from fsys. Requests
// for existing files are served as-is. A request for a path that does not
// exist falls back to index.html when it has no file extension (so client-side
// SPA routes resolve); a missing path that looks like an asset returns 404.
func New(fsys fs.FS) http.Handler {
	index, err := indexHTML(fsys)
	if err != nil {
		slog.Error("failed to prepare index.html", "error", err)
	}

	fileServer := http.FileServer(http.FS(fsys))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		// index.html carries runtime config injected from the environment, so it
		// is always served from the prepared bytes — never straight off the file
		// server, which would return the un-injected embedded copy.
		if name == "index.html" {
			serveIndex(w, r, index)
			return
		}

		if fileExists(fsys, name) {
			fileServer.ServeHTTP(w, r)
			return
		}

		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}

		serveIndex(w, r, index)
	})

	return withLogging(handler)
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

func serveIndex(w http.ResponseWriter, r *http.Request, index []byte) {
	if index == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

// indexHTML loads index.html and injects runtime configuration into the
// window.__ENV__ placeholder, so the browser receives values such as the API
// URL at serve time — no rebuild required. When no runtime config is set the
// embedded bytes are returned unchanged and the app falls back to its
// build-time (import.meta.env) configuration.
func indexHTML(fsys fs.FS) ([]byte, error) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, err
	}

	// Client runtime config injected into window.__ENV__. The PostHog project key
	// (phc_…) is public — it already ships in the browser bundle — so exposing it
	// is safe; it maps from the same POSTHOG_* vars the server reads for its own
	// telemetry.
	env := map[string]string{}
	putEnv(env, "API_URL", "API_URL")
	putEnv(env, "POSTHOG_KEY", "POSTHOG_API_KEY")
	putEnv(env, "POSTHOG_HOST", "POSTHOG_HOST")
	if len(env) == 0 {
		return data, nil
	}

	// json.Marshal HTML-escapes <, >, & so the value cannot break out of the
	// surrounding <script> element.
	encoded, err := json.Marshal(env)
	if err != nil {
		return data, nil
	}
	return bytes.Replace(
		data,
		[]byte("window.__ENV__ = {}"),
		[]byte("window.__ENV__ = "+string(encoded)),
		1,
	), nil
}

// putEnv copies a non-empty environment variable into m under key.
func putEnv(m map[string]string, key, envVar string) {
	if v := os.Getenv(envVar); v != "" {
		m[key] = v
	}
}

// statusRecorder captures the status code written to the response.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withLogging emits one slog record per request via the default logger, so
// requests reach PostHog when telemetry is enabled and stdout otherwise.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}
