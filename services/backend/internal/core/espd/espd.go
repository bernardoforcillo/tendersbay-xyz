// Package espd composes a European Single Procurement Document (ESPD; DGUE in
// Italy) from the company dossier, one bid's per-gara data and, when the buyer
// published one, the buyer's ESPD request.
//
// The whole package is a pure function of its inputs plus the ports it declares
// in ports.go. Compose never does I/O, never knows XML, and never invents a
// value: a field the document needs and no authoritative fact covers is a Gap,
// not a default. The two serializers (ESPD-EDM 2.1.1 for the Italian eDGUE-IT
// profile, ESPD-EDM 4.x for eForms-era buyers) and the PDF renderer live in
// internal/adapter/espd/* and consume the Response this package produces.
//
// Dependency direction: espd → company, bid (read-only). Neither company nor
// bid knows espd exists — they store the criterion key as a plain string.
// Nothing here imports internal/adapter/* (hexagonal, per
// .claude/rules/code-organization.md; enforced by boundary_test.go).
package espd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// ── Version and format ──────────────────────────────────────────────────────

// Version is the target ESPD Exchange Data Model. It is a property of the
// EXPORT, not of the Response: the same Response serializes to both.
type Version string

const (
	// EDM211 is ESPD-EDM 2.1.1, the model the Italian eDGUE-IT specification
	// (AgID) binds to and the one national platforms accept today.
	EDM211 Version = "edm_2_1_1"
	// EDM4 is the eForms-era ESPD-EDM 3.x/4.x line the EU service publishes.
	EDM4 Version = "edm_4"
)

// Valid reports whether v names a version this package knows.
func (v Version) Valid() bool { return v == EDM211 || v == EDM4 }

// Format is the export artefact kind.
type Format string

const (
	FormatXML Format = "xml"
	FormatPDF Format = "pdf"
)

// Valid reports whether f names a format this package knows.
func (f Format) Valid() bool { return f == FormatXML || f == FormatPDF }

// ── Position in the document ────────────────────────────────────────────────

// Part is a section of the document, named version-independently. Mapping a
// Part to an XML element and to a code-list UUID is the serializers' job.
type Part string

const (
	PartI    Part = "I"     // procedure and contracting authority
	PartIIA  Part = "II.A"  // economic operator identity
	PartIIB  Part = "II.B"  // representatives
	PartIIC  Part = "II.C"  // reliance on other entities
	PartIID  Part = "II.D"  // subcontractors
	PartIIIA Part = "III.A" // convictions
	PartIIIB Part = "III.B" // taxes and social security
	PartIIIC Part = "III.C" // insolvency, conflicts, misconduct
	PartIIID Part = "III.D" // purely national exclusion grounds
	PartIVA  Part = "IV.A"  // suitability
	PartIVB  Part = "IV.B"  // economic and financial standing
	PartIVC  Part = "IV.C"  // technical and professional ability
	PartIVD  Part = "IV.D"  // quality assurance and environmental management
	PartVI   Part = "VI"    // concluding statements
)

// CriterionKey is our stable identifier for one criterion. It is ours and not a
// code-list UUID because the UUIDs differ between EDM 2.1.1 and 4.x while the
// question does not; the serializers translate per version.
type CriterionKey string

