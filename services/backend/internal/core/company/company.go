// Package company owns the company dossier — the facts about the operator
// ITSELF (registry identity, SOA attestations, certifications, esercizi,
// referenze, iscrizioni) — and the eligibility question those facts answer:
// "may we lawfully be admitted to this gara at all".
//
// It is a separate package from core/clientprofile, and the dividing test is
// mechanical rather than aesthetic:
//
//	If changing the field changes WHICH TENDERS APPEAR, it belongs in
//	clientprofile. If it changes WHETHER WE MAY LAWFULLY BID on a tender that
//	already appeared, it belongs here.
//
// clientprofile.Profile.ValueMax = 500_000 ("I don't want jobs bigger than
// this") and FinancialYear{2024, Turnover: 2_100_000_00} ("I invoiced this")
// look superficially alike and are categorically different: the first is a
// preference the user may change on a whim, the second is a fact a buyer can
// demand a bilancio for. A preference has no provenance; a fact must have one.
// That asymmetry alone forbids the merge — clientprofile has no provenance
// columns and should never grow any.
//
// Downward, this package must NOT import core/bid. Eligibility is a pure
// function of (requirements × dossier); the DECISION is bid.GoNoGo and stays
// there. Like core/bid and core/document, nothing here imports
// internal/adapter/* — this package defines its ports and the adapters
// implement them (hexagonal, per .claude/rules/code-organization.md).
package company

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ── Provenance ──────────────────────────────────────────────────────────────

// Provenance says WHO asserted a fact, and it is the gate on what the
// eligibility engine may conclude from it. A value the agent derived is not the
// same kind of thing as a value the user typed, and the difference is legal,
// not cosmetic: the human carries the liability for a bid, so only a
// human-sourced fact may produce a blocking "no".
type Provenance string

const (
	// ProvenanceUserStated — typed into the dossier form by a human.
	ProvenanceUserStated Provenance = "user_stated"
	// ProvenanceUserConfirmed — the agent asked a closed question via
	// ask_choice and the human answered it. The answer physically arrives as a
	// choice_response row, so a human really did state it; what the agent
	// contributed is the question, not the value.
	ProvenanceUserConfirmed Provenance = "user_confirmed"
	// ProvenanceAgentInferred — derived by a model and never shown for
	// confirmation. NOTHING IN PHASE 2 PRODUCES ONE. The constant exists now so
	// the engine's asymmetry (a no_go may never rest on one) is written and
	// tested before a producer arrives — adding the rule after the producer is
	// how it gets skipped.
	ProvenanceAgentInferred Provenance = "agent_inferred"
	// ProvenanceImported — read out of a document or a registry (a visura, an
	// old DGUE). Deferred with the file substrate; same treatment as
	// agent_inferred.
	ProvenanceImported Provenance = "imported"
)

// Authoritative reports whether a fact may, on its own, produce a blocking
// verdict. Prefer it over comparing Provenance at a call site, so the rule
// stays visible in reading code — the same reason tender.AwardCriterion.HasWeight
// exists.
func (p Provenance) Authoritative() bool {
	return p == ProvenanceUserStated || p == ProvenanceUserConfirmed
}

// Valid reports whether p is one of the four known provenances. An unknown
// string must never reach storage: a provenance nobody can interpret is worse
// than none, because Authoritative silently answers false for it and the fact
// then looks deliberately down-weighted rather than corrupt.
func (p Provenance) Valid() bool {
	switch p {
	case ProvenanceUserStated, ProvenanceUserConfirmed, ProvenanceAgentInferred, ProvenanceImported:
		return true
	}
	return false
}

// PromptSource records WHY we asked. It is what makes the PRD's just-in-time
// vs up-front hypothesis falsifiable rather than a belief: the same field feeds
// company_dossier_field_answered's prompted_by property.
type PromptSource string

const (
	PromptTender     PromptSource = "tender"     // a specific gara demanded it
	PromptOnboarding PromptSource = "onboarding" // asked in the abstract
	PromptExtraction PromptSource = "extraction" // pulled from a document
)

