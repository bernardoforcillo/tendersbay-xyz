package bid

import (
	"errors"
	"testing"
)

func TestEnumConstants(t *testing.T) {
	if GoNoGoUndecided != "undecided" || GoNoGoGo != "go" || GoNoGoNoGo != "no_go" {
		t.Fatalf("go_no_go enum drift: %q %q %q", GoNoGoUndecided, GoNoGoGo, GoNoGoNoGo)
	}
	if StageShortlisted != "shortlisted" || StagePreparing != "preparing" || StageSubmitted != "submitted" {
		t.Fatalf("stage enum drift: %q %q %q", StageShortlisted, StagePreparing, StageSubmitted)
	}
	if OutcomeWon != "won" || OutcomeLost != "lost" || OutcomeWithdrawn != "withdrawn" {
		t.Fatalf("outcome enum drift: %q %q %q", OutcomeWon, OutcomeLost, OutcomeWithdrawn)
	}
}

func TestSentinelErrorsDistinct(t *testing.T) {
	all := []error{
		ErrBidNotFound, ErrBidExists, ErrBidNotGo,
		ErrInvalidTransition, ErrChecklistItemNotFound, ErrInvalidArgument,
	}
	for i := range all {
		if all[i] == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		for j := range all {
			if i != j && errors.Is(all[i], all[j]) {
				t.Fatalf("sentinels %d and %d must be distinct", i, j)
			}
		}
	}
}

func TestBidViewComposes(t *testing.T) {
	// Forces the compile dependency on the tender-additions structs (spec §5).
	var v BidView
	if v.TenderAvailable || v.ChecklistDone != 0 || v.ChecklistTotal != 0 {
		t.Fatal("unexpected BidView zero value")
	}
	v.Bid.GoNoGo = GoNoGoUndecided
	v.Summary.ID = 1        // tender.TenderSummary.ID (int64)
	v.Fit.HasProfile = true // tender.TenderFitResult.HasProfile (bool)
	if v.Summary.ID != 1 || !v.Fit.HasProfile {
		t.Fatal("BidView field wiring wrong")
	}
}