// Exclusion grounds — Part III, Directive 2014/24/EU Art. 57. The set is the
// closed list every ESPD asks; a dossier is complete when each has an answer.
const (
	CritParticipationCriminalOrg  CriterionKey = "iii.a.participation_criminal_organisation"
	CritCorruption                CriterionKey = "iii.a.corruption"
	CritFraud                     CriterionKey = "iii.a.fraud"
	CritTerroristOffences         CriterionKey = "iii.a.terrorist_offences"
	CritMoneyLaundering           CriterionKey = "iii.a.money_laundering"
	CritChildLabour               CriterionKey = "iii.a.child_labour_human_trafficking"
	CritPaymentTaxes              CriterionKey = "iii.b.payment_of_taxes"
	CritPaymentSocialSecurity     CriterionKey = "iii.b.payment_of_social_security"
	CritEnvironmentalLaw          CriterionKey = "iii.c.breach_environmental_obligations"
	CritSocialLaw                 CriterionKey = "iii.c.breach_social_obligations"
	CritLabourLaw                 CriterionKey = "iii.c.breach_labour_obligations"
	CritBankruptcy                CriterionKey = "iii.c.bankruptcy"
	CritInsolvency                CriterionKey = "iii.c.insolvency"
	CritCreditorsArrangement      CriterionKey = "iii.c.arrangement_with_creditors"
	CritAnalogousSituation        CriterionKey = "iii.c.analogous_situation"
	CritAssetsByLiquidator        CriterionKey = "iii.c.assets_administered_by_liquidator"
	CritActivitiesSuspended       CriterionKey = "iii.c.business_activities_suspended"
	CritProfessionalMisconduct    CriterionKey = "iii.c.grave_professional_misconduct"
	CritDistortingCompetition     CriterionKey = "iii.c.agreements_distorting_competition"
	CritConflictOfInterest        CriterionKey = "iii.c.conflict_of_interest"
	CritInvolvementInPreparation  CriterionKey = "iii.c.involvement_in_preparation"
	CritEarlyTermination          CriterionKey = "iii.c.early_termination"
	CritMisrepresentation         CriterionKey = "iii.c.misrepresentation"
	CritPurelyNationalGrounds     CriterionKey = "iii.d.purely_national_grounds"
	CritIdentity                  CriterionKey = "ii.a.identity"
	CritRepresentatives           CriterionKey = "ii.b.representatives"
	CritReliance                  CriterionKey = "ii.c.reliance"
	CritSubcontracting            CriterionKey = "ii.d.subcontracting"
	CritProcedure                 CriterionKey = "i.procedure"
	CritLots                      CriterionKey = "i.lots"
	CritEnrolmentProfessionalReg  CriterionKey = "iv.a.enrolment_professional_register"
	CritEnrolmentTradeReg         CriterionKey = "iv.a.enrolment_trade_register"
	CritOtherRegistration         CriterionKey = "iv.a.other_registration"
	CritGeneralTurnover           CriterionKey = "iv.b.general_yearly_turnover"
	CritSpecificTurnover          CriterionKey = "iv.b.specific_yearly_turnover"
	CritReferences                CriterionKey = "iv.c.references"
	CritAverageAnnualManpower     CriterionKey = "iv.c.average_annual_manpower"
	CritSOAAttestation            CriterionKey = "iv.c.soa_attestation"
	CritQualityAssurance          CriterionKey = "iv.d.quality_assurance_certificates"
	CritEnvironmentalManagement   CriterionKey = "iv.d.environmental_management_certificates"
	CritDeclarationsConfirmation  CriterionKey = "iii.confirmation"
	critUnknownNationalGroundBase CriterionKey = "iii.d."
)

// ExclusionCriteria is the ordered closed list of Part III.A–C questions. The
// order is the document order, which is also the order a person answers them.
func ExclusionCriteria() []CriterionKey {
	return []CriterionKey{
		CritParticipationCriminalOrg, CritCorruption, CritFraud, CritTerroristOffences,
		CritMoneyLaundering, CritChildLabour,
		CritPaymentTaxes, CritPaymentSocialSecurity,
		CritEnvironmentalLaw, CritSocialLaw, CritLabourLaw,
		CritBankruptcy, CritInsolvency, CritCreditorsArrangement, CritAnalogousSituation,
		CritAssetsByLiquidator, CritActivitiesSuspended,
		CritProfessionalMisconduct, CritDistortingCompetition, CritConflictOfInterest,
		CritInvolvementInPreparation, CritEarlyTermination, CritMisrepresentation,
	}
}

