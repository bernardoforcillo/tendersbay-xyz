package espd

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

var (
	now      = time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	userID   = "11111111-1111-1111-1111-111111111111"
	statedAt = now.Add(-24 * time.Hour)
)

func ptr[T any](v T) *T { return &v }

func human() company.Attribution {
	return company.Attribution{Provenance: company.ProvenanceUserStated, StatedBy: userID, StatedAt: statedAt, PromptedBy: company.PromptOnboarding}
}

func inferred() company.Attribution {
	return company.Attribution{Provenance: company.ProvenanceAgentInferred, Confidence: ptr(0.8), StatedAt: statedAt, PromptedBy: company.PromptExtraction}
}

// completeDossier is a PMI with every Part answered by a human.
func completeDossier() company.Dossier {
	attr := map[company.FieldKey]company.Attribution{}
	for _, k := range []company.FieldKey{company.FieldLegalName, company.FieldVATNumber, company.FieldFiscalCode, company.FieldLegalForm, company.FieldCCIAA, company.FieldCountry, company.FieldNUTS, company.FieldIsSME} {
		attr[k] = human()
	}
	d := company.Dossier{
		WorkspaceID: "ws-1",
		Identity: company.Identity{
			LegalName: "Acme Costruzioni Srl", VATNumber: "IT01234567890", FiscalCode: "01234567890",
			LegalForm: company.LegalForm("srl"), CCIAAOffice: "MI", CCIAANumber: "1234567",
			Country: "IT", NUTS: "ITC4C", IsSME: true, Attribution: attr,
		},
		Representatives: []company.Representative{{ID: "rep-1", Role: "legale_rappresentante", GivenName: "Anna", FamilyName: "Rossi", Attribution: human()}},
		FinancialYears:  []company.FinancialYear{{Year: 2025, TurnoverMinor: ptr(int64(2_100_000_00)), Currency: "EUR", Headcount: ptr(int32(18)), Attribution: human()}},
		PastContracts:   []company.PastContract{{ID: "pc-1", Description: "Rifacimento copertura scuola", BuyerName: "Comune di Lodi", ValueMinor: ptr(int64(450_000_00)), Currency: "EUR", Role: company.ContractRole("principal"), Attribution: human()}},
		SOA:             []company.SOACategory{{ID: "soa-1", Category: "OG1", Classifica: company.ClassificaIII, Attribution: human()}},
		Certifications:  []company.Certification{{ID: "cert-1", Standard: company.CertISO9001, Scope: "edilizia", Attribution: human()}, {ID: "cert-2", Standard: company.CertISO14001, Attribution: human()}},
		Registrations:   []company.Registration{{ID: "reg-1", Kind: company.RegWhiteList, Authority: "Prefettura di Milano", Attribution: human()}},
		NationalGrounds: []company.NationalGround{{ID: "ng-1", Country: "IT", Criterion: "art94.c1", Answer: false, Attribution: human()}, {ID: "ng-2", Country: "DE", Criterion: "x", Answer: true, Attribution: human()}},
	}
	for _, k := range ExclusionCriteria() {
		d.Declarations = append(d.Declarations, company.Declaration{ID: "dec-" + string(k), Criterion: string(k), Answer: false, Attribution: human()})
	}
	return d
}

func confirmedInput(d company.Dossier) BidInput {
	return BidInput{
		Bid:          bid.Bid{ID: "bid-1", WorkbenchID: "wb-1", TenderID: 42},
		Data:         bid.EspdData{Lots: []bid.Lot{{ID: "lot-1", LotRef: "LOT-0001"}}},
		Confirmation: &bid.DeclarationConfirmation{BidID: "bid-1", UserID: userID, ConfirmedAt: now.Add(-time.Hour), DeclarationsHash: HashDeclarations(d.Declarations)},
		Procedure:    Procedure{BuyerName: "Comune di Milano", Title: "Manutenzione strade", Reference: "A0B1C2D3E4", Country: "IT"},
	}
}

func gapsByReason(r Response) map[GapReason]int {
	m := map[GapReason]int{}
	for _, g := range r.Gaps {
		m[g.Reason]++
	}
	return m
}

