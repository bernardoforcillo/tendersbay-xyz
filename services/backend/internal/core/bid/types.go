// Package bid owns the bid lifecycle (go/no-go -> ESPD checklist -> stage ->
// outcome) inside a workbench. It defines the ports it needs (workbench access,
// tender fit, tender summaries, its own repository) and never imports
// internal/adapter/* — the adapters implement its ports (hexagonal, per
// .claude/rules/code-organization.md).
package bid

import (
	"errors"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// GoNoGo is the pursue/skip decision on a tracked bid.
type GoNoGo string

const (
	GoNoGoUndecided GoNoGo = "undecided" // default on AddBid
	GoNoGoGo        GoNoGo = "go"
	GoNoGoNoGo      GoNoGo = "no_go"
)

// Stage is the forward-only preparation stage.
type Stage string

const (
	StageShortlisted Stage = "shortlisted" // default on AddBid
	StagePreparing   Stage = "preparing"
	StageSubmitted   Stage = "submitted"
)

// Outcome is the terminal close. "" means still open.
type Outcome string

const (
	OutcomeWon       Outcome = "won"
	OutcomeLost      Outcome = "lost"
	OutcomeWithdrawn Outcome = "withdrawn"
)

// DecisionRecord is the eligibility recommendation a go/no-go was taken
// against, captured at the moment the human decided.
//
// It is STORED, unlike fit and unlike the assessment itself, and the reason is
// the one thing it is for: "how often does the user disagree with us, and about
// what" is the trust signal this product lives or dies on, and it is
// unanswerable after the fact. The assessment is computed fresh on every read
// (company.Assessment is deliberately never persisted), so by the time anyone
// asks the question, the dossier has moved, the requirements have moved, and the
// recommendation that was actually on screen is gone. A client-side analytics
// event cannot stand in for this: the client would be asserting what we
// recommended, and a disagreement metric whose baseline the disagreeing party
// supplies measures nothing.
//
// Recording it does NOT gate the decision. The human carries the liability and
// decides; this records what they decided against, and nothing here can refuse
// a go on a no_go recommendation — that is the whole point of an override being
// representable.
type DecisionRecord struct {
	// Recommendation is the verdict the eligibility engine produced for this
	// bid's tender at decision time. "" means NO recommendation existed — the
	// check could not be run at all — which is a different fact from
	// company.VerdictInsufficientData ("it ran, and the evidence was too thin"),
	// and collapsing the two would make the override rate un-interpretable.
	Recommendation company.Verdict
	// Overridden reports that the decision CONTRADICTS the recommendation. It is
	// derived, never taken from a caller.
	Overridden bool
	// BlockingGapCount is how many blocking gaps stood at decision time. It is
	// the size of the disagreement: overriding a no_go that rests on one lapsed
	// certificate is a different act from overriding one with four blocking gaps.
	BlockingGapCount int
	// RecordedAt is when the decision was taken. nil on a bid still undecided.
	RecordedAt *time.Time
}

// Overrides reports whether decision d contradicts recommendation v.
//
// company.VerdictInsufficientData is deliberately NOT an override, whichever way
// the user decides. There was no recommendation to contradict — the engine said
// "we cannot tell" — and counting a decision under acknowledged uncertainty as
// a disagreement would inflate the override rate with exactly the cases where
// the product declined to have an opinion.
func Overrides(v company.Verdict, d GoNoGo) bool {
	switch v {
	case company.VerdictNoGo:
		return d == GoNoGoGo
	case company.VerdictGo:
		return d == GoNoGoNoGo
	default:
		return false
	}
}

// Bid is one tender tracked inside a workbench. Fit and the live tender
// summary are computed fresh on read (spec §5) and are NOT stored here.
type Bid struct {
	ID          string
	WorkbenchID string
	TenderID    int64
	GoNoGo      GoNoGo
	Stage       Stage
	Outcome     Outcome
	// Decision is the eligibility recommendation this bid's go/no-go was taken
	// against. Zero-valued until SetGoNoGo runs.
	Decision  DecisionRecord
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChecklistItem is one persisted ESPD/DGUE line for a bid. SectionCode/ItemCode
// are stable i18n code stems, never label text — the frontend localizes them.
type ChecklistItem struct {
	ID          string
	BidID       string
	SectionCode string
	ItemCode    string
	Status      string // "pending" | "done" | "na"
	Note        string
	Required    bool
	Position    int
	UpdatedAt   time.Time
}

// ChecklistItemSeed is a template row the service seeds on AddBid.
type ChecklistItemSeed struct {
	SectionCode string
	ItemCode    string
	Required    bool
	Position    int
}

// BidView is the assembled read model: the stored bid plus a fresh tender
// summary and fit, and checklist progress. Assembled by the read methods
// (ListBids/GetBid) — added in the tender-additions-dependent part.
type BidView struct {
	Bid             Bid
	TenderAvailable bool
	Summary         tender.TenderSummary
	Fit             tender.TenderFitResult
	ChecklistDone   int
	ChecklistTotal  int
}

var (
	ErrBidNotFound           = errors.New("bid: not found")
	ErrBidExists             = errors.New("bid: already tracked for this tender in this workbench")
	ErrBidNotGo              = errors.New("bid: action requires a go decision")
	ErrInvalidTransition     = errors.New("bid: invalid stage transition")
	ErrChecklistItemNotFound = errors.New("bid: checklist item not found")
	// ErrInvalidArgument is the bad-input sentinel (invalid enum value, missing
	// referenced tender, invalid checklist status). toConnectError has no generic
	// bad-input catch-all (its default returns CodeInternal), so P1.8 adds an
	// explicit `case errors.Is(err, bid.ErrInvalidArgument)` -> CodeInvalidArgument
	// (spec §7). Without that case these validation errors would 500.
	ErrInvalidArgument = errors.New("bid: invalid argument")
)
