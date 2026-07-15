package eforms_test

import (
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
)

func loadNotice(t *testing.T, path string) eforms.Notice {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	n, err := eforms.Decode(data)
	if err != nil {
		t.Fatalf("Decode(%s): %v", path, err)
	}
	return n
}

func TestMap_CNStandard_SingleLot(t *testing.T) {
	n := loadNotice(t, "testdata/cn_standard.json")
	got := eforms.Map(n)

	if got.Source != "ted" {
		t.Errorf("Source = %q, want %q", got.Source, "ted")
	}
	if got.SourceRef != "proc-cn-1" {
		t.Errorf("SourceRef = %q, want %q (procedure-identifier, not publication-number)", got.SourceRef, "proc-cn-1")
	}
	if got.Status != tender.StatusOpen {
		t.Errorf("Status = %q, want %q (cn-* prefix)", got.Status, tender.StatusOpen)
	}
	if got.Title != "Lucrări de drum" {
		t.Errorf("Title = %q, want the RON title (official-language)", got.Title)
	}
	if got.Buyer.Name != "Municipiul Blaj" || got.Buyer.ID != "RO 4563007" {
		t.Errorf("Buyer = %+v, want Name=Municipiul Blaj ID=RO 4563007", got.Buyer)
	}
	if got.Language != "ro" {
		t.Errorf("Language = %q, want %q", got.Language, "ro")
	}
	if got.Country != "RO" {
		t.Errorf("Country = %q, want %q (alpha-3 ROU converted to alpha-2)", got.Country, "RO")
	}
	if got.CPV != "45233220" || len(got.CPVSecondary) != 2 {
		t.Errorf("CPV = %q, CPVSecondary = %v, want primary=45233220 and 2 secondary codes", got.CPV, got.CPVSecondary)
	}
	if got.Value == nil || *got.Value != 2213454901 {
		t.Errorf("Value = %v, want 2213454901 (22134549.01 in minor units)", got.Value)
	}
	if got.Currency != "RON" {
		t.Errorf("Currency = %q, want %q", got.Currency, "RON")
	}
	wantDeadline := time.Date(2026, 8, 11, 15, 0, 0, 0, time.FixedZone("", 3*3600))
	if got.Deadline == nil || !got.Deadline.Equal(wantDeadline) {
		t.Errorf("Deadline = %v, want %v", got.Deadline, wantDeadline)
	}
	wantPublished := time.Date(2026, 7, 9, 0, 0, 0, 0, time.FixedZone("", 2*3600))
	if got.PublishedAt == nil || !got.PublishedAt.Equal(wantPublished) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, wantPublished)
	}
	if len(got.Lots) != 0 {
		t.Errorf("Lots = %+v, want empty (single-lot tender keeps scope on Tender itself)", got.Lots)
	}
	if len(got.Documents) != 1 || got.Documents[0].URL != "https://ted.europa.eu/ro/notice/472141-2026/pdf" || got.Documents[0].Type != "notice" {
		t.Errorf("Documents = %+v, want one RON pdf link of Type notice", got.Documents)
	}
}

func TestMap_CANStandard_MissingOptionalFields(t *testing.T) {
	n := loadNotice(t, "testdata/can_standard.json")
	got := eforms.Map(n)

	if got.Status != tender.StatusAwarded {
		t.Errorf("Status = %q, want %q (can-* prefix)", got.Status, tender.StatusAwarded)
	}
	if got.Value != nil {
		t.Errorf("Value = %v, want nil (estimated-value-proc absent from fixture)", got.Value)
	}
	if got.Deadline != nil {
		t.Errorf("Deadline = %v, want nil (deadline fields absent from fixture)", got.Deadline)
	}
	if got.Country != "DE" {
		t.Errorf("Country = %q, want %q", got.Country, "DE")
	}
	if got.Buyer.ID != "fb197f94-7578-4673-8a57-4642ae120532" {
		t.Errorf("Buyer.ID = %q, want the raw organisation-identifier-buyer value verbatim", got.Buyer.ID)
	}
}

func TestMap_MultiLot_PopulatesLotsWithRefAndTitleOnly(t *testing.T) {
	n := loadNotice(t, "testdata/can_standard_multilot.json")
	got := eforms.Map(n)

	if len(got.Lots) != 3 {
		t.Fatalf("len(Lots) = %d, want 3", len(got.Lots))
	}
	if got.Lots[0].Ref != "LOT-0001" || got.Lots[0].Title != "Instrumente chirurgicale" {
		t.Errorf("Lots[0] = %+v, want Ref=LOT-0001 Title=Instrumente chirurgicale", got.Lots[0])
	}
	if got.Lots[1].Ref != "LOT-0002" || got.Lots[2].Ref != "LOT-0003" {
		t.Errorf("Lots refs = [%q, %q], want [LOT-0002, LOT-0003]", got.Lots[1].Ref, got.Lots[2].Ref)
	}
	for i, l := range got.Lots {
		if l.CPV != "" || l.Value != nil || l.Deadline != nil {
			t.Errorf("Lots[%d] = %+v, want zero CPV/Value/Deadline (not populated in this cut)", i, l)
		}
	}
	if got.Deadline != nil {
		t.Errorf("Deadline = %v, want nil for a multi-lot notice (no single value represents 3 lots)", got.Deadline)
	}
}

func TestMap_StatusUnknown_ForUnrecognizedNoticeType(t *testing.T) {
	n := eforms.Notice{NoticeType: "pin-standard", ProcedureIdentifier: "proc-x"}
	got := eforms.Map(n)
	if got.Status != tender.StatusUnknown {
		t.Errorf("Status = %q, want %q for an unmapped notice-type prefix", got.Status, tender.StatusUnknown)
	}
}

func TestMap_RawPreservesOriginalBytes(t *testing.T) {
	n := loadNotice(t, "testdata/cn_standard.json")
	got := eforms.Map(n)
	if len(got.Raw) == 0 {
		t.Error("Raw is empty, want the original notice bytes")
	}
}