func hasGap(r Response, k CriterionKey, field string) bool {
	for _, g := range r.Gaps {
		if g.Criterion == k && (field == "" || g.Field == field) {
			return true
		}
	}
	return false
}

func hasLeaf(r Response, k CriterionKey, field string) bool {
	for _, l := range r.Leaves {
		if l.Criterion == k && l.Field == field {
			return true
		}
	}
	return false
}

func TestComposeCompleteDossierIsReady(t *testing.T) {
	d := completeDossier()
	r := Compose(d, confirmedInput(d), nil, now)
	if !r.Ready() {
		t.Fatalf("Ready() = false; gaps = %+v", r.Gaps)
	}
	ready, missing := r.Readiness()
	if ready == 0 || missing != 0 {
		t.Errorf("Readiness() = %d, %d", ready, missing)
	}
	if !r.Declarations.Complete() || !r.Declarations.Confirmed() {
		t.Error("declarations should be complete and confirmed")
	}
	for _, want := range []struct {
		k     CriterionKey
		field string
	}{
		{CritIdentity, "is_sme"}, {CritRepresentatives, "given_name"}, {CritEnrolmentTradeReg, "number"},
		{CritGeneralTurnover, "amount"}, {CritAverageAnnualManpower, "headcount"}, {CritReferences, "amount"},
		{CritSOAAttestation, "classifica"}, {CritQualityAssurance, "standard"}, {CritEnvironmentalManagement, "standard"},
		{CritOtherRegistration, "authority"}, {CritLots, "lot_ref"}, {CritProcedure, "reference"},
		{CritFraud, "answer"}, {CritReliance, "relies_on_other_entities"}, {CritSubcontracting, "intends_to_subcontract"},
	} {
		if !hasLeaf(r, want.k, want.field) {
			t.Errorf("missing leaf %s/%s", want.k, want.field)
		}
	}
	// III.D carries only the buyer country's grounds.
	if !hasLeaf(r, CriterionKey("iii.d.it.art94.c1"), "answer") || hasLeaf(r, CriterionKey("iii.d.de.x"), "answer") {
		t.Errorf("national grounds not filtered by the buyer country: %+v", r.Leaves)
	}
	if r.Request != nil {
		t.Error("Request must be nil when composed without one")
	}
	if !r.ComposedAt.Equal(now) {
		t.Error("ComposedAt must be the injected clock")
	}
}

// TestComposeNeverInfers is the mechanical form of the PRD rule: a fact whose
// provenance is agent_inferred or imported never becomes a leaf, whatever its
// confidence, and shows up as a NotAuthoritative gap the dossier must fix.
func TestComposeNeverInfers(t *testing.T) {
	d := completeDossier()
	d.FinancialYears[0].Attribution = inferred()
	d.Identity.Attribution[company.FieldVATNumber] = company.Attribution{Provenance: company.ProvenanceImported, Confidence: ptr(0.99)}
	d.Declarations[0].Attribution = inferred() // bypasses the write-time guard on purpose

	r := Compose(d, confirmedInput(d), nil, now)
	if r.Ready() {
		t.Fatal("a response with inferred facts must not be ready")
	}
	if hasLeaf(r, CritGeneralTurnover, "amount") || hasLeaf(r, CritAverageAnnualManpower, "headcount") {
		t.Error("an inferred esercizio leaked into the document")
	}
	if hasLeaf(r, CritIdentity, "vat_number") {
		t.Error("an imported VAT number leaked into the document")
	}
	if hasLeaf(r, ExclusionCriteria()[0], "answer") {
		t.Error("an inferred declaration leaked into the document")
	}
	if got := gapsByReason(r)[ReasonNotAuthoritative]; got != 3 {
		t.Errorf("NotAuthoritative gaps = %d, want 3: %+v", got, r.Gaps)
	}
	for _, g := range r.Gaps {
		if g.Reason == ReasonNotAuthoritative && g.Scope != ScopeCompany {
			t.Errorf("a NotAuthoritative gap must be fixed in the dossier, got scope %s", g.Scope)
		}
	}
	if r.Declarations.Complete() {
		t.Error("a set with a refused declaration is not complete")
	}
}