// Valid reports whether s is one of the three known prompt sources.
func (s PromptSource) Valid() bool {
	switch s {
	case PromptTender, PromptOnboarding, PromptExtraction:
		return true
	}
	return false
}

// Attribution is the per-fact provenance envelope. It is EMBEDDED in every
// repeated record and stored per-field for the identity scalars.
type Attribution struct {
	Provenance Provenance
	// Confidence is nil unless Provenance is agent_inferred or imported. A
	// human-stated fact has no model confidence — that is not 1.0, it is
	// not-applicable, and coalescing it to 1.0 would let a scoring pass treat
	// the two as interchangeable.
	Confidence *float64 // [0,1]
	StatedBy   string   // user id; "" when no human was involved
	StatedAt   time.Time
	PromptedBy PromptSource
	// PromptedByTenderID is the gara that earned the question. A plain *int64
	// with no foreign key — tenders.ingested_tenders is owned by
	// services/ingestion, the same rule bids.tender_id already follows.
	PromptedByTenderID *int64
	// SourceNote is a short human-readable trace ("gara 545620-2026,
	// disciplinare §7.2"). Never a document body; never enters analytics.
	SourceNote string
}

// Authoritative is the shorthand every engine rule reads through.
func (a Attribution) Authoritative() bool { return a.Provenance.Authoritative() }

const maxSourceNoteLen = 500

// validate rejects an envelope that cannot be interpreted downstream. The
// confidence rule is the load-bearing one: a confidence attached to a
// human-stated fact would be a model score on a datum no model touched.
func (a Attribution) validate() error {
	if !a.Provenance.Valid() {
		return fmt.Errorf("%w: unknown provenance %q", ErrInvalidArgument, a.Provenance)
	}
	if a.PromptedBy != "" && !a.PromptedBy.Valid() {
		return fmt.Errorf("%w: unknown prompt source %q", ErrInvalidArgument, a.PromptedBy)
	}
	if a.Confidence != nil {
		if a.Provenance.Authoritative() {
			return fmt.Errorf("%w: a %s fact carries no model confidence", ErrInvalidArgument, a.Provenance)
		}
		if *a.Confidence < 0 || *a.Confidence > 1 {
			return fmt.Errorf("%w: confidence %v outside [0,1]", ErrInvalidArgument, *a.Confidence)
		}
	}
	if len(a.SourceNote) > maxSourceNoteLen {
		return fmt.Errorf("%w: source note must be %d characters or fewer", ErrInvalidArgument, maxSourceNoteLen)
	}
	return nil
}

// ── Identity ────────────────────────────────────────────────────────────────

// LegalForm is the company's forma giuridica.
type LegalForm string

const (
	LegalFormSRL              LegalForm = "srl"
	LegalFormSRLS             LegalForm = "srls"
	LegalFormSPA              LegalForm = "spa"
	LegalFormSAPA             LegalForm = "sapa"
	LegalFormSAS              LegalForm = "sas"
	LegalFormSNC              LegalForm = "snc"
	LegalFormDittaIndividuale LegalForm = "ditta_individuale"
	LegalFormCooperativa      LegalForm = "cooperativa"
	LegalFormConsorzio        LegalForm = "consorzio"
	LegalFormSocietaSemplice  LegalForm = "societa_semplice"
	LegalFormEntePubblico     LegalForm = "ente_pubblico"
	LegalFormOther            LegalForm = "other"
)

var validLegalForms = map[LegalForm]bool{
	LegalFormSRL: true, LegalFormSRLS: true, LegalFormSPA: true, LegalFormSAPA: true,
	LegalFormSAS: true, LegalFormSNC: true, LegalFormDittaIndividuale: true,
	LegalFormCooperativa: true, LegalFormConsorzio: true, LegalFormSocietaSemplice: true,
	LegalFormEntePubblico: true, LegalFormOther: true,
}

