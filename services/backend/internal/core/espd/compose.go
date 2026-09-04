package espd

import (
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// Procedure is the minimal Part I a bid needs when the buyer published no ESPD
// request: who is buying, what, and under which reference (the CIG in Italy).
// The service fills it from the tender the bid tracks; Compose only reads it.
type Procedure struct {
	BuyerName string
	Title     string
	Reference string // ContractFolderID / CIG
	NoticeRef string // TED publication number, "" when unknown
	Country   string // alpha-2 of the buyer, decides which national grounds apply
}

// BidInput is everything about ONE bid that Compose reads. It is a struct and
// not five parameters so the service assembles it once and the tests build it
// in one literal.
type BidInput struct {
	Bid          bid.Bid
	Data         bid.EspdData
	Confirmation *bid.DeclarationConfirmation // nil when never confirmed
	Procedure    Procedure
}

// Compose builds the document. It is a pure function: same inputs, same
// Response, no clock read (now is a parameter), no port call.
//
// The composition rule, part by part, is documented inline; the two rules that
// hold everywhere are:
//
//   - A dossier fact whose Attribution is not authoritative (agent_inferred,
//     imported) never becomes a Leaf. It becomes a Gap with
//     ReasonNotAuthoritative, in the company scope, so the fix is "confirm it in
//     the dossier" — once, for every future gara. This is the mechanical form
//     of the PRD's "never inferred".
//   - The Part III declarations must be complete AND confirmed for this bid
//     against their current content hash; otherwise Part III carries a Stale
//     gap in the bid scope.
//
// When req is nil the document is composed from the dossier alone (the PRD's
// Approach A): Part I is read from in.Procedure and Part IV carries no gaps,
// because without a request nobody knows which selection criteria the buyer
// will ask. When req is present, every criterion it requests and the dossier
// cannot answer is a Gap.
func Compose(d company.Dossier, in BidInput, req *Request, now time.Time) Response {
	c := &composer{now: now, req: req}

	c.partI(in, req)
	c.partIIA(d.Identity)
	c.partIIB(d.Representatives)
	c.partIIC(in.Data.Reliances)
	c.partIID(in.Data.Subcontractors)
	c.partIII(d, in)
	c.partIV(d, req)

	return Response{
		Leaves:       c.leaves,
		Gaps:         c.gaps,
		Request:      req,
		Declarations: c.declarations,
		ComposedAt:   now,
	}
}

type composer struct {
	now          time.Time
	req          *Request
	leaves       []Leaf
	gaps         []Gap
	declarations DeclarationSet
	// answered records which Part IV criteria have at least one leaf, so the
	// request-driven gap pass can see what the dossier covered.
	answered map[CriterionKey]bool
}

func (c *composer) leaf(l Leaf) {
	if c.answered == nil {
		c.answered = map[CriterionKey]bool{}
	}
	c.answered[l.Criterion] = true
	c.leaves = append(c.leaves, l)
}

func (c *composer) gap(k CriterionKey, field string, scope GapScope, reason GapReason) {
	c.gaps = append(c.gaps, Gap{Part: k.Part(), Criterion: k, Field: field, Scope: scope, Reason: reason})
}

// fact applies the authoritative-only rule to one dossier record: a leaf when
// the attribution is authoritative, a company-scoped NotAuthoritative gap
// otherwise. It returns whether the caller may emit further leaves for the
// same record — a record is admitted or refused as a whole, never per field.
func (c *composer) fact(k CriterionKey, field string, a company.Attribution) bool {
	if a.Authoritative() {
		return true
	}
	c.gap(k, field, ScopeCompany, ReasonNotAuthoritative)
	return false
}

func bidAttribution() company.Attribution {
	return company.Attribution{Provenance: company.ProvenanceUserStated}
}

func requestAttribution() company.Attribution {
	return company.Attribution{Provenance: company.ProvenanceImported}
}

// ── Part I ──────────────────────────────────────────────────────────────────

func (c *composer) partI(in BidInput, req *Request) {
	if req != nil {
		src := SourceRef{Kind: SourceRequest, ID: req.SHA256}
		for _, f := range []struct{ field, value string }{
			{"buyer_name", req.BuyerName}, {"title", req.ProcedureTitle},
			{"reference", req.ProcedureReference}, {"notice_ref", req.NoticeRef},
		} {
			if f.value != "" {
				c.leaf(Leaf{Part: PartI, Criterion: CritProcedure, Field: f.field, Value: TextValue(f.value), Attribution: requestAttribution(), Source: src})
			}
		}
		if req.BuyerName == "" {
			c.gap(CritProcedure, "buyer_name", ScopeBid, ReasonMissing)
		}
		if req.ProcedureReference == "" {
			c.gap(CritProcedure, "reference", ScopeBid, ReasonMissing)
		}
		c.lots(in.Data.Lots, len(req.Lots) > 0)
		return
	}

	p := in.Procedure
	src := SourceRef{Kind: SourceBid, ID: in.Bid.ID}
	for _, f := range []struct {
		field, value string
		required     bool
	}{
		{"buyer_name", p.BuyerName, true}, {"title", p.Title, false},
		{"reference", p.Reference, true}, {"notice_ref", p.NoticeRef, false},
	} {
		switch {
		case f.value != "":
			c.leaf(Leaf{Part: PartI, Criterion: CritProcedure, Field: f.field, Value: TextValue(f.value), Attribution: bidAttribution(), Source: src})
		case f.required:
			c.gap(CritProcedure, f.field, ScopeBid, ReasonMissing)
		}
	}
	c.lots(in.Data.Lots, false)
}

// lots emits one leaf per lot the bid tenders for. A gap is raised only when
// the buyer's request divides the procedure into lots and the bid names none:
// without a request, "no lots" is as likely "single-lot procedure" as
// "forgot", and Compose does not guess.
func (c *composer) lots(lots []bid.Lot, procedureHasLots bool) {
	for _, l := range lots {
		c.leaf(Leaf{Part: PartI, Criterion: CritLots, Field: "lot_ref", Value: TextValue(l.LotRef), Attribution: bidAttribution(), Source: SourceRef{Kind: SourceBid, ID: l.ID}})
	}
	if procedureHasLots && len(lots) == 0 {
		c.gap(CritLots, "lot_ref", ScopeBid, ReasonMissing)
	}
}

// ── Part II.A — identity ────────────────────────────────────────────────────

// identityField is one Part II.A scalar: how it is read off the Identity and
// whether the document needs it. Optional fields produce a leaf when attributed
// and nothing when not; required ones produce a Missing gap when nobody has
// ever asserted them.
type identityField struct {
	key      company.FieldKey
	field    string
	required bool
	value    func(company.Identity) (Value, bool) // ok=false: empty
}

var identityFields = []identityField{
	{company.FieldLegalName, "legal_name", true, func(id company.Identity) (Value, bool) { return TextValue(id.LegalName), id.LegalName != "" }},
	{company.FieldVATNumber, "vat_number", true, func(id company.Identity) (Value, bool) { return TextValue(id.VATNumber), id.VATNumber != "" }},
	{company.FieldFiscalCode, "fiscal_code", false, func(id company.Identity) (Value, bool) { return TextValue(id.FiscalCode), id.FiscalCode != "" }},
	{company.FieldLegalForm, "legal_form", false, func(id company.Identity) (Value, bool) { return CodeValue(string(id.LegalForm)), id.LegalForm != "" }},
	{company.FieldCountry, "country", true, func(id company.Identity) (Value, bool) { return CodeValue(id.Country), id.Country != "" }},
	{company.FieldNUTS, "nuts", false, func(id company.Identity) (Value, bool) { return CodeValue(id.NUTS), id.NUTS != "" }},
	{company.FieldIsSME, "is_sme", true, func(id company.Identity) (Value, bool) { return BoolValue(id.IsSME), true }},
}

func (c *composer) partIIA(id company.Identity) {
	src := SourceRef{Kind: SourceDossier, ID: "identity"}
	for _, f := range identityFields {
		attr, asserted := id.Attribution[f.key]
		v, present := f.value(id)
		switch {
		case !asserted || !present:
			// A value nobody stamped is treated as never asserted: the
			// attribution map, not the column, is the record of who said it.
			if f.required {
				c.gap(CritIdentity, f.field, ScopeCompany, ReasonMissing)
			}
		case !attr.Authoritative():
			c.gap(CritIdentity, f.field, ScopeCompany, ReasonNotAuthoritative)
		default:
			c.leaf(Leaf{Part: PartIIA, Criterion: CritIdentity, Field: f.field, Value: v, Attribution: attr, Source: src})
		}
	}
}

// ── Part II.B — representatives ─────────────────────────────────────────────

func (c *composer) partIIB(reps []company.Representative) {
	if len(reps) == 0 {
		c.gap(CritRepresentatives, "representative", ScopeCompany, ReasonMissing)
		return
	}
	for _, r := range reps {
		if !c.fact(CritRepresentatives, "representative", r.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: r.ID}
		add := func(field string, v Value) {
			c.leaf(Leaf{Part: PartIIB, Criterion: CritRepresentatives, Field: field, Value: v, Attribution: r.Attribution, Source: src})
		}
		add("role", TextValue(r.Role))
		add("given_name", TextValue(r.GivenName))
		add("family_name", TextValue(r.FamilyName))
		if r.BirthDate != nil {
			add("birth_date", DateValue(*r.BirthDate))
		}
		if r.BirthPlace != "" {
			add("birth_place", TextValue(r.BirthPlace))
		}
		if r.Address != "" {
			add("address", TextValue(r.Address))
		}
		if r.Email != "" {
			add("email", TextValue(r.Email))
		}
		add("power_of_attorney", BoolValue(r.PowerOfAttorney))
	}
}

// ── Part II.C / II.D — other entities ───────────────────────────────────────

func (c *composer) partIIC(rels []bid.Reliance) {
	c.leaf(Leaf{Part: PartIIC, Criterion: CritReliance, Field: "relies_on_other_entities", Value: BoolValue(len(rels) > 0), Attribution: bidAttribution(), Source: SourceRef{Kind: SourceBid}})
	for _, r := range rels {
		src := SourceRef{Kind: SourceBid, ID: r.ID}
		c.leaf(Leaf{Part: PartIIC, Criterion: CritReliance, Field: "entity_name", Value: TextValue(r.EntityName), Attribution: bidAttribution(), Source: src})
		c.leaf(Leaf{Part: PartIIC, Criterion: CritReliance, Field: "vat", Value: TextValue(r.VAT), Attribution: bidAttribution(), Source: src})
		c.leaf(Leaf{Part: PartIIC, Criterion: CritReliance, Field: "criterion", Value: CodeValue(r.Criterion), Attribution: bidAttribution(), Source: src})
	}
}

func (c *composer) partIID(subs []bid.Subcontractor) {
	c.leaf(Leaf{Part: PartIID, Criterion: CritSubcontracting, Field: "intends_to_subcontract", Value: BoolValue(len(subs) > 0), Attribution: bidAttribution(), Source: SourceRef{Kind: SourceBid}})
	for _, s := range subs {
		src := SourceRef{Kind: SourceBid, ID: s.ID}
		c.leaf(Leaf{Part: PartIID, Criterion: CritSubcontracting, Field: "name", Value: TextValue(s.Name), Attribution: bidAttribution(), Source: src})
		c.leaf(Leaf{Part: PartIID, Criterion: CritSubcontracting, Field: "vat", Value: TextValue(s.VAT), Attribution: bidAttribution(), Source: src})
		if s.Country != "" {
			c.leaf(Leaf{Part: PartIID, Criterion: CritSubcontracting, Field: "country", Value: CodeValue(s.Country), Attribution: bidAttribution(), Source: src})
		}
		if s.Share != nil {
			c.leaf(Leaf{Part: PartIID, Criterion: CritSubcontracting, Field: "share_pct", Value: IntValue(int64(*s.Share)), Attribution: bidAttribution(), Source: src})
		}
	}
}

// ── Part III — exclusion grounds ────────────────────────────────────────────

func (c *composer) partIII(d company.Dossier, in BidInput) {
	set := DeclarationSet{Hash: HashDeclarations(d.Declarations), Confirmation: in.Confirmation}
	for _, k := range ExclusionCriteria() {
		dec, ok := d.DeclarationFor(string(k))
		if !ok {
			set.Answers = append(set.Answers, Answer{Criterion: k})
			c.gap(k, "answer", ScopeCompany, ReasonMissing)
			continue
		}
		// company.Declaration.validate already refuses a non-authoritative
		// declaration at write time; the check is repeated here because Compose
		// is the last line and must not trust the store.
		if !c.fact(k, "answer", dec.Attribution) {
			set.Answers = append(set.Answers, Answer{Criterion: k})
			continue
		}
		set.Answers = append(set.Answers, Answer{Criterion: k, Answered: true, Applies: dec.Answer, SelfCleaning: dec.SelfCleaning, Attribution: dec.Attribution})
		src := SourceRef{Kind: SourceDeclaration, ID: dec.ID}
		c.leaf(Leaf{Part: k.Part(), Criterion: k, Field: "answer", Value: BoolValue(dec.Answer), Attribution: dec.Attribution, Source: src})
		if dec.Answer && strings.TrimSpace(dec.SelfCleaning) != "" {
			c.leaf(Leaf{Part: k.Part(), Criterion: k, Field: "self_cleaning", Value: TextValue(dec.SelfCleaning), Attribution: dec.Attribution, Source: src})
		}
	}

	// III.D — national grounds of the buyer's Member State. The list of
	// grounds is national law, which this package does not encode, so there is
	// no Missing gap here: only what the operator declared is carried.
	country := in.Procedure.Country
	if c.req != nil && c.req.Country != "" {
		country = c.req.Country
	}
	if country == "" {
		country = d.Identity.Country
	}
	for _, g := range d.NationalGrounds {
		if country != "" && g.Country != country {
			continue
		}
		k := CriterionKey(string(critUnknownNationalGroundBase) + strings.ToLower(g.Country) + "." + g.Criterion)
		if !c.fact(k, "answer", g.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDeclaration, ID: g.ID}
		c.leaf(Leaf{Part: PartIIID, Criterion: k, Field: "answer", Value: BoolValue(g.Answer), Attribution: g.Attribution, Source: src})
		if strings.TrimSpace(g.Note) != "" {
			c.leaf(Leaf{Part: PartIIID, Criterion: k, Field: "note", Value: TextValue(g.Note), Attribution: g.Attribution, Source: src})
		}
	}

	// The confirmation gap is raised only once the set is complete: while
	// answers are missing, "confirm your answers" is not yet the next step.
	if set.Complete() && !set.Confirmed() {
		c.gap(CritDeclarationsConfirmation, "confirmation", ScopeBid, ReasonStale)
	}
	c.declarations = set
}

// ── Part IV — selection criteria ────────────────────────────────────────────

func (c *composer) partIV(d company.Dossier, req *Request) {
	// IV.A — registrations. The chamber-of-commerce identity is itself an
	// enrolment in the trade register.
	if attr, ok := d.Identity.Attribution[company.FieldCCIAA]; ok && d.Identity.CCIAANumber != "" {
		if c.fact(CritEnrolmentTradeReg, "cciaa", attr) {
			src := SourceRef{Kind: SourceDossier, ID: "identity"}
			c.leaf(Leaf{Part: PartIVA, Criterion: CritEnrolmentTradeReg, Field: "register", Value: CodeValue("cciaa"), Attribution: attr, Source: src})
			c.leaf(Leaf{Part: PartIVA, Criterion: CritEnrolmentTradeReg, Field: "office", Value: TextValue(d.Identity.CCIAAOffice), Attribution: attr, Source: src})
			c.leaf(Leaf{Part: PartIVA, Criterion: CritEnrolmentTradeReg, Field: "number", Value: TextValue(d.Identity.CCIAANumber), Attribution: attr, Source: src})
		}
	}
	for _, r := range d.Registrations {
		k := CritOtherRegistration
		switch r.Kind {
		case company.RegCCIAA:
			k = CritEnrolmentTradeReg
		case company.RegAlboProfessionale:
			k = CritEnrolmentProfessionalReg
		}
		if !c.fact(k, "registration", r.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: r.ID}
		c.leaf(Leaf{Part: PartIVA, Criterion: k, Field: "register", Value: CodeValue(string(r.Kind)), Attribution: r.Attribution, Source: src})
		if r.Authority != "" {
			c.leaf(Leaf{Part: PartIVA, Criterion: k, Field: "authority", Value: TextValue(r.Authority), Attribution: r.Attribution, Source: src})
		}
		if r.Identifier != "" {
			c.leaf(Leaf{Part: PartIVA, Criterion: k, Field: "number", Value: TextValue(r.Identifier), Attribution: r.Attribution, Source: src})
		}
		if r.Section != "" {
			c.leaf(Leaf{Part: PartIVA, Criterion: k, Field: "section", Value: TextValue(r.Section), Attribution: r.Attribution, Source: src})
		}
		if r.ValidUntil != nil {
			c.leaf(Leaf{Part: PartIVA, Criterion: k, Field: "valid_until", Value: DateValue(*r.ValidUntil), Attribution: r.Attribution, Source: src})
		}
	}

	// IV.B — turnover, IV.C — headcount, per esercizio.
	for _, fy := range d.FinancialYears {
		if !c.fact(CritGeneralTurnover, "financial_year", fy.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: "fy-" + itoa(int64(fy.Year))}
		if fy.TurnoverMinor != nil {
			c.leaf(Leaf{Part: PartIVB, Criterion: CritGeneralTurnover, Field: "year", Value: IntValue(int64(fy.Year)), Attribution: fy.Attribution, Source: src})
			c.leaf(Leaf{Part: PartIVB, Criterion: CritGeneralTurnover, Field: "amount", Value: AmountValue(*fy.TurnoverMinor, fy.Currency), Attribution: fy.Attribution, Source: src})
		}
		if fy.SpecificTurnoverMinor != nil {
			c.leaf(Leaf{Part: PartIVB, Criterion: CritSpecificTurnover, Field: "year", Value: IntValue(int64(fy.Year)), Attribution: fy.Attribution, Source: src})
			c.leaf(Leaf{Part: PartIVB, Criterion: CritSpecificTurnover, Field: "amount", Value: AmountValue(*fy.SpecificTurnoverMinor, fy.Currency), Attribution: fy.Attribution, Source: src})
		}
		if fy.Headcount != nil {
			c.leaf(Leaf{Part: PartIVC, Criterion: CritAverageAnnualManpower, Field: "year", Value: IntValue(int64(fy.Year)), Attribution: fy.Attribution, Source: src})
			c.leaf(Leaf{Part: PartIVC, Criterion: CritAverageAnnualManpower, Field: "headcount", Value: IntValue(int64(*fy.Headcount)), Attribution: fy.Attribution, Source: src})
		}
	}

	// IV.C — references and SOA.
	for _, p := range d.PastContracts {
		if !c.fact(CritReferences, "reference", p.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: p.ID}
		add := func(field string, v Value) {
			c.leaf(Leaf{Part: PartIVC, Criterion: CritReferences, Field: field, Value: v, Attribution: p.Attribution, Source: src})
		}
		add("description", TextValue(p.Description))
		if p.BuyerName != "" {
			add("recipient", TextValue(p.BuyerName))
		}
		if v := p.EffectiveValueMinor(); v != nil {
			add("amount", AmountValue(*v, p.Currency))
		}
		if p.StartedOn != nil {
			add("started_on", DateValue(*p.StartedOn))
		}
		if p.EndedOn != nil {
			add("ended_on", DateValue(*p.EndedOn))
		}
		add("role", CodeValue(string(p.Role)))
	}
	for _, s := range d.SOA {
		if !c.fact(CritSOAAttestation, "soa", s.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: s.ID}
		c.leaf(Leaf{Part: PartIVC, Criterion: CritSOAAttestation, Field: "category", Value: CodeValue(s.Category), Attribution: s.Attribution, Source: src})
		c.leaf(Leaf{Part: PartIVC, Criterion: CritSOAAttestation, Field: "classifica", Value: CodeValue(s.Classifica.String()), Attribution: s.Attribution, Source: src})
		if s.IssuedBy != "" {
			c.leaf(Leaf{Part: PartIVC, Criterion: CritSOAAttestation, Field: "issued_by", Value: TextValue(s.IssuedBy), Attribution: s.Attribution, Source: src})
		}
		if s.ValidUntil != nil {
			c.leaf(Leaf{Part: PartIVC, Criterion: CritSOAAttestation, Field: "valid_until", Value: DateValue(*s.ValidUntil), Attribution: s.Attribution, Source: src})
		}
	}

	// IV.D — certificates.
	for _, cert := range d.Certifications {
		k := CritQualityAssurance
		switch cert.Standard {
		case company.CertISO14001, company.CertEMAS, company.CertISO50001:
			k = CritEnvironmentalManagement
		}
		if !c.fact(k, "certificate", cert.Attribution) {
			continue
		}
		src := SourceRef{Kind: SourceDossier, ID: cert.ID}
		name := string(cert.Standard)
		if cert.Standard == company.CertOtherStd {
			name = cert.StandardRaw
		}
		c.leaf(Leaf{Part: PartIVD, Criterion: k, Field: "standard", Value: CodeValue(name), Attribution: cert.Attribution, Source: src})
		if cert.Scope != "" {
			c.leaf(Leaf{Part: PartIVD, Criterion: k, Field: "scope", Value: TextValue(cert.Scope), Attribution: cert.Attribution, Source: src})
		}
		if cert.IssuedBy != "" {
			c.leaf(Leaf{Part: PartIVD, Criterion: k, Field: "issued_by", Value: TextValue(cert.IssuedBy), Attribution: cert.Attribution, Source: src})
		}
		if cert.ValidUntil != nil {
			c.leaf(Leaf{Part: PartIVD, Criterion: k, Field: "valid_until", Value: DateValue(*cert.ValidUntil), Attribution: cert.Attribution, Source: src})
		}
	}

	// Request-driven gaps: every selection criterion the buyer asks for that
	// the dossier left unanswered. Without a request there is nothing to be
	// missing against.
	if req == nil {
		return
	}
	for _, k := range req.Criteria {
		if IsExclusionCriterion(k) || k.Part() == PartI || k.Part() == PartIIA || k.Part() == PartIIB || k.Part() == PartIIC || k.Part() == PartIID {
			continue
		}
		if !c.answered[k] && !c.alreadyGapped(k) {
			c.gap(k, "criterion", ScopeCompany, ReasonMissing)
		}
	}
}

func (c *composer) alreadyGapped(k CriterionKey) bool {
	for _, g := range c.gaps {
		if g.Criterion == k {
			return true
		}
	}
	return false
}

func itoa(n int64) string {
	return IntValue(n).String()
}
