package document_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/document"
)

func TestExtract_ReturnsTextFromRealPDF(t *testing.T) {
	parts, err := document.Extract("testdata/fixture.pdf")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if parts[0] != "Comune di Roma appalto lavori stradali" {
		t.Errorf("parts[0] = %q, want %q", parts[0], "Comune di Roma appalto lavori stradali")
	}
}

func TestExtract_NonexistentFile(t *testing.T) {
	_, err := document.Extract("testdata/does-not-exist.pdf")
	if err == nil {
		t.Fatal("Extract: want error for a missing file, got nil")
	}
}

func TestClient_FetchAndExtract(t *testing.T) {
	fixture, err := os.ReadFile("testdata/fixture.pdf")
	if err != nil {
		t.Fatalf("ReadFile(testdata/fixture.pdf): %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	c := document.NewClient()
	parts, err := c.FetchAndExtract(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAndExtract: %v", err)
	}
	if len(parts) != 1 || parts[0] != "Comune di Roma appalto lavori stradali" {
		t.Errorf("parts = %v, want [Comune di Roma appalto lavori stradali]", parts)
	}
}
