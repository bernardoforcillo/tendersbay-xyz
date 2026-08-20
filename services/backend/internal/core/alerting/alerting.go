// Package alerting decides when the product should reach out about a bid the
// user is already working on.
//
// It exists because nothing in this system has ever contacted a user
// unprompted: the mail adapter ships five templates and all five are
// authentication or invitation. Every competitor's core product is a daily
// alert; this is deliberately NOT that. A "new matches" digest competes with a
// free one from TenderWolf and is judged on match precision from the first
// send. A reminder about a deadline on a gara the user THEMSELVES tracked
// competes with nothing, carries no precision risk, and points at something
// real — which is also what keeps its urgency honest rather than manufactured.
//
// The decision here is pure. Fetching candidates, rendering mail and recording
// what was sent are the caller's business, behind the ports in ports.go, so the
// rule about when a person deserves an email is exhaustively testable without a
// database or an SMTP server anywhere near it.
package alerting

import (
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// buckets are the days-before-deadline at which a bid earns one reminder, from
// widest to narrowest. They are the whole idempotency mechanism.
//
// A scheduler that runs hourly and reminds on "deadline is near" sends
// twenty-four mails a day. Bucketing turns the continuous countdown into four
// discrete events: a bid gets at most one mail per bucket it passes through, so
// the ceiling is four mails per bid over its entire life, no matter how often
// the job runs or how many times it retries. Nothing anywhere needs a lock.
//
// The values are spaced so each one still leaves time to act on what it says:
// fourteen days to arrange an avvalimento or find an RTI partner, seven to
// gather certificates, three to finish the paperwork, one as a last call.
var buckets = []int{14, 7, 3, 1}

// Reason is why a reminder is being sent, and it is carried rather than
// re-derived at render time so a consumer cannot quietly change what the mail
// claims.
type Reason string

const (
	// ReasonDeadline — the deadline is approaching and the bid looks ready.
	ReasonDeadline Reason = "deadline"
	// ReasonUnconfirmedRequirements — the deadline is approaching AND
	// requirements are still unconfirmed. A strictly more urgent thing to say,
	// so it is a different reason rather than a flag on the first: the mail
	// leads with the blocker, not with the date.
	ReasonUnconfirmedRequirements Reason = "unconfirmed_requirements"
)

// Candidate is one tracked bid, with everything the rule needs and nothing
// else. It deliberately carries no recipient: who gets told is a workspace
// membership question, resolved by the caller after this decides that anyone
// should be told at all.
type Candidate struct {
	BidID       string
	WorkbenchID string
	TenderID    int64
	TenderTitle string

	// Deadline is the tender's submission deadline. nil means the notice never
	// published one, which is common enough that it must be a normal skip
	// rather than an error: a reminder needs something to count down to.
	Deadline *time.Time

	Decision bid.GoNoGo
	Stage    bid.Stage
	Outcome  bid.Outcome

	// UnconfirmedRequirements is how many requirements are still unconfirmed —
	// the count that separates "your deadline is near" from "your deadline is
	// near and you are not ready".
	UnconfirmedRequirements int

	// LastRemindedBucket is the narrowest bucket already mailed for this bid; 0
	// means none has been. It is the watermark, and it is per bid rather than
	// per user so that one noisy bid cannot suppress a reminder about another.
	LastRemindedBucket int
}

// Reminder is one mail to send.
type Reminder struct {
	Candidate Candidate
	Reason    Reason
	// Bucket is the threshold this reminder fires at, and it is what the caller
	// must persist as the new watermark. Persisting the actual days-remaining
	// instead would re-arm the same bucket on the next run.
	Bucket int
	// DaysLeft is the real countdown at decision time, for the mail to render.
	// It can be smaller than Bucket when a job has not run for a while.
	DaysLeft int
}

// Due returns the reminders that should be sent right now, in the order given.
//
// A candidate is skipped, silently and by design, when it is closed, already
// submitted, decided against, has no deadline, or has already been reminded at
// this bucket or a narrower one. None of those is an error: they are the normal
// state of most bids most of the time, and a rule that logged them would drown
// the one signal worth reading.
func Due(candidates []Candidate, now time.Time) []Reminder {
	var out []Reminder
	for _, c := range candidates {
		r, ok := due(c, now)
		if !ok {
			continue
		}
		out = append(out, r)
	}
	return out
}

func due(c Candidate, now time.Time) (Reminder, bool) {
	// A closed bid is finished, whatever the calendar says.
	if c.Outcome != "" {
		return Reminder{}, false
	}
	// Submitted means the deadline no longer threatens anything.
	if c.Stage == bid.StageSubmitted {
		return Reminder{}, false
	}
	// A no_go is a decision not to bid. Reminding someone about a gara they
	// deliberately dropped is the product second-guessing them, which is the
	// opposite of what an override-respecting design does.
	if c.Decision == bid.GoNoGoNoGo {
		return Reminder{}, false
	}
	if c.Deadline == nil {
		return Reminder{}, false
	}

	days := daysUntil(*c.Deadline, now)
	// A passed deadline is not a reminder. Nothing can be done about it, and
	// saying so is noise dressed as urgency.
	if days < 0 {
		return Reminder{}, false
	}

	bucket, ok := bucketFor(days)
	if !ok {
		return Reminder{}, false
	}
	// Already reminded at this bucket, or at a narrower (later, more urgent)
	// one. The watermark only ever moves toward the deadline.
	if c.LastRemindedBucket != 0 && bucket >= c.LastRemindedBucket {
		return Reminder{}, false
	}

	reason := ReasonDeadline
	if c.UnconfirmedRequirements > 0 {
		reason = ReasonUnconfirmedRequirements
	}
	return Reminder{Candidate: c, Reason: reason, Bucket: bucket, DaysLeft: days}, true
}

// bucketFor returns the NARROWEST bucket the countdown has already entered —
// the smallest threshold that still contains days.
//
// Narrowest, not widest, and the difference is the whole mechanism. At seven
// days a bid has entered both the 14 and the 7 bucket; returning 14 would
// compare equal to a watermark already at 14 and the seven-day reminder would
// never fire at all. Scanning from the tightest threshold upward returns 7,
// which is strictly narrower than the watermark, so it fires and then advances
// it.
//
// It also means a job that has not run for a week fires the bucket the bid is
// ACTUALLY in rather than replaying every one it slept through: at two days
// with a watermark of 14, this returns 3, one mail, not two.
func bucketFor(days int) (int, bool) {
	for i := len(buckets) - 1; i >= 0; i-- {
		if days <= buckets[i] {
			return buckets[i], true
		}
	}
	return 0, false
}

// daysUntil counts whole days between now and the deadline, truncating toward
// zero, so "23 hours away" is 0 days — the last-call bucket — and not 1.
func daysUntil(deadline, now time.Time) int {
	return int(deadline.Sub(now).Hours() / 24)
}
