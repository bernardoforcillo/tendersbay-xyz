package bid

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// decisionFixture wires a service whose eligibility port returns a, tracks one
// bid, and hands back everything an assertion needs.
func decisionFixture(t *testing.T, a company.Assessment) (*Service, *fakeRepo, *fakeEligibility, Bid) {
	t.Helper()
	svc, repo, access, tenders, eligibility := newBidTestService()
	access.workspace["wb1"] = "ws1"
	tenders.summaries[42] = tender.TenderSummary{ID: 42, Title: "Lavori stradali"}
	eligibility.assessment = a

	b, err := svc.AddBid(context.Background(), "u1", "wb1", 42)
	if err != nil {
		t.Fatalf("AddBid: %v", err)
	}
	return svc, repo, eligibility, b
}

// TestSetGoNoGoRecordsTheRecommendationItAgreedWith — the baseline is stored
// even when nobody disagreed. Without the agreeing case an override rate has no
// denominator.
func TestSetGoNoGoRecordsTheRecommendationItAgreedWith(t *testing.T) {
	svc, repo, eligibility, b := decisionFixture(t, company.Assessment{
		Verdict: company.VerdictGo, RequirementCount: 2, AuthoritativeRequirements: 2,
	})

	got, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, GoNoGoGo)
	if err != nil {
		t.Fatalf("SetGoNoGo: %v", err)
	}
	if eligibility.calls != 1 {
		t.Fatalf("eligibility consulted %d times, want exactly 1", eligibility.calls)
	}
	if got.Decision.Recommendation != company.VerdictGo {
		t.Errorf("recommendation = %q, want go", got.Decision.Recommendation)
	}
	if got.Decision.Overridden {
		t.Error("agreeing with the recommendation is not an override")
	}
	if got.Decision.RecordedAt == nil || !got.Decision.RecordedAt.Equal(decidedAt) {
		t.Errorf("recorded_at = %v, want %v", got.Decision.RecordedAt, decidedAt)
	}
	// The record must survive the round trip, not merely be returned.
	if stored := repo.bids[b.ID]; stored.Decision != got.Decision {
		t.Errorf("stored decision = %+v, want %+v", stored.Decision, got.Decision)
	}
}

// TestSetGoNoGoRecordsAnOverride is the trust signal: the user pursued a gara
// the engine advised against. It must be RECORDABLE — the human carries the
// liability and decides — and it must be VISIBLE, because "how often do they
// disagree with us, and about what" is unanswerable after the fact (the
// assessment is never persisted, so the dossier and the requirements have both
// moved by the time anyone asks).
func TestSetGoNoGoRecordsAnOverride(t *testing.T) {
	svc, repo, _, b := decisionFixture(t, company.Assessment{
		Verdict: company.VerdictNoGo, RequirementCount: 3,
		AuthoritativeRequirements: 3, BlockingGapCount: 2,
	})

	got, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, GoNoGoGo)
	if err != nil {
		t.Fatalf("a decision that contradicts the recommendation must still be recordable: %v", err)
	}
	if got.GoNoGo != GoNoGoGo {
		t.Fatalf("go_no_go = %q, want go — the recommendation never gates the decision", got.GoNoGo)
	}
	if !got.Decision.Overridden {
		t.Error("a go against a no_go recommendation is an override")
	}
	if got.Decision.Recommendation != company.VerdictNoGo {
		t.Errorf("recommendation = %q, want no_go", got.Decision.Recommendation)
	}
	// The size of the disagreement: overriding one lapsed certificate is a
	// different act from overriding two blocking gaps.
	if got.Decision.BlockingGapCount != 2 {
		t.Errorf("blocking gaps = %d, want 2", got.Decision.BlockingGapCount)
	}
	if stored := repo.bids[b.ID]; !stored.Decision.Overridden {
		t.Error("the override must be persisted, not only returned")
	}
}

// TestSetGoNoGoRecordsTheOppositeOverride — skipping a gara we said was fine is
// the other half of the signal, and the more interesting half: it is where the
// engine is wrong in the direction that costs the user money.
func TestSetGoNoGoRecordsTheOppositeOverride(t *testing.T) {
	svc, _, _, b := decisionFixture(t, company.Assessment{
		Verdict: company.VerdictGo, RequirementCount: 4, AuthoritativeRequirements: 4,
	})

	got, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, GoNoGoNoGo)
	if err != nil {
		t.Fatalf("SetGoNoGo: %v", err)
	}
	if !got.Decision.Overridden {
		t.Error("a no_go against a go recommendation is an override")
	}
}

