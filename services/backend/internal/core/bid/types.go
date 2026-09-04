// Package bid owns the bid lifecycle (go/no-go -> ESPD checklist -> stage ->
// outcome) inside a workbench. It defines the ports it needs (workbench access,
// tender fit, tender summaries, its own repository) and never imports
// internal/adapter/* — the adapters implement its ports (hexagonal, per
// .claude/rules/code-organization.md).
package bid

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
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

// ── ESPD per-bid data (Part I lots, II.C reliance, II.D subcontracting) ──────
//
// These are facts about ONE bid, not about the company: which lots we tender
// for, whose capacities we rely on (avvalimento), whom we subcontract to. They
// carry no provenance envelope because they are not facts a buyer can demand a
// certificate for — they are the operator's choices for this gara, made in the
// workbench by a member who may manage it.

// Lot is one lot of the procedure this bid tenders for. LotRef is the buyer's
// lot identifier as published (eForms "LOT-0001", or a free "1" / "A" on an
// Italian portal); Position is display order.
type Lot struct {
	ID       string
	BidID    string
	LotRef   string
	Position int32
}

// Subcontractor is a Part II.D entry. VAT is the natural key within a bid: a
// portal that lists the same subcontractor twice describes one relationship.
// Share is the percentage of the contract subcontracted, nil when unstated.
type Subcontractor struct {
	ID      string
	BidID   string
	Name    string
	VAT     string
	Country string // alpha-2, "" when unstated
	Share   *int32 // percent, [0,100]
}

// Reliance is a Part II.C entry: this bid relies on another entity's capacity
// for one selection criterion (avvalimento, Art. 63 of Directive 2014/24/EU).
// Criterion is the espd.CriterionKey the reliance covers, a string here so bid
// never imports espd.
type Reliance struct {
	ID         string
	BidID      string
	EntityName string
	VAT        string
	Criterion  string
}

// DeclarationConfirmation records that a member re-confirmed, for THIS bid,
// every Part III declaration the dossier held at that moment. DeclarationsHash
// is the espd.HashDeclarations of that set: when the dossier's declarations
// change afterwards the hash no longer matches, the confirmation is stale, and
// the composed response reports a gap rather than silently exporting an answer
// nobody re-read. No cron and no flag — staleness is recomputed on every read.
type DeclarationConfirmation struct {
	BidID            string
	UserID           string
	ConfirmedAt      time.Time
	DeclarationsHash string
}

// EspdData is the per-bid ESPD input read back as one unit, which is how the
// scheda gara renders it and how espd.Compose consumes it.
type EspdData struct {
	Lots           []Lot
	Subcontractors []Subcontractor
	Reliances      []Reliance
}

const (
	maxLotRefLen  = 64
	maxPartyLen   = 200
	maxVATLen     = 32
	maxCriterion  = 128
	maxShareValue = 100
)

func (l Lot) validate() error {
	ref := strings.TrimSpace(l.LotRef)
	if ref == "" || len(ref) > maxLotRefLen {
		return fmt.Errorf("%w: lot_ref is required and at most %d characters", ErrInvalidArgument, maxLotRefLen)
	}
	if l.Position < 0 {
		return fmt.Errorf("%w: lot position must not be negative", ErrInvalidArgument)
	}
	return nil
}

func (s Subcontractor) validate() error {
	if strings.TrimSpace(s.Name) == "" || len(s.Name) > maxPartyLen {
		return fmt.Errorf("%w: subcontractor name is required and at most %d characters", ErrInvalidArgument, maxPartyLen)
	}
	if strings.TrimSpace(s.VAT) == "" || len(s.VAT) > maxVATLen {
		return fmt.Errorf("%w: subcontractor vat is required and at most %d characters", ErrInvalidArgument, maxVATLen)
	}
	if s.Country != "" && !alpha2Re.MatchString(s.Country) {
		return fmt.Errorf("%w: subcontractor country must be an uppercase ISO-3166-1 alpha-2 code", ErrInvalidArgument)
	}
	if s.Share != nil && (*s.Share < 0 || *s.Share > maxShareValue) {
		return fmt.Errorf("%w: subcontracted share %d outside [0,%d]", ErrInvalidArgument, *s.Share, maxShareValue)
	}
	return nil
}

func (r Reliance) validate() error {
	if strings.TrimSpace(r.EntityName) == "" || len(r.EntityName) > maxPartyLen {
		return fmt.Errorf("%w: relied-on entity name is required and at most %d characters", ErrInvalidArgument, maxPartyLen)
	}
	if strings.TrimSpace(r.VAT) == "" || len(r.VAT) > maxVATLen {
		return fmt.Errorf("%w: relied-on entity vat is required and at most %d characters", ErrInvalidArgument, maxVATLen)
	}
	if strings.TrimSpace(r.Criterion) == "" || len(r.Criterion) > maxCriterion {
		return fmt.Errorf("%w: reliance criterion is required and at most %d characters", ErrInvalidArgument, maxCriterion)
	}
	return nil
}

var alpha2Re = regexp.MustCompile(`^[A-Z]{2}$`)

// ErrConfirmationNotFound is returned by GetDeclarationConfirmation when the
// bid's Part III declarations have never been confirmed. It is a normal state
// on a fresh bid, not a failure; Compose turns it into a Stale gap.
var ErrConfirmationNotFound = errors.New("bid: declarations not yet confirmed for this bid")
