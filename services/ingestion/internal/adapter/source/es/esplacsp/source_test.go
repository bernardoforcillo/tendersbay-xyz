package esplacsp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/placspapi"
)

func TestSource_Name(t *testing.T) {
	if got := New().Name(); got != "es-placsp" {
		t.Errorf("Name() = %q, want %q", got, "es-placsp")
	}
}

// freshFeed returns a one-entry PLACSP ATOM feed whose <updated> is recent, so
// it always falls inside Source.Fetch's rolling fetch window regardless of when
// the test runs (the PLACSP feed is filtered client-side by publication time).
func freshFeed() string {
	updated := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed>
  <entry>
    <updated>%s</updated>
    <cac-place-ext:ContractFolderStatus>
      <cbc:ContractFolderID>777/2026</cbc:ContractFolderID>
      <cbc-place-ext:ContractFolderStatusCode>EV</cbc-place-ext:ContractFolderStatusCode>
      <cac:ProcurementProject>
        <cbc:Name>Servicio de limpieza viaria</cbc:Name>
        <cac:BudgetAmount><cbc:TaxExclusiveAmount currencyID="EUR">125000</cbc:TaxExclusiveAmount></cac:BudgetAmount>
        <cac:RequiredCommodityClassification><cbc:ItemClassificationCode>90610000</cbc:ItemClassificationCode></cac:RequiredCommodityClassification>
      </cac:ProcurementProject>
    </cac-place-ext:ContractFolderStatus>
  </entry>
</feed>`, updated)
}

func TestSource_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(freshFeed()))
	}))
	defer srv.Close()

	src := &Source{api: placspapi.NewWithURL(srv.URL)}
	tenders, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(tenders) != 1 {
		t.Fatalf("len(tenders) = %d, want 1", len(tenders))
	}
	got := tenders[0]
	if got.Source != "es-placsp" {
		t.Errorf("Source = %q, want es-placsp", got.Source)
	}
	if got.SourceRef != "777/2026" {
		t.Errorf("SourceRef = %q, want 777/2026", got.SourceRef)
	}
	if got.Country != "ES" || got.Language != "es" {
		t.Errorf("locale = %q/%q, want ES/es", got.Country, got.Language)
	}
	if got.CPV != "90610000" {
		t.Errorf("CPV = %q, want 90610000", got.CPV)
	}
	if got.Value == nil || *got.Value != 12500000 {
		t.Errorf("Value = %v, want 12500000 (125000.00 EUR) — CODICE carries a value for ES", got.Value)
	}
}

func TestSource_Fetch_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{api: placspapi.NewWithURL(srv.URL)}

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

// qualifiedFeed is freshFeed's folder plus the qualification block PLACSP
// publishes inline — the whole reason this provider implements
// ingestion.DetailedSource rather than plain Source.
func qualifiedFeed() string {
	updated := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed>
  <entry>
    <updated>%s</updated>
    <cac-place-ext:ContractFolderStatus>
      <cbc:ContractFolderID>777/2026</cbc:ContractFolderID>
      <cbc-place-ext:ContractFolderStatusCode>EV</cbc-place-ext:ContractFolderStatusCode>
      <cac:ProcurementProject>
        <cbc:Name>Servicio de limpieza viaria</cbc:Name>
      </cac:ProcurementProject>
      <cac:TenderingTerms>
        <cac:TendererQualificationRequest>
          <cac:FinancialEvaluationCriteria>
            <cbc:EvaluationCriteriaTypeCode>5</cbc:EvaluationCriteriaTypeCode>
            <cbc:Description>Volumen anual de negocios igual o superior a 309.552,00 EUR.</cbc:Description>
          </cac:FinancialEvaluationCriteria>
        </cac:TendererQualificationRequest>
      </cac:TenderingTerms>
    </cac-place-ext:ContractFolderStatus>
  </entry>
</feed>`, updated)
}

func TestFetchWithDetail_KeysCriteriaBySourceRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(qualifiedFeed()))
	}))
	defer srv.Close()
	src := &Source{api: placspapi.NewWithURL(srv.URL)}

	tenders, criteria, err := src.FetchWithDetail(context.Background())
	if err != nil {
		t.Fatalf("FetchWithDetail: %v", err)
	}
	if len(tenders) != 1 {
		t.Fatalf("len(tenders) = %d, want 1", len(tenders))
	}

	// The join the sink performs: the map key must be the tender's SourceRef.
	// If these ever drift apart the criteria are silently dropped, so assert the
	// relationship rather than the literal.
	set, ok := criteria[tenders[0].SourceRef]
	if !ok {
		t.Fatalf("criteria keyed %v, want a key equal to SourceRef %q", keysOf(criteria), tenders[0].SourceRef)
	}
	if len(set) != 1 || set[0].Category != "financial" || set[0].Origin != "es-placsp" {
		t.Fatalf("criteria = %+v, want one financial criterion originating from es-placsp", set)
	}
}

// TestFetchWithDetail_NoCriteriaYieldsNoEntry pins that a folder publishing no
// qualification block contributes nothing to the map, rather than an empty set
// the sink would then write (and, under replace semantics, use to clear rows).
func TestFetchWithDetail_NoCriteriaYieldsNoEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(freshFeed()))
	}))
	defer srv.Close()
	src := &Source{api: placspapi.NewWithURL(srv.URL)}

	_, criteria, err := src.FetchWithDetail(context.Background())
	if err != nil {
		t.Fatalf("FetchWithDetail: %v", err)
	}
	if len(criteria) != 0 {
		t.Errorf("criteria = %+v, want empty for a folder with no qualification block", criteria)
	}
}

func keysOf(m map[string][]tender.SelectionCriterion) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
