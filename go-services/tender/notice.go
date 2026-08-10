package tender

import "time"

// AwardCriterion is one entry of a notice's award grid: a dimension the buyer
// scores bids on, optionally with the weight it carries. It is the answer to
// "where are the points won", which is why it is modelled as its own type
// rather than folded into a free-text blob — a criterion's weight has to be
// comparable and aggregatable across notices.
//
// Criteria are published either once for the whole procurement or once per lot;
// LotRef records which. Entries keep the order the notice published them,
// because that order is itself information (buyers list the dominant criterion
// first) and because nothing else about a criterion is unique enough to key on.
type AwardCriterion struct {
	// LotRef ties the criterion to a lot by that lot's own identifier; "" means
	// the criterion is notice-level and applies to the whole procurement. It is
	// the notice's identifier rather than a foreign key so that a criteria set
	// can be written in one statement, without first resolving lot rows.
	LotRef string

	// Ordinal is the criterion's position as published, within its LotRef. It is
	// the stable sort key and the only part of a criterion guaranteed to be
	// distinct, so it (with LotRef) is what identifies an entry.
	Ordinal int

	Type        string // "quality" | "cost" | "price" as declared by the notice
	Name        string
	Description string

	// Weight is the criterion's share of the score. It is a pointer because a
	// zero weight and an absent weight are different facts, and absent is the
	// common case: 24 of 87 sampled criterion entries carry no weight at all.
	// Notices routinely publish a lone criterion naming the award *method*
	// ("offerta economicamente più vantaggiosa") with nothing attached to it —
	// present, but useless for scoring.
	//
	// This is not a style preference. Counting weightless entries as zero-weight
	// ones makes "criteria published" look like "grid recoverable", which
	// overstates real coverage by roughly 2x. Keeping the absence representable
	// blocks that error at the type level instead of relying on every consumer to
	// remember it — so never flatten nil to 0 to simplify a call site.
	Weight *float64

	// WeightRaw is the weight exactly as published. Buyers write it
	// inconsistently ("30", "30%", "30,5"), so the verbatim string is kept
	// alongside the parsed number: a parse failure then loses only the
	// comparability, not the datum, and any parse can be audited after the fact.
	WeightRaw string

	// Lang is the language of Name/Description as the notice declared it, in ISO
	// 639-2/T ("ITA", "ENG"). Multilingual notices repeat every text field once
	// per language, so without recording which one was taken, a reader cannot
	// tell a language choice from a parser accident.
	Lang string
}

// HasWeight reports whether the criterion carries a published weight. Prefer it
// over comparing Weight against nil at call sites, so the nil-is-meaningful
// invariant documented on Weight stays visible in the reading code.
func (c AwardCriterion) HasWeight() bool { return c.Weight != nil }

// Organization is a party named in the notice. Beyond the buyer, notices name
// the bodies a bidder needs in order to challenge an award — which is only
// actionable if the contact details travel with the notice rather than being
// looked up per procurement.
//
// Ref is notice-internal: the notice references its parties by an id it mints
// for itself, so the same real organization has a different Ref in every
// notice. It identifies a row within one tender, and nothing across tenders.
type Organization struct {
	Ref       string // e.g. "ORG-0001"; unique within the notice only
	Name      string
	CompanyID string   // national registration number — codice fiscale / partita IVA in Italy
	Roles     []string // "buyer" | "appeal-body" | "appeal-information" | "mediation"; a party can hold several

	// Email is usually an institutional address (an Italian PEC box, say), but
	// some notices name an individual instead. The data is public — it is
	// published in the EU's own register — yet Name, Email and Phone must never
	// leave this layer into product analytics or any other secondary use.
	Email string

	Phone, Website, City, Country string
}

