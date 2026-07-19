package codice_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/codice"
)

func TestMap_Fields(t *testing.T) {
	value := int64(7085000)
	deadline := time.Date(2026, 5, 13, 23, 59, 0, 0, time.UTC)
	d := codice.Document{
		ContractFolderID:   "104/2026",
		StatusCode:         "EV",
		Title:              "Festejos taurinos",
		CPV:                []string{"79954000", "92310000"},
		EstimatedValue:     &value,
		Currency:           "EUR",
		SubmissionDeadline: &deadline,
		BuyerName:          "Alcaldía del Ayuntamiento de Navas del Madroño",
		NUTS:               "ES432",
		Raw:                []byte(`<ContractFolderStatus>...</ContractFolderStatus>`),
	}

	got := codice.Map(d, "es-placsp")

	if got.Source != "es-placsp" {
		t.Errorf("Source = %q, want es-placsp", got.Source)
	}
	if got.SourceRef != "104/2026" {
		t.Errorf("SourceRef = %q, want 104/2026", got.SourceRef)
	}
	if got.Country != "ES" || got.Language != "es" {
		t.Errorf("locale = %q/%q, want ES/es", got.Country, got.Language)
	}
	if got.Title != "Festejos taurinos" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Buyer.Name != "Alcaldía del Ayuntamiento de Navas del Madroño" {
		t.Errorf("Buyer.Name = %q", got.Buyer.Name)
	}
	if got.CPV != "79954000" {
		t.Errorf("CPV = %q, want primary 79954000", got.CPV)
	}
	if len(got.CPVSecondary) != 1 || got.CPVSecondary[0] != "92310000" {
		t.Errorf("CPVSecondary = %v, want [92310000]", got.CPVSecondary)
	}
	if got.Value == nil || *got.Value != 7085000 {
		t.Errorf("Value = %v, want 7085000 (CODICE carries a value for ES)", got.Value)
	}
	if got.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", got.Currency)
	}
	if got.NUTS != "ES432" {
		t.Errorf("NUTS = %q, want ES432", got.NUTS)
	}
	if got.Deadline == nil || !got.Deadline.Equal(deadline) {
		t.Errorf("Deadline = %v, want %v", got.Deadline, deadline)
	}
	if got.Status != tender.StatusOpen {
		t.Errorf("Status = %q, want open", got.Status)
	}
	if !json.Valid(got.Raw) {
		t.Errorf("Raw must be valid JSON for the jsonb column, got %s", got.Raw)
	}
}

func TestMap_StatusFromCode(t *testing.T) {
	cases := []struct {
		code string
		want tender.Status
	}{
		{"EV", tender.StatusOpen},
		{"PUB", tender.StatusOpen},
		{"pub", tender.StatusOpen},
		{"ADJ", tender.StatusAwarded},
		{"RES", tender.StatusAwarded},
		{"ANUL", tender.StatusCancelled},
		{"", tender.StatusUnknown},
		{"WTF", tender.StatusUnknown},
	}
	for _, tc := range cases {
		got := codice.Map(codice.Document{StatusCode: tc.code}, "es-placsp")
		if got.Status != tc.want {
			t.Errorf("statusFromCode(%q) = %q, want %q", tc.code, got.Status, tc.want)
		}
	}
}

func TestMap_SingleCPVHasNoSecondary(t *testing.T) {
	got := codice.Map(codice.Document{CPV: []string{"79954000"}}, "es-placsp")
	if got.CPV != "79954000" {
		t.Errorf("CPV = %q", got.CPV)
	}
	if got.CPVSecondary != nil {
		t.Errorf("CPVSecondary = %v, want nil for a single-CPV folder", got.CPVSecondary)
	}
}

func TestMap_NoValueStaysNil(t *testing.T) {
	got := codice.Map(codice.Document{ContractFolderID: "1/2026"}, "es-placsp")
	if got.Value != nil {
		t.Errorf("Value = %v, want nil when the folder carries no amount", got.Value)
	}
	if got.Raw != nil {
		t.Errorf("Raw = %s, want nil when payload is empty", got.Raw)
	}
}
