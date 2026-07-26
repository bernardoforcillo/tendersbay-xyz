// Package bid owns the bid lifecycle (go/no-go -> ESPD checklist -> stage ->
// outcome) inside a workbench. It defines the ports it needs (workbench access,
// tender fit, tender summaries, its own repository) and never imports
// internal/adapter/* — the adapters implement its ports (hexagonal, per
// .claude/rules/code-organization.md).
package bid

import (
	"errors"
	"time"

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

// Bid is one tender tracked inside a workbench. Fit and the live tender
// summary are computed fresh on read (spec §5) and are NOT stored here.
type Bid struct {
	ID          string
	WorkbenchID string
	TenderID    int64
	GoNoGo      GoNoGo
	Stage       Stage
	Outcome     Outcome
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
