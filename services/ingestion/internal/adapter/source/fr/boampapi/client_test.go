package boampapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
)

func TestFetchSince_ParsesRecordsFromFixture(t *testing.T) {
	body, err := os.ReadFile("../testdata/record_sample.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var gotDataset string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDataset = r.URL.Query().Get("dataset")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	records, err := boampapi.NewWithURL(srv.URL).
		FetchSince(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("want >=1 record from the fixture")
	}
	if records[0].Idweb == "" {
		t.Fatal("Idweb not decoded — reconcile struct tags with the fixture")
	}
	if records[0].Idweb != "26-71206" {
		t.Errorf("Idweb = %q, want 26-71206", records[0].Idweb)
	}
	if records[0].Objet == "" {
		t.Error("Objet not decoded from fields.objet")
	}
	if records[0].NomAcheteur == "" {
		t.Error("NomAcheteur not decoded from fields.nomacheteur")
	}
	if records[0].Donnees == "" {
		t.Error("Donnees (fields.donnees full-notice blob, carries real CPV) not decoded")
	}
	if len(records[0].Raw) == 0 {
		t.Error("Raw (untouched record payload) not set")
	}
	if gotDataset != "boamp" {
		t.Errorf("request dataset = %q, want boamp", gotDataset)
	}
}

func TestFetchSince_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid parameter: bogus"}`))
	}))
	defer srv.Close()

	_, err := boampapi.NewWithURL(srv.URL).FetchSince(context.Background(), time.Now())
	if err == nil {
		t.Fatal("FetchSince: want error on 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error = %q, want it to include the response body snippet", err.Error())
	}
}

func TestFetchSince_PagesUntilShortPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")
		if start == "0" {
			// A full page (len == rows) signals "there may be more" → client pages again.
			recs := make([]string, rows)
			for i := range recs {
				recs[i] = fmt.Sprintf(`{"recordid":"r%d","fields":{"idweb":"x-%d","dateparution":"2026-07-19"}}`, i, i)
			}
			_, _ = fmt.Fprintf(w, `{"nhits":999999,"records":[%s]}`, strings.Join(recs, ","))
			return
		}
		// A short page (len < rows) ends pagination.
		_, _ = w.Write([]byte(`{"nhits":999999,"records":[{"recordid":"last","fields":{"idweb":"last","dateparution":"2026-07-19"}}]}`))
	}))
	defer srv.Close()

	records, err := boampapi.NewWithURL(srv.URL).
		FetchSince(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if calls != 2 {
		t.Fatalf("server got %d calls, want 2 (full page then short page)", calls)
	}
	if len(records) < 2 {
		t.Fatalf("len(records) = %d, want the full page plus the final short page", len(records))
	}
	if records[len(records)-1].Idweb != "last" {
		t.Errorf("last record Idweb = %q, want last", records[len(records)-1].Idweb)
	}
}

func TestFetchSince_StopsAtRecordOlderThanSince(t *testing.T) {
	// Descending sort (sort=-dateparution) means once a record predates `since`,
	// every later record is older too — so the client stops and drops it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nhits":2,"records":[` +
			`{"recordid":"new","fields":{"idweb":"new","dateparution":"2026-07-10"}},` +
			`{"recordid":"old","fields":{"idweb":"old","dateparution":"2026-06-01"}}]}`))
	}))
	defer srv.Close()

	records, err := boampapi.NewWithURL(srv.URL).
		FetchSince(context.Background(), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchSince: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1 (the older-than-since record dropped)", len(records))
	}
	if records[0].Idweb != "new" {
		t.Errorf("Idweb = %q, want new", records[0].Idweb)
	}
}

func TestFetchSince_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := boampapi.NewWithURL(srv.URL)

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
