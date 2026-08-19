// Package tender — this file holds the notice-detail types that only a
// machine-readable notice document yields: the award grid a bid is scored
// against, and the parties the notice names.
//
// They are defined here rather than imported from go-services/tender for the
// reason this package already states about ScoredChunk: core defines its own
// minimal shapes so that a domain type never drags a write-path module into
// this service's build. services/backend does not depend on
// go-services/tender today, and these types do not make it start.
package tender

// AwardCriterion is one entry in a tender's scoring grid, as published.
//
// It is modelled as a type rather than folded into free text because a
// criterion's weight has to be comparable across notices — "where are the
// points won" is the question the whole grid exists to answer, and it is not
// answerable from prose.
type AwardCriterion struct {
	// LotRef is "" for a notice-level criterion, which is the single most
	// common shape: a multi-lot notice usually states one grid once and every
	// lot inherits it. It keys on the notice's OWN lot identifier — the same
	// value Lot.Ref holds — because the ingestion table stores lot_ref as
	// plain text with no foreign key, so a criterion can name a lot that was
	// never inserted.
	LotRef string
	// Ordinal is the criterion's position as published, within its LotRef. It
	// is the stable sort key and the only part of a criterion guaranteed to be
	// distinct: entries are frequently untitled and often identical in type,
	// so nothing else orders them reproducibly.
	Ordinal     int
	Type        string // "quality" | "cost" | "price", as declared by the notice
	Name        string
	Description string
	// Weight is the criterion's share of the score, and nil is the COMMON case
	// — 24 of 87 sampled entries carry none, and 32 of 50 sampled notices
	// carry no usable grid at all. Notices routinely publish a lone entry
	// naming the award METHOD ("offerta economicamente più vantaggiosa") with
	// nothing attached to it: present, and useless for scoring.
	//
	// A pointer, never a float64. A zero weight and an absent weight are
	// different facts, and flattening them makes "criteria published" look
	// like "grid recoverable", overstating real coverage by roughly 2x. Never
	// coalesce nil to 0 to simplify a call site.
	Weight *float64
	// WeightRaw is the weight exactly as published ("30", "30%", "30,5"), so a
	// numeric parse failure loses only comparability rather than the datum,
	// and any parse stays auditable after the fact.
	WeightRaw string
	// Lang is the language of Name/Description as the notice declared it, in
	// ISO 639-2/T ("ITA", "ENG"). Multilingual notices repeat every text field
	// once per language, so without it a reader cannot tell a deliberate
	// language choice from a parser accident.
	Lang string
}

// HasWeight reports whether the criterion carries a published weight. Prefer
// it over comparing Weight against nil at a call site, so the
// nil-is-meaningful invariant documented on Weight stays visible in the
// reading code rather than living only in this file.
func (c AwardCriterion) HasWeight() bool { return c.Weight != nil }

// Organization is one party named by the notice — the buyer, the competent
// appeal body, the mediation body.
//
// Ref is notice-internal: a notice references its parties by an id it mints
// for itself, so the same real organization carries a different Ref in every
// notice. It identifies a row within one tender and nothing across tenders.
//
// Email and Phone are frequently institutional addresses (an Italian PEC box)
// but occasionally name an individual. Persisting them is defensible — they
// are published in the EU's own register — but they must never enter a
// PostHog event, a log line, or an agent tool result. Nothing in this service
// puts them on such a payload, and nothing should start.
type Organization struct {
	Ref       string // e.g. "ORG-0001"; unique within the notice only
	Name      string
	CompanyID string   // national registration number — codice fiscale / partita IVA in Italy
	Roles     []string // "buyer" | "appeal-body" | "appeal-information" | "mediation"
	Email     string
	Phone     string
	Website   string
	City      string
	Country   string
}

// CriteriaForLot returns the criteria that apply to the lot identified by ref,
// falling back to the notice-level set when that lot declares none of its own.
//
// The fallback is the real publication pattern rather than a convenience: a
// multi-lot notice usually states one grid once, at notice level, and every
// lot inherits it. A caller that filtered on LotRef alone would report "this
// lot has no published criteria" for the majority of real multi-lot notices —
// the same inaccessible-versus-absent error the coverage model exists to stop,
// expressed one level down.
//
// Criteria are stored flat rather than nested inside Lot for two reasons:
// nesting would need a grouping pass on every read, and it would have no home
// at all for the notice-level entry, which is the most common shape there is.
//
// The result preserves the order of d.Criteria, which the repository returns
// sorted by (lot_ref, ordinal) — so entries come back in published order.
func (d TenderDetail) CriteriaForLot(ref string) []AwardCriterion {
	var own, notice []AwardCriterion
	for _, c := range d.Criteria {
		switch c.LotRef {
		case ref:
			own = append(own, c)
		case "":
			notice = append(notice, c)
		}
	}
	if len(own) > 0 {
		return own
	}
	return notice
}

// SelectionCriterion is one condition a bidder must satisfy to be ADMITTED to a
// procedure, as published by the notice or the source's feed. It is the "may we
// bid" half of what a notice says about qualification, where AwardCriterion is
// the "where are the points won" half.
//
// It carries no weight and no parsed threshold, and neither omission is an
// oversight — see the migration that stores it
// (services/ingestion/migrations/0012_selection_criteria.up.sql) for the long
// version. The short version: selection is pass/fail, so a weight would be
// absent on every row ever written; and no source observed publishes a
// machine-comparable threshold, so a threshold column would be a claim wearing
// the clothes of a measurement.
//
// What it does carry is the requirement as PROSE, which for Spain is
// substantial — "un volumen anual de negocios […] igual o superior a
// 309.552,00 €, equivalente a una vez y media el VEC (artículo 87.3.a) LCSP)".
// The number is in there. It is in there as Spanish, next to a legal basis and
// a reference period, which is exactly why it reaches the eligibility engine as
// an un-modelled requirement a human reads rather than as a comparison a
// machine makes.
type SelectionCriterion struct {
	// LotRef is "" for a notice-level criterion. CODICE publishes the
	// qualification block once per contract folder and never per lot, so today
	// every row is notice-level; the field exists because eForms can scope one
	// to a lot and a reader should not have to change shape when it does.
	LotRef string
	// Ordinal is the position as published within its LotRef — the stable sort
	// key, and the only part guaranteed distinct, since two notices routinely
	// publish the same boilerplate requirement verbatim.
	Ordinal int
	// Category is the requirement family: "technical" | "financial" |
	// "declaration". It is separate from Type because Type alone is ambiguous —
	// PLACSP publishes the bare code "5" under a financial-capability list and
	// the bare code "1" under a declaration list, and without the family both
	// are integers that collide.
	Category string
	// Type is the source's own code within Category, verbatim.
	Type string
	// Name and Description are alternatives, not a pair: a source publishes one
	// or the other. PLACSP carries only prose in Description.
	Name        string
	Description string
	// Origin names the reading that produced this entry ("es-placsp"), so a
	// consumer can grade trust per row. It is deliberately NOT an authority
	// level: what may block a bid is decided by the domain that owns
	// admissibility, not recorded here.
	Origin string
	// Lang is the language of Name/Description in ISO 639-2/T.
	Lang string
}