// FieldKey names one identity scalar for per-field attribution. It is a closed
// set: the attribution map is keyed by these and validated on write, so a typo
// cannot silently create a provenance entry nothing will ever read.
type FieldKey string

const (
	FieldLegalName   FieldKey = "legal_name"
	FieldVATNumber   FieldKey = "vat_number"
	FieldFiscalCode  FieldKey = "fiscal_code"
	FieldLegalForm   FieldKey = "legal_form"
	FieldCCIAA       FieldKey = "cciaa" // office + REA together
	FieldCountry     FieldKey = "country"
	FieldNUTS        FieldKey = "nuts"
	FieldFoundedYear FieldKey = "founded_year"
)

var validFieldKeys = map[FieldKey]bool{
	FieldLegalName: true, FieldVATNumber: true, FieldFiscalCode: true, FieldLegalForm: true,
	FieldCCIAA: true, FieldCountry: true, FieldNUTS: true, FieldFoundedYear: true,
}

// ValidFieldKey reports whether k names an identity scalar. Exported because
// the transport layer and the agent tool both accept a field key from outside
// and must reject an unknown one before it reaches the service.
func ValidFieldKey(k FieldKey) bool { return validFieldKeys[k] }

// Identity is the company's registry identity — the part a DGUE Part II reads
// straight off.
//
// VATNumber and FiscalCode are separate fields and not one, because for an
// Italian company they usually coincide (11 digits) and for a ditta
// individuale they do not (16-character codice fiscale, 11-digit partita IVA).
// Storing one and deriving the other is wrong for the exact population this
// product is built for.
type Identity struct {
	LegalName   string
	VATNumber   string // partita IVA — 11 digits for IT
	FiscalCode  string // codice fiscale — 11 or 16 chars for IT
	LegalForm   LegalForm
	CCIAAOffice string // provincia sigla, e.g. "MI"
	CCIAANumber string // numero REA
	Country     string // alpha-2, matching clientprofile.Countries' convention
	NUTS        string // uppercase NUTS code of the legal seat
	FoundedYear *int32
	// Attribution is per-field, keyed by the FieldKey constants above. A field
	// with no entry has never been asserted by anyone — which is a different
	// state from an empty string the user deliberately cleared.
	Attribution map[FieldKey]Attribution
}