var exclusionSet = func() map[CriterionKey]bool {
	m := map[CriterionKey]bool{}
	for _, k := range ExclusionCriteria() {
		m[k] = true
	}
	return m
}()

// IsExclusionCriterion reports whether k is one of the Part III.A–C questions.
func IsExclusionCriterion(k CriterionKey) bool { return exclusionSet[k] }

// Part derives the document section from the key prefix. Keys are spelled so
// this is a lookup on the first segment, never a table to keep in sync.
func (k CriterionKey) Part() Part {
	s := string(k)
	switch {
	case strings.HasPrefix(s, "i.lots"):
		return PartI
	case strings.HasPrefix(s, "i."):
		return PartI
	case strings.HasPrefix(s, "ii.a."):
		return PartIIA
	case strings.HasPrefix(s, "ii.b."):
		return PartIIB
	case strings.HasPrefix(s, "ii.c."):
		return PartIIC
	case strings.HasPrefix(s, "ii.d."):
		return PartIID
	case strings.HasPrefix(s, "iii.a."):
		return PartIIIA
	case strings.HasPrefix(s, "iii.b."):
		return PartIIIB
	case strings.HasPrefix(s, "iii.c."):
		return PartIIIC
	case strings.HasPrefix(s, "iii.d."):
		return PartIIID
	case strings.HasPrefix(s, "iii."):
		return PartIIIA
	case strings.HasPrefix(s, "iv.a."):
		return PartIVA
	case strings.HasPrefix(s, "iv.b."):
		return PartIVB
	case strings.HasPrefix(s, "iv.c."):
		return PartIVC
	case strings.HasPrefix(s, "iv.d."):
		return PartIVD
	}
	return PartVI
}

// ── Values ──────────────────────────────────────────────────────────────────

// ValueKind is the wire-independent type of a Leaf's value.
type ValueKind string

const (
	KindText   ValueKind = "text"
	KindBool   ValueKind = "bool"
	KindAmount ValueKind = "amount" // minor units + ISO-4217 currency
	KindInt    ValueKind = "int"
	KindDate   ValueKind = "date"
	KindCode   ValueKind = "code" // a code-list value (country, legal form, standard)
)

// Value is one typed document value. It is a tagged struct rather than an
// interface so a serializer can switch on Kind without reflection and the
// transport can carry it in one proto message.
type Value struct {
	Kind     ValueKind
	Text     string
	Bool     bool
	Int      int64  // KindInt, and the minor-unit amount for KindAmount
	Currency string // KindAmount only
	Date     time.Time
}

func TextValue(s string) Value    { return Value{Kind: KindText, Text: s} }
func CodeValue(s string) Value    { return Value{Kind: KindCode, Text: s} }
func BoolValue(b bool) Value      { return Value{Kind: KindBool, Bool: b} }
func IntValue(n int64) Value      { return Value{Kind: KindInt, Int: n} }
func DateValue(t time.Time) Value { return Value{Kind: KindDate, Date: t} }

// AmountValue is money in MINOR units with its currency, the scale every money
// column in this service stores.
func AmountValue(minor int64, currency string) Value {
	if currency == "" {
		currency = "EUR"
	}
	return Value{Kind: KindAmount, Int: minor, Currency: currency}
}

// String renders the value for a human (the PDF, a log-free debug view). It is
// not the XML encoding — that is the serializer's.
func (v Value) String() string {
	switch v.Kind {
	case KindBool:
		if v.Bool {
			return "yes"
		}
		return "no"
	case KindInt:
		return fmt.Sprintf("%d", v.Int)
	case KindAmount:
		return fmt.Sprintf("%d.%02d %s", v.Int/100, v.Int%100, v.Currency)
	case KindDate:
		return v.Date.Format("2006-01-02")
	default:
		return v.Text
	}
}

// ── Leaves and gaps ─────────────────────────────────────────────────────────

// SourceKind says where a leaf's value came from.
type SourceKind string

