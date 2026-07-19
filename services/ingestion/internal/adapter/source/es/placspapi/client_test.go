package placspapi_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/placspapi"
)

// entry builds one ATOM <entry> with an inline CODICE contract folder, for the
// synthesized paging/cutoff tests. Namespace prefixes are left undeclared on
// purpose — Go's encoding/xml matches by local name and tolerates unbound
// prefixes, exactly as codice.Parse relies on once the folder is lifted out of
// the feed.
func entry(id, updated, amount string) string {
	return fmt.Sprintf(`<entry>
  <updated>%s</updated>
  <cac-place-ext:ContractFolderStatus>
    <cbc:ContractFolderID>%s</cbc:ContractFolderID>
    <cbc-place-ext:ContractFolderStatusCode>EV</cbc-place-ext:ContractFolderStatusCode>
    <cac:ProcurementProject>
      <cbc:Name>Tender %s</cbc:Name>
      <cac:BudgetAmount><cbc:TaxExclusiveAmount currencyID="EUR">%s</cbc:TaxExclusiveAmount></cac:BudgetAmount>
    </cac:ProcurementProject>
  </cac-place-ext:ContractFolderStatus>
</entry>`, updated, id, id, amount)
}

func feed(next string, entries ...string) string {
	nextLink := ""
	if next != "" {
		nextLink = fmt.Sprintf(`<link rel="next" href="%s"/>`, next)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<feed>` + nextLink + strings.Join(entries, "") + `</feed>`
}

func TestFetchSince_ParsesAtomFixture(t *testing.T) {
	atom, err := os.ReadFile("../testdata/atom_sample.xml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Neutralise the feed-level rel="next": the committed fixture points it at
	// the real PLACSP host, and this test must stay offline.
	atom = bytes.Replace(atom, []byte(`rel="next"`), []byte(`rel="next-disabled"`), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write(atom)
	}))
	defer srv.Close()

	docs, err := placspapi.NewWithURL(srv.URL).FetchSince(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("len(docs) = %d, want 2 (the fixture's two entries)", len(docs))
	}
	if docs[0].ContractFolderID != "104/2026" {
		t.Errorf("docs[0].ContractFolderID = %q, want 104/2026", docs[0].ContractFolderID)
	}
	if docs[0].EstimatedValue == nil {
		t.Error("docs[0].EstimatedValue is nil — CODICE carries a value for ES")
	}
	if docs[1].ContractFolderID != "1275/2026" {
		t.Errorf("docs[1].ContractFolderID = %q, want 1275/2026", docs[1].ContractFolderID)
	}
	// entry 2 has five ItemClassificationCode elements.
	if len(docs[1].CPV) != 5 {
		t.Errorf("docs[1].CPV = %v, want 5 codes", docs[1].CPV)
	}
}

func TestFetchSince_PagesUntilNoNext(t *testing.T) {
	var calls int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/atom+xml")
		if calls == 1 {
			_, _ = w.Write([]byte(feed(srv.URL+"/page2", entry("A/2026", "2026-07-18T10:00:00+02:00", "1000"))))
			return
		}
		_, _ = w.Write([]byte(feed("", entry("B/2026", "2026-07-17T10:00:00+02:00", "2000"))))
	}))
	defer srv.Close()

	docs, err := placspapi.NewWithURL(srv.URL).FetchSince(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 2 {
		t.Fatalf("server got %d calls, want 2 (follow rel=next once)", calls)
	}
	if len(docs) != 2 {
		t.Fatalf("len(docs) = %d, want 2", len(docs))
	}
	if docs[0].ContractFolderID != "A/2026" || docs[1].ContractFolderID != "B/2026" {
		t.Errorf("ids = %q,%q, want A/2026,B/2026", docs[0].ContractFolderID, docs[1].ContractFolderID)
	}
}

func TestFetchSince_StopsAtCutoff(t *testing.T) {
	var calls int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/atom+xml")
		// Page 1 (newest-first): one entry after the cutoff, one before it, and
		// a rel=next. Crossing the cutoff must stop paging.
		_, _ = w.Write([]byte(feed(srv.URL+"/page2",
			entry("NEW/2026", "2026-06-10T10:00:00+02:00", "1000"),
			entry("OLD/2026", "2026-05-01T10:00:00+02:00", "2000"),
		)))
	}))
	defer srv.Close()

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	docs, err := placspapi.NewWithURL(srv.URL).FetchSince(context.Background(), since)
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 1 {
		t.Fatalf("server got %d calls, want 1 (stop paging once the cutoff is crossed)", calls)
	}
	if len(docs) != 1 || docs[0].ContractFolderID != "NEW/2026" {
		t.Fatalf("docs = %+v, want only NEW/2026 (OLD/2026 predates the cutoff)", docs)
	}
}

func TestFetchSince_SkipsEntryWithoutFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(feed("",
			`<entry><updated>2026-07-18T10:00:00+02:00</updated></entry>`,
			entry("HAS/2026", "2026-07-18T10:00:00+02:00", "1000"),
		)))
	}))
	defer srv.Close()

	docs, err := placspapi.NewWithURL(srv.URL).FetchSince(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(docs) != 1 || docs[0].ContractFolderID != "HAS/2026" {
		t.Fatalf("docs = %+v, want only HAS/2026 (the folderless entry is skipped)", docs)
	}
}

func TestFetchSince_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, err := placspapi.NewWithURL(srv.URL).FetchSince(context.Background(), time.Now())
	if err == nil {
		t.Fatal("FetchSince: want error on 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %q, want it to include the response body", err.Error())
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
		_, err := placspapi.NewWithURL(srv.URL).FetchSince(ctx, time.Now())
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
