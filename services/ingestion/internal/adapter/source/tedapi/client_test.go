package tedapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/tedapi"
)

func TestFetchSince_SendsExpectedQueryAndPaginationMode(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1"}],"totalNoticeCount":1,"iterationNextToken":null}`))
	}))
	defer srv.Close()

	c := tedapi.NewWithURL(srv.URL)
	notices, err := c.FetchSince(context.Background(), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(notices) != 1 || notices[0].ProcedureIdentifier != "proc-1" {
		t.Fatalf("notices = %+v, want one notice with ProcedureIdentifier=proc-1", notices)
	}
	if gotBody["query"] != "publication-date>=20260701" {
		t.Errorf("query = %v, want publication-date>=20260701", gotBody["query"])
	}
	if gotBody["paginationMode"] != "ITERATION" {
		t.Errorf("paginationMode = %v, want ITERATION", gotBody["paginationMode"])
	}
}

func TestFetchSince_PagesUntilTokenIsNull(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1"}],"totalNoticeCount":2,"iterationNextToken":"tok-abc"}`))
			return
		}
		_, _ = w.Write([]byte(`{"notices":[{"publication-number":"2-2026","procedure-identifier":"proc-2"}],"totalNoticeCount":2,"iterationNextToken":null}`))
	}))
	defer srv.Close()

	c := tedapi.NewWithURL(srv.URL)
	notices, err := c.FetchSince(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 2 {
		t.Fatalf("server got %d calls, want 2", calls)
	}
	if len(notices) != 2 {
		t.Fatalf("len(notices) = %d, want 2", len(notices))
	}
}

func TestFetchSince_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid field name: bogus-field"}`))
	}))
	defer srv.Close()

	c := tedapi.NewWithURL(srv.URL)
	_, err := c.FetchSince(context.Background(), time.Now())
	if err == nil {
		t.Fatal("FetchSince: want error on 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "bogus-field") {
		t.Errorf("error = %q, want it to include the response body", err.Error())
	}
}

func TestFetchSince_ContextCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(block)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := tedapi.NewWithURL(srv.URL)

	errCh := make(chan error, 1)
	go func() {
		_, err := c.FetchSince(ctx, time.Now())
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("FetchSince: want error after ctx cancellation, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FetchSince did not return after ctx cancellation")
	}
}

func TestFetchSince_StopsOnEmptyPageDespiteToken(t *testing.T) {
	// Real TED behavior (observed live 2026-07-14): ITERATION mode never
	// returns a null iterationNextToken — the end-of-results signal is an
	// empty notices page (still carrying a token), and iterating past it
	// wraps back around to the first page.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1"}],"totalNoticeCount":2,"iterationNextToken":"tok-1"}`))
		case 2:
			_, _ = w.Write([]byte(`{"notices":[{"publication-number":"2-2026","procedure-identifier":"proc-2"}],"totalNoticeCount":2,"iterationNextToken":"tok-2"}`))
		case 3:
			_, _ = w.Write([]byte(`{"notices":[],"totalNoticeCount":2,"iterationNextToken":"tok-3"}`))
		default: // wrap-around: the iteration restarts from the first page
			_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1"}],"totalNoticeCount":2,"iterationNextToken":"tok-4"}`))
		}
	}))
	defer srv.Close()

	c := tedapi.NewWithURL(srv.URL)
	notices, err := c.FetchSince(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 3 {
		t.Fatalf("server got %d calls, want 3 (stop at the empty page)", calls)
	}
	if len(notices) != 2 {
		t.Fatalf("len(notices) = %d, want 2", len(notices))
	}
}

func TestFetchSince_PaginationLoopSafetyCap(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		// Always return a non-empty iterationNextToken, simulating a stuck pagination loop.
		_, _ = w.Write([]byte(`{"notices":[{"publication-number":"1-2026","procedure-identifier":"proc-1"}],"totalNoticeCount":999999,"iterationNextToken":"tok-stuck"}`))
	}))
	defer srv.Close()

	c := tedapi.NewWithURL(srv.URL)
	_, err := c.FetchSince(context.Background(), time.Now())
	if err == nil {
		t.Fatal("FetchSince: want error on stuck pagination loop, got nil")
	}
	if calls != 100 {
		t.Fatalf("server got %d calls, want 100 (maxPages cap)", calls)
	}
}
