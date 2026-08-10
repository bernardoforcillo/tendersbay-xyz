package connectapi

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
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

// TestToProtoBid_CarriesTheDecisionRecord pins the wire half of the override
// measurement. The recommendation, the derived override flag and the size of
// the disagreement have to leave the server: the assessment is computed fresh on
// every read, so nothing downstream can reconstruct what the user decided
// AGAINST once the dossier moves on.
func TestToProtoBid_CarriesTheDecisionRecord(t *testing.T) {
	at := time.Date(2026, 5, 2, 11, 30, 0, 0, time.UTC)
	got := toProtoBidEntity(bid.Bid{
		ID: "bid-1", WorkbenchID: "wb-1", TenderID: 42, GoNoGo: bid.GoNoGoGo,
		Decision: bid.DecisionRecord{
			Recommendation:   company.VerdictNoGo,
			Overridden:       true,
			BlockingGapCount: 3,
			RecordedAt:       &at,
		},
	})
	if got.DecisionRecommendation != "no_go" {
		t.Errorf("decision_recommendation = %q, want no_go", got.DecisionRecommendation)
	}
	if !got.DecisionOverridden {
		t.Error("decision_overridden = false — go_no_go_overridden can never fire without it")
	}
	if got.DecisionBlockingGapCount != 3 {
		t.Errorf("decision_blocking_gap_count = %d, want 3", got.DecisionBlockingGapCount)
	}
	if got.DecisionRecordedAt != at.Format(time.RFC3339) {
		t.Errorf("decision_recorded_at = %q, want %q", got.DecisionRecordedAt, at.Format(time.RFC3339))
	}
}

// TestToProtoBid_DistinguishesNoCheckFromInsufficientData is the distinction the
// whole record exists to preserve: "" means no eligibility check existed, and
// "insufficient_data" means one ran and found the evidence too thin. Collapsing
// them would put every undecided bid in the same bucket as every genuinely
// unanswerable one and make the override rate un-interpretable.
func TestToProtoBid_DistinguishesNoCheckFromInsufficientData(t *testing.T) {
	undecided := toProtoBidEntity(bid.Bid{ID: "bid-1", GoNoGo: bid.GoNoGoUndecided})
	if undecided.DecisionRecommendation != "" {
		t.Errorf("decision_recommendation = %q, want \"\" for a bid no check ever ran on", undecided.DecisionRecommendation)
	}
	if undecided.DecisionRecordedAt != "" {
		t.Errorf("decision_recorded_at = %q, want \"\" — an undecided bid has no decision time", undecided.DecisionRecordedAt)
	}

	thin := toProtoBidEntity(bid.Bid{ID: "bid-2", Decision: bid.DecisionRecord{
		Recommendation: company.VerdictInsufficientData,
	}})
	if thin.DecisionRecommendation != "insufficient_data" {
		t.Errorf("decision_recommendation = %q, want insufficient_data", thin.DecisionRecommendation)
	}
	if thin.DecisionOverridden {
		t.Error("decision_overridden = true — a decision under acknowledged uncertainty contradicts nothing")
	}
}
