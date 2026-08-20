package alerting

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

var now = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

func candidate(daysOut int, mut ...func(*Candidate)) Candidate {
	d := now.Add(time.Duration(daysOut) * 24 * time.Hour)
	c := Candidate{BidID: "b1", TenderTitle: "Servizio", Deadline: &d, Decision: bid.GoNoGoGo, Stage: bid.StagePreparing}
	for _, m := range mut {
		m(&c)
	}
	return c
}

// TestBucketsFireOnceEachAsTheDeadlineApproaches is the idempotency mechanism:
// an hourly job must not send an hourly mail. A bid walking 20 → 0 days should
// produce exactly four reminders, one per threshold.
func TestBucketsFireOnceEachAsTheDeadlineApproaches(t *testing.T) {
	watermark := 0
	var fired []int
	for _, days := range []int{20, 15, 14, 13, 10, 8, 7, 6, 4, 3, 2, 1, 0} {
		c := candidate(days, func(c *Candidate) { c.LastRemindedBucket = watermark })
		got := Due([]Candidate{c}, now)
		if len(got) == 0 {
			continue
		}
		if len(got) != 1 {
			t.Fatalf("at %d days: %d reminders, want at most 1", days, len(got))
		}
		fired = append(fired, got[0].Bucket)
		watermark = got[0].Bucket
	}

	want := []int{14, 7, 3, 1}
	if len(fired) != len(want) {
		t.Fatalf("fired buckets %v, want %v — one mail per threshold, no more", fired, want)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Fatalf("fired buckets %v, want %v", fired, want)
		}
	}
}

// TestSkippedBucketsDoNotReplay pins that a job which slept through thresholds
// sends ONE mail for where the bid actually is, not one per threshold missed.
func TestSkippedBucketsDoNotReplay(t *testing.T) {
	c := candidate(2, func(c *Candidate) { c.LastRemindedBucket = 14 })
	got := Due([]Candidate{c}, now)
	if len(got) != 1 {
		t.Fatalf("got %d reminders, want 1", len(got))
	}
	if got[0].Bucket != 3 {
		t.Errorf("bucket = %d, want 3 — the bid is in the 3-day window, not the 7", got[0].Bucket)
	}
	if got[0].DaysLeft != 2 {
		t.Errorf("DaysLeft = %d, want the real countdown 2", got[0].DaysLeft)
	}
}

func TestSuppressions(t *testing.T) {
	noDeadline := candidate(5)
	noDeadline.Deadline = nil

	tests := []struct {
		name string
		c    Candidate
	}{
		{"closed bid", candidate(5, func(c *Candidate) { c.Outcome = bid.OutcomeWon })},
		{"withdrawn bid", candidate(5, func(c *Candidate) { c.Outcome = bid.OutcomeWithdrawn })},
		{"already submitted", candidate(5, func(c *Candidate) { c.Stage = bid.StageSubmitted })},
		{"decided against", candidate(5, func(c *Candidate) { c.Decision = bid.GoNoGoNoGo })},
		{"no deadline published", noDeadline},
		{"deadline already passed", candidate(-1)},
		{"too far out", candidate(30)},
		{"already reminded at this bucket", candidate(5, func(c *Candidate) { c.LastRemindedBucket = 7 })},
		{"already reminded at a narrower one", candidate(5, func(c *Candidate) { c.LastRemindedBucket = 1 })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Due([]Candidate{tt.c}, now); len(got) != 0 {
				t.Errorf("got %+v, want no reminder", got)
			}
		})
	}
}

// TestUndecidedStillReminded: only an explicit no_go suppresses. A bid the user
// has not decided on yet is exactly the one a deadline should chase.
func TestUndecidedStillReminded(t *testing.T) {
	c := candidate(5, func(c *Candidate) { c.Decision = bid.GoNoGoUndecided })
	if got := Due([]Candidate{c}, now); len(got) != 1 {
		t.Errorf("an undecided bid was not reminded — that is the one that needs chasing")
	}
}

// TestReasonLeadsWithTheBlocker: when requirements are unconfirmed, the mail
// must say so rather than leading with the date.
func TestReasonLeadsWithTheBlocker(t *testing.T) {
	plain := Due([]Candidate{candidate(5)}, now)
	if len(plain) != 1 || plain[0].Reason != ReasonDeadline {
		t.Fatalf("reason = %+v, want deadline", plain)
	}
	blocked := Due([]Candidate{candidate(5, func(c *Candidate) { c.UnconfirmedRequirements = 2 })}, now)
	if len(blocked) != 1 || blocked[0].Reason != ReasonUnconfirmedRequirements {
		t.Fatalf("reason = %+v, want unconfirmed_requirements", blocked)
	}
}

// TestLastCallBucketCountsPartialDays: a deadline 23 hours away is the 1-day
// last call, not two days out.
func TestLastCallBucketCountsPartialDays(t *testing.T) {
	d := now.Add(23 * time.Hour)
	c := Candidate{BidID: "b1", Deadline: &d, Decision: bid.GoNoGoGo, Stage: bid.StagePreparing}
	got := Due([]Candidate{c}, now)
	if len(got) != 1 || got[0].Bucket != 1 {
		t.Fatalf("got %+v, want the 1-day bucket", got)
	}
}
