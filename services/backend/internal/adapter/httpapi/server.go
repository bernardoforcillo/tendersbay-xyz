// Package httpapi is the driving adapter for the surfaces that cannot be
// ConnectRPC: liveness and readiness, and the unauthenticated unsubscribe
// endpoint a reminder mail links to.
//
// Unsubscribe lives here and not on the RPC surface for one reason: it must
// work for someone who is not logged in, months after the mail arrived, from a
// mail client acting on their behalf. An RPC that required a session would make
// the escape hatch harder to use than the thing it escapes.
package httpapi

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
)

// Unsubscriber records an opt-out for the holder of a token. Satisfied by
// *alerting.Service; declared here so this adapter depends on a one-method
// shape rather than on that whole domain.
type Unsubscriber interface {
	Unsubscribe(ctx context.Context, token string) error
}

// New returns an http.Handler exposing liveness and readiness endpoints backed
// by svc, wrapped in request logging.
func New(svc *health.Service, unsub Unsubscriber) http.Handler {
	mux := http.NewServeMux()

	// GET renders a confirmation form and CHANGES NOTHING. Mail scanners and
	// link prefetchers follow every URL in a message; a GET that unsubscribed
	// would opt people out who never clicked, and the first symptom would be
	// reminders silently stopping for users who still wanted them.
	mux.HandleFunc("GET /unsubscribe", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, http.StatusOK, confirmTmpl, r.URL.Query().Get("t"))
	})

	// POST performs it. This is also the RFC 8058 one-click target: a mail
	// client posting "List-Unsubscribe=One-Click" lands here, which is why the
	// token is read from the query string rather than the body — the body is
	// the client's, the URL is ours.
	mux.HandleFunc("POST /unsubscribe", func(w http.ResponseWriter, r *http.Request) {
		if unsub == nil {
			// Not wired. Say so rather than showing the "Done" page: telling a
			// reader they are unsubscribed when nothing was recorded is the one
			// outcome this endpoint must never produce.
			writeHTML(w, http.StatusServiceUnavailable, failedTmpl, "")
			return
		}
		if err := unsub.Unsubscribe(r.Context(), r.URL.Query().Get("t")); err != nil {
			slog.ErrorContext(r.Context(), "unsubscribe failed", "error", err)
			// Do NOT claim success on a failure: the reader would stop looking
			// for a way out while the mail kept coming.
			writeHTML(w, http.StatusInternalServerError, failedTmpl, "")
			return
		}
		writeHTML(w, http.StatusOK, doneTmpl, "")
	})

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

// The three pages are deliberately plain HTML with no styling, no script and no
// link back into the app. This is the last thing a leaving reader sees; it
// should load instantly in any mail-client browser and ask nothing of them.
var (
	confirmTmpl = template.Must(template.New("").Parse(
		`<!doctype html><meta charset="utf-8"><title>Unsubscribe</title>` +
			`<p>Stop receiving deadline reminders from Tendersbay?</p>` +
			`<form method="post" action="/unsubscribe?t={{.}}"><button type="submit">Unsubscribe</button></form>`))
	doneTmpl = template.Must(template.New("").Parse(
		`<!doctype html><meta charset="utf-8"><title>Unsubscribed</title>` +
			`<p>Done. You will not receive deadline reminders any more.</p>`))
	failedTmpl = template.Must(template.New("").Parse(
		`<!doctype html><meta charset="utf-8"><title>Unsubscribe</title>` +
			`<p>Something went wrong and you are still subscribed. Please try again shortly.</p>`))
)

func writeHTML(w http.ResponseWriter, status int, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = t.Execute(w, data)
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
