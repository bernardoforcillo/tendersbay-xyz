package frboamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
)

func TestSource_Name(t *testing.T) {
	if got := New().Name(); got != "fr-boamp" {
		t.Errorf("Name() = %q, want %q", got, "fr-boamp")
	}
}

// fixtureBody builds a one-record BOAMP search response dated today, so it
// always falls inside the Source's rolling 24h fetch window regardless of when
// the test runs. The donnees blob carries a real CPV to exercise the full
// transport -> parser -> mapper chain.
func fixtureBody(t *testing.T) []byte {
	t.Helper()
	donnees, err := json.Marshal(map[string]any{
		"EFORMS": map[string]any{
			"ContractNotice": map[string]any{
				"cac:ProcurementProject": map[string]any{
					"cac:MainCommodityClassification": map[string]any{
						"cbc:ItemClassificationCode": map[string]any{"@listName": "cpv", "#text": "34100000"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build donnees: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"nhits": 1,
		"records": []map[string]any{{
			"recordid": "cdd6cc56",
			"fields": map[string]any{
				"idweb":             "26-71206",
				"objet":             "Marché de véhicules",
				"nomacheteur":       "Commune de Saint-Benoît",
				"datelimitereponse": "2026-08-18T08:00:00+00:00",
				"dateparution":      time.Now().UTC().Format("2006-01-02"),
				"nature":            "APPEL_OFFRE",
				"nature_categorise": "appeloffre/standard",
				"donnees":           string(donnees),
			},
		}},
	})
	if err != nil {
		t.Fatalf("build body: %v", err)
	}
	return body
}

func TestSource_Fetch(t *testing.T) {
	body := fixtureBody(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := &Source{api: boampapi.NewWithURL(srv.URL)}
	tenders, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(tenders) != 1 {
		t.Fatalf("len(tenders) = %d, want 1", len(tenders))
	}
	got := tenders[0]
	if got.Source != "fr-boamp" {
		t.Errorf("Source = %q, want fr-boamp", got.Source)
	}
	if got.SourceRef != "26-71206" {
		t.Errorf("SourceRef = %q, want 26-71206", got.SourceRef)
	}
	if got.Country != "FR" || got.Language != "fr" {
		t.Errorf("locale = %q/%q, want FR/fr", got.Country, got.Language)
	}
	if got.Status != tender.StatusOpen {
		t.Errorf("Status = %q, want open", got.Status)
	}
	if got.CPV != "34100000" {
		t.Errorf("CPV = %q, want 34100000 (dug out of donnees end-to-end)", got.CPV)
	}
}

func TestSource_Fetch_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	src := &Source{api: boampapi.NewWithURL(srv.URL)}

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
