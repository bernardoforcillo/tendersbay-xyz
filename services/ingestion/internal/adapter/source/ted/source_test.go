package ted

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/tedapi"
)

func TestSource_Name(t *testing.T) {
	if got := New().Name(); got != "ted" {
		t.Errorf("Name() = %q, want %q", got, "ted")
	}
}

func TestSource_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1","notice-type":"cn-standard","notice-title":{"eng":"Test tender"},"buyer-country":["ITA"]}],"totalNoticeCount":1,"iterationNextToken":null}`))
	}))
	defer srv.Close()

	src := &Source{api: tedapi.NewWithURL(srv.URL)}
	tenders, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(tenders) != 1 {
		t.Fatalf("len(tenders) = %d, want 1", len(tenders))
	}
	if tenders[0].SourceRef != "proc-1" {
		t.Errorf("SourceRef = %q, want %q", tenders[0].SourceRef, "proc-1")
	}
	if tenders[0].Title != "Test tender" {
		t.Errorf("Title = %q, want %q", tenders[0].Title, "Test tender")
	}
	if tenders[0].Country != "IT" {
		t.Errorf("Country = %q, want %q", tenders[0].Country, "IT")
	}
	if tenders[0].Source != "ted" {
		t.Errorf("Source = %q, want %q", tenders[0].Source, "ted")
	}
}

func TestSource_Fetch_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{api: tedapi.NewWithURL(srv.URL)}

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