func TestComposeMissingDeclarationsAreCompanyGaps(t *testing.T) {
	d := completeDossier()
	d.Declarations = d.Declarations[:len(d.Declarations)-2]
	in := confirmedInput(d)
	r := Compose(d, in, nil, now)
	if r.Ready() {
		t.Fatal("not ready with unanswered exclusion grounds")
	}
	if !hasGap(r, CritEarlyTermination, "answer") || !hasGap(r, CritMisrepresentation, "answer") {
		t.Errorf("gaps = %+v", r.Gaps)
	}
	// While answers are missing, the confirmation step is not yet raised.
	if hasGap(r, CritDeclarationsConfirmation, "") {
		t.Error("confirmation gap raised before the set is complete")
	}
}

// TestComposeStaleConfirmation: a confirmation binds to the content hash of
// the declarations. Changing one answer afterwards makes it stale and the
// response carries a bid-scoped Stale gap — no cron, no flag, recomputed.
func TestComposeStaleConfirmation(t *testing.T) {
	d := completeDossier()
	in := confirmedInput(d)

	// No confirmation at all.
	none := in
	none.Confirmation = nil
	r := Compose(d, none, nil, now)
	if r.Ready() || !hasGap(r, CritDeclarationsConfirmation, "confirmation") {
		t.Fatalf("unconfirmed set must carry the Stale gap: %+v", r.Gaps)
	}
	if g := r.GapsIn(PartIIIA); len(g) != 1 || g[0].Scope != ScopeBid || g[0].Reason != ReasonStale {
		t.Errorf("gap = %+v, want one bid-scoped Stale gap in III.A", g)
	}

	// Confirmed, then a declaration changes.
	d.Declarations[3].Answer = true
	d.Declarations[3].SelfCleaning = "misure adottate"
	r = Compose(d, in, nil, now)
	if r.Ready() {
		t.Fatal("a changed declaration must invalidate the confirmation")
	}
	if !hasGap(r, CritDeclarationsConfirmation, "confirmation") || !hasLeaf(r, ExclusionCriteria()[3], "self_cleaning") {
		t.Errorf("gaps = %+v", r.Gaps)
	}

	// Re-saving an unchanged answer (new timestamp) does NOT invalidate.
	d2 := completeDossier()
	d2.Declarations[0].StatedAt = now
	if HashDeclarations(d2.Declarations) != HashDeclarations(completeDossier().Declarations) {
		t.Error("the hash must ignore attribution timestamps")
	}
}

func TestComposeWithoutRequestPartIComesFromTheBid(t *testing.T) {
	d := completeDossier()
	in := confirmedInput(d)
	in.Procedure = Procedure{Country: "IT"} // nothing known about the gara yet
	in.Data.Lots = nil
	r := Compose(d, in, nil, now)
	if !hasGap(r, CritProcedure, "buyer_name") || !hasGap(r, CritProcedure, "reference") {
		t.Errorf("Part I gaps = %+v", r.GapsIn(PartI))
	}
	for _, g := range r.GapsIn(PartI) {
		if g.Scope != ScopeBid {
			t.Errorf("Part I gap must be bid-scoped: %+v", g)
		}
	}
	if hasGap(r, CritLots, "") {
		t.Error("no lot gap without a request that declares lots")
	}
	// And without a request, Part IV has no gaps to be measured against.
	for _, p := range []Part{PartIVA, PartIVB, PartIVC, PartIVD} {
		if len(r.GapsIn(p)) != 0 {
			t.Errorf("Part %s gaps without a request: %+v", p, r.GapsIn(p))
		}
	}
}

