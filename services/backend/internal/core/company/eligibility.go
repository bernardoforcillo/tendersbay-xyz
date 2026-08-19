// Package company — this file holds the participation requirements a gara
// publishes, and the pure engine that checks them against the dossier.
//
// One finding shapes everything below, and it must not be designed around:
// **the system holds almost no selection criteria, and none it may decide on**.
// Award criteria (tenders.ingested_tender_award_criteria, surfaced as
// tender.AwardCriterion) say how an ADMITTED bid is scored. Eligibility asks
// whether we may be admitted at all — a different section of an Italian
// disciplinare, and a different element of a notice. Presenting an award grid
// as an eligibility check is a wrong answer, not a partial one.
//
// What has changed since that was written, precisely and no further: Spain's
// PLACSP publishes its qualification block inline in its feed, so
// tenders.ingested_tender_selection_criteria now holds real Spanish
// requirements, surfaced as tender.SelectionCriterion and mapped in by
// RequirementsFromSelectionCriteria. What has NOT changed:
//
//   - The eForms path (cac:TenderingTerms/cac:TendererQualificationRequest,
//     BT-747/748/749) is still unread — and reading it would gain less than it
//     sounds. Sampled Italian notices set selection-criteria-source =
//     epo-sub-espd, a POINTER meaning "the criteria are in the ESPD document",
//     with no name, amount or category attached.
//   - Italy therefore still has nothing, which is the market that matters most.
//   - What Spain gives is PROSE, not comparables. It arrives as
//     RequirementOther under the non-authoritative RequirementNoticePublished
//     source, so it is shown to a human and never decides. The engine still
//     cannot derive a single BLOCKING participation requirement from structured
//     notice data; everything it evaluates was captured by a human or quoted by
//     the agent out of a document.
package company

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// ── Requirements ────────────────────────────────────────────────────────────

// RequirementKind selects which machine-comparable payload a requirement
// carries.
type RequirementKind string

const (
	RequirementSOA           RequirementKind = "soa"
	RequirementCertification RequirementKind = "certification"
	RequirementTurnover      RequirementKind = "turnover"
	RequirementPastContracts RequirementKind = "past_contracts"
	RequirementHeadcount     RequirementKind = "headcount"
	RequirementRegistration  RequirementKind = "registration"
	// RequirementOther is captured VERBATIM and is never machine-decided. Its
	// gap status is always GapUnknown and its remedy is always "read it
	// yourself". That is the point: a requirement the engine cannot model must
	// be visible as un-modelled, not silently absent.
	RequirementOther RequirementKind = "other"
)

var validRequirementKinds = map[RequirementKind]bool{
	RequirementSOA: true, RequirementCertification: true, RequirementTurnover: true,
	RequirementPastContracts: true, RequirementHeadcount: true,
	RequirementRegistration: true, RequirementOther: true,
}

// ValidRequirementKind reports whether k names a modelled requirement kind.
func ValidRequirementKind(k RequirementKind) bool { return validRequirementKinds[k] }

// RequirementSource says where a requirement came from, and it is the ceiling
// on what a verdict resting on it may say.
type RequirementSource string

const (
	RequirementUserStated      RequirementSource = "user_stated"
	RequirementDocumentExcerpt RequirementSource = "document_excerpt"
	// RequirementAwardCriteria is derived from tender.AwardCriterion. It is
	// carried ONLY so the UI can say "this is the award grid, not the
	// requisiti"; it is never a participation requirement and can never produce
	// a verdict.
	RequirementAwardCriteria RequirementSource = "award_criteria"
	RequirementAgentInferred RequirementSource = "agent_inferred"
	// RequirementNoticePublished is a selection criterion the notice or the
	// source's own feed published — today Spain's PLACSP, which embeds its
	// qualification block in the search feed rather than linking a document.
	//
	// It is NOT authoritative, and that is a deliberate choice rather than a
	// gap waiting to be filled. What PLACSP publishes is prose: "un volumen
	// anual de negocios […] igual o superior a 309.552,00 €, equivalente a una
	// vez y media el VEC (artículo 87.3.a) LCSP)". The figure is in there, next
	// to a legal basis, a reference period and a derivation — none of which
	// survive being flattened into a comparison. Until an extraction pass with
	// an eval behind it can recover a threshold AND be checked, this source
	// informs a human and never decides for one.
	//
	// Note it needs no branch in CanBlock: that switch already returns false by
	// default, so a source is non-blocking unless someone deliberately makes it
	// otherwise. The safe direction is the one you get for free.
	RequirementNoticePublished RequirementSource = "notice_published"
)

var validRequirementSources = map[RequirementSource]bool{
	RequirementUserStated: true, RequirementDocumentExcerpt: true,
	RequirementAwardCriteria: true, RequirementAgentInferred: true,
	RequirementNoticePublished: true,
}

// Authoritative reports whether a requirement from this source may, once
// confirmed where confirmation applies, support a blocking verdict.
//
// Note what this does NOT say: a document_excerpt requirement is authoritative
// as a SOURCE, but Requirement.CanBlock additionally demands a human
// confirmation, because a quote nobody checked is a claim about a PDF, not
// about the gara.
func (s RequirementSource) Authoritative() bool {
	return s == RequirementUserStated || s == RequirementDocumentExcerpt
}

// SOARequirement is a demanded attestazione line.
type SOARequirement struct {
	Category   string
	Classifica Classifica
	// Prevalente distinguishes the categoria prevalente (mandatory, and the one
	// that cannot be subcontracted away) from a scorporabile one (which CAN be
	// covered by subcontracting or an RTI member). It changes the REMEDY, not
	// the gap, and getting it wrong turns a solvable gap into a false no_go.
	Prevalente bool
}

// CertificationRequirement is a demanded certificate.
type CertificationRequirement struct {
	Standard    CertificationStandard
	StandardRaw string
	Scope       string // "" = any scope accepted
}

