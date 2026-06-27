// Package httpapi is the driving adapter: it exposes the health use case over
// HTTP (liveness + readiness), wrapped in request logging.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
)

// New returns an http.Handler exposing liveness and readiness endpoints backed
// by svc, wrapped in request logging.
func New(svc *health.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		st := svc.Ready(r.Context())
		if st.OK {
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"checks": st.Checks,
		})
	})

	return withLogging(mux)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
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
