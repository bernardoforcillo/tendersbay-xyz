package alerting

import "context"

// Recipient is one person to mail about a bid.
//
// Locale travels with the address because a reminder is the first thing this
// product ever sends unprompted, and sending it in the wrong language would be
// a poor introduction. It is the workspace member's own setting, not the
// tender's.
type Recipient struct {
	Email       string
	DisplayName string
	Locale      string
	// UnsubscribeToken authorises a one-click opt-out WITHOUT a login. It is
	// resolved per recipient rather than built here because minting a signed
	// token is a credential operation and this package holds no keys.
	//
	// It is not optional. A reminder is direct-marketing-adjacent even though
	// the user tracked the gara themselves, so every message needs a working
	// one-click unsubscribe and the List-Unsubscribe headers that go with it.
	// A Recipient without a token is a bug, and the orchestrator refuses to
	// mail one rather than sending a message the reader cannot escape.
	UnsubscribeToken string
}

// Repo is the persistence port.
type Repo interface {
	// ListDueCandidates returns tracked bids worth considering, with their
	// deadline, decision, stage, outcome, unconfirmed-requirement count and
	// watermark. It may over-return: Due applies the rule, so the query is free
	// to be a coarse filter and stay simple.
	ListDueCandidates(ctx context.Context) ([]Candidate, error)

	// RecipientsFor returns the workspace members who should hear about a
	// workbench and have NOT opted out. Filtering opt-outs here rather than in
	// the orchestrator keeps "who may be mailed" one question with one answer,
	// in the layer that can see the suppression list.
	RecipientsFor(ctx context.Context, workbenchID string) ([]Recipient, error)

	// OptOut records that the holder of token no longer wants reminders, and
	// reports whether a row actually matched.
	//
	// The bool exists so the caller can LOG a miss without TELLING one: an
	// unknown token must produce the same page as a known one, or the endpoint
	// becomes an oracle for which tokens are live. It is unauthenticated by
	// necessity — the whole point is that a reader can escape without finding
	// their password first.
	OptOut(ctx context.Context, token string) (bool, error)

	// MarkReminded advances a bid's watermark to bucket.
	//
	// It records that a bucket was PROCESSED, not that mail was delivered, and
	// the distinction is deliberate. A bid whose recipients have all opted out
	// still advances, or the job would reconsider it every hour forever for
	// nobody's benefit. What must not advance is a bucket where every send
	// failed — see Service.Run.
	MarkReminded(ctx context.Context, bidID string, bucket int) error
}

// Mailer sends one reminder. Implemented by the email adapter, which owns
// rendering, the sending domain, and the List-Unsubscribe headers.
//
// The reminder and the recipient are passed whole rather than pre-rendered
// strings: the adapter localises, and a caller that formatted the subject line
// here would have put copy in the domain layer.
type Mailer interface {
	SendReminder(ctx context.Context, to Recipient, r Reminder) error
}
