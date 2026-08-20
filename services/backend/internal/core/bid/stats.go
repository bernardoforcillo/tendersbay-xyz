// Package bid — this file derives the portfolio numbers from a set of bids:
// how often the user disagreed with the engine, and how the bids they pursued
// actually closed.
//
// It exists because the answer is currently only computable client-side, from
// PostHog events, and the DecisionRecord comment in types.go argues at length
// that a client-side answer is not an answer: "a disagreement metric whose
// baseline the disagreeing party supplies measures nothing". This is the
// server-side derivation that comment implies must exist.
//
// It is a PURE FUNCTION over bids already loaded, not a SQL aggregate, and that
// is deliberate. Overrides() is the single definition of "the user disagreed";
// a hand-written SQL aggregate would be a second definition, free to drift from
// it silently and precisely where nobody would look. Deriving in Go keeps one
// rule, exhaustively testable, in the layer that owns it.
package bid

import "github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"

// DecisionStats is one scope's decision and outcome tally.
//
// The counts are deliberately not collapsed into two headline numbers, because
// the denominators are not interchangeable and the whole value of this metric is
// in which denominator a rate was taken over.
type DecisionStats struct {
	// Tracked is every bid in scope, decided or not.
	Tracked int
	// Decided is the bids carrying a recorded decision.
	Decided int

	// Unrecommended counts decisions taken with NO recommendation at all — the
	// eligibility check could not be run. types.go is explicit that this is a
	// different fact from insufficient_data ("it ran, and the evidence was too
	// thin"), and that collapsing the two makes the override rate
	// un-interpretable. So they are two fields, and neither is in Comparable.
	Unrecommended int
	// Inconclusive counts decisions taken against insufficient_data. Overrides()
	// deliberately does not treat these as disagreement: there was no opinion to
	// contradict, and counting them would inflate the rate with exactly the
	// cases where the product declined to have one.
	Inconclusive int

	// Comparable is the only honest denominator for OverrideRate: decisions
	// taken against an actual go or no_go recommendation.
	Comparable int
	// Overridden is how many of Comparable contradicted the recommendation.
	Overridden int

	Won, Lost, Withdrawn, Open int
}

// DecisionStatsOf tallies a set of bids. Bids still undecided contribute to
// Tracked and to the outcome counts only.
func DecisionStatsOf(bids []Bid) DecisionStats {
	var s DecisionStats
	s.Tracked = len(bids)
	for _, b := range bids {
		switch b.Outcome {
		case OutcomeWon:
			s.Won++
		case OutcomeLost:
			s.Lost++
		case OutcomeWithdrawn:
			s.Withdrawn++
		default:
			s.Open++
		}

		// A decision is counted from RecordedAt rather than from GoNoGo being
		// non-undecided: RecordedAt is stamped by the same write that stores the
		// baseline, so a row with one and not the other cannot be half-counted
		// here.
		if b.Decision.RecordedAt == nil {
			continue
		}
		s.Decided++
		switch b.Decision.Recommendation {
		case "":
			s.Unrecommended++
		case company.VerdictInsufficientData:
			s.Inconclusive++
		default:
			s.Comparable++
			if b.Decision.Overridden {
				s.Overridden++
			}
		}
	}
	return s
}

// OverrideRate is Overridden over Comparable, and the bool reports whether the
// rate means anything at all.
//
// It returns false rather than 0 when there is nothing to divide by. Zero is a
// real and very different answer — "the user agreed with us every time" — and a
// caller that rendered an empty portfolio as a 0% override rate would be showing
// a reassuring number derived from no evidence. Every consumer must decide what
// to show when there is no answer; none of them may be handed a fabricated one.
func (s DecisionStats) OverrideRate() (float64, bool) {
	if s.Comparable == 0 {
		return 0, false
	}
	return float64(s.Overridden) / float64(s.Comparable), true
}

// WinRate is Won over decided outcomes, and the bool carries the same
// no-denominator contract as OverrideRate.
//
// Withdrawn bids are excluded from BOTH sides, not counted as losses. A
// withdrawal is our own change of mind before the buyer ever judged us; folding
// it into the denominator would measure how often we start bids we do not
// finish, under a label that reads as how often we win the ones we do.
func (s DecisionStats) WinRate() (float64, bool) {
	judged := s.Won + s.Lost
	if judged == 0 {
		return 0, false
	}
	return float64(s.Won) / float64(judged), true
}
