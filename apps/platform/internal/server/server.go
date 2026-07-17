// Package server provides the HTTP handler that serves the embedded
// single-page frontend with client-side routing fallback.
package server

import (
	"bytes"
	"context"
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
// for existing files are served as-is. A request whose first path segment is a
// locale with a prepared <locale>/index.html (emitted per locale by the seo
// plugin) serves that locale's index, including for extensionless SPA paths
// below it. Any other path that does not exist falls back to the root
// index.html when it has no file extension (so client-side SPA routes
// resolve); a missing path that looks like an asset returns 404.
func New(fsys fs.FS) http.Handler {
	index, err := indexHTML(fsys, "index.html")
	if err != nil {
		slog.Error("failed to prepare index.html", "error", err)
	}
	locales := localeIndexes(fsys)

	fileServer := http.FileServer(http.FS(fsys))

	metas := newMetaCache()
	sitemaps := newSitemapCache()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		// index.html carries runtime config injected from the environment, so it
		// is always served from the prepared bytes — never straight off the file
		// server, which would return the un-injected embedded copy.
		if name == "index.html" {
			w.Header().Set("X-Robots-Tag", "noindex")
			serveIndex(w, r, index)
			return
		}

		// The sitemap is generated dynamically from the backend, not embedded.
		if name == "sitemap-tenders.xml" {
			if xml, ok := sitemaps.get(); ok {
				w.Header().Set("Content-Type", "application/xml; charset=utf-8")
				_, _ = w.Write(xml)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			scheme := "https"
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "" {
				scheme = "http"
			}
			xml, err := tenderSitemapXML(ctx, apiURLFromEnv(), scheme+"://"+r.Host, localeNames(locales))
			if err != nil {
				http.Error(w, "sitemap unavailable", http.StatusBadGateway)
				return
			}
			sitemaps.put(xml)
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			_, _ = w.Write(xml)
			return
		}

		// /<locale>, /<locale>/index.html, and extensionless SPA paths below a
		// locale serve that locale's prepared index (same env injection). A
		// /<locale>/tenders/<id> path gets a server-rendered head with the
		// tender's title/meta/JSON-LD instead of the plain shell.
		segment, rest, _ := strings.Cut(name, "/")
		if prepared, ok := locales[segment]; ok {
			if id, isTender := tenderIDFromPath(rest); isTender {
				serveTenderPage(w, r, prepared, segment, id, metas)
				return
			}
			if rest == "" || rest == "index.html" || path.Ext(rest) == "" {
				serveIndex(w, r, prepared)
				return
			}
		}

		if fileExists(fsys, name) {
			fileServer.ServeHTTP(w, r)
			return
		}

		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("X-Robots-Tag", "noindex")
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

// isLocaleDir reports whether name has the locale shape (`xx-yy`), so stale or
// unexpected top-level directories are never served as locale pages.
func isLocaleDir(name string) bool {
	if len(name) != 5 || name[2] != '-' {
		return false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if name[i] < 'a' || name[i] > 'z' {
			return false
		}
	}
	return true
}

// localeIndexes prepares every top-level locale-shaped <dir>/index.html found
// in the embedded FS (the seo plugin emits one per locale with localized head
// tags), keyed by the directory name, with the same env injection as the root
// index.
func localeIndexes(fsys fs.FS) map[string][]byte {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	prepared := map[string][]byte{}
	for _, entry := range entries {
		if !entry.IsDir() || !isLocaleDir(entry.Name()) {
			continue
		}
		name := entry.Name() + "/index.html"
		if !fileExists(fsys, name) {
			continue
		}
		data, err := indexHTML(fsys, name)
		if err != nil {
			slog.Error("failed to prepare locale index", "name", name, "error", err)
			continue
		}
		prepared[entry.Name()] = data
	}
	return prepared
}

// indexHTML loads the named index.html and injects runtime configuration into
// the window.__ENV__ placeholder, so the browser receives values such as the
// API URL at serve time — no rebuild required. When no runtime config is set
// the embedded bytes are returned unchanged and the app falls back to its
// build-time (import.meta.env) configuration.
func indexHTML(fsys fs.FS, name string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, name)
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
