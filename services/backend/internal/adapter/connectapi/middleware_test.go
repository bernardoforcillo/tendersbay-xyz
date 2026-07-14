package connectapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
)

func TestClientIPMiddleware_TrustsRightmostForwardedForEntry(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = connectapi.ClientIPFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	rec := httptest.NewRecorder()

	connectapi.ClientIPMiddleware(next).ServeHTTP(rec, req)

	if got != "2.2.2.2" {
		t.Errorf("ClientIPFromContext = %q, want %q (rightmost entry — the hop Traefik itself appended, not the client-controlled leftmost one)", got, "2.2.2.2")
	}
}

func TestClientIPMiddleware_SingleValueForwardedFor(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = connectapi.ClientIPFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "3.3.3.3")
	rec := httptest.NewRecorder()

	connectapi.ClientIPMiddleware(next).ServeHTTP(rec, req)

	if got != "3.3.3.3" {
		t.Errorf("ClientIPFromContext = %q, want %q", got, "3.3.3.3")
	}
}

func TestClientIPMiddleware_FallsBackToRemoteAddrWhenHeaderAbsent(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = connectapi.ClientIPFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rec := httptest.NewRecorder()

	connectapi.ClientIPMiddleware(next).ServeHTTP(rec, req)

	if got != "9.9.9.9:1234" {
		t.Errorf("ClientIPFromContext = %q, want %q (RemoteAddr fallback)", got, "9.9.9.9:1234")
	}
}