const (
	SourceDossier     SourceKind = "dossier"     // a company fact
	SourceDeclaration SourceKind = "declaration" // a Part III answer
	SourceBid         SourceKind = "bid"         // per-gara data (lots, parties)
	SourceRequest     SourceKind = "request"     // the buyer's ESPD request
)

// SourceRef points back at the record a leaf was read from, so the UI can open
// it and the audit can name it.
type SourceRef struct {
	Kind SourceKind
	ID   string
}

// Leaf is ONE value of the document together with its origin. It is the unit of
// provenance: Compose refuses to make one from a dossier fact whose Attribution
// is not authoritative — that fact becomes a Gap instead.
//
// Attribution is meaningful for dossier and declaration leaves. A bid leaf is a
// choice a workbench manager made for this gara (user_stated, no per-row
// author in this phase); a request leaf is the buyer's own text (imported). The
// authoritative-only rule is a rule about statements the operator makes about
// ITSELF, which is why it applies to the first two kinds and not to the others.
type Leaf struct {
	Part        Part
	Criterion   CriterionKey
	Field       string // stable field name within the criterion
	Value       Value
	Attribution company.Attribution
	Source      SourceRef
}

// GapScope says where the fix lives: in the dossier (once, for every gara) or
// on this bid.
type GapScope string

const (
	ScopeCompany GapScope = "company"
	ScopeBid     GapScope = "bid"
)

// GapReason says why the field is empty.
type GapReason string

const (
	// ReasonMissing: nobody has ever asserted the fact.
	ReasonMissing GapReason = "missing"
	// ReasonNotAuthoritative: a value exists but its provenance is
	// agent_inferred or imported, and this document only carries facts a
	// human stated or confirmed.
	ReasonNotAuthoritative GapReason = "not_authoritative"
	// ReasonStale: the Part III declarations exist but have not been
	// re-confirmed for this bid since they last changed.
	ReasonStale GapReason = "stale"
)

// Gap is a field the document requires that no authoritative fact covers.
type Gap struct {
	Part      Part
	Criterion CriterionKey
	Field     string
	Scope     GapScope
	Reason    GapReason
}

// ── Declarations ────────────────────────────────────────────────────────────

// Answer is the state of one Part III.A–C question on the composed document.
type Answer struct {
	Criterion    CriterionKey
	Answered     bool // false: never declared
	Applies      bool // the exclusion ground applies to the operator
	SelfCleaning string
	Attribution  company.Attribution
}

// DeclarationSet is Part III.A–C as a whole, with the per-bid re-confirmation
// state. Hash is HashDeclarations over the dossier's current declarations;
// Confirmation is what the bid holds. They match or they don't — no flag.
type DeclarationSet struct {
	Answers      []Answer
	Hash         string
	Confirmation *bid.DeclarationConfirmation
}

// Complete reports whether every exclusion ground has an answer.
func (s DeclarationSet) Complete() bool {
	if len(s.Answers) == 0 {
		return false
	}
	for _, a := range s.Answers {
		if !a.Answered {
			return false
		}
	}
	return true
}

// Confirmed reports whether the CURRENT set of declarations carries a
// confirmation for this bid. A confirmation of an earlier set is stale, and
// stale is not confirmed.
func (s DeclarationSet) Confirmed() bool {
	return s.Complete() && s.Confirmation != nil && s.Confirmation.DeclarationsHash == s.Hash
}

