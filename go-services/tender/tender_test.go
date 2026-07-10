package tender_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

func TestTenderJSONRoundTrip(t *testing.T) {
	deadline := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	value := int64(150000)
	want := tender.Tender{
		Source:    "ted",
		SourceRef: "2026/S 123-456789",
		Title:     "Supply of office chairs",
		Buyer:     tender.Buyer{Name: "Comune di Roma", ID: "IT-VAT-12345"},
		Status:    tender.StatusOpen,
		Country:   "IT",
		CPV:       "39112000",
		Value:     &value,
		Currency:  "EUR",
		Deadline:  &deadline,
		Documents: []tender.Document{{URL: "https://example.org/notice.pdf", Type: "notice"}},
		Lots:      []tender.Lot{{Ref: "LOT-1", Title: "Chairs", CPV: "39112000", Currency: "EUR"}},
		Raw:       json.RawMessage(`{"id":"raw-payload"}`),
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got tender.Tender
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Source != want.Source || got.Buyer != want.Buyer || got.Status != want.Status {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Documents) != 1 || got.Documents[0] != want.Documents[0] {
		t.Fatalf("Documents round-trip mismatch: got %+v", got.Documents)
	}
	if len(got.Lots) != 1 || got.Lots[0].Ref != want.Lots[0].Ref {
		t.Fatalf("Lots round-trip mismatch: got %+v", got.Lots)
	}
	if got.Value == nil || *got.Value != *want.Value {
		t.Fatalf("Value round-trip mismatch: got %v, want %d", got.Value, *want.Value)
	}
	if got.Deadline == nil || !got.Deadline.Equal(*want.Deadline) {
		t.Fatalf("Deadline round-trip mismatch: got %v, want %v", got.Deadline, want.Deadline)
	}
}

func TestStatusConstants(t *testing.T) {
	all := []tender.Status{
		tender.StatusOpen, tender.StatusAwarded, tender.StatusCancelled,
		tender.StatusClosed, tender.StatusUnknown,
	}
	want := []string{"open", "awarded", "cancelled", "closed", "unknown"}
	for i, s := range all {
		if string(s) != want[i] {
			t.Errorf("status %d = %q, want %q", i, s, want[i])
		}
	}
}
