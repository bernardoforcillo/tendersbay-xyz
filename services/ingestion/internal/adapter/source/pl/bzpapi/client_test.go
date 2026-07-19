package bzpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzpapi"
)

// notice_sample.json holds five real BZP notices, all published 2024-02-14
// (see FINDINGS.md). Tests therefore use a `since` before that date so the
// publication-date cutoff keeps them — the fixture wins over the plan's
// illustrative 2026 timestamps.
var fixtureSince = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestFetchSince_ParsesNoticesAndAppliesCutoff(t *testing.T) {
	body, err := os.ReadFile("../testdata/notice_sample.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The board search endpoint pages by pageNumber; only page 1 carries
		// the fixture, later pages are empty so FetchSince terminates.
		if r.URL.Query().Get("pageNumber") == "1" {
			_, _ = w.Write(body)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	notices, err := bzpapi.NewWithURL(srv.URL).FetchSince(context.Background(), fixtureSince)
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(notices) != 5 {
		t.Fatalf("len(notices) = %d, want 5", len(notices))
	}
	first := notices[0]
	if first.ObjectID != "08dc2d33-3bfb-c867-cf03-f600119345c3" {
		t.Errorf("ObjectID = %q, want the fixture's first objectId (reconcile struct tags)", first.ObjectID)
	}
	if first.OrderObject == "" {
		t.Error("OrderObject not decoded")
	}
	if first.OrganizationName != "Gmina Pyrzyce" {
		t.Errorf("OrganizationName = %q, want %q", first.OrganizationName, "Gmina Pyrzyce")
	}
	if first.NoticeType != "ContractNotice" {
		t.Errorf("NoticeType = %q, want %q", first.NoticeType, "ContractNotice")
	}
	if !first.IsTenderAmountBelowEU {
		t.Error("IsTenderAmountBelowEU = false, want true (below-EU flag kept for Spec 2)")
	}
	if len(first.Raw) == 0 {
		t.Error("Raw not populated with the untouched notice element")
	}
}

func TestFetchSince_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"title":"invalid pageSize value"}`))
	}))
	defer srv.Close()

	_, err := bzpapi.NewWithURL(srv.URL).FetchSince(context.Background(), fixtureSince)
	if err == nil {
		t.Fatal("FetchSince: want error on 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "invalid pageSize value") {
		t.Errorf("error = %q, want it to include the response body snippet", err.Error())
	}
}

func TestFetchSince_PagesUntilEmpty(t *testing.T) {
	// Two non-empty pages of in-window notices, then an empty page → stop.
	calls := 0
	page := func(n int) string {
		return fmt.Sprintf(`[{"objectId":"a-%d","publicationDate":"2024-06-01T00:00:00Z","noticeType":"ContractNotice"},{"objectId":"b-%d","publicationDate":"2024-06-01T00:00:00Z","noticeType":"ContractNotice"}]`, n, n)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("pageNumber") {
		case "1":
			_, _ = w.Write([]byte(page(1)))
		case "2":
			_, _ = w.Write([]byte(page(2)))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	notices, err := bzpapi.NewWithURL(srv.URL).FetchSince(context.Background(), fixtureSince)
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 3 {
		t.Fatalf("server got %d calls, want 3 (two data pages + one empty)", calls)
	}
	if len(notices) != 4 {
		t.Fatalf("len(notices) = %d, want 4", len(notices))
	}
}

func TestFetchSince_StopsAtNoticeOlderThanSince(t *testing.T) {
	// Board search returns notices newest-first; once one predates `since`
	// every later notice is older, so FetchSince stops without a next page.
	since := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"objectId":"fresh","publicationDate":"2024-06-01T00:00:00Z","noticeType":"ContractNotice"},{"objectId":"stale","publicationDate":"2024-01-01T00:00:00Z","noticeType":"ContractNotice"}]`))
	}))
	defer srv.Close()

	notices, err := bzpapi.NewWithURL(srv.URL).FetchSince(context.Background(), since)
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 1 {
		t.Fatalf("server got %d calls, want 1 (stop at the first stale notice)", calls)
	}
	if len(notices) != 1 || notices[0].ObjectID != "fresh" {
		t.Fatalf("notices = %+v, want only the fresh in-window notice", notices)
	}
}

func TestFetchSince_PaginationSafetyCap(t *testing.T) {
	// Every page is non-empty and in-window → the loop must bail at the cap.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"objectId":"x","publicationDate":"2024-06-01T00:00:00Z","noticeType":"ContractNotice"}]`))
	}))
	defer srv.Close()

	_, err := bzpapi.NewWithURL(srv.URL).FetchSince(context.Background(), fixtureSince)
	if err == nil {
		t.Fatal("FetchSince: want error on runaway pagination, got nil")
	}
	if calls != 100 {
		t.Fatalf("server got %d calls, want 100 (maxPages cap)", calls)
	}
}

func TestFetchSince_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := bzpapi.NewWithURL(srv.URL).FetchSince(ctx, fixtureSince)
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