// NoticeDetail is everything a machine-readable notice document yields about
// one procurement: the structure that is not present in the search-result
// payload the tender was first ingested from. It is the unit written in a
// single transaction, so a reader never observes criteria without the lots they
// score, or a derived flag without the rows it was derived from.
//
// It carries no identifiers of its own. The tender it belongs to is decided by
// the caller that fetched the document, which keeps this type free of any
// storage or transport concern.
type NoticeDetail struct {
	// Lots carry their own criteria; Criteria here holds only the notice-level
	// ones (LotRef == ""). The two partition the criteria set — they never
	// overlap, so counting both is counting each entry once. AllCriteria returns
	// the union in the order it should be persisted.
	Lots     []Lot
	Criteria []AwardCriterion

	Organizations []Organization

	// DocumentsURL and SubmissionURL are the notice-level buyer-portal entry
	// points; a lot may override either.
	DocumentsURL  string
	SubmissionURL string

	// Deadline is the submission deadline with its published UTC offset intact.
	// It is a legally binding instant: dropping the offset to keep a wall-clock
	// time shifts it by up to two hours, so this field must be built from the
	// notice's date *and* time *and* offset, never from a truncated time string.
	//
	// At notice level it is a parse-time value rather than a stored one: it is
	// what lets a notice publishing no notice-level submission period inherit
	// the deadline its lots carry, and the tender's own deadline column is
	// already written from the search payload by a writer that runs hourly.
	// Lot.Deadline, which nothing else supplies, IS persisted.
	Deadline *time.Time

	// Language is the notice language actually selected when reading its
	// repeated per-language text elements, in ISO 639-2/T. It drives every text
	// selection in the parse, and it is reported per criterion (AwardCriterion.
	// Lang) where it is persisted — a stored criterion whose language is unknown
	// cannot be told apart from one whose language was never chosen. The
	// notice-level value is not stored again on the tender row, which already
	// carries a language from the search payload.
	Language string
}

// AllCriteria returns the notice's complete criteria set — notice-level entries
// first, then each lot's in lot order — which is the order it is persisted in.
// Since Criteria and Lot.Criteria partition the set, this is the single
// definition of "every criterion in this notice"; deriving counts and flags
// from it (rather than from either half) is what keeps the stored aggregates
// and the child rows in agreement.
//
// It returns nil when the notice publishes no criteria at all, which is a real
// and frequent outcome, not an error.
func (d NoticeDetail) AllCriteria() []AwardCriterion {
	n := len(d.Criteria)
	for _, l := range d.Lots {
		n += len(l.Criteria)
	}
	if n == 0 {
		return nil
	}

	all := make([]AwardCriterion, 0, n)
	all = append(all, d.Criteria...)
	for _, l := range d.Lots {
		all = append(all, l.Criteria...)
	}
	return all
}

// CriteriaCount is the number of criterion entries the notice publishes, at any
// level. Reported together with WeightedCount — never alone. On its own it
// answers "did the buyer say anything about scoring", which reads as coverage
// but is not: most of the gap between the two numbers is notices that name an
// award method and stop there.
func (d NoticeDetail) CriteriaCount() int {
	n := len(d.Criteria)
	for _, l := range d.Lots {
		n += len(l.Criteria)
	}
	return n
}

// WeightedCount is the number of criterion entries that carry a published
// weight — the entries a scoring grid can actually be built from.
func (d NoticeDetail) WeightedCount() int {
	n := weightedCount(d.Criteria)
	for _, l := range d.Lots {
		n += weightedCount(l.Criteria)
	}
	return n
}

// GridUsable reports whether the notice publishes at least one weighted
// criterion, at notice or lot level. It is the derivation behind
// Tender.GridUsable and must stay the only one: the stored column is verified
// by recomputing it from the persisted child rows and asserting zero
// mismatches, which only holds while both sides come from this rule.
//
// A notice with criteria but no weights is GridUsable false, not true — that
// distinction is the whole reason Weight is a pointer.
func (d NoticeDetail) GridUsable() bool {
	if weightedCount(d.Criteria) > 0 {
		return true
	}
	for _, l := range d.Lots {
		if weightedCount(l.Criteria) > 0 {
			return true
		}
	}
	return false
}

func weightedCount(criteria []AwardCriterion) int {
	n := 0
	for _, c := range criteria {
		if c.HasWeight() {
			n++
		}
	}
	return n
}
