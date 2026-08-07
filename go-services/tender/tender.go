// Package tender defines the normalized tender domain model shared between
// services/ingestion (the writer) and backend (a future reader). It is pure
// data — no methods beyond what's needed for encoding, no DB tags, no I/O —
// so importing it never pulls in a database driver or any write-path
// dependency.
package tender

import (
	"encoding/json"
	"time"
)

// Tender is the canonical shape any provider produces.
//
// The fields from PublicationNumber down describe the machine-readable notice
// document a source may publish (today only TED's eForms XML); every other
// provider leaves them zero, which is why each is nullable or defaulted in
// storage — see GridUsable for the one case where the zero value is *not* an
// acceptable stand-in for "not looked at yet".
//
// Only PublicationNumber and XMLURL are populated by a provider's own mapping,
// because they are the two the SEARCH result carries. The rest exist only
// inside the notice document, which is fetched separately and later: they are
// produced as a NoticeDetail and written by that pass, so on a Tender value
// coming out of a provider they are always zero. Read them from the store or
// from NoticeDetail, never from a freshly-mapped Tender.
type Tender struct {
	Source        string // e.g. "ted", "it-mepa" — the provider's Name()
	SourceRef     string // stable per-provider id (notice number, etc.)
	Title         string
	Description   string // free-text scope of work; "" when the source exposes none
	Buyer         Buyer
	Status        Status   // normalized lifecycle status — see below
	ProcedureType string   // e.g. "open", "restricted", "negotiated", "competitive-dialogue"
	Language      string   // ISO 639-1 of the original text ("it", "fr", …); "" when unknown
	Country       string   // ISO-3166 alpha-2
	NUTS          string   // NUTS region code; "" when unknown
	CPV           string   // main/primary Common Procurement Vocabulary code
	CPVSecondary  []string // additional CPV codes; nil when none
	Value         *int64   // minor units; nil when unknown
	Currency      string   // ISO-4217; "" when unknown
	PublishedAt   *time.Time
	Deadline      *time.Time
	Documents     []Document      // notice PDF, technical specs, corrigenda, …
	Lots          []Lot           // procurement lots; nil for single-lot tenders
	Raw           json.RawMessage // untouched provider payload

	// PublicationNumber is the notice's own publication id (TED: "545620-2026"),
	// which is distinct from SourceRef — SourceRef is the *procedure* identifier,
	// deliberately stable so a contract notice and its later award notice collapse
	// onto one row. Publication numbers are immutable: a corrected notice is
	// republished under a new one. That makes this field the change key for
	// notice-document enrichment — same number means the document we already read
	// cannot have changed, so it is never re-fetched.
	PublicationNumber string

	// XMLURL is the machine-readable notice document (TED's eForms XML). Kept
	// separate from Documents because Documents holds files a *bidder* reads,
	// while this is a file the *ingester* reads.
	XMLURL string

	// DocumentsURL and SubmissionURL are the buyer-portal entry points published
	// in the notice: where the tender documents live, and where a bid is
	// submitted. They are stored, never followed — fetching a buyer portal carries
	// robots-compliance and SSRF obligations this layer's writers do not have.
	DocumentsURL  string
	SubmissionURL string

	// Criteria are the notice-level award criteria (those with an empty LotRef).
	// Lot-scoped criteria live on their Lot, not here, so the two never
	// double-count — see NoticeDetail.AllCriteria for the flat union.
	Criteria []AwardCriterion

	// Organizations are the parties named in the notice: the buyer plus the
	// appeal, appeal-information and mediation bodies a bidder needs in order to
	// challenge an award.
	Organizations []Organization

	// GridUsable records whether this notice publishes at least one *weighted*
	// award criterion — i.e. whether a scoring grid can actually be reconstructed
	// from it. It is persisted rather than derived on read because it is a filter
	// predicate over the whole corpus; computing it on read turns an indexed
	// lookup into a correlated subquery against a child table per row, which this
	// codebase has already paid for once and reverted.
	//
	// It is deliberately three-valued in storage (NULL / false / true): NULL means
	// "not enriched yet, or a source that publishes no criteria at all", false
	// means "we read the notice and there is no grid". Collapsing those two makes
	// absence of evidence indistinguishable from evidence of absence, which is the
	// exact conflation the criteria coverage numbers exist to prevent. This bool
	// therefore only carries meaning alongside the row's enrichment status; do not
	// treat a zero value here as a measured false.
	GridUsable bool
}

// Buyer identifies the contracting authority. ID is a stable
// registration/VAT number when the provider exposes one, letting
// cross-tender analysis follow the same buyer over time; it is "" when
// unknown.
type Buyer struct {
	Name string
	ID   string
}

// Document is one file attached to a notice (the notice itself, technical
// specs, a corrigendum, …).
type Document struct {
	URL  string
	Type string // e.g. "notice", "spec", "corrigendum"
}

// Lot is one procurement lot within a multi-lot tender, carrying its own
// scope, CPV, value, and deadline. Single-lot tenders have no Lot entries —
// their scope lives directly on Tender's own CPV/Value/Deadline.
//
// The eForms XML nests each lot's terms structurally inside the lot element, so
// these per-lot fields are genuinely per-lot and not an index-aligned guess.
// In practice only Value and Title reliably differ between the lots of one
// notice — CPV, the two URLs and the criteria set are frequently identical
// across every lot. That degeneracy is expected, not a parse bug.
type Lot struct {
	Ref      string // lot number/identifier within the tender
	Title    string
	CPV      string
	Value    *int64
	Currency string
	Deadline *time.Time

	CPVSecondary []string // additional CPV codes for this lot; nil when none

	// DocumentsURL and SubmissionURL override the notice-level ones when the
	// buyer publishes a per-lot portal entry point; "" means "use the notice's".
	DocumentsURL  string
	SubmissionURL string

	// Criteria are the award criteria scoped to this lot (LotRef == Ref). A
	// notice publishing one criteria set for the whole procurement leaves this
	// empty and carries them at notice level instead.
	Criteria []AwardCriterion
}

// DocumentPart is one extracted chunk of a tender document, carrying the
// provenance a citation needs: which pages it came from and where it sits in
// the document's heading hierarchy. Text alone is retrievable but not citable —
// an answer that cannot say "capitolato, §4.2, p. 17" cannot be checked, and an
// unverifiable answer about award requirements is worse than no answer.
//
// PageStart/PageEnd are 1-based and inclusive. Zero means the extractor
// reported no page — true for every part extracted before this metadata was
// captured, which are not backfilled (re-deriving them means re-fetching and
// re-extracting the entire PDF corpus). Readers must treat 0 as unknown, never
// as page zero.
type DocumentPart struct {
	Text         string
	PageStart    int
	PageEnd      int
	SectionPath  []string // heading ancestry, outermost first; nil when the document has no detected structure
	SectionTitle string   // the innermost heading this part sits under
	HasTable     bool     // the part contains tabular content — requirement grids are usually tables
}

// Status is the normalized tender lifecycle, shared across all providers.
// Each connector maps its source's native status text onto this fixed enum
// inside its Fetch implementation; a provider that can't confidently map a
// native status emits StatusUnknown rather than guessing.
type Status string

const (
	StatusOpen      Status = "open"      // accepting bids
	StatusAwarded   Status = "awarded"   // contract awarded
	StatusCancelled Status = "cancelled" // withdrawn/annulled
	StatusClosed    Status = "closed"    // deadline passed, no award recorded
	StatusUnknown   Status = "unknown"   // provider status didn't map cleanly
)
