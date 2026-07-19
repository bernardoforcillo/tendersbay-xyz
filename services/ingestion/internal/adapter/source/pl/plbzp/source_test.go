package plbzp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzpapi"
)

func TestSource_Name(t *testing.T) {
	if got := New().Name(); got != "pl-bzp" {
		t.Errorf("Name() = %q, want %q", got, "pl-bzp")
	}
}

func TestSource_Fetch(t *testing.T) {
	body, err := os.ReadFile("../testdata/notice_sample.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageNumber") == "1" {
			_, _ = w.Write(body)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// The fixture's notices are dated 2024-02-14; look back far enough to keep
	// them (the real Source uses a rolling window off time.Now).
	src := &Source{api: bzpapi.NewWithURL(srv.URL), window: 10 * 365 * 24 * time.Hour}
	tenders, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(tenders) != 5 {
		t.Fatalf("len(tenders) = %d, want 5", len(tenders))
	}
	first := tenders[0]
	if first.Source != "pl-bzp" {
		t.Errorf("Source = %q, want %q", first.Source, "pl-bzp")
	}
	if first.SourceRef != "08dc2d33-3bfb-c867-cf03-f600119345c3" {
		t.Errorf("SourceRef = %q, want the fixture's first objectId", first.SourceRef)
	}
	if first.Country != "PL" || first.Language != "pl" {
		t.Errorf("locale = %q/%q, want PL/pl", first.Country, first.Language)
	}
	if first.Value != nil {
		t.Errorf("Value = %v, want nil for BZP", first.Value)
	}
}

func TestSource_Fetch_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{api: bzpapi.NewWithURL(srv.URL), window: 24 * time.Hour}

	errCh := make(chan error, 1)
	go func() {
		_, err := src.Fetch(ctx)
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Fetch: want error after ctx cancellation, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Fetch did not return after ctx cancellation")
	}
}