// TestSetGoNoGoInsufficientDataIsNeverAnOverride — the engine declined to have
// an opinion, so there was nothing to contradict. Counting these would inflate
// the override rate with exactly the cases where the product said "we cannot
// tell", and would then read as users distrusting advice we never gave.
func TestSetGoNoGoInsufficientDataIsNeverAnOverride(t *testing.T) {
	for _, d := range []GoNoGo{GoNoGoGo, GoNoGoNoGo} {
		svc, _, _, b := decisionFixture(t, company.Assessment{
			Verdict: company.VerdictInsufficientData, RequirementCount: 0,
		})
		got, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, d)
		if err != nil {
			t.Fatalf("SetGoNoGo(%s): %v", d, err)
		}
		if got.Decision.Overridden {
			t.Errorf("decision %q against insufficient_data must not count as an override", d)
		}
		if got.Decision.Recommendation != company.VerdictInsufficientData {
			t.Errorf("recommendation = %q, want insufficient_data recorded verbatim", got.Decision.Recommendation)
		}
	}
}

// TestSetGoNoGoSurvivesAnUnavailableEligibilityCheck — a decision the user has
// already made must be recordable even when we cannot say what we would have
// advised. Failing the write would turn a missing assessment into an outage in
// the lifecycle, and would do so precisely where the engine is weakest: a tender
// we hold no documents for.
func TestSetGoNoGoSurvivesAnUnavailableEligibilityCheck(t *testing.T) {
	svc, repo, eligibility, b := decisionFixture(t, company.Assessment{})
	eligibility.err = errors.New("tender detail unavailable")

	got, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, GoNoGoGo)
	if err != nil {
		t.Fatalf("an unavailable eligibility check must not block the decision: %v", err)
	}
	if got.GoNoGo != GoNoGoGo {
		t.Fatalf("go_no_go = %q, want go", got.GoNoGo)
	}
	// "" is the honest value: no recommendation EXISTED. It is a different fact
	// from insufficient_data ("it ran, and the evidence was too thin"), and
	// collapsing the two would make the override rate un-interpretable.
	if got.Decision.Recommendation != "" {
		t.Errorf("recommendation = %q, want \"\" (no recommendation existed)", got.Decision.Recommendation)
	}
	if got.Decision.Overridden {
		t.Error("there is nothing to override when no recommendation was produced")
	}
	if got.Decision.RecordedAt == nil {
		t.Error("we still know WHEN the human decided, even without a baseline")
	}
	if stored := repo.bids[b.ID]; stored.GoNoGo != GoNoGoGo {
		t.Error("the decision itself must be persisted regardless")
	}
}

// TestSetGoNoGoStillValidatesBeforeConsultingEligibility — an invalid decision
// is rejected before any eligibility work is done. Consulting the engine for a
// decision that will not be stored is both wasted and misleading: the
// recommendation would be computed against a decision that never happened.
func TestSetGoNoGoStillValidatesBeforeConsultingEligibility(t *testing.T) {
	svc, _, eligibility, b := decisionFixture(t, company.Assessment{Verdict: company.VerdictGo})

	if _, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", b.ID, GoNoGoUndecided); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
	if eligibility.calls != 0 {
		t.Errorf("eligibility consulted %d times for a rejected decision, want 0", eligibility.calls)
	}
}

// TestOverrides pins the rule itself, decoupled from the service, since it is
// the one line the whole trust metric rests on.
func TestOverrides(t *testing.T) {
	tests := []struct {
		verdict company.Verdict
		d       GoNoGo
		want    bool
	}{
		{company.VerdictNoGo, GoNoGoGo, true},
		{company.VerdictNoGo, GoNoGoNoGo, false},
		{company.VerdictGo, GoNoGoNoGo, true},
		{company.VerdictGo, GoNoGoGo, false},
		{company.VerdictInsufficientData, GoNoGoGo, false},
		{company.VerdictInsufficientData, GoNoGoNoGo, false},
		// "" is "no recommendation existed" — never an override.
		{"", GoNoGoGo, false},
		{"", GoNoGoNoGo, false},
	}
	for _, tt := range tests {
		if got := Overrides(tt.verdict, tt.d); got != tt.want {
			t.Errorf("Overrides(%q, %q) = %t, want %t", tt.verdict, tt.d, got, tt.want)
		}
	}
}