// AmountRequirement is a money threshold over a window of esercizi.
type AmountRequirement struct {
	MinMinor int64
	Currency string
	// Years is how many esercizi the threshold applies over (3 for the usual
	// "ultimi tre esercizi"); 0 means a single-contract threshold.
	Years int32
	// Specific is true for "fatturato specifico nel settore"; false for globale.
	Specific bool
	CPV      string // set when the requirement scopes references by sector
}

// CountRequirement is a plain "at least N" threshold — "almeno 2 servizi
// analoghi", "organico minimo 15".
type CountRequirement struct {
	Min int32
}

// RegistrationRequirement is a demanded iscrizione.
type RegistrationRequirement struct {
	Kind    RegistrationKind
	Section string
}

// Requirement is one participation requirement for one tender, scoped to the
// workspace that captured it.
//
// It is workspace-scoped and NOT corpus-shared, deliberately. One workspace's
// mis-extraction must not decide another workspace's go/no-go, and a
// cross-tenant write path into shared tender data is a much larger security
// surface than a duplicated extraction is a cost. A corpus-wide requirement
// store is a later change and needs the eval harness the PRD demands (mirroring
// core/tender/eval/) before it is defensible.
type Requirement struct {
	ID          string
	WorkspaceID string
	TenderID    int64
	// LotRef is "" for a notice-level requirement, keyed on the notice's OWN lot
	// identifier — same convention and same reason as tender.AwardCriterion.LotRef.
	LotRef string
	Kind   RequirementKind

	SOA           *SOARequirement
	Certification *CertificationRequirement
	Amount        *AmountRequirement
	Count         *CountRequirement
	Registration  *RegistrationRequirement

	// Text is the requirement as published, ALWAYS set. It is what the UI shows
	// and what a human checks the parse against.
	Text string
	// Blocking distinguishes a requisito di partecipazione from a criterio
	// premiante. Defaults to TRUE on capture: treating an unclassified
	// requirement as blocking produces a question; treating it as non-blocking
	// produces a silent pass.
	Blocking bool

	Source RequirementSource
	// Citation and Excerpt are the verification affordance — a claim the user
	// can check without leaving the product. Nil/empty for user_stated. Reuses
	// core/document's type; core→core, no adapter edge.
	Citation *document.Citation
	Excerpt  string

	// ConfirmedBy is the user who checked the extraction against the quoted
	// passage. "" = unconfirmed, which caps what the verdict may be.
	ConfirmedBy string
	ConfirmedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Confirmed reports whether a human has checked this requirement against its
// quoted passage.
func (r Requirement) Confirmed() bool { return r.ConfirmedBy != "" }

// CanBlock reports whether a gap against this requirement may contribute to a
// no_go. This is the ConfirmedBy gate, and it is the stand-in for the
// extraction eval the PRD demands and this phase does not have: an agent-quoted
// requirement nobody checked can raise a QUESTION, never a verdict.
func (r Requirement) CanBlock() bool {
	switch r.Source {
	case RequirementUserStated:
		return true
	case RequirementDocumentExcerpt:
		return r.Confirmed()
	default:
		return false
	}
}

// DedupeKey is the stable identity of a requirement within (workspace, tender,
// lot): kind plus a normalised form of the payload (or of Text for "other"). It
// backs the unique constraint so an agent re-running extraction on every turn
// updates rows instead of multiplying them.
//
// The SOA key deliberately omits the classifica: a re-extraction that reads a
// different classifica for the same category is a CORRECTION of one requirement,
// not the discovery of a second one, and a key that included it would leave both
// readings in the table with no way to tell which is current.
func (r Requirement) DedupeKey() string {
	switch r.Kind {
	case RequirementSOA:
		if r.SOA != nil {
			cat, ok := NormalizeSOACategory(r.SOA.Category)
			if !ok {
				cat = strings.ToUpper(strings.TrimSpace(r.SOA.Category))
			}
			return "soa:" + cat
		}
	case RequirementCertification:
		if r.Certification != nil {
			std := string(r.Certification.Standard)
			if r.Certification.Standard == CertOtherStd {
				std = "other/" + foldSpace(r.Certification.StandardRaw)
			}
			return "certification:" + std + ":" + foldSpace(r.Certification.Scope)
		}
	case RequirementTurnover:
		if r.Amount != nil {
			return fmt.Sprintf("turnover:specific=%t:years=%d:cpv=%s",
				r.Amount.Specific, r.Amount.Years, strings.TrimSpace(r.Amount.CPV))
		}
	case RequirementPastContracts:
		if r.Amount != nil {
			return fmt.Sprintf("past_contracts:years=%d:cpv=%s", r.Amount.Years, strings.TrimSpace(r.Amount.CPV))
		}
		if r.Count != nil {
			return "past_contracts:count"
		}
	case RequirementHeadcount:
		if r.Amount != nil {
			return fmt.Sprintf("headcount:years=%d", r.Amount.Years)
		}
		if r.Count != nil {
			return "headcount"
		}
	case RequirementRegistration:
		if r.Registration != nil {
			return "registration:" + string(r.Registration.Kind) + ":" + foldSpace(r.Registration.Section)
		}
	}
	// RequirementOther, and any requirement whose payload the caller omitted,
	// fall back to the published text. It is hashed because the key backs a
	// btree unique index and requirement text routinely runs long.
	return string(r.Kind) + ":text:" + shortHash(foldSpace(r.Text))
}

const maxRequirementTextLen = 4000

// Validate checks a requirement is storable and evaluable. Exported because the
// transport layer accepts requirements from outside and must reject a malformed
// one before it reaches the repository, where the dedupe key would be computed
// from a payload nothing can interpret.
func (r Requirement) Validate() error {
	if !validRequirementKinds[r.Kind] {
		return fmt.Errorf("%w: unknown requirement kind %q", ErrInvalidArgument, r.Kind)
	}
	if !validRequirementSources[r.Source] {
		return fmt.Errorf("%w: unknown requirement source %q", ErrInvalidArgument, r.Source)
	}
	if strings.TrimSpace(r.Text) == "" {
		return fmt.Errorf("%w: requirement text is always required — it is what a human checks the parse against", ErrInvalidArgument)
	}
	if len(r.Text) > maxRequirementTextLen {
		return fmt.Errorf("%w: requirement text must be %d characters or fewer", ErrInvalidArgument, maxRequirementTextLen)
	}
	switch r.Kind {
	case RequirementSOA:
		if r.SOA == nil {
			return fmt.Errorf("%w: kind soa requires an soa payload", ErrInvalidArgument)
		}
		if _, ok := NormalizeSOACategory(r.SOA.Category); !ok {
			return ErrInvalidSOACategory
		}
		if !r.SOA.Classifica.Valid() {
			return ErrInvalidClassifica
		}
	case RequirementCertification:
		if r.Certification == nil {
			return fmt.Errorf("%w: kind certification requires a certification payload", ErrInvalidArgument)
		}
		if !validCertificationStandards[r.Certification.Standard] {
			return fmt.Errorf("%w: unknown certification standard %q", ErrInvalidArgument, r.Certification.Standard)
		}
	case RequirementTurnover:
		if r.Amount == nil {
			return fmt.Errorf("%w: kind turnover requires an amount payload", ErrInvalidArgument)
		}
		if r.Amount.MinMinor < 0 || r.Amount.Years < 0 {
			return fmt.Errorf("%w: turnover threshold and window must not be negative", ErrInvalidArgument)
		}
	case RequirementPastContracts:
		if r.Count == nil && r.Amount == nil {
			return fmt.Errorf("%w: kind past_contracts requires a count or an amount payload", ErrInvalidArgument)
		}
	case RequirementHeadcount:
		if r.Count == nil {
			return fmt.Errorf("%w: kind headcount requires a count payload", ErrInvalidArgument)
		}
	case RequirementRegistration:
		if r.Registration == nil {
			return fmt.Errorf("%w: kind registration requires a registration payload", ErrInvalidArgument)
		}
		if !validRegistrationKinds[r.Registration.Kind] {
			return fmt.Errorf("%w: unknown registration kind %q", ErrInvalidArgument, r.Registration.Kind)
		}
	case RequirementOther:
		// Payload-less by construction — nothing more to check.
	}
	if r.Count != nil && r.Count.Min < 0 {
		return fmt.Errorf("%w: count threshold must not be negative", ErrInvalidArgument)
	}
	return nil
}

// ── Gaps ────────────────────────────────────────────────────────────────────

// GapStatus is one requirement's answer against the dossier.
type GapStatus string

const (
	GapMet       GapStatus = "met"
	GapMissing   GapStatus = "missing"   // dossier holds facts of this kind, none matching
	GapShortfall GapStatus = "shortfall" // held, but below the threshold
	GapExpiring  GapStatus = "expiring"  // valid today, lapses before the deadline
	GapExpired   GapStatus = "expired"   // held, already lapsed
	// GapUnknown — the dossier has no fact either way. This is NOT "missing":
	// missing means we asked and the answer was no. Unknown means we never
	// asked, and it is the just-in-time capture trigger.
	GapUnknown GapStatus = "unknown"
)

// RemedyKind is what would close a gap.
type RemedyKind string

const (
	RemedyNone        RemedyKind = ""
	RemedyProvideFact RemedyKind = "provide_fact" // answer the question
	RemedyRenew       RemedyKind = "renew"        // it lapsed / lapses in time
	RemedyUpgrade     RemedyKind = "upgrade"      // higher classifica, more turnover
	// RemedyAvvalimento — borrow another operator's capacity (art. 104
	// D.Lgs 36/2023). For SOA and turnover gaps this is the normal Italian
	// answer, which is why a blocking gap is NOT a no_go by itself.
	RemedyAvvalimento RemedyKind = "avvalimento"
	RemedyRTI         RemedyKind = "rti"
	RemedySubcontract RemedyKind = "subcontract" // only for a scorporabile SOA category
)

// Remedy is the machine-readable size and shape of what would close a gap, so
// the UI can render "serve OG1 III, hai OG1 I — due classifiche" without
// re-deriving it.
type Remedy struct {
	Kind               RemedyKind
	MissingClassifica  *Classifica
	MissingAmountMinor *int64
	MissingCount       *int32
	DeadlineRelevant   bool // true when the remedy must land before the tender deadline
}

// Gap is one requirement checked against the dossier. Held is what the dossier
// actually says, rendered so the UI can lead with the FACT — "richiede SOA OG1
// III; il profilo indica OG1 I" — rather than with a verdict. A bare "no" is
// what makes a loss-framed recommendation get discounted.
type Gap struct {
	Requirement Requirement
	Status      GapStatus
	Blocking    bool // Requirement.Blocking && Status in {missing, shortfall, expired}
	Held        string
	Remedy      Remedy
	// Question is set exactly when Status == GapUnknown: the closed-ended
	// prompt the agent should ask, pre-shaped so record_company_fact can write
	// the answer without a second round trip.
	Question *CaptureQuestion

	// factAuthoritative records whether every dossier record this status rests
	// on was asserted by a human. It is unexported because it is evidence for
	// the verdict rule and not a display field: a consumer that rendered it
	// would be showing the reader a second, weaker version of Provenance that
	// is already on every record inside the dossier.
	factAuthoritative bool
}

// CaptureQuestion is a just-in-time capture prompt. It carries the FieldKey /
// record kind the answer writes to, so the question and its destination cannot
// drift apart.
type CaptureQuestion struct {
	Kind        RequirementKind
	Field       string // the Fact field the answer writes to: "soa", "certification", ...
	Prompt      string
	Options     []string // closed options where the answer space is closed
	AllowCustom bool
}

// Verdict is the eligibility answer.
type Verdict string

const (
	// VerdictInsufficientData is the DEFAULT and the most common real answer.
	VerdictInsufficientData Verdict = "insufficient_data"
	VerdictGo               Verdict = "go"
	VerdictNoGo             Verdict = "no_go"
)

// Assessment is one tender × dossier evaluation. It is computed FRESH on every
// read and never persisted — the same choice bid.BidView already makes for fit.
// Persisting it would create a fourth thing that can drift from the dossier, the
// requirements and the decision.
type Assessment struct {
	TenderID int64
	LotRef   string
	// Gaps is sorted by the DOMAIN: blocking first, then unknown, then
	// expiring, then the remaining unmet, then met. Ordering is enforced here
	// so no consumer can accidentally lead with a verdict — the proto message
	// and the agent tool's JSON both put gaps before verdict for the same reason.
	Gaps    []Gap
	Verdict Verdict

	// The evidence the verdict rests on, carried so Consistent() is checkable
	// rather than merely intended — the same argument document.Availability
	// makes for its evidence fields.
	RequirementCount          int
	AuthoritativeRequirements int // user_stated + CONFIRMED document_excerpt
	BlockingGapCount          int
	UnknownGapCount           int
	// DocumentCoverage/Reason are copied verbatim from document.Availability so
	// the UI can say "we hold only the notice PDF, so we have not seen the
	// requisiti at all" as one sentence with two facts in it.
	DocumentCoverage document.Coverage
	DocumentReason   document.Reason
	EvaluatedAt      time.Time
}

// AwardGridOnly reports whether every captured requirement came from the award
// grid — the case where the product holds scoring data and nothing about
// admission. It is a distinct sentence from "we captured nothing", and the
// agent's steering says so.
func (a Assessment) AwardGridOnly() bool {
	if len(a.Gaps) == 0 {
		return false
	}
	for _, g := range a.Gaps {
		if g.Requirement.Source != RequirementAwardCriteria {
			return false
		}
	}
	return true
}

// HasUnconfirmedExtraction reports whether any gap rests on a document quote no
// human has checked.
func (a Assessment) HasUnconfirmedExtraction() bool {
	for _, g := range a.Gaps {
		if g.Requirement.Source == RequirementDocumentExcerpt && !g.Requirement.Confirmed() {
			return true
		}
	}
	return false
}

// Consistent encodes the invariant this package exists to protect: a verdict is
// never stronger than the evidence under it.
//
//   - VerdictNoGo requires at least one BLOCKING gap resting on an AUTHORITATIVE
//     requirement and an AUTHORITATIVE dossier fact.
//   - VerdictGo requires at least one authoritative requirement, zero unknown
//     blocking gaps, and zero blocking gaps of ANY provenance. "We checked
//     nothing and found nothing wrong" is not a go, and neither is "we found
//     something wrong but cannot stand behind it".
//   - Everything else is VerdictInsufficientData.
//
// The blocking-gap test on the VerdictGo arm is deliberately WEAKER than the one
// on the VerdictNoGo arm — bare g.Blocking, not the three-conjunct form. Using
// the same test on both arms is what let a blocking gap that fails either
// conjunct satisfy the invariant while being rendered on screen, so the panel
// could read "this gara requires SOA OG1 III" and "you appear eligible" in
// consecutive lines and still pass. Asserting a go is a positive claim; it
// requires the absence of contrary evidence, not merely the absence of
// *authoritative* contrary evidence.
//
// Mirrors document.Availability.Consistent, and is asserted directly by a table
// test rather than merely intended.
func (a Assessment) Consistent() bool {
	switch a.Verdict {
	case VerdictNoGo:
		for _, g := range a.Gaps {
			if g.Blocking && g.Requirement.CanBlock() && g.factAuthoritative {
				return true
			}
		}
		return false
	case VerdictGo:
		if a.AuthoritativeRequirements == 0 {
			return false
		}
		for _, g := range a.Gaps {
			if g.Status == GapUnknown && g.Requirement.Blocking {
				return false
			}
			if g.Blocking {
				return false
			}
		}
		return true
	case VerdictInsufficientData:
		return true
	default:
		return false
	}
}

// ── The engine ──────────────────────────────────────────────────────────────

// Evaluate is a PURE function: no I/O, no clock read (now is passed), no
// randomness. That is what makes it testable against a fixture table and what
// keeps the legal reasoning reviewable in one file.
//
// deadline is the tender's submission deadline and drives GapExpiring: a
// certificate valid today but lapsing before the deadline is a warning, not a
// gap, and reporting it as either "fine" or "missing" is wrong.
func Evaluate(reqs []Requirement, d Dossier, deadline *time.Time, now time.Time) Assessment {
	a := Assessment{
		EvaluatedAt:      now,
		RequirementCount: len(reqs),
		Verdict:          VerdictInsufficientData,
	}

	gaps := make([]Gap, 0, len(reqs))
	for _, r := range reqs {
		if r.CanBlock() {
			a.AuthoritativeRequirements++
		}
		g := evaluateOne(r, d, deadline, now)
		g.Blocking = r.Blocking && (g.Status == GapMissing || g.Status == GapShortfall || g.Status == GapExpired)
		if g.Blocking {
			a.BlockingGapCount++
		}
		if g.Status == GapUnknown {
			a.UnknownGapCount++
		}
		gaps = append(gaps, g)
	}
	sortGaps(gaps)
	a.Gaps = gaps
	a.Verdict = verdict(a)
	return a
}

// gapRank is the domain's reading order. Blocking gaps lead because they are
// what a decision turns on; unknown comes next because it is an ACTION (ask the
// question) rather than an outcome; met is last because it is the least
// informative line on the panel.
func gapRank(g Gap) int {
	switch {
	case g.Blocking:
		return 0
	case g.Status == GapUnknown:
		return 1
	case g.Status == GapExpiring:
		return 2
	case g.Status != GapMet:
		return 3
	default:
		return 4
	}
}

// sortGaps orders by rank, then by requirement text, so two runs over the same
// inputs produce byte-identical output. sort.SliceStable alone would preserve
// the repository's ordering, which is not a domain fact.
func sortGaps(gaps []Gap) {
	sort.SliceStable(gaps, func(i, j int) bool {
		ri, rj := gapRank(gaps[i]), gapRank(gaps[j])
		if ri != rj {
			return ri < rj
		}
		return gaps[i].Requirement.Text < gaps[j].Requirement.Text
	})
}

// verdict applies the five ordered rules. Rule 3's second conjunct — the
// contradicting DOSSIER fact must also be authoritative — is the asymmetry that
// matters: an agent_inferred dossier fact can never CAUSE a no_go, it can only
// pre-fill the question. Rule 4 is what makes just-in-time capture a first-class
// outcome rather than a fallback: the product's answer to an unmodelled gara is
// a question, and that is a better product than a guess. Rule 5 is the other
// half of rule 3's asymmetry, and it is the one that is easy to leave out: not
// reaching no_go is not the same as reaching go.
func verdict(a Assessment) Verdict {
	// 1. Nothing captured — dossier completeness is irrelevant, we do not know
	//    what this gara asks.
	if a.RequirementCount == 0 {
		return VerdictInsufficientData
	}
	// 2. Everything captured is award-grid or unconfirmed extraction.
	if a.AuthoritativeRequirements == 0 {
		return VerdictInsufficientData
	}
	// 3. A blocking gap that rests on authoritative evidence on BOTH sides.
	for _, g := range a.Gaps {
		if g.Blocking && g.Requirement.CanBlock() && g.factAuthoritative {
			return VerdictNoGo
		}
	}
	// 4. Ask, don't guess.
	for _, g := range a.Gaps {
		if g.Status == GapUnknown && g.Requirement.Blocking {
			return VerdictInsufficientData
		}
	}
	// 5. A blocking gap we cannot stand behind. Every gap still marked Blocking
	//    here failed one of rule 3's conjuncts — the requirement is an
	//    unconfirmed extraction, or the dossier fact contradicting it is
	//    agent-inferred rather than user-stated. Rule 3 correctly refuses to
	//    turn that into a no_go. Falling through to VerdictGo, however, turns it
	//    into the OPPOSITE claim: the gap is still rendered ("this gara requires
	//    SOA OG1 III") and the verdict beside it reads "you appear eligible".
	//    A gap we are not sure enough about to reject on is exactly a gap we are
	//    not sure enough about to clear on, and the honest answer to it is the
	//    same one rule 4 gives to an unknown — ask.
	for _, g := range a.Gaps {
		if g.Blocking {
			return VerdictInsufficientData
		}
	}
	return VerdictGo
}

// evaluateOne dispatches one requirement to its per-kind matcher. Every matcher
// returns a Gap with Status, Held, Remedy, factAuthoritative and (for
// GapUnknown) Question set; Blocking is derived by the caller so the rule lives
// in exactly one place.
func evaluateOne(r Requirement, d Dossier, deadline *time.Time, now time.Time) Gap {
	switch r.Kind {
	case RequirementSOA:
		return evaluateSOA(r, d, deadline, now)
	case RequirementCertification:
		return evaluateCertification(r, d, deadline, now)
	case RequirementTurnover:
		return evaluateTurnover(r, d)
	case RequirementPastContracts:
		return evaluatePastContracts(r, d, now)
	case RequirementHeadcount:
		return evaluateHeadcount(r, d)
	case RequirementRegistration:
		return evaluateRegistration(r, d, deadline, now)
	default:
		return unknownGap(r, CaptureQuestion{
			Kind:        RequirementOther,
			Field:       "other",
			Prompt:      "Questo requisito non è interpretabile automaticamente. Leggilo e dicci se lo soddisfi: " + r.Text,
			AllowCustom: true,
		})
	}
}

// unknownGap is the single constructor for "we never asked". Centralised so
// that the remedy and the blocking derivation cannot drift between kinds: an
// unknown is always RemedyProvideFact and always carries a question.
func unknownGap(r Requirement, q CaptureQuestion) Gap {
	return Gap{
		Requirement: r,
		Status:      GapUnknown,
		Remedy:      Remedy{Kind: RemedyProvideFact},
		Question:    &q,
	}
}

// ── SOA ─────────────────────────────────────────────────────────────────────

func evaluateSOA(r Requirement, d Dossier, deadline *time.Time, now time.Time) Gap {
	want := r.SOA
	if want == nil {
		return unknownGap(r, soaQuestion(r, ""))
	}
	category, ok := NormalizeSOACategory(want.Category)
	if !ok {
		category = strings.ToUpper(strings.TrimSpace(want.Category))
	}

	// No SOA rows at all is IGNORANCE, not absence — the exact
	// "inaccessible ≠ absent" error core/document guards one level down.
	if len(d.SOA) == 0 {
		return unknownGap(r, soaQuestion(r, category))
	}

	var (
		held      []SOACategory
		allAuthor = true
	)
	for _, c := range d.SOA {
		if norm, ok := NormalizeSOACategory(c.Category); ok && norm == category {
			held = append(held, c)
		}
		if !c.Authoritative() {
			allAuthor = false
		}
	}

	if len(held) == 0 {
		// We asked, and the company holds attestations — just not this one. The
		// claim rests on the dossier's SOA set as a whole, so it is only
		// authoritative when every line in it is.
		return Gap{
			Requirement:       r,
			Status:            GapMissing,
			Held:              renderSOAHeld(d.SOA),
			Remedy:            Remedy{Kind: RemedyRTI, DeadlineRelevant: true},
			factAuthoritative: allAuthor,
		}
	}

	best := bestSOA(held, now)
	gap := Gap{Requirement: r, Held: renderSOALine(best), factAuthoritative: best.Authoritative()}

	switch {
	case !best.Classifica.AtLeast(want.Classifica):
		missing := want.Classifica
		gap.Status = GapShortfall
		gap.Remedy = Remedy{
			Kind:              soaShortfallRemedy(want),
			MissingClassifica: &missing,
			DeadlineRelevant:  true,
		}
	case best.LapsedAt(now):
		gap.Status = GapExpired
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	case deadline != nil && best.LapsedAt(*deadline):
		gap.Status = GapExpiring
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	default:
		gap.Status = GapMet
	}
	return gap
}

// soaShortfallRemedy encodes the prevalente/scorporabile distinction: a
// scorporabile category may be covered by subappalto, a prevalente one may not
// — the difference between a solvable gap and a false no_go.
func soaShortfallRemedy(want *SOARequirement) RemedyKind {
	if !want.Prevalente {
		return RemedySubcontract
	}
	return RemedyAvvalimento
}

// bestSOA picks the line the verdict speaks about: the highest classifica among
// the still-valid ones, falling back to the highest overall when every line has
// lapsed. Choosing a lapsed higher line over a valid lower one would report an
// expiry the company can actually work around.
func bestSOA(held []SOACategory, now time.Time) SOACategory {
	var (
		best      SOACategory
		bestValid bool
	)
	for i, c := range held {
		valid := !c.LapsedAt(now)
		switch {
		case i == 0:
			best, bestValid = c, valid
		case valid && !bestValid:
			best, bestValid = c, valid
		case valid == bestValid && c.Classifica > best.Classifica:
			best = c
		}
	}
	return best
}

func renderSOALine(c SOACategory) string {
	cat := c.Category
	if norm, ok := NormalizeSOACategory(cat); ok {
		cat = norm
	}
	return cat + " " + c.Classifica.String()
}

func renderSOAHeld(all []SOACategory) string {
	parts := make([]string, 0, len(all))
	for _, c := range all {
		parts = append(parts, renderSOALine(c))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func soaQuestion(r Requirement, category string) CaptureQuestion {
	prompt := "Questa gara richiede un'attestazione SOA. La possiedi?"
	if r.SOA != nil && category != "" {
		prompt = fmt.Sprintf("Questa gara richiede SOA %s classifica %s. La possiedi?",
			category, r.SOA.Classifica.String())
	}
	return CaptureQuestion{
		Kind:        RequirementSOA,
		Field:       "soa",
		Prompt:      prompt,
		Options:     []string{"Sì", "No", "Non lo so"},
		AllowCustom: true,
	}
}

// ── Certifications ──────────────────────────────────────────────────────────

func evaluateCertification(r Requirement, d Dossier, deadline *time.Time, now time.Time) Gap {
	want := r.Certification
	if want == nil {
		return unknownGap(r, certificationQuestion(r, ""))
	}
	label := certificationLabel(want)

	if len(d.Certifications) == 0 {
		return unknownGap(r, certificationQuestion(r, label))
	}

	var (
		matches   []Certification
		allAuthor = true
	)
	for _, c := range d.Certifications {
		if !c.Authoritative() {
			allAuthor = false
		}
		if !certificationMatches(c, want) {
			continue
		}
		matches = append(matches, c)
	}

	if len(matches) == 0 {
		return Gap{
			Requirement:       r,
			Status:            GapMissing,
			Held:              renderCertificationsHeld(d.Certifications),
			Remedy:            Remedy{Kind: RemedyRTI, DeadlineRelevant: true},
			factAuthoritative: allAuthor,
		}
	}

	best := bestCertification(matches, now)
	gap := Gap{Requirement: r, Held: renderCertificationLine(best), factAuthoritative: best.Authoritative()}
	switch {
	case best.LapsedAt(now):
		gap.Status = GapExpired
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	case deadline != nil && best.LapsedAt(*deadline):
		gap.Status = GapExpiring
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	default:
		gap.Status = GapMet
	}
	return gap
}

// certificationMatches compares the standard, and the scope ONLY when the
// requirement names one. A certificate is scope-bounded, so ignoring a named
// scope would report an ISO 9001 for catering as covering an ISO 9001 for
// construction works.
func certificationMatches(c Certification, want *CertificationRequirement) bool {
	if c.Standard != want.Standard {
		return false
	}
	if want.Standard == CertOtherStd {
		wantRaw := foldSpace(want.StandardRaw)
		if wantRaw != "" && !strings.Contains(foldSpace(c.StandardRaw), wantRaw) {
			return false
		}
	}
	if scope := foldSpace(want.Scope); scope != "" {
		return strings.Contains(foldSpace(c.Scope), scope)
	}
	return true
}

// bestCertification prefers a still-valid certificate, then the one that lasts
// longest — the line a renewal reminder should speak about.
func bestCertification(matches []Certification, now time.Time) Certification {
	best := matches[0]
	for _, c := range matches[1:] {
		bestValid, valid := !best.LapsedAt(now), !c.LapsedAt(now)
		if valid && !bestValid {
			best = c
			continue
		}
		if valid == bestValid && laterUntil(c.ValidUntil, best.ValidUntil) {
			best = c
		}
	}
	return best
}

// laterUntil reports whether a expires after b, treating nil ("no stated
// expiry") as the latest possible date.
func laterUntil(a, b *time.Time) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}
	return a.After(*b)
}

func certificationLabel(want *CertificationRequirement) string {
	if want.Standard == CertOtherStd && strings.TrimSpace(want.StandardRaw) != "" {
		return strings.TrimSpace(want.StandardRaw)
	}
	return string(want.Standard)
}

func renderCertificationLine(c Certification) string {
	name := string(c.Standard)
	if c.Standard == CertOtherStd && strings.TrimSpace(c.StandardRaw) != "" {
		name = strings.TrimSpace(c.StandardRaw)
	}
	if s := strings.TrimSpace(c.Scope); s != "" {
		name += " (" + s + ")"
	}
	if c.ValidUntil != nil {
		name += " — valida fino al " + c.ValidUntil.Format("2006-01-02")
	}
	return name
}

func renderCertificationsHeld(all []Certification) string {
	parts := make([]string, 0, len(all))
	for _, c := range all {
		name := string(c.Standard)
		if c.Standard == CertOtherStd && strings.TrimSpace(c.StandardRaw) != "" {
			name = strings.TrimSpace(c.StandardRaw)
		}
		parts = append(parts, name)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func certificationQuestion(r Requirement, label string) CaptureQuestion {
	prompt := "Questa gara richiede una certificazione. La possiedi?"
	if label != "" {
		prompt = fmt.Sprintf("Questa gara richiede la certificazione %s. La possiedi?", label)
	}
	return CaptureQuestion{
		Kind:        RequirementCertification,
		Field:       "certification",
		Prompt:      prompt,
		Options:     []string{"Sì", "No", "Non lo so"},
		AllowCustom: true,
	}
}

// ── Turnover ────────────────────────────────────────────────────────────────

func evaluateTurnover(r Requirement, d Dossier) Gap {
	want := r.Amount
	if want == nil || len(d.FinancialYears) == 0 {
		return unknownGap(r, turnoverQuestion(r))
	}

	years := int(want.Years)
	if years < 1 {
		years = 1
	}

	// The window is the `years` esercizi ending at the newest one on file, and
	// it must be COMPLETE. A partial sum reported as the total is the quiet way
	// an SME gets told it fails a threshold it actually meets.
	byYear := make(map[int32]FinancialYear, len(d.FinancialYears))
	newest := d.FinancialYears[0].Year
	for _, f := range d.FinancialYears {
		byYear[f.Year] = f
		if f.Year > newest {
			newest = f.Year
		}
	}

	var (
		sum       int64
		allAuthor = true
	)
	for i := range years {
		f, ok := byYear[newest-int32(i)]
		if !ok {
			return unknownGap(r, turnoverQuestion(r))
		}
		v := f.TurnoverMinor
		if want.Specific {
			v = f.SpecificTurnoverMinor
		}
		if v == nil {
			return unknownGap(r, turnoverQuestion(r))
		}
		// Currencies are not converted here. A cross-currency comparison needs a
		// rate and a date, neither of which the dossier holds, and inventing one
		// would put a fabricated number under a legal verdict.
		if want.Currency != "" && f.Currency != "" && !strings.EqualFold(want.Currency, f.Currency) {
			return unknownGap(r, turnoverQuestion(r))
		}
		if !f.Authoritative() {
			allAuthor = false
		}
		sum += *v
	}

	gap := Gap{
		Requirement:       r,
		Held:              renderAmount(sum, currencyOr(want.Currency, d.FinancialYears[0].Currency)) + fmt.Sprintf(" su %d esercizi", years),
		factAuthoritative: allAuthor,
	}
	if sum >= want.MinMinor {
		gap.Status = GapMet
		return gap
	}
	missing := want.MinMinor - sum
	gap.Status = GapShortfall
	gap.Remedy = Remedy{Kind: RemedyAvvalimento, MissingAmountMinor: &missing, DeadlineRelevant: true}
	return gap
}

func turnoverQuestion(r Requirement) CaptureQuestion {
	prompt := "Questa gara richiede un fatturato minimo. Qual è stato il tuo fatturato negli ultimi esercizi?"
	if r.Amount != nil {
		kindLabel := "globale"
		if r.Amount.Specific {
			kindLabel = "specifico nel settore"
		}
		years := int(r.Amount.Years)
		if years < 1 {
			years = 1
		}
		prompt = fmt.Sprintf("Questa gara richiede un fatturato %s di almeno %s sugli ultimi %d esercizi. Qual è il tuo?",
			kindLabel, renderAmount(r.Amount.MinMinor, r.Amount.Currency), years)
	}
	return CaptureQuestion{
		Kind:        RequirementTurnover,
		Field:       "turnover",
		Prompt:      prompt,
		AllowCustom: true,
	}
}

// ── Past contracts ──────────────────────────────────────────────────────────

func evaluatePastContracts(r Requirement, d Dossier, now time.Time) Gap {
	if len(d.PastContracts) == 0 {
		return unknownGap(r, pastContractsQuestion(r))
	}

	min := int32(1)
	if r.Count != nil && r.Count.Min > 0 {
		min = r.Count.Min
	}

	var (
		cutoff    *time.Time
		wantCPV   string
		wantValue int64
	)
	if r.Amount != nil {
		wantCPV = strings.TrimSpace(r.Amount.CPV)
		wantValue = r.Amount.MinMinor
		if r.Amount.Years > 0 {
			c := now.AddDate(-int(r.Amount.Years), 0, 0)
			cutoff = &c
		}
	}

	var (
		count     int32
		allAuthor = true
	)
	for _, p := range d.PastContracts {
		if !p.Authoritative() {
			allAuthor = false
		}
		if wantCPV != "" && !strings.HasPrefix(strings.TrimSpace(p.CPV), wantCPV) {
			continue
		}
		if cutoff != nil {
			// A contract with no end date cannot be placed in the window. It is
			// excluded rather than assumed recent — the same direction of
			// caution SharePct takes.
			if p.EndedOn == nil || p.EndedOn.Before(*cutoff) {
				continue
			}
		}
		if wantValue > 0 {
			eff := p.EffectiveValueMinor()
			if eff == nil || *eff < wantValue {
				continue
			}
		}
		count++
	}

	gap := Gap{
		Requirement:       r,
		Held:              fmt.Sprintf("%d referenze conformi su %d in profilo", count, len(d.PastContracts)),
		factAuthoritative: allAuthor,
	}
	if count >= min {
		gap.Status = GapMet
		return gap
	}
	missing := min - count
	gap.Status = GapShortfall
	gap.Remedy = Remedy{Kind: RemedyRTI, MissingCount: &missing, DeadlineRelevant: true}
	return gap
}

func pastContractsQuestion(r Requirement) CaptureQuestion {
	prompt := "Questa gara richiede referenze di lavori o servizi analoghi. Ne hai?"
	if r.Count != nil && r.Count.Min > 0 {
		prompt = fmt.Sprintf("Questa gara richiede almeno %d referenze analoghe. Ne hai?", r.Count.Min)
	}
	return CaptureQuestion{
		Kind:        RequirementPastContracts,
		Field:       "past_contract",
		Prompt:      prompt,
		AllowCustom: true,
	}
}

// ── Headcount ───────────────────────────────────────────────────────────────

func evaluateHeadcount(r Requirement, d Dossier) Gap {
	if r.Count == nil || len(d.FinancialYears) == 0 {
		return unknownGap(r, headcountQuestion(r))
	}

	years := 1
	if r.Amount != nil && r.Amount.Years > 1 {
		years = int(r.Amount.Years)
	}

	byYear := make(map[int32]FinancialYear, len(d.FinancialYears))
	newest := d.FinancialYears[0].Year
	for _, f := range d.FinancialYears {
		byYear[f.Year] = f
		if f.Year > newest {
			newest = f.Year
		}
	}

	var (
		total     int32
		allAuthor = true
	)
	for i := range years {
		f, ok := byYear[newest-int32(i)]
		if !ok || f.Headcount == nil {
			return unknownGap(r, headcountQuestion(r))
		}
		if !f.Authoritative() {
			allAuthor = false
		}
		total += *f.Headcount
	}
	// "Organico medio annuo del triennio" is a mean, not a sum — summing three
	// years of the same 12 people would answer 36.
	mean := total / int32(years)

	gap := Gap{
		Requirement:       r,
		Held:              fmt.Sprintf("organico medio %d su %d esercizi", mean, years),
		factAuthoritative: allAuthor,
	}
	if mean >= r.Count.Min {
		gap.Status = GapMet
		return gap
	}
	missing := r.Count.Min - mean
	gap.Status = GapShortfall
	gap.Remedy = Remedy{Kind: RemedyRTI, MissingCount: &missing, DeadlineRelevant: true}
	return gap
}

func headcountQuestion(r Requirement) CaptureQuestion {
	prompt := "Questa gara richiede un organico minimo. Quante persone impiega l'azienda?"
	if r.Count != nil && r.Count.Min > 0 {
		prompt = fmt.Sprintf("Questa gara richiede un organico di almeno %d persone. Qual è il tuo organico medio annuo?", r.Count.Min)
	}
	return CaptureQuestion{
		Kind:        RequirementHeadcount,
		Field:       "headcount",
		Prompt:      prompt,
		AllowCustom: true,
	}
}

// ── Registrations ───────────────────────────────────────────────────────────

func evaluateRegistration(r Requirement, d Dossier, deadline *time.Time, now time.Time) Gap {
	want := r.Registration
	if want == nil || len(d.Registrations) == 0 {
		return unknownGap(r, registrationQuestion(r))
	}

	var (
		matches   []Registration
		allAuthor = true
	)
	for _, reg := range d.Registrations {
		if !reg.Authoritative() {
			allAuthor = false
		}
		if reg.Kind != want.Kind {
			continue
		}
		if s := foldSpace(want.Section); s != "" && !strings.Contains(foldSpace(reg.Section), s) {
			continue
		}
		matches = append(matches, reg)
	}

	if len(matches) == 0 {
		return Gap{
			Requirement:       r,
			Status:            GapMissing,
			Held:              renderRegistrationsHeld(d.Registrations),
			Remedy:            Remedy{Kind: RemedyRTI, DeadlineRelevant: true},
			factAuthoritative: allAuthor,
		}
	}

	best := matches[0]
	for _, reg := range matches[1:] {
		bestValid, valid := !best.LapsedAt(now), !reg.LapsedAt(now)
		if (valid && !bestValid) || (valid == bestValid && laterUntil(reg.ValidUntil, best.ValidUntil)) {
			best = reg
		}
	}

	gap := Gap{Requirement: r, Held: renderRegistrationLine(best), factAuthoritative: best.Authoritative()}
	switch {
	case best.LapsedAt(now):
		gap.Status = GapExpired
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	case deadline != nil && best.LapsedAt(*deadline):
		gap.Status = GapExpiring
		gap.Remedy = Remedy{Kind: RemedyRenew, DeadlineRelevant: true}
	default:
		gap.Status = GapMet
	}
	return gap
}

// renderRegistrationLine deliberately omits Registration.Identifier. The
// registration number is on the PRD's never-leaves-the-backend list, and Held
// is rendered into an agent tool result and a proto message.
func renderRegistrationLine(r Registration) string {
	line := string(r.Kind)
	if s := strings.TrimSpace(r.Section); s != "" {
		line += " (" + s + ")"
	}
	if r.ValidUntil != nil {
		line += " — valida fino al " + r.ValidUntil.Format("2006-01-02")
	}
	return line
}

func renderRegistrationsHeld(all []Registration) string {
	parts := make([]string, 0, len(all))
	for _, r := range all {
		parts = append(parts, string(r.Kind))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func registrationQuestion(r Requirement) CaptureQuestion {
	prompt := "Questa gara richiede un'iscrizione. La possiedi?"
	if r.Registration != nil {
		prompt = fmt.Sprintf("Questa gara richiede l'iscrizione %s. La possiedi?", r.Registration.Kind)
	}
	return CaptureQuestion{
		Kind:        RequirementRegistration,
		Field:       "registration",
		Prompt:      prompt,
		Options:     []string{"Sì", "No", "Non lo so"},
		AllowCustom: true,
	}
}

// ── Rendering helpers ───────────────────────────────────────────────────────

// currencyOr picks the first non-empty currency, so a Held line says "EUR" when
// either side named one and stays silent when neither did.
func currencyOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// renderAmount formats minor units as a plain major-unit figure. It is
// deliberately not localised: this string is read by the model and by a proto
// consumer that formats for the user's locale itself, and a Go-side thousands
// separator would have to be undone there.
func renderAmount(minor int64, currency string) string {
	major := float64(minor) / 100
	if c := strings.TrimSpace(currency); c != "" {
		return fmt.Sprintf("%.2f %s", major, c)
	}
	return fmt.Sprintf("%.2f", major)
}