func TestComposeWithRequestAsksForWhatTheBuyerAsked(t *testing.T) {
	d := completeDossier()
	d.Registrations = nil // no albo professionale
	in := confirmedInput(d)
	in.Data.Lots = nil
	req := &Request{
		Version: EDM211, BuyerName: "Comune di Milano", ProcedureReference: "CIG123", Country: "IT",
		Lots:     []string{"LOT-0001", "LOT-0002"},
		Criteria: []CriterionKey{CritFraud, CritGeneralTurnover, CritEnrolmentProfessionalReg, CritSpecificTurnover},
		SHA256:   "abc",
	}
	r := Compose(d, in, req, now)
	if r.Request != req {
		t.Fatal("Request must be carried on the response")
	}
	if !hasLeaf(r, CritProcedure, "buyer_name") || hasGap(r, CritProcedure, "") {
		t.Errorf("Part I from the request: leaves/gaps wrong: %+v", r.GapsIn(PartI))
	}
	if !hasGap(r, CritLots, "lot_ref") {
		t.Error("a request with lots and a bid with none must gap")
	}
	if !hasGap(r, CritEnrolmentProfessionalReg, "criterion") || !hasGap(r, CritSpecificTurnover, "criterion") {
		t.Errorf("requested criteria the dossier cannot answer must gap: %+v", r.Gaps)
	}
	if hasGap(r, CritGeneralTurnover, "") || hasGap(r, CritFraud, "criterion") {
		t.Errorf("answered criteria must not gap: %+v", r.Gaps)
	}
	for _, l := range r.Leaves {
		if l.Part == PartI && l.Source.Kind != SourceRequest {
			t.Errorf("Part I leaf must cite the request: %+v", l)
		}
	}
}

func TestComposeRepresentativeRequired(t *testing.T) {
	d := completeDossier()
	d.Representatives = nil
	r := Compose(d, confirmedInput(d), nil, now)
	if !hasGap(r, CritRepresentatives, "representative") {
		t.Errorf("gaps = %+v", r.Gaps)
	}
	d.Representatives = []company.Representative{{ID: "rep-2", Role: "procuratore", GivenName: "B", FamilyName: "C", Attribution: inferred()}}
	r = Compose(d, confirmedInput(d), nil, now)
	if g := r.GapsIn(PartIIB); len(g) != 1 || g[0].Reason != ReasonNotAuthoritative {
		t.Errorf("an imported representative must be a NotAuthoritative gap: %+v", g)
	}
}

func TestComposeIsSMEIsAskedNotDerived(t *testing.T) {
	d := completeDossier()
	delete(d.Identity.Attribution, company.FieldIsSME)
	d.Identity.IsSME = false
	r := Compose(d, confirmedInput(d), nil, now)
	if !hasGap(r, CritIdentity, "is_sme") || hasLeaf(r, CritIdentity, "is_sme") {
		t.Errorf("unanswered is_sme must be a gap, not a false: leaves/gaps %+v", r.GapsIn(PartIIA))
	}
}

func TestCriterionKeyPart(t *testing.T) {
	for k, want := range map[CriterionKey]Part{
		CritProcedure: PartI, CritLots: PartI, CritIdentity: PartIIA, CritRepresentatives: PartIIB,
		CritReliance: PartIIC, CritSubcontracting: PartIID, CritFraud: PartIIIA, CritPaymentTaxes: PartIIIB,
		CritBankruptcy: PartIIIC, CritPurelyNationalGrounds: PartIIID, CritDeclarationsConfirmation: PartIIIA,
		CritEnrolmentTradeReg: PartIVA, CritGeneralTurnover: PartIVB, CritReferences: PartIVC, CritQualityAssurance: PartIVD,
	} {
		if got := k.Part(); got != want {
			t.Errorf("%s.Part() = %s, want %s", k, got, want)
		}
	}
	if len(ExclusionCriteria()) != 23 {
		t.Errorf("ExclusionCriteria() has %d entries, want the 23 Art. 57 grounds", len(ExclusionCriteria()))
	}
}

func TestValueString(t *testing.T) {
	if got := AmountValue(2_100_000_05, "").String(); got != "2100000.05 EUR" {
		t.Errorf("amount = %q", got)
	}
	if BoolValue(true).String() != "yes" || BoolValue(false).String() != "no" || IntValue(7).String() != "7" {
		t.Error("bool/int rendering")
	}
	if got := DateValue(now).String(); got != "2026-09-04" {
		t.Errorf("date = %q", got)
	}
}
