package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

func ptr[T any](v T) *T { return &v }

func mustDay(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// ── get_company_profile ──

func TestGetCompanyProfileTool_EmptyDossierCarriesTheDoNotInventNotice(t *testing.T) {
	tool := newGetCompanyProfileTool(&turnState{}, func() (company.Dossier, error) {
		return company.Dossier{WorkspaceID: "ws-1"}, nil
	})
	out, err := tool.Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res getCompanyProfileResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if !res.Empty {
		t.Fatal("empty = false for a dossier nobody has written to")
	}
	if !strings.Contains(res.Notice, "Do NOT invent company facts") {
		t.Fatalf("notice = %q, want the do-not-invent steering", res.Notice)
	}
	// The three record lists must serialise as empty arrays and not as null: a
	// null reads as "the field did not apply", which is a different claim from
	// "we hold none of these".
	if !strings.Contains(out, `"soa":[]`) || !strings.Contains(out, `"financial_years":[]`) {
		t.Fatalf("empty lists must render as [], got %s", out)
	}
}

func TestGetCompanyProfileTool_RendersRecordsWithProvenance(t *testing.T) {
	validUntil := mustDay(t, "2029-04-30")
	verifiedUntil := mustDay(t, "2026-04-30")
	dossier := company.Dossier{
		WorkspaceID: "ws-1",
		Identity: company.Identity{
			LegalName: "ACME S.r.l.", VATNumber: "01234567891",
			CCIAAOffice: "MI", CCIAANumber: "1234567",
			Attribution: map[company.FieldKey]company.Attribution{
				company.FieldLegalName: {Provenance: company.ProvenanceUserStated},
			},
		},
		SOA: []company.SOACategory{{
			Category: "OG1", Classifica: company.ClassificaI,
			ValidUntil: &validUntil, VerifiedUntil: &verifiedUntil,
			Attribution: company.Attribution{Provenance: company.ProvenanceUserConfirmed},
		}},
		FinancialYears: []company.FinancialYear{
			{Year: 2024, TurnoverMinor: ptr(int64(2_100_000_00)), Currency: "EUR",
				Attribution: company.Attribution{Provenance: company.ProvenanceUserStated}},
			{Year: 2023, Attribution: company.Attribution{Provenance: company.ProvenanceUserStated}},
			{Year: 2022, Attribution: company.Attribution{Provenance: company.ProvenanceUserStated}},
			{Year: 2021, Attribution: company.Attribution{Provenance: company.ProvenanceUserStated}},
		},
		PastContracts: []company.PastContract{{
			Description: "Manutenzione strade", CPV: "45233",
			ValueMinor: ptr(int64(5_000_000_00)), Currency: "EUR",
			Role: company.RoleRTIMandante, SharePct: ptr(20.0),
			Attribution: company.Attribution{Provenance: company.ProvenanceUserStated},
		}},
		Registrations: []company.Registration{{
			Kind: company.RegWhiteList, Authority: "Prefettura di Milano",
			Identifier: "WL-99887", Section: "movimento terra",
			Attribution: company.Attribution{Provenance: company.ProvenanceUserStated},
		}},
	}
	tool := newGetCompanyProfileTool(&turnState{}, func() (company.Dossier, error) { return dossier, nil })
	out, err := tool.Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res getCompanyProfileResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if res.Empty {
		t.Fatal("empty = true for a populated dossier")
	}
	if res.Identity.CCIAA != "MI/1234567" {
		t.Fatalf("cciaa = %q, want the sigla and REA together", res.Identity.CCIAA)
	}
	if res.Identity.Provenance["legal_name"] != "user_stated" {
		t.Fatalf("identity provenance = %v", res.Identity.Provenance)
	}
	if len(res.SOA) != 1 || res.SOA[0].VerifiedUntil != "2026-04-30" {
		t.Fatalf("soa = %+v, want the triennial verification date carried", res.SOA)
	}
	if res.SOA[0].Provenance != "user_confirmed" {
		t.Fatalf("soa provenance = %q", res.SOA[0].Provenance)
	}
	// Only the window Italian selection criteria actually name.
	if len(res.FinancialYears) != companyProfileFinancialYearLimit {
		t.Fatalf("financial_years = %d, want %d", len(res.FinancialYears), companyProfileFinancialYearLimit)
	}
	// A 20% mandante on a €5M contract carries €1M of reference, not €5M, and
	// the model must not have to derive that itself.
	if got := res.PastContracts[0].EffectiveValueMinor; got == nil || *got != 1_000_000_00 {
		t.Fatalf("effective_value_minor = %v, want 100000000", got)
	}
	// The registration NUMBER must never reach the model — there is no field for
	// it, and this asserts nobody added one.
	if strings.Contains(out, "WL-99887") {
		t.Fatalf("the registration identifier leaked into the tool result: %s", out)
	}
}

// ── check_eligibility ──

func executeCheckEligibility(t *testing.T, a company.Assessment) checkEligibilityResult {
	t.Helper()
	tool := newCheckEligibilityTool(&turnState{}, func(int64, string) (company.Assessment, error) { return a, nil })
	out, err := tool.Execute(context.Background(), `{"tender_id":"545620"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res checkEligibilityResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	return res
}

// TestCheckEligibilityTool_NoRequirementsIsNotAPass is the failure this whole
// phase exists to prevent: an empty gap list must read as "we know nothing about
// this gara's requisiti", never as "the company qualifies".
func TestCheckEligibilityTool_NoRequirementsIsNotAPass(t *testing.T) {
	res := executeCheckEligibility(t, company.Assessment{
		TenderID: 545620, Verdict: company.VerdictInsufficientData,
		DocumentCoverage: document.CoverageNotice, DocumentReason: document.ReasonBodyNotRetrieved,
	})
	if res.Verdict != "insufficient_data" {
		t.Fatalf("verdict = %q", res.Verdict)
	}
	if len(res.Gaps) != 0 {
		t.Fatalf("gaps = %+v, want none", res.Gaps)
	}
	if !strings.Contains(res.Notice, "NO eligibility judgement is possible") {
		t.Fatalf("notice = %q, want the insufficient-data steering", res.Notice)
	}
	if !strings.Contains(res.Notice, "nobody and nothing has read its requisiti section") {
		t.Fatalf("notice = %q, want the documents-unread steering alongside it", res.Notice)
	}
	if res.DocumentCoverage != "notice_only" || res.DocumentReason != "body_not_retrieved" {
		t.Fatalf("coverage=%q reason=%q, want both carried verbatim", res.DocumentCoverage, res.DocumentReason)
	}
}

// TestCheckEligibilityTool_ThreeAnswersStayDistinguishable is the core contract:
// a blocking gap, a non-blocking gap and "unknown — nobody has told us" must
// reach the model as three different things, not two.
func TestCheckEligibilityTool_ThreeAnswersStayDistinguishable(t *testing.T) {
	confirmed := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	blocking := company.Requirement{
		Kind: company.RequirementSOA, Text: "SOA OG1 classifica III", Blocking: true,
		Source: company.RequirementUserStated, ConfirmedBy: "user-1", ConfirmedAt: &confirmed,
	}
	premiante := company.Requirement{
		Kind: company.RequirementCertification, Text: "ISO 14001 (criterio premiante)", Blocking: false,
		Source:   company.RequirementDocumentExcerpt,
		Citation: &document.Citation{DocumentURL: "https://example.test/disciplinare.pdf"},
		Excerpt:  "punteggio aggiuntivo per operatori certificati ISO 14001",
	}
	unknown := company.Requirement{
		Kind: company.RequirementTurnover, Text: "fatturato globale triennio", Blocking: true,
		Source: company.RequirementUserStated, ConfirmedBy: "user-1", ConfirmedAt: &confirmed,
	}

	res := executeCheckEligibility(t, company.Assessment{
		TenderID: 545620,
		Gaps: []company.Gap{
			{Requirement: blocking, Status: company.GapShortfall, Blocking: true, Held: "OG1 I",
				Remedy: company.Remedy{Kind: company.RemedyAvvalimento}},
			{Requirement: unknown, Status: company.GapUnknown,
				Remedy:   company.Remedy{Kind: company.RemedyProvideFact},
				Question: &company.CaptureQuestion{Prompt: "Qual è il tuo fatturato degli ultimi tre esercizi?"}},
			{Requirement: premiante, Status: company.GapMissing, Blocking: false, Held: ""},
		},
		Verdict:                   company.VerdictNoGo,
		RequirementCount:          3,
		AuthoritativeRequirements: 2,
		BlockingGapCount:          1,
		UnknownGapCount:           1,
		DocumentCoverage:          document.CoverageFull,
	})

	if len(res.Gaps) != 3 {
		t.Fatalf("gaps = %d, want 3", len(res.Gaps))
	}
	// 1. blocking
	if res.Gaps[0].Status != "shortfall" || !res.Gaps[0].Blocking || res.Gaps[0].Held != "OG1 I" {
		t.Fatalf("gap[0] = %+v, want the blocking shortfall with its held fact", res.Gaps[0])
	}
	if res.Gaps[0].Remedy != "avvalimento" {
		t.Fatalf("gap[0].remedy = %q, want the remedy that makes it solvable", res.Gaps[0].Remedy)
	}
	// 2. unknown — an action, not an outcome
	if res.Gaps[1].Status != "unknown" || res.Gaps[1].Question == "" || res.Gaps[1].Blocking {
		t.Fatalf("gap[1] = %+v, want an unknown carrying its question and NOT marked blocking", res.Gaps[1])
	}
	// 3. unmet but non-blocking — costs points, not admission
	if res.Gaps[2].Status != "missing" || res.Gaps[2].Blocking {
		t.Fatalf("gap[2] = %+v, want an unmet criterio premiante", res.Gaps[2])
	}
	if res.Gaps[2].Confirmed {
		t.Fatal("an unconfirmed document extraction must report confirmed=false")
	}
	if res.Gaps[2].DocumentURL == "" || res.Gaps[2].Excerpt == "" {
		t.Fatalf("gap[2] = %+v, want the quote and its document so the user can check it", res.Gaps[2])
	}

	if !strings.Contains(res.Notice, "record_company_fact") {
		t.Fatalf("notice = %q, want the ask-don't-guess steering", res.Notice)
	}
	if !strings.Contains(res.Notice, "UNCONFIRMED") {
		t.Fatalf("notice = %q, want the unconfirmed-extraction steering", res.Notice)
	}
	if !strings.Contains(res.Notice, "Report the blocking FACT first") {
		t.Fatalf("notice = %q, want the lead-with-the-fact steering", res.Notice)
	}
}

// TestCheckEligibilityTool_HeldRendersEvenWhenEmpty pins the missing omitempty:
// an omitted key is a fact the model has to infer, and the inference it reaches
// for is "the field did not apply".
func TestCheckEligibilityTool_HeldRendersEvenWhenEmpty(t *testing.T) {
	tool := newCheckEligibilityTool(&turnState{}, func(int64, string) (company.Assessment, error) {
		return company.Assessment{
			TenderID: 1,
			Gaps: []company.Gap{{
				Requirement: company.Requirement{Kind: company.RequirementOther, Text: "leggi §12"},
				Status:      company.GapUnknown,
			}},
			RequirementCount: 1,
			Verdict:          company.VerdictInsufficientData,
		}, nil
	})
	out, err := tool.Execute(context.Background(), `{"tender_id":"1"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, `"held":""`) {
		t.Fatalf("held must render as an explicit empty string, got %s", out)
	}
}

func TestCheckEligibilityTool_RejectsGarbledTenderID(t *testing.T) {
	tool := newCheckEligibilityTool(&turnState{}, func(int64, string) (company.Assessment, error) {
		t.Fatal("the port must not be reached for an unparseable id")
		return company.Assessment{}, nil
	})
	if _, err := tool.Execute(context.Background(), `{"tender_id":"545620-2026"}`); err == nil {
		t.Fatal("Execute accepted a non-numeric tender_id")
	}
}

// ── record_tender_requirement ──

func TestRecordTenderRequirementTool_BuildsSOAPayloadAndStaysUnconfirmed(t *testing.T) {
	var got company.Requirement
	var gotTender int64
	tool := newRecordTenderRequirementTool(&turnState{}, func(tenderID int64, r company.Requirement) error {
		gotTender, got = tenderID, r
		return nil
	})
	out, err := tool.Execute(context.Background(), `{
		"tender_id":"545620","kind":"soa","requirement_text":"Attestazione SOA OG1 classifica III",
		"excerpt":"§7.1 … SOA OG1 cl. III …","document_url":"https://example.test/disciplinare.pdf",
		"detail":"{\"category\":\"og 1\",\"classifica\":\"III\"}"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTender != 545620 {
		t.Fatalf("tender id = %d", gotTender)
	}
	if got.SOA == nil || got.SOA.Classifica != company.ClassificaIII {
		t.Fatalf("soa payload = %+v", got.SOA)
	}
	// The category is normalised through the domain's own parser, so "og 1" and
	// "OG1" cannot become two different requirements.
	if got.SOA.Category != "og 1" && got.SOA.Category != "OG1" {
		t.Fatalf("category = %q", got.SOA.Category)
	}
	if got.Source != company.RequirementDocumentExcerpt {
		t.Fatalf("source = %q, want document_excerpt — an agent cannot state a requirement on its own authority", got.Source)
	}
	if got.ConfirmedBy != "" {
		t.Fatalf("confirmed_by = %q, want an unconfirmed extraction", got.ConfirmedBy)
	}
	// Blocking defaults to TRUE: an unclassified requirement treated as blocking
	// produces a question, treated as non-blocking produces a silent pass.
	if !got.Blocking {
		t.Fatal("blocking defaulted to false")
	}
	// Prevalente defaults to TRUE, which offers avvalimento rather than telling
	// the user they may subcontract a category they may not.
	if !got.SOA.Prevalente {
		t.Fatal("prevalente defaulted to false")
	}
	if got.Citation == nil || got.Citation.DocumentURL == "" {
		t.Fatal("the document the quote came from was dropped")
	}
	if !strings.Contains(out, "UNCONFIRMED") {
		t.Fatalf("result = %s, want the unconfirmed steering", out)
	}
}

func TestRecordTenderRequirementTool_HonoursExplicitBlockingFalse(t *testing.T) {
	var got company.Requirement
	tool := newRecordTenderRequirementTool(&turnState{}, func(_ int64, r company.Requirement) error {
		got = r
		return nil
	})
	_, err := tool.Execute(context.Background(), `{
		"tender_id":"1","kind":"certification","requirement_text":"ISO 14001 premiante","blocking":false,
		"detail":"{\"standard\":\"iso_14001\"}"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Blocking {
		t.Fatal("an explicit blocking=false must survive")
	}
}

// TestRecordTenderRequirementTool_OtherNeedsNoDetail keeps the un-modellable
// case usable: a requirement the engine cannot model must be visible as
// un-modelled, not rejected and therefore silently absent.
func TestRecordTenderRequirementTool_OtherNeedsNoDetail(t *testing.T) {
	var got company.Requirement
	tool := newRecordTenderRequirementTool(&turnState{}, func(_ int64, r company.Requirement) error {
		got = r
		return nil
	})
	_, err := tool.Execute(context.Background(),
		`{"tender_id":"1","kind":"other","requirement_text":"Sopralluogo obbligatorio entro il 10/09"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Kind != company.RequirementOther || got.Text == "" {
		t.Fatalf("requirement = %+v", got)
	}
}

func TestRecordTenderRequirementTool_RejectsMalformedInput(t *testing.T) {
	fail := func(int64, company.Requirement) error {
		t.Fatal("the port must not be reached for a malformed requirement")
		return nil
	}
	cases := []struct {
		name string
		args string
	}{
		{"unknown kind", `{"tender_id":"1","kind":"soa_bis","requirement_text":"x"}`},
		{"empty text", `{"tender_id":"1","kind":"other","requirement_text":"   "}`},
		{"unknown classifica", `{"tender_id":"1","kind":"soa","requirement_text":"x","detail":"{\"category\":\"OG1\",\"classifica\":\"IX\"}"}`},
		{"bad soa category", `{"tender_id":"1","kind":"soa","requirement_text":"x","detail":"{\"category\":\"OG99\",\"classifica\":\"III\"}"}`},
		{"detail is not json", `{"tender_id":"1","kind":"soa","requirement_text":"x","detail":"category=OG1"}`},
		{"unknown standard", `{"tender_id":"1","kind":"certification","requirement_text":"x","detail":"{\"standard\":\"iso_0000\"}"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool := newRecordTenderRequirementTool(&turnState{}, fail)
			if _, err := tool.Execute(context.Background(), c.args); err == nil {
				t.Fatalf("Execute accepted %s", c.args)
			}
		})
	}
}

// ── record_company_fact ──

func TestRecordCompanyFactTool_ParsesEachRecordKind(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value string
		check func(t *testing.T, f company.Fact)
	}{
		{
			name: "soa", field: "soa",
			value: `{"category":"OG1","classifica":"III","valid_until":"2029-04-30","verified_until":"2026-04-30"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.SOA == nil || f.SOA.Classifica != company.ClassificaIII {
					t.Fatalf("soa = %+v", f.SOA)
				}
				if f.SOA.VerifiedUntil == nil {
					t.Fatal("the triennial verification date was dropped")
				}
			},
		},
		{
			name: "certification", field: "certification",
			value: `{"standard":"iso_9001","scope":"costruzioni","valid_until":"2027-06-30T00:00:00Z"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.Certification == nil || f.Certification.Standard != company.CertISO9001 {
					t.Fatalf("certification = %+v", f.Certification)
				}
				if f.Certification.Scope != "costruzioni" {
					t.Fatal("a certificate is scope-bounded and the scope was dropped")
				}
			},
		},
		{
			name: "turnover", field: "turnover",
			value: `{"year":2024,"turnover_minor":210000000,"specific_turnover_minor":90000000,"currency":"EUR"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.FinancialYear == nil || f.FinancialYear.Year != 2024 {
					t.Fatalf("financial year = %+v", f.FinancialYear)
				}
				if f.FinancialYear.SpecificTurnoverMinor == nil {
					t.Fatal("fatturato specifico is the figure most requisiti name and it was dropped")
				}
			},
		},
		{
			name: "headcount", field: "headcount",
			value: `{"year":2024,"headcount":18}`,
			check: func(t *testing.T, f company.Fact) {
				if f.FinancialYear == nil || f.FinancialYear.Headcount == nil || *f.FinancialYear.Headcount != 18 {
					t.Fatalf("financial year = %+v", f.FinancialYear)
				}
				// Turnover must stay UNSTATED here rather than becoming zero — the
				// merge against what is already on file is what fills it in.
				if f.FinancialYear.TurnoverMinor != nil {
					t.Fatal("a headcount answer must not assert a turnover of zero")
				}
			},
		},
		{
			name: "past_contract", field: "past_contract",
			value: `{"description":"Strade","cpv":"45233","value_minor":500000000,"currency":"EUR","role":"rti_mandante","share_pct":20,"ended_on":"2024-05-30"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.PastContract == nil || f.PastContract.Role != company.RoleRTIMandante {
					t.Fatalf("past contract = %+v", f.PastContract)
				}
				if eff := f.PastContract.EffectiveValueMinor(); eff == nil || *eff != 100_000_000 {
					t.Fatalf("effective value = %v, want the company's own 20%% share", eff)
				}
			},
		},
		{
			name: "registration", field: "registration",
			value: `{"kind":"white_list","authority":"Prefettura di Milano","valid_until":"2027-01-31"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.Registration == nil || f.Registration.Kind != company.RegWhiteList {
					t.Fatalf("registration = %+v", f.Registration)
				}
			},
		},
		{
			name: "identity scalar", field: "cciaa",
			value: `{"value":"MI/1234567"}`,
			check: func(t *testing.T, f company.Fact) {
				if f.Identity == nil || f.Identity.Key != company.FieldCCIAA || f.Identity.Value != "MI/1234567" {
					t.Fatalf("identity = %+v", f.Identity)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v companyFactValueArgs
			if err := json.Unmarshal([]byte(c.value), &v); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			f, err := buildCompanyFact(c.field, v)
			if err != nil {
				t.Fatalf("buildCompanyFact: %v", err)
			}
			// Every fact must carry exactly one record: a Fact is the answer to
			// one question and its provenance is stamped once, for one datum.
			if _, ok := f.Kind(); !ok {
				t.Fatalf("fact carries no record, or more than one: %+v", f)
			}
			c.check(t, f)
		})
	}
}

// TestRecordCompanyFactTool_NeverStampsProvenance is the whole basis of the
// legal asymmetry: the claim "a human said this" is derived by company.Service
// from HOW the call arrived, and a tool that pre-filled it would let the model
// assert it for itself.
func TestRecordCompanyFactTool_NeverStampsProvenance(t *testing.T) {
	var v companyFactValueArgs
	if err := json.Unmarshal([]byte(`{"category":"OG1","classifica":"III"}`), &v); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	f, err := buildCompanyFact("soa", v)
	if err != nil {
		t.Fatalf("buildCompanyFact: %v", err)
	}
	if f.SOA.Provenance != "" || f.SOA.StatedBy != "" || !f.SOA.StatedAt.IsZero() {
		t.Fatalf("attribution = %+v, want an empty envelope for the service to stamp", f.SOA.Attribution)
	}
}

func TestRecordCompanyFactTool_RejectsMalformedInput(t *testing.T) {
	cases := []struct{ name, field, value string }{
		{"unknown field", "turnover_2024", `{"value":"x"}`},
		{"unknown classifica", "soa", `{"category":"OG1","classifica":"IX"}`},
		{"esercizio with no year", "turnover", `{"turnover_minor":1000}`},
		{"unparseable date", "certification", `{"standard":"iso_9001","valid_until":"30/06/2027"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v companyFactValueArgs
			if err := json.Unmarshal([]byte(c.value), &v); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			if _, err := buildCompanyFact(c.field, v); err == nil {
				t.Fatalf("buildCompanyFact accepted %s / %s", c.field, c.value)
			}
		})
	}
}

// TestRecordCompanyFactTool_PassesTheGaraThatPromptedIt covers the field that
// makes the just-in-time-versus-up-front question falsifiable rather than a
// belief.
func TestRecordCompanyFactTool_PassesTheGaraThatPromptedIt(t *testing.T) {
	var gotField string
	var gotTender *int64
	tool := newRecordCompanyFactTool(&turnState{}, func(field string, _ companyFactValueArgs, tenderID *int64) error {
		gotField, gotTender = field, tenderID
		return nil
	})
	out, err := tool.Execute(context.Background(),
		`{"field":"soa","value":"{\"category\":\"OG1\",\"classifica\":\"III\"}","prompted_by_tender_id":"545620"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotField != "soa" {
		t.Fatalf("field = %q", gotField)
	}
	if gotTender == nil || *gotTender != 545620 {
		t.Fatalf("prompted_by_tender_id = %v, want 545620", gotTender)
	}
	if !strings.Contains(out, "never your own reading of it") {
		t.Fatalf("result = %s, want the only-what-the-user-answered steering", out)
	}
}

func TestRecordCompanyFactTool_RejectsNonJSONValue(t *testing.T) {
	tool := newRecordCompanyFactTool(&turnState{}, func(string, companyFactValueArgs, *int64) error {
		t.Fatal("the port must not be reached for a malformed value")
		return nil
	})
	if _, err := tool.Execute(context.Background(), `{"field":"soa","value":"OG1 III"}`); err == nil {
		t.Fatal("Execute accepted a value that is not a JSON object")
	}
}

// ── shared parsing ──

func TestParseToolDate_AcceptsADayOrATimestamp(t *testing.T) {
	for _, in := range []string{"2027-06-30", "2027-06-30T00:00:00Z"} {
		got, err := parseToolDate("value.valid_until", in)
		if err != nil || got == nil {
			t.Fatalf("parseToolDate(%q) = %v, %v", in, got, err)
		}
	}
	if got, err := parseToolDate("value.valid_until", "  "); err != nil || got != nil {
		t.Fatalf("an unstated date must be nil and not an error, got %v, %v", got, err)
	}
	if _, err := parseToolDate("value.valid_until", "30/06/2027"); err == nil {
		t.Fatal("a stated but unparseable date must not be silently dropped")
	}
}
