package bid

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

func decided(rec company.Verdict, overridden bool, outcome Outcome) Bid {
	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	return Bid{
		Outcome:  outcome,
		Decision: DecisionRecord{Recommendation: rec, Overridden: overridden, RecordedAt: &at},
	}
}

// TestDecisionStatsKeepsDenominatorsApart is the point of the type. types.go
// argues that "" (the check could not run) and insufficient_data (it ran, the
// evidence was thin) are different facts, and that collapsing them makes the
// override rate un-interpretable. Neither may land in Comparable.
func TestDecisionStatsKeepsDenominatorsApart(t *testing.T) {
	s := DecisionStatsOf([]Bid{
		decided("", false, ""),                              // engine could not run
		decided(company.VerdictInsufficientData, false, ""), // engine declined to opine
		decided(company.VerdictGo, false, ""),               // agreed
		decided(company.VerdictNoGo, true, ""),              // overrode
		{},                                                  // never decided
	})

	if s.Tracked != 5 {
		t.Errorf("Tracked = %d, want 5", s.Tracked)
	}
	if s.Decided != 4 {
		t.Errorf("Decided = %d, want 4 — the undecided bid must not count", s.Decided)
	}
	if s.Unrecommended != 1 || s.Inconclusive != 1 {
		t.Errorf("Unrecommended/Inconclusive = %d/%d, want 1/1 — these are two different facts",
			s.Unrecommended, s.Inconclusive)
	}
	if s.Comparable != 2 {
		t.Errorf("Comparable = %d, want 2 — only go/no_go recommendations are overridable", s.Comparable)
	}
	if s.Overridden != 1 {
		t.Errorf("Overridden = %d, want 1", s.Overridden)
	}
	if r, ok := s.OverrideRate(); !ok || r != 0.5 {
		t.Errorf("OverrideRate = %v/%v, want 0.5/true", r, ok)
	}
}

// TestOverrideRateDistinguishesZeroFromUnknown is the contract that stops a
// consumer rendering an empty portfolio as a reassuring 0%.
func TestOverrideRateDistinguishesZeroFromUnknown(t *testing.T) {
	// Agreed every time: a real 0%.
	agreed := DecisionStatsOf([]Bid{decided(company.VerdictGo, false, "")})
	if r, ok := agreed.OverrideRate(); !ok || r != 0 {
		t.Errorf("agreed-every-time = %v/%v, want 0/true — that is a real answer", r, ok)
	}

	// Nothing comparable: NOT 0%, no answer at all.
	for _, s := range []DecisionStats{
		DecisionStatsOf(nil),
		DecisionStatsOf([]Bid{decided("", false, "")}),
		DecisionStatsOf([]Bid{decided(company.VerdictInsufficientData, false, "")}),
	} {
		if _, ok := s.OverrideRate(); ok {
			t.Errorf("OverrideRate reported an answer with Comparable=%d", s.Comparable)
		}
	}
}

// TestWinRateExcludesWithdrawnFromBothSides pins that a withdrawal is not a
// loss: it is our own change of mind before the buyer judged us.
func TestWinRateExcludesWithdrawnFromBothSides(t *testing.T) {
	s := DecisionStatsOf([]Bid{
		decided(company.VerdictGo, false, OutcomeWon),
		decided(company.VerdictGo, false, OutcomeLost),
		decided(company.VerdictGo, false, OutcomeWithdrawn),
		decided(company.VerdictGo, false, ""), // still open
	})
	if s.Won != 1 || s.Lost != 1 || s.Withdrawn != 1 || s.Open != 1 {
		t.Fatalf("outcome tally = %+v", s)
	}
	r, ok := s.WinRate()
	if !ok || r != 0.5 {
		t.Errorf("WinRate = %v/%v, want 0.5/true — withdrawn and open are in neither side", r, ok)
	}

	// Only withdrawals: no judged bids, so no rate.
	only := DecisionStatsOf([]Bid{decided(company.VerdictGo, false, OutcomeWithdrawn)})
	if _, ok := only.WinRate(); ok {
		t.Error("WinRate reported an answer when every bid was withdrawn")
	}
}

func TestWorkbenchStats(t *testing.T) {
	svc, repo, access, _, _ := newBidTestService()
	ctx := context.Background()

	// Empty workbench: a zero tally, not an error, and rates that report
	// themselves as unavailable rather than as a reassuring zero.
	s, err := svc.WorkbenchStats(ctx, "u1", "wb1")
	if err != nil {
		t.Fatalf("WorkbenchStats on empty workbench: %v", err)
	}
	if s.Tracked != 0 {
		t.Errorf("Tracked = %d, want 0", s.Tracked)
	}
	if _, ok := s.OverrideRate(); ok {
		t.Error("an empty workbench reported an override rate")
	}

	repo.bids = map[string]Bid{
		"b1": decided(company.VerdictNoGo, true, OutcomeWon),
		"b2": decided(company.VerdictGo, false, OutcomeLost),
	}
	for id, b := range repo.bids {
		b.ID, b.WorkbenchID = id, "wb1"
		repo.bids[id] = b
	}

	s, err = svc.WorkbenchStats(ctx, "u1", "wb1")
	if err != nil {
		t.Fatalf("WorkbenchStats: %v", err)
	}
	if s.Comparable != 2 || s.Overridden != 1 {
		t.Errorf("Comparable/Overridden = %d/%d, want 2/1", s.Comparable, s.Overridden)
	}
	if r, ok := s.WinRate(); !ok || r != 0.5 {
		t.Errorf("WinRate = %v/%v, want 0.5/true", r, ok)
	}

	// Read access is enforced, same as ListBids.
	access.accessErr["wb1"] = errors.New("denied")
	if _, err := svc.WorkbenchStats(ctx, "u1", "wb1"); err == nil {
		t.Error("WorkbenchStats returned stats for a workbench the user cannot access")
	}
}
