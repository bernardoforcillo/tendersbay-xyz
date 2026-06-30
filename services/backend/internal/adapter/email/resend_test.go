package email_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/email"
)

func TestResendSender_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := email.NewResendWithURL("test-key", "noreply@example.com", srv.URL)
	err := sender.SendVerification(context.Background(), "user@example.com", "Alice", "https://link")
	if err != nil {
		t.Fatalf("SendVerification: %v", err)
	}
}
