package boamp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/boampapi"
)

// donnees builds the JSON-in-a-string blob BOAMP nests under fields.donnees,
// carrying the real CPV inside the EFORMS notice. main is the primary CPV;
// additional are the secondary codes (emitted as an array, matching the feed).
func donnees(t *testing.T, main string, additional ...string) string {
	t.Helper()
	project := map[string]any{}
	if main != "" {
		project["cac:MainCommodityClassification"] = map[string]any{
			"cbc:ItemClassificationCode": map[string]any{"@listName": "cpv", "#text": main},
		}
	}
	if len(additional) > 0 {
		codes := make([]any, len(additional))
		for i, c := range additional {
			codes[i] = map[string]any{
				"cbc:ItemClassificationCode": map[string]any{"@listName": "cpv", "#text": c},
			}
		}
		project["cac:AdditionalCommodityClassification"] = codes
	}
	b, err := json.Marshal(map[string]any{
		"EFORMS": map[string]any{
			"ContractNotice": map[string]any{"cac:ProcurementProject": project},
		},
	})
	if err != nil {
		t.Fatalf("build donnees: %v", err)
	}
	return string(b)
}

func TestMap(t *testing.T) {
	got := Map(boampapi.Record{
		Idweb:             "26-71206",
		Objet:             "Marché de véhicules",
		NomAcheteur:       "Commune de Saint-Benoît",
		DateLimiteReponse: "2026-08-18T08:00:00+00:00",
		DateParution:      "2026-07-19",
		Nature:            "APPEL_OFFRE",
		NatureCategorise:  "appeloffre/standard",
		Donnees:           donnees(t, "34100000"),
		Raw:               json.RawMessage(`{"recordid":"x"}`),
	}, "fr-boamp")

	if got.Source != "fr-boamp" {
		t.Errorf("Source = %q, want fr-boamp", got.Source)
	}
	if got.SourceRef != "26-71206" {
		t.Errorf("SourceRef = %q, want 26-71206", got.SourceRef)
	}
	if got.Title != "Marché de véhicules" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Buyer.Name != "Commune de Saint-Benoît" {
		t.Errorf("Buyer.Name = %q", got.Buyer.Name)
	}
	if got.Country != "FR" || got.Language != "fr" {
		t.Errorf("locale = %q/%q, want FR/fr", got.Country, got.Language)
	}
	if got.Status != tender.StatusOpen {
		t.Errorf("Status = %q, want open", got.Status)
	}
	if got.CPV != "34100000" {
		t.Errorf("CPV = %q, want 34100000 (dug out of donnees)", got.CPV)
	}
	if got.CPVSecondary != nil {
		t.Errorf("CPVSecondary = %v, want nil (no additional classifications)", got.CPVSecondary)
	}
	if got.Deadline == nil || !got.Deadline.Equal(time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("Deadline = %v, want 2026-08-18T08:00:00Z", got.Deadline)
	}
	if got.PublishedAt == nil || !got.PublishedAt.Equal(time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("PublishedAt = %v, want 2026-07-19", got.PublishedAt)
	}
	if string(got.Raw) != `{"recordid":"x"}` {
		t.Errorf("Raw = %s, want the untouched record payload", got.Raw)
	}
	if got.Value != nil {
		t.Errorf("Value = %v, want nil (BOAMP search feed carries no clean estimate here)", got.Value)
	}
}

func TestMap_SecondaryCPVFromAdditionalArray(t *testing.T) {
	got := Map(boampapi.Record{
		Idweb:        "26-71214",
		Nature:       "APPEL_OFFRE",
		Donnees:      donnees(t, "71351810", "71353000", "71355000"),
		DateParution: "2026-07-19",
	}, "fr-boamp")

	if got.CPV != "71351810" {
		t.Errorf("CPV = %q, want 71351810", got.CPV)
	}
	want := []string{"71353000", "71355000"}
	if len(got.CPVSecondary) != len(want) {
		t.Fatalf("CPVSecondary = %v, want %v", got.CPVSecondary, want)
	}
	for i := range want {
		if got.CPVSecondary[i] != want[i] {
			t.Errorf("CPVSecondary[%d] = %q, want %q", i, got.CPVSecondary[i], want[i])
		}
	}
}

func TestMap_Status(t *testing.T) {
	cases := []struct {
		name             string
		nature           string
		natureCategorise string
		want             tender.Status
	}{
		{"appel offre code", "APPEL_OFFRE", "appeloffre/standard", tender.StatusOpen},
		{"attribution", "ATTRIBUTION", "attribution/standard", tender.StatusAwarded},
		{"attribution wins over other tokens", "APPEL_OFFRE", "attribution/rectificatif", tender.StatusAwarded},
		{"avis de marche label", "avis de marché", "", tender.StatusOpen},
		{"unrecognized", "AVIS_DIVERS", "autre", tender.StatusUnknown},
		{"empty", "", "", tender.StatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Map(boampapi.Record{Nature: tc.nature, NatureCategorise: tc.natureCategorise}, "fr-boamp")
			if got.Status != tc.want {
				t.Errorf("Status = %q, want %q", got.Status, tc.want)
			}
		})
	}
}

func TestMap_MissingOptionalFields(t *testing.T) {
	got := Map(boampapi.Record{Idweb: "26-0", Nature: "APPEL_OFFRE"}, "fr-boamp")

	if got.Deadline != nil {
		t.Errorf("Deadline = %v, want nil (no datelimitereponse)", got.Deadline)
	}
	if got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil (no dateparution)", got.PublishedAt)
	}
	if got.CPV != "" {
		t.Errorf("CPV = %q, want empty (no donnees to dig CPV out of)", got.CPV)
	}
	if got.CPVSecondary != nil {
		t.Errorf("CPVSecondary = %v, want nil", got.CPVSecondary)
	}
}