var (
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	nutsRe    = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{0,3}$`)
)

// minFoundedYear is the earliest year the dossier accepts. It is not a guess at
// the oldest European firm; it is a typo fence — a four-digit year below it is
// far more likely a mis-keyed VAT fragment than a real foundation date.
const minFoundedYear = 1500

func (id Identity) validate(now time.Time) error {
	if id.LegalForm != "" && !validLegalForms[id.LegalForm] {
		return fmt.Errorf("%w: unknown legal form %q", ErrInvalidArgument, id.LegalForm)
	}
	if id.Country != "" && !countryRe.MatchString(id.Country) {
		return fmt.Errorf("%w: country must be an uppercase ISO-3166-1 alpha-2 code", ErrInvalidArgument)
	}
	if id.NUTS != "" && !nutsRe.MatchString(id.NUTS) {
		return fmt.Errorf("%w: nuts must be an uppercase NUTS code", ErrInvalidArgument)
	}
	if id.FoundedYear != nil {
		if y := int(*id.FoundedYear); y < minFoundedYear || y > now.Year() {
			return fmt.Errorf("%w: founded year %d outside [%d,%d]", ErrInvalidArgument, y, minFoundedYear, now.Year())
		}
	}
	for k, a := range id.Attribution {
		if !validFieldKeys[k] {
			return fmt.Errorf("%w: %q", ErrUnknownFieldKey, k)
		}
		if err := a.validate(); err != nil {
			return fmt.Errorf("attribution[%s]: %w", k, err)
		}
	}
	return nil
}

// ── SOA ─────────────────────────────────────────────────────────────────────

// Classifica is a SOA classifica, ordered. It is an int and not a string
// because the entire eligibility question for SOA is an ORDERING question:
// classifica III satisfies a requirement of classifica II. A string would make
// the one comparison that matters impossible without a lookup table at every
// call site.
type Classifica int

const (
	ClassificaI Classifica = iota + 1
	ClassificaII
	ClassificaIII
	ClassificaIIIBis // the "bis" tiers are real and are NOT decorative
	ClassificaIV
	ClassificaIVBis
	ClassificaV
	ClassificaVI
	ClassificaVII
	ClassificaVIII
)

// classificaNames is the published rendering of each tier, indexed by the
// Classifica value itself (slot 0 is unused so the index is the value).
var classificaNames = [...]string{
	"", "I", "II", "III", "III-bis", "IV", "IV-bis", "V", "VI", "VII", "VIII",
}

// AtLeast reports whether c covers a requirement of want.
func (c Classifica) AtLeast(want Classifica) bool { return c >= want }

// Valid reports whether c names a real tier.
func (c Classifica) Valid() bool { return c >= ClassificaI && c <= ClassificaVIII }

// String renders "I", "III-bis", "VIII". An out-of-range value renders as its
// number rather than as an empty string, so a corrupt row is visible in a log
// line instead of vanishing from it.
func (c Classifica) String() string {
	if !c.Valid() {
		return fmt.Sprintf("classifica(%d)", int(c))
	}
	return classificaNames[c]
}

// classificaCeilings is the works value each classifica covers, in EUR minor
// units (the thresholds of allegato II.12 to D.Lgs 36/2023). Slot 0 is unused;
// VIII is unbounded and carries no entry.
var classificaCeilings = map[Classifica]int64{
	ClassificaI:      258_000_00,
	ClassificaII:     516_000_00,
	ClassificaIII:    1_033_000_00,
	ClassificaIIIBis: 1_500_000_00,
	ClassificaIV:     2_582_000_00,
	ClassificaIVBis:  3_500_000_00,
	ClassificaV:      5_165_000_00,
	ClassificaVI:     10_329_000_00,
	ClassificaVII:    15_494_000_00,
}

// CeilingEUR is the works value c covers, in EUR minor units (VIII is
// unbounded and returns nil). It exists for ONE purpose: the "what would change
// it" sentence — "OG1 III copre lavori fino a 1.033.000 EUR; questa gara vale
// 2.400.000 EUR". Without it the remedy line can only say "get a higher
// classifica", which the user already knew.
func (c Classifica) CeilingEUR() *int64 {
	v, ok := classificaCeilings[c]
	if !ok {
		return nil
	}
	return &v
}

// ParseClassifica reads the published rendering back into the ordered value.
// It exists because every wire format this dossier crosses (the protobuf
// contract, the agent tool's JSON) carries the classifica as its published
// string, and a parse that lived at each of those call sites would drift.
func ParseClassifica(s string) (Classifica, bool) {
	norm := strings.ToUpper(strings.TrimSpace(s))
	norm = strings.ReplaceAll(norm, " ", "")
	norm = strings.ReplaceAll(norm, "BIS", "-BIS")
	norm = strings.ReplaceAll(norm, "--BIS", "-BIS")
	for i, name := range classificaNames {
		if i == 0 {
			continue
		}
		if strings.ToUpper(name) == norm {
			return Classifica(i), true
		}
	}
	return 0, false
}

// soaCategoryRe is the closed set of SOA categories: OG1-OG13 (opere generali)
// and OS1-OS35 (opere specializzate). Validated rather than free text because a
// category is matched by equality in the engine — "OG.1" or "og 1" would read
// as a category the company does not hold and turn a met requirement into a gap.
var soaCategoryRe = regexp.MustCompile(`^(OG([1-9]|1[0-3])|OS([1-9]|[12][0-9]|3[0-5]))$`)

// NormalizeSOACategory upper-cases and trims a published category. It returns
// false for anything outside the closed set.
func NormalizeSOACategory(s string) (string, bool) {
	norm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if !soaCategoryRe.MatchString(norm) {
		return "", false
	}
	return norm, true
}

// SOACategory is one attestazione SOA line.
type SOACategory struct {
	ID         string
	Category   string // "OG1".."OG13", "OS1".."OS35" — validated closed set
	Classifica Classifica
	IssuedBy   string // the SOA body
	ValidUntil *time.Time
	// VerifiedUntil is the triennial verification deadline, which lapses THREE
	// YEARS before ValidUntil and invalidates the attestation on its own. A
	// dossier that tracked only ValidUntil would report a lapsed attestation as
	// valid for two more years — the single most likely wrong "go" this engine
	// could produce.
	VerifiedUntil *time.Time
	Attribution
}

// LapsedAt reports whether the attestation is invalid at t, by EITHER deadline.
// The disjunction is the whole point of carrying two dates.
func (c SOACategory) LapsedAt(t time.Time) bool {
	return (c.ValidUntil != nil && !c.ValidUntil.After(t)) ||
		(c.VerifiedUntil != nil && !c.VerifiedUntil.After(t))
}

func (c SOACategory) validate() error {
	if _, ok := NormalizeSOACategory(c.Category); !ok {
		return ErrInvalidSOACategory
	}
	if !c.Classifica.Valid() {
		return ErrInvalidClassifica
	}
	return c.Attribution.validate()
}

// ── Certifications ──────────────────────────────────────────────────────────

// CertificationStandard is a certification scheme the dossier models by name.
type CertificationStandard string

const (
	CertISO9001   CertificationStandard = "iso_9001"
	CertISO14001  CertificationStandard = "iso_14001"
	CertISO45001  CertificationStandard = "iso_45001"
	CertISO27001  CertificationStandard = "iso_27001"
	CertISO37001  CertificationStandard = "iso_37001"
	CertISO50001  CertificationStandard = "iso_50001"
	CertSA8000    CertificationStandard = "sa_8000"
	CertUNIPdR125 CertificationStandard = "uni_pdr_125"
	CertISO20121  CertificationStandard = "iso_20121"
	CertEMAS      CertificationStandard = "emas"
	CertOtherStd  CertificationStandard = "other"
)

var validCertificationStandards = map[CertificationStandard]bool{
	CertISO9001: true, CertISO14001: true, CertISO45001: true, CertISO27001: true,
	CertISO37001: true, CertISO50001: true, CertSA8000: true, CertUNIPdR125: true,
	CertISO20121: true, CertEMAS: true, CertOtherStd: true,
}

// Certification is one held certificate.
type Certification struct {
	ID          string
	Standard    CertificationStandard
	StandardRaw string // the published name when Standard == "other"
	Scope       string // certified scope / IAF sector — a cert is scope-bounded
	IssuedBy    string // certification body (never enters analytics)
	ValidFrom   *time.Time
	ValidUntil  *time.Time
	Attribution
}

// LapsedAt reports whether the certificate is invalid at t.
func (c Certification) LapsedAt(t time.Time) bool {
	return c.ValidUntil != nil && !c.ValidUntil.After(t)
}

func (c Certification) validate() error {
	if !validCertificationStandards[c.Standard] {
		return fmt.Errorf("%w: unknown certification standard %q", ErrInvalidArgument, c.Standard)
	}
	if c.Standard == CertOtherStd && strings.TrimSpace(c.StandardRaw) == "" {
		return fmt.Errorf("%w: standard \"other\" requires the published name in standard_raw", ErrInvalidArgument)
	}
	if c.ValidFrom != nil && c.ValidUntil != nil && c.ValidUntil.Before(*c.ValidFrom) {
		return fmt.Errorf("%w: certification valid_until precedes valid_from", ErrInvalidArgument)
	}
	return c.Attribution.validate()
}

// ── Financial years ─────────────────────────────────────────────────────────

// FinancialYear carries turnover AND headcount together because Italian
// selection criteria ask for both per esercizio ("fatturato globale degli
// ultimi tre esercizi", "organico medio annuo del triennio"). A scalar
// Headcount on Identity would be a point-in-time value that silently answers a
// three-year question with today's number.
type FinancialYear struct {
	Year          int32
	TurnoverMinor *int64 // fatturato globale, minor units
	// SpecificTurnoverMinor is fatturato "nel settore oggetto della gara" — the
	// figure most requisiti actually name, and always ≤ TurnoverMinor.
	SpecificTurnoverMinor *int64
	Currency              string // ISO-4217; "EUR" in practice
	Headcount             *int32
	Attribution
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func (f FinancialYear) validate(now time.Time) error {
	if int(f.Year) < minFoundedYear || int(f.Year) > now.Year() {
		return fmt.Errorf("%w: financial year %d outside [%d,%d]", ErrInvalidArgument, f.Year, minFoundedYear, now.Year())
	}
	if f.Currency != "" && !currencyRe.MatchString(f.Currency) {
		return fmt.Errorf("%w: currency must be an uppercase ISO-4217 code", ErrInvalidArgument)
	}
	if f.TurnoverMinor != nil && *f.TurnoverMinor < 0 {
		return fmt.Errorf("%w: turnover must not be negative", ErrInvalidArgument)
	}
	if f.SpecificTurnoverMinor != nil {
		if *f.SpecificTurnoverMinor < 0 {
			return fmt.Errorf("%w: specific turnover must not be negative", ErrInvalidArgument)
		}
		// Fatturato specifico is a subset of fatturato globale by definition. A
		// dossier that stated otherwise would satisfy a specific-turnover
		// requirement it does not meet, which is the wrong direction to fail in.
		if f.TurnoverMinor != nil && *f.SpecificTurnoverMinor > *f.TurnoverMinor {
			return fmt.Errorf("%w: specific turnover exceeds global turnover", ErrInvalidArgument)
		}
	}
	if f.Headcount != nil && *f.Headcount < 0 {
		return fmt.Errorf("%w: headcount must not be negative", ErrInvalidArgument)
	}
	return f.Attribution.validate()
}

// ── Past contracts ──────────────────────────────────────────────────────────

// ContractRole is how the company participated in a past contract.
type ContractRole string

const (
	RolePrincipal     ContractRole = "principal"
	RoleSubcontractor ContractRole = "subcontractor"
	RoleRTIMandataria ContractRole = "rti_mandataria"
	RoleRTIMandante   ContractRole = "rti_mandante"
)

const maxDescriptionLen = 1000

var validContractRoles = map[ContractRole]bool{
	RolePrincipal: true, RoleSubcontractor: true, RoleRTIMandataria: true, RoleRTIMandante: true,
}

// Shared reports whether the role is one where the company carried only part of
// the contract, so SharePct is the figure that counts toward a requirement.
func (r ContractRole) Shared() bool {
	return r == RoleRTIMandataria || r == RoleRTIMandante || r == RoleSubcontractor
}

// PastContract is one referenza / lavoro analogo.
type PastContract struct {
	ID          string
	Description string // short; the user's own words
	BuyerName   string
	CPV         string
	ValueMinor  *int64
	Currency    string
	StartedOn   *time.Time
	EndedOn     *time.Time
	Role        ContractRole
	// SharePct is the company's own share, in percent [0,100], when Role is a
	// shared one. It is what actually counts toward a requirement — a 20%
	// mandante on a €5M contract carries €1M of reference, not €5M — and
	// omitting it is the classic way an SME's dossier overstates its own track
	// record.
	SharePct *float64
	Attribution
}

// EffectiveValueMinor is the slice of the contract that counts toward a
// requirement. An unshared role counts in full; a shared role with no stated
// share returns nil rather than the full value, because guessing 100% is the
// exact overstatement SharePct exists to prevent.
func (p PastContract) EffectiveValueMinor() *int64 {
	if p.ValueMinor == nil {
		return nil
	}
	if !p.Role.Shared() {
		v := *p.ValueMinor
		return &v
	}
	if p.SharePct == nil {
		return nil
	}
	v := int64(float64(*p.ValueMinor) * *p.SharePct / 100)
	return &v
}

var cpvRe = regexp.MustCompile(`^\d{2,8}$`)

func (p PastContract) validate() error {
	if !validContractRoles[p.Role] {
		return fmt.Errorf("%w: unknown contract role %q", ErrInvalidArgument, p.Role)
	}
	if len(p.Description) > maxDescriptionLen {
		return fmt.Errorf("%w: description must be %d characters or fewer", ErrInvalidArgument, maxDescriptionLen)
	}
	if p.CPV != "" && !cpvRe.MatchString(p.CPV) {
		return fmt.Errorf("%w: cpv must be a 2-8 digit code or prefix", ErrInvalidArgument)
	}
	if p.Currency != "" && !currencyRe.MatchString(p.Currency) {
		return fmt.Errorf("%w: currency must be an uppercase ISO-4217 code", ErrInvalidArgument)
	}
	if p.ValueMinor != nil && *p.ValueMinor < 0 {
		return fmt.Errorf("%w: contract value must not be negative", ErrInvalidArgument)
	}
	if p.SharePct != nil && (*p.SharePct < 0 || *p.SharePct > 100) {
		return fmt.Errorf("%w: share_pct %v outside [0,100]", ErrInvalidArgument, *p.SharePct)
	}
	if p.StartedOn != nil && p.EndedOn != nil && p.EndedOn.Before(*p.StartedOn) {
		return fmt.Errorf("%w: contract ended_on precedes started_on", ErrInvalidArgument)
	}
	return p.Attribution.validate()
}

// ── Registrations ───────────────────────────────────────────────────────────

// RegistrationKind is an iscrizione the company holds.
type RegistrationKind string

const (
	RegWhiteList             RegistrationKind = "white_list"
	RegCCIAA                 RegistrationKind = "cciaa"
	RegAlboGestoriAmbientali RegistrationKind = "albo_gestori_ambientali"
	RegMEPA                  RegistrationKind = "mepa"
	RegSistemaQualificazione RegistrationKind = "sistema_qualificazione"
	RegAlboProfessionale     RegistrationKind = "albo_professionale"
	RegANAC                  RegistrationKind = "anac"
	RegOther                 RegistrationKind = "other"
)

var validRegistrationKinds = map[RegistrationKind]bool{
	RegWhiteList: true, RegCCIAA: true, RegAlboGestoriAmbientali: true, RegMEPA: true,
	RegSistemaQualificazione: true, RegAlboProfessionale: true, RegANAC: true, RegOther: true,
}

// Registration is one iscrizione. Identifier is a registration number and must
// never leave the backend into analytics — see the PRD's never-a-property list.
type Registration struct {
	ID         string
	Kind       RegistrationKind
	Authority  string // "Prefettura di Milano"
	Identifier string // registration number
	Section    string // white-list activity section / albo category
	ValidFrom  *time.Time
	ValidUntil *time.Time
	Attribution
}

// LapsedAt reports whether the registration is invalid at t.
func (r Registration) LapsedAt(t time.Time) bool {
	return r.ValidUntil != nil && !r.ValidUntil.After(t)
}

func (r Registration) validate() error {
	if !validRegistrationKinds[r.Kind] {
		return fmt.Errorf("%w: unknown registration kind %q", ErrInvalidArgument, r.Kind)
	}
	if r.ValidFrom != nil && r.ValidUntil != nil && r.ValidUntil.Before(*r.ValidFrom) {
		return fmt.Errorf("%w: registration valid_until precedes valid_from", ErrInvalidArgument)
	}
	return r.Attribution.validate()
}

// ── The dossier ─────────────────────────────────────────────────────────────

// Dossier is the whole company, one per workspace. There is deliberately no
// company id: the workspace IS the company, exactly as the workspace IS the
// client for clientprofile. A multi-company model would need a company_id on
// every child row, every port signature and every RPC — see the PRD's settled
// decision.
type Dossier struct {
	WorkspaceID    string
	Identity       Identity
	SOA            []SOACategory
	Certifications []Certification
	FinancialYears []FinancialYear // newest first
	PastContracts  []PastContract
	Registrations  []Registration
	UpdatedAt      time.Time
}

// Empty reports whether nothing has ever been asserted about this company. It
// is the trigger for the "do not invent company facts" steering the agent tool
// emits, and it is deliberately not "Identity is zero": a dossier whose only
// content is one SOA line answered in chat is not empty.
func (d Dossier) Empty() bool {
	return d.Identity.LegalName == "" && d.Identity.VATNumber == "" && d.Identity.FiscalCode == "" &&
		len(d.SOA) == 0 && len(d.Certifications) == 0 && len(d.FinancialYears) == 0 &&
		len(d.PastContracts) == 0 && len(d.Registrations) == 0
}

// ── Facts (the just-in-time capture unit) ───────────────────────────────────

// FactKind names which record a Fact carries. It doubles as the closed enum the
// agent tool and the transport layer accept.
type FactKind string

const (
	FactIdentity      FactKind = "identity"
	FactSOA           FactKind = "soa"
	FactCertification FactKind = "certification"
	FactFinancialYear FactKind = "financial_year"
	FactPastContract  FactKind = "past_contract"
	FactRegistration  FactKind = "registration"
)

// IdentityFact is one identity scalar answered in isolation — the shape a
// just-in-time answer takes, since a chat answer names one field and not a
// whole identity block. Value is text for every key; FieldFoundedYear parses as
// an integer year, and FieldCCIAA takes "<sigla>/<numero REA>" because the two
// halves are one answer to one question.
type IdentityFact struct {
	Key   FieldKey
	Value string
}

// Fact is exactly one dossier record, captured in answer to exactly one
// question. The union is a struct of pointers rather than an interface so that
// the transport and agent layers can build one field-by-field from a decoded
// payload without a type switch on an unexported type.
type Fact struct {
	Identity      *IdentityFact
	SOA           *SOACategory
	Certification *Certification
	FinancialYear *FinancialYear
	PastContract  *PastContract
	Registration  *Registration
}

// Kind reports which record f carries, and false when it carries none or more
// than one. "More than one" is an error and not a merge: a Fact is the answer
// to one question, and its Attribution is stamped once, for one datum.
func (f Fact) Kind() (FactKind, bool) {
	var (
		kind  FactKind
		count int
	)
	if f.Identity != nil {
		kind, count = FactIdentity, count+1
	}
	if f.SOA != nil {
		kind, count = FactSOA, count+1
	}
	if f.Certification != nil {
		kind, count = FactCertification, count+1
	}
	if f.FinancialYear != nil {
		kind, count = FactFinancialYear, count+1
	}
	if f.PastContract != nil {
		kind, count = FactPastContract, count+1
	}
	if f.Registration != nil {
		kind, count = FactRegistration, count+1
	}
	return kind, count == 1
}

// ── Sentinel errors ─────────────────────────────────────────────────────────

var (
	ErrDossierNotFound    = errors.New("company: no dossier for workspace")
	ErrInvalidArgument    = errors.New("company: invalid argument")
	ErrRecordNotFound     = errors.New("company: record not found")
	ErrUnknownFieldKey    = errors.New("company: unknown identity field key")
	ErrInvalidSOACategory = errors.New("company: SOA category must be OG1-OG13 or OS1-OS35")
	ErrInvalidClassifica  = errors.New("company: classifica must be I-VIII (incl. III-bis, IV-bis)")
)

// ── Shared normalisation helpers ────────────────────────────────────────────

// foldSpace lower-cases and collapses whitespace, for the case-insensitive
// substring matching the engine does on scopes and sections. It is not a
// general slug: it deliberately preserves punctuation, because an albo section
// like "5-bis" loses its meaning without the hyphen.
func foldSpace(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// shortHash is the bounded stand-in for a long free-text value inside a dedupe
// key. The key backs a btree unique index, which has a hard tuple-size limit, so
// verbatim requirement text — which routinely runs to several hundred
// characters — cannot be used directly.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}
