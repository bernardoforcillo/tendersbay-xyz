package connectapi

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func TestToProtoBid_MapsAvailableTenderWithFit(t *testing.T) {
	val := int64(150000)
	days := 12
	view := bid.BidView{
		Bid: bid.Bid{
			ID: "bid-1", WorkbenchID: "wb-1", TenderID: 42,
			GoNoGo: bid.GoNoGoGo, Stage: bid.StagePreparing, Outcome: "",
		},
		TenderAvailable: true,
		Summary: tender.TenderSummary{
			ID: 42, Title: "Lavori stradali", BuyerName: "Comune di Roma",
			Country: "IT", CPV: "45210000", Currency: "EUR",
			Deadline: "2026-09-01T00:00:00Z", Status: "open", Value: &val,
		},
		Fit: tender.TenderFitResult{
			Tier:       tender.FitTier("strong"),
			Reason:     tender.ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "in_band", DeadlineDays: &days},
			HasProfile: true,
			Available:  true,
		},
		ChecklistDone:  3,
		ChecklistTotal: 8,
	}

	got := toProtoBid(view)

	if got.Id != "bid-1" || got.WorkbenchId != "wb-1" {
		t.Fatalf("ids = %q/%q, want bid-1/wb-1", got.Id, got.WorkbenchId)
	}
	if got.TenderId != "42" {
		t.Fatalf("TenderId = %q, want \"42\" (stringified int64)", got.TenderId)
	}
	if got.GoNoGo != "go" || got.Stage != "preparing" {
		t.Fatalf("go_no_go/stage = %q/%q, want go/preparing", got.GoNoGo, got.Stage)
	}
	if !got.TenderAvailable {
		t.Fatal("TenderAvailable = false, want true")
	}
	if got.TenderTitle != "Lavori stradali" || got.TenderCountry != "IT" || got.TenderCpv != "45210000" {
		t.Fatalf("summary = %+v, want title/country/cpv populated", got)
	}
	if got.TenderValue != 150000 || got.TenderCurrency != "EUR" {
		t.Fatalf("value/currency = %d/%q, want 150000/EUR", got.TenderValue, got.TenderCurrency)
	}
	if got.NeedsProfile {
		t.Fatal("NeedsProfile = true, want false (HasProfile is true)")
	}
	if got.FitTier != "strong" {
		t.Fatalf("FitTier = %q, want strong", got.FitTier)
	}
	if got.Reason == nil || !got.Reason.SectorMatch || !got.Reason.CountryMatch {
		t.Fatalf("Reason = %+v, want sector+country match", got.Reason)
	}
	if !got.Reason.HasDeadline || got.Reason.DeadlineDays != 12 {
		t.Fatalf("Reason deadline = %v/%d, want true/12", got.Reason.HasDeadline, got.Reason.DeadlineDays)
	}
	if got.ChecklistDone != 3 || got.ChecklistTotal != 8 {
		t.Fatalf("checklist = %d/%d, want 3/8", got.ChecklistDone, got.ChecklistTotal)
	}
}

func TestToProtoBid_DanglingTenderLeavesSummaryEmpty(t *testing.T) {
	view := bid.BidView{
		Bid:             bid.Bid{ID: "bid-2", WorkbenchID: "wb-1", TenderID: 99, GoNoGo: bid.GoNoGoUndecided, Stage: bid.StageShortlisted},
		TenderAvailable: false,
		Fit:             tender.TenderFitResult{Tier: "", HasProfile: false, Available: false},
		ChecklistDone:   0,
		ChecklistTotal:  8,
	}

	got := toProtoBid(view)

	if got.TenderAvailable {
		t.Fatal("TenderAvailable = true, want false (dangling)")
	}
	if got.TenderTitle != "" || got.TenderCpv != "" || got.TenderValue != 0 {
		t.Fatalf("dangling summary must stay empty, got title=%q cpv=%q value=%d", got.TenderTitle, got.TenderCpv, got.TenderValue)
	}
	if !got.NeedsProfile {
		t.Fatal("NeedsProfile = false, want true (HasProfile is false)")
	}
	if got.FitTier != "" {
		t.Fatalf("FitTier = %q, want empty (no profile)", got.FitTier)
	}
	if got.TenderId != "99" {
		t.Fatalf("TenderId = %q, want \"99\"", got.TenderId)
	}
}

func TestToProtoChecklistItem_MapsFields(t *testing.T) {
	got := toProtoChecklistItem(bid.ChecklistItem{
		ID: "ci-1", BidID: "bid-1", SectionCode: "part_iii", ItemCode: "iii_a_convictions",
		Status: "done", Note: "n/a", Required: true, Position: 4,
	})
	if got.Id != "ci-1" || got.SectionCode != "part_iii" || got.ItemCode != "iii_a_convictions" {
		t.Fatalf("got = %+v", got)
	}
	if got.Status != "done" || got.Note != "n/a" || !got.Required || got.Position != 4 {
		t.Fatalf("got = %+v, want status/note/required/position mapped", got)
	}
}