// HashDeclarations is the content hash a DeclarationConfirmation binds to. It
// covers exactly what a signatory re-reads — criterion, answer, self-cleaning
// text — and deliberately NOT the attribution timestamps, so re-saving an
// unchanged answer does not invalidate a confirmation. Order-independent.
func HashDeclarations(decls []company.Declaration) string {
	lines := make([]string, 0, len(decls))
	for _, d := range decls {
		lines = append(lines, fmt.Sprintf("%s\t%t\t%s", d.Criterion, d.Answer, strings.TrimSpace(d.SelfCleaning)))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// ── The composed document ───────────────────────────────────────────────────

// Response is the composed document. It has no I/O and knows no XML.
type Response struct {
	Leaves       []Leaf
	Gaps         []Gap
	Request      *Request // nil when composed without a buyer request
	Declarations DeclarationSet
	ComposedAt   time.Time
}

// Ready reports whether the document can be exported: nothing missing and the
// Part III declarations confirmed for this bid.
func (r Response) Ready() bool { return len(r.Gaps) == 0 && r.Declarations.Confirmed() }

// Readiness is the progress pair the UI renders: how many fields are filled
// and how many are still open.
func (r Response) Readiness() (ready, missing int) { return len(r.Leaves), len(r.Gaps) }

// GapsIn returns the gaps of one part, in document order.
func (r Response) GapsIn(p Part) []Gap {
	var out []Gap
	for _, g := range r.Gaps {
		if g.Part == p {
			out = append(out, g)
		}
	}
	return out
}

// ── Export audit ────────────────────────────────────────────────────────────

// Export is the FACT of an export, never the bytes: who exported which version
// in which format, the content hash so a file can be matched to its record,
// and the declaration confirmation the export rested on.
type Export struct {
	ID                      string
	BidID                   string
	UserID                  string
	Version                 Version
	Format                  Format
	ContentSHA256           string
	DeclarationsConfirmedAt time.Time
	ExportedAt              time.Time
}

// ── Sentinel errors ─────────────────────────────────────────────────────────

var (
	ErrInvalidArgument    = errors.New("espd: invalid argument")
	ErrNotReady           = errors.New("espd: response has open gaps or unconfirmed declarations")
	ErrRequestNotFound    = errors.New("espd: no buyer request stored for this bid")
	ErrUnsupportedRequest = errors.New("espd: unsupported request document")
	// ErrNotEntitled is the export refusal: the workspace's plan does not carry
	// espd.export, or the feature is switched off. It is deliberately NOT the
	// same error as a permission failure — the caller may write to this
	// workbench, the PLAN is what says no — so the transport can answer with a
	// reason a person can act on ("upgrade") rather than a bare 403.
	ErrNotEntitled = errors.New("espd: the workspace's plan does not include the ESPD export")
	// ErrCriterionUnsupported means a serializer has no code-list entry for a
	// criterion in the target version. It is an error and never a silently
	// omitted element: a DGUE missing a criterion the buyer asked about is
	// worse than no DGUE, because it looks complete.
	ErrCriterionUnsupported = errors.New("espd: criterion is not expressible in this ESPD version")
)

// NotReadyError carries WHY the document could not be exported, so the client
// renders the same gap list the preview shows rather than a bare refusal.
type NotReadyError struct {
	Gaps                  []Gap
	DeclarationsConfirmed bool
}

func (e *NotReadyError) Error() string {
	return fmt.Sprintf("espd: %d open gap(s), declarations confirmed: %t", len(e.Gaps), e.DeclarationsConfirmed)
}

func (e *NotReadyError) Is(target error) bool { return target == ErrNotReady }

// NotEntitledError carries featurelayer's reason for the refusal, which is what
// tells "not on your plan" apart from "switched off for everyone".
type NotEntitledError struct {
	Reason string
	Detail string
}

func (e *NotEntitledError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("espd: export not entitled (%s: %s)", e.Reason, e.Detail)
	}
	return fmt.Sprintf("espd: export not entitled (%s)", e.Reason)
}

func (e *NotEntitledError) Is(target error) bool { return target == ErrNotEntitled }

// CriterionUnsupportedError names the criterion and the version that cannot
// express it, because "which one" is the whole content of the bug report.
type CriterionUnsupportedError struct {
	Key     CriterionKey
	Version Version
}

func (e *CriterionUnsupportedError) Error() string {
	return fmt.Sprintf("espd: criterion %q has no code-list entry in %s", e.Key, e.Version)
}

func (e *CriterionUnsupportedError) Is(target error) bool { return target == ErrCriterionUnsupported }
