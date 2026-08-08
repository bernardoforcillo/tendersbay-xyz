package company

import (
	"strings"
	"testing"
	"time"
)

var (
	evalNow      = time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	evalDeadline = evalNow.AddDate(0, 1, 0) // 2026-09-08
)

// stated / inferred are the two provenance envelopes every fixture below picks
// between. The whole engine turns on the difference.
func stated() Attribution {
	return Attribution{Provenance: ProvenanceUserStated, StatedBy: "u1", StatedAt: evalNow}
}
func confirmed() Attribution {
	return Attribution{Provenance: ProvenanceUserConfirmed, StatedBy: "u1", StatedAt: evalNow}
}
func inferred() Attribution {
	return Attribution{Provenance: ProvenanceAgentInferred, Confidence: p(0.7), StatedAt: evalNow}
}

func soaReq(category string, c Classifica, prevalente bool) Requirement {
	return Requirement{
		Kind:     RequirementSOA,
		Source:   RequirementUserStated,
		Text:     "SOA " + category + " classifica " + c.String(),
		Blocking: true,
		SOA:      &SOARequirement{Category: category, Classifica: c, Prevalente: prevalente},
	}
}

// ── Per-kind matching ───────────────────────────────────────────────────────

func TestEvaluateSOA(t *testing.T) {
	tests := []struct {
		name       string
		dossier    Dossier
		req        Requirement
		wantStatus GapStatus
		wantRemedy RemedyKind
		wantHeld   string
	}{
		{
			// An EMPTY dossier is ignorance, never absence — the same
			// "inaccessible ≠ absent" error core/document guards one level down.
			name:       "no SOA rows at all is unknown, not missing",
			dossier:    Dossier{},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapUnknown,
			wantRemedy: RemedyProvideFact,
		},
		{
			name: "category absent while others are held is missing",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG3", Classifica: ClassificaIV, Attribution: stated()},
			}},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapMissing,
			wantRemedy: RemedyRTI,
			wantHeld:   "OG3 IV",
		},
		{
			name: "held but lower is a shortfall, avvalimento for a prevalente",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
			}},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapShortfall,
			wantRemedy: RemedyAvvalimento,
			wantHeld:   "OG1 I",
		},
		{
			// The prevalente flag changes the REMEDY, not the gap. Getting it
			// wrong turns a solvable gap into a false no_go.
			name: "held but lower on a scorporabile can be subcontracted",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OS30", Classifica: ClassificaI, Attribution: stated()},
			}},
			req:        soaReq("OS30", ClassificaIII, false),
			wantStatus: GapShortfall,
			wantRemedy: RemedySubcontract,
			wantHeld:   "OS30 I",
		},
		{
			name: "sufficient and valid is met",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG1", Classifica: ClassificaIV, ValidUntil: p(evalNow.AddDate(3, 0, 0)), VerifiedUntil: p(evalNow.AddDate(1, 0, 0)), Attribution: stated()},
			}},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapMet,
			wantHeld:   "OG1 IV",
		},
		{
			// The single most likely wrong "go": the triennial verification
			// lapses three years before the attestation.
			name: "lapsed triennial verification is expired even with a live attestation",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG1", Classifica: ClassificaIV, ValidUntil: p(evalNow.AddDate(2, 0, 0)), VerifiedUntil: p(evalNow.AddDate(-1, 0, 0)), Attribution: stated()},
			}},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapExpired,
			wantRemedy: RemedyRenew,
		},
		{
			name: "lapses before the submission deadline is expiring",
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG1", Classifica: ClassificaIV, ValidUntil: p(evalNow.AddDate(0, 0, 10)), Attribution: stated()},
			}},
			req:        soaReq("OG1", ClassificaIII, true),
			wantStatus: GapExpiring,
			wantRemedy: RemedyRenew,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{tt.req}, tt.dossier, &evalDeadline, evalNow)
			if len(a.Gaps) != 1 {
				t.Fatalf("got %d gaps, want 1", len(a.Gaps))
			}
			g := a.Gaps[0]
			if g.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", g.Status, tt.wantStatus)
			}
			if tt.wantRemedy != "" && g.Remedy.Kind != tt.wantRemedy {
				t.Errorf("remedy = %q, want %q", g.Remedy.Kind, tt.wantRemedy)
			}
			if tt.wantHeld != "" && g.Held != tt.wantHeld {
				t.Errorf("held = %q, want %q", g.Held, tt.wantHeld)
			}
			if tt.wantStatus == GapUnknown && g.Question == nil {
				t.Error("an unknown gap must carry a capture question")
			}
			if tt.wantStatus != GapUnknown && g.Question != nil {
				t.Error("only an unknown gap may carry a capture question")
			}
			if tt.wantStatus == GapShortfall && g.Remedy.MissingClassifica == nil {
				t.Error("a SOA shortfall must report the classifica the gara wants")
			}
		})
	}
}

func TestEvaluateCertification(t *testing.T) {
	iso9001 := Requirement{
		Kind: RequirementCertification, Source: RequirementUserStated, Blocking: true,
		Text:          "ISO 9001",
		Certification: &CertificationRequirement{Standard: CertISO9001},
	}
	scoped := Requirement{
		Kind: RequirementCertification, Source: RequirementUserStated, Blocking: true,
		Text:          "ISO 9001 per costruzioni",
		Certification: &CertificationRequirement{Standard: CertISO9001, Scope: "Costruzioni"},
	}

	tests := []struct {
		name       string
		dossier    Dossier
		req        Requirement
		wantStatus GapStatus
	}{
		{"empty dossier is unknown", Dossier{}, iso9001, GapUnknown},
		{
			name: "different standard held is missing",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO14001, Attribution: stated()},
			}},
			req:        iso9001,
			wantStatus: GapMissing,
		},
		{
			name: "held and valid is met",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(2, 0, 0)), Attribution: stated()},
			}},
			req:        iso9001,
			wantStatus: GapMet,
		},
		{
			name: "lapsed is expired",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(0, -1, 0)), Attribution: stated()},
			}},
			req:        iso9001,
			wantStatus: GapExpired,
		},
		{
			name: "lapses before the deadline is expiring",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(0, 0, 5)), Attribution: stated()},
			}},
			req:        iso9001,
			wantStatus: GapExpiring,
		},
		{
			// A certificate is scope-bounded: an ISO 9001 for catering does not
			// cover an ISO 9001 requirement for construction works.
			name: "a named scope must match",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, Scope: "Ristorazione collettiva", Attribution: stated()},
			}},
			req:        scoped,
			wantStatus: GapMissing,
		},
		{
			name: "scope matches case-insensitively as a substring",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, Scope: "Esecuzione di opere di COSTRUZIONI edili", Attribution: stated()},
			}},
			req:        scoped,
			wantStatus: GapMet,
		},
		{
			// An unscoped requirement accepts any scope — the dossier is more
			// specific than the ask, which is not a gap.
			name: "an unscoped requirement accepts a scoped certificate",
			dossier: Dossier{Certifications: []Certification{
				{Standard: CertISO9001, Scope: "Ristorazione collettiva", Attribution: stated()},
			}},
			req:        iso9001,
			wantStatus: GapMet,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{tt.req}, tt.dossier, &evalDeadline, evalNow)
			if got := a.Gaps[0].Status; got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestEvaluateTurnover(t *testing.T) {
	threeYears := func(specific bool) Requirement {
		return Requirement{
			Kind: RequirementTurnover, Source: RequirementUserStated, Blocking: true,
			Text:   "Fatturato sugli ultimi tre esercizi",
			Amount: &AmountRequirement{MinMinor: 3_000_000_00, Currency: "EUR", Years: 3, Specific: specific},
		}
	}
	year := func(y int32, global, specific int64) FinancialYear {
		return FinancialYear{Year: y, TurnoverMinor: p(global), SpecificTurnoverMinor: p(specific), Currency: "EUR", Attribution: stated()}
	}

	tests := []struct {
		name       string
		dossier    Dossier
		req        Requirement
		wantStatus GapStatus
		wantMiss   *int64
	}{
		{"no esercizi at all is unknown", Dossier{}, threeYears(false), GapUnknown, nil},
		{
			name: "complete window over the threshold is met",
			dossier: Dossier{FinancialYears: []FinancialYear{
				year(2025, 1_500_000_00, 900_000_00),
				year(2024, 1_200_000_00, 800_000_00),
				year(2023, 1_000_000_00, 700_000_00),
			}},
			req:        threeYears(false),
			wantStatus: GapMet,
		},
		{
			name: "complete window under the threshold is a shortfall",
			dossier: Dossier{FinancialYears: []FinancialYear{
				year(2025, 500_000_00, 100_000_00),
				year(2024, 500_000_00, 100_000_00),
				year(2023, 500_000_00, 100_000_00),
			}},
			req:        threeYears(false),
			wantStatus: GapShortfall,
			wantMiss:   p(int64(1_500_000_00)),
		},
		{
			// A partial sum reported as the total is the quiet way an SME gets
			// told it fails a threshold it actually meets.
			name: "a hole in the window is unknown, never a partial sum",
			dossier: Dossier{FinancialYears: []FinancialYear{
				year(2025, 1_500_000_00, 900_000_00),
				year(2023, 1_500_000_00, 900_000_00),
			}},
			req:        threeYears(false),
			wantStatus: GapUnknown,
		},
		{
			name: "a nil turnover in the window is unknown",
			dossier: Dossier{FinancialYears: []FinancialYear{
				year(2025, 1_500_000_00, 900_000_00),
				{Year: 2024, Currency: "EUR", Attribution: stated()},
				year(2023, 1_500_000_00, 900_000_00),
			}},
			req:        threeYears(false),
			wantStatus: GapUnknown,
		},
		{
			// The requirement most requisiti actually name reads the specific
			// figure, which is a subset of the global one.
			name: "specific turnover reads the specific column",
			dossier: Dossier{FinancialYears: []FinancialYear{
				year(2025, 1_500_000_00, 300_000_00),
				year(2024, 1_500_000_00, 300_000_00),
				year(2023, 1_500_000_00, 300_000_00),
			}},
			req:        threeYears(true),
			wantStatus: GapShortfall,
			wantMiss:   p(int64(2_100_000_00)),
		},
		{
			// A cross-currency comparison needs a rate and a date the dossier
			// does not hold; inventing one would put a fabricated number under
			// a legal verdict.
			name: "a currency mismatch is unknown, not a raw comparison",
			dossier: Dossier{FinancialYears: []FinancialYear{
				{Year: 2025, TurnoverMinor: p(int64(9_000_000_00)), Currency: "PLN", Attribution: stated()},
				{Year: 2024, TurnoverMinor: p(int64(9_000_000_00)), Currency: "PLN", Attribution: stated()},
				{Year: 2023, TurnoverMinor: p(int64(9_000_000_00)), Currency: "PLN", Attribution: stated()},
			}},
			req:        threeYears(false),
			wantStatus: GapUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{tt.req}, tt.dossier, &evalDeadline, evalNow)
			g := a.Gaps[0]
			if g.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", g.Status, tt.wantStatus)
			}
			if tt.wantMiss != nil {
				if g.Remedy.MissingAmountMinor == nil {
					t.Fatalf("a turnover shortfall must report the missing amount")
				}
				if *g.Remedy.MissingAmountMinor != *tt.wantMiss {
					t.Fatalf("missing = %d, want %d", *g.Remedy.MissingAmountMinor, *tt.wantMiss)
				}
				if g.Remedy.Kind != RemedyAvvalimento {
					t.Errorf("remedy = %q, want avvalimento", g.Remedy.Kind)
				}
			}
		})
	}
}

func TestEvaluatePastContracts(t *testing.T) {
	req := Requirement{
		Kind: RequirementPastContracts, Source: RequirementUserStated, Blocking: true,
		Text:   "Almeno 2 servizi analoghi da 1.000.000 EUR negli ultimi 5 anni",
		Count:  &CountRequirement{Min: 2},
		Amount: &AmountRequirement{MinMinor: 1_000_000_00, Currency: "EUR", Years: 5, CPV: "45"},
	}
	contract := func(cpv string, value int64, role ContractRole, share *float64, endedYearsAgo int) PastContract {
		return PastContract{
			CPV: cpv, ValueMinor: p(value), Currency: "EUR", Role: role, SharePct: share,
			EndedOn: p(evalNow.AddDate(-endedYearsAgo, 0, 0)), Attribution: stated(),
		}
	}

	tests := []struct {
		name       string
		dossier    Dossier
		wantStatus GapStatus
	}{
		{"no referenze at all is unknown", Dossier{}, GapUnknown},
		{
			name: "two conforming references meet it",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("45300", 2_000_000_00, RolePrincipal, nil, 4),
			}},
			wantStatus: GapMet,
		},
		{
			name: "a reference outside the CPV sector does not count",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("79000", 5_000_000_00, RolePrincipal, nil, 1),
			}},
			wantStatus: GapShortfall,
		},
		{
			name: "a reference older than the window does not count",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("45300", 2_000_000_00, RolePrincipal, nil, 9),
			}},
			wantStatus: GapShortfall,
		},
		{
			// A 20% mandante on a €5M contract carries €1M of reference. Here
			// 15% of €5M is €750k, below the €1M threshold.
			name: "an RTI share below the threshold does not count",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("45300", 5_000_000_00, RoleRTIMandante, p(15.0), 1),
			}},
			wantStatus: GapShortfall,
		},
		{
			name: "an RTI share above the threshold counts",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("45300", 5_000_000_00, RoleRTIMandante, p(40.0), 1),
			}},
			wantStatus: GapMet,
		},
		{
			// A shared role with no stated share counts for nothing, which is
			// the direction that stops a dossier overstating its track record.
			name: "an RTI reference with no stated share does not count",
			dossier: Dossier{PastContracts: []PastContract{
				contract("45211", 1_200_000_00, RolePrincipal, nil, 2),
				contract("45300", 5_000_000_00, RoleRTIMandataria, nil, 1),
			}},
			wantStatus: GapShortfall,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{req}, tt.dossier, &evalDeadline, evalNow)
			if got := a.Gaps[0].Status; got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestEvaluateHeadcount(t *testing.T) {
	req := Requirement{
		Kind: RequirementHeadcount, Source: RequirementUserStated, Blocking: true,
		Text:   "Organico medio annuo del triennio: almeno 15 persone",
		Count:  &CountRequirement{Min: 15},
		Amount: &AmountRequirement{Years: 3},
	}
	year := func(y int32, headcount int32) FinancialYear {
		return FinancialYear{Year: y, Headcount: p(headcount), Currency: "EUR", Attribution: stated()}
	}

	tests := []struct {
		name       string
		dossier    Dossier
		wantStatus GapStatus
	}{
		{"no esercizi is unknown", Dossier{}, GapUnknown},
		{
			// "Organico medio annuo del triennio" is a MEAN. Summing three years
			// of the same twelve people would answer 36.
			name:       "the triennium is averaged, not summed",
			dossier:    Dossier{FinancialYears: []FinancialYear{year(2025, 12), year(2024, 12), year(2023, 12)}},
			wantStatus: GapShortfall,
		},
		{
			name:       "an average above the threshold meets it",
			dossier:    Dossier{FinancialYears: []FinancialYear{year(2025, 20), year(2024, 15), year(2023, 16)}},
			wantStatus: GapMet,
		},
		{
			name:       "a missing headcount in the window is unknown",
			dossier:    Dossier{FinancialYears: []FinancialYear{year(2025, 20), {Year: 2024, Attribution: stated()}, year(2023, 16)}},
			wantStatus: GapUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{req}, tt.dossier, &evalDeadline, evalNow)
			if got := a.Gaps[0].Status; got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestEvaluateRegistration(t *testing.T) {
	req := Requirement{
		Kind: RequirementRegistration, Source: RequirementUserStated, Blocking: true,
		Text:         "Iscrizione alla white list prefettizia",
		Registration: &RegistrationRequirement{Kind: RegWhiteList},
	}

	tests := []struct {
		name       string
		dossier    Dossier
		wantStatus GapStatus
	}{
		{"no iscrizioni at all is unknown", Dossier{}, GapUnknown},
		{
			name:       "a different kind held is missing",
			dossier:    Dossier{Registrations: []Registration{{Kind: RegMEPA, Attribution: stated()}}},
			wantStatus: GapMissing,
		},
		{
			name:       "held and valid is met",
			dossier:    Dossier{Registrations: []Registration{{Kind: RegWhiteList, ValidUntil: p(evalNow.AddDate(1, 0, 0)), Attribution: stated()}}},
			wantStatus: GapMet,
		},
		{
			name:       "lapsed is expired",
			dossier:    Dossier{Registrations: []Registration{{Kind: RegWhiteList, ValidUntil: p(evalNow.AddDate(0, -2, 0)), Attribution: stated()}}},
			wantStatus: GapExpired,
		},
		{
			name:       "lapses before the deadline is expiring",
			dossier:    Dossier{Registrations: []Registration{{Kind: RegWhiteList, ValidUntil: p(evalNow.AddDate(0, 0, 3)), Attribution: stated()}}},
			wantStatus: GapExpiring,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate([]Requirement{req}, tt.dossier, &evalDeadline, evalNow)
			if got := a.Gaps[0].Status; got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

// TestEvaluateRegistrationHeldOmitsIdentifier guards the PRD's
// never-leaves-the-backend list: Held is rendered into an agent tool result and
// a proto message, so a registration number must not appear in it.
func TestEvaluateRegistrationHeldOmitsIdentifier(t *testing.T) {
	req := Requirement{
		Kind: RequirementRegistration, Source: RequirementUserStated, Blocking: true,
		Text: "White list", Registration: &RegistrationRequirement{Kind: RegWhiteList},
	}
	d := Dossier{Registrations: []Registration{{
		Kind: RegWhiteList, Identifier: "WL-2024-000123", Authority: "Prefettura di Milano",
		ValidUntil: p(evalNow.AddDate(1, 0, 0)), Attribution: stated(),
	}}}
	a := Evaluate([]Requirement{req}, d, &evalDeadline, evalNow)
	if got := a.Gaps[0].Held; strings.Contains(got, "WL-2024-000123") {
		t.Fatalf("Held leaked the registration identifier: %q", got)
	}
}

// TestEvaluateOtherIsAlwaysUnknown pins the point of RequirementOther: a
// requirement the engine cannot model must be visible as un-modelled, not
// silently absent.
func TestEvaluateOtherIsAlwaysUnknown(t *testing.T) {
	req := Requirement{
		Kind: RequirementOther, Source: RequirementUserStated, Blocking: true,
		Text: "Polizza assicurativa RCT/RCO con massimale non inferiore a 5.000.000 EUR",
	}
	// Even a richly populated dossier cannot answer it.
	d := Dossier{
		SOA:            []SOACategory{{Category: "OG1", Classifica: ClassificaVIII, Attribution: stated()}},
		Certifications: []Certification{{Standard: CertISO9001, Attribution: stated()}},
		FinancialYears: []FinancialYear{{Year: 2025, TurnoverMinor: p(int64(9_000_000_00)), Attribution: stated()}},
	}
	a := Evaluate([]Requirement{req}, d, &evalDeadline, evalNow)
	g := a.Gaps[0]
	if g.Status != GapUnknown {
		t.Fatalf("status = %q, want unknown", g.Status)
	}
	if g.Remedy.Kind != RemedyProvideFact {
		t.Errorf("remedy = %q, want provide_fact", g.Remedy.Kind)
	}
	if g.Question == nil || !strings.Contains(g.Question.Prompt, "Polizza assicurativa") {
		t.Error("the question must quote the requirement the user has to read")
	}
	if a.Verdict != VerdictInsufficientData {
		t.Errorf("verdict = %q, want insufficient_data", a.Verdict)
	}
}

// ── Verdict rules ───────────────────────────────────────────────────────────

func TestEvaluateVerdict(t *testing.T) {
	og1III := Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaIII, Attribution: stated()}}}
	og1I := Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: stated()}}}
	og1IInferred := Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: inferred()}}}

	docExcerpt := func(confirmed bool) Requirement {
		r := soaReq("OG1", ClassificaIII, true)
		r.Source = RequirementDocumentExcerpt
		if confirmed {
			r.ConfirmedBy = "u1"
			r.ConfirmedAt = &evalNow
		}
		return r
	}

	tests := []struct {
		name    string
		reqs    []Requirement
		dossier Dossier
		want    Verdict
	}{
		{
			// Rule 1. Dossier completeness is irrelevant — we do not know what
			// this gara asks.
			name:    "no requirements captured is insufficient data even with a full dossier",
			reqs:    nil,
			dossier: og1III,
			want:    VerdictInsufficientData,
		},
		{
			// Rule 2. An unconfirmed quote is a claim about a PDF, not about the
			// gara.
			name:    "an unconfirmed extraction can never produce a verdict",
			reqs:    []Requirement{docExcerpt(false)},
			dossier: og1I,
			want:    VerdictInsufficientData,
		},
		{
			name:    "a confirmed extraction can",
			reqs:    []Requirement{docExcerpt(true)},
			dossier: og1I,
			want:    VerdictNoGo,
		},
		{
			// The award grid answers "how is an admitted bid scored", never
			// "may we be admitted".
			name: "award-grid requirements alone are insufficient data",
			reqs: []Requirement{func() Requirement {
				r := soaReq("OG1", ClassificaIII, true)
				r.Source = RequirementAwardCriteria
				return r
			}()},
			dossier: og1I,
			want:    VerdictInsufficientData,
		},
		{
			// Rule 3, both conjuncts satisfied.
			name:    "a blocking shortfall on authoritative evidence is a no_go",
			reqs:    []Requirement{soaReq("OG1", ClassificaIII, true)},
			dossier: og1I,
			want:    VerdictNoGo,
		},
		{
			// A user_confirmed dossier fact — the agent asked, the human
			// answered — is authoritative on the dossier side too.
			name:    "a shortfall on a user-confirmed dossier fact is a no_go",
			reqs:    []Requirement{soaReq("OG1", ClassificaIII, true)},
			dossier: Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: confirmed()}}},
			want:    VerdictNoGo,
		},
		{
			// Rule 3's SECOND conjunct: an agent_inferred dossier fact can never
			// CAUSE a no_go, only pre-fill the question. Rule 5 is the other half
			// of that asymmetry — not reaching no_go must not become go. The gap
			// is still rendered ("richiede OG1 III; il profilo indica OG1 I"), so
			// a go beside it would have the panel contradict itself in the
			// direction that costs a user a rejected bid.
			name:    "the same shortfall on an agent-inferred dossier fact is neither a no_go nor a go",
			reqs:    []Requirement{soaReq("OG1", ClassificaIII, true)},
			dossier: og1IInferred,
			want:    VerdictInsufficientData,
		},
		{
			// Rule 5 via the OTHER failing conjunct. The user_stated OG2 is MET,
			// so rule 2 sees an authoritative requirement and rule 4 sees no
			// unknown — the unconfirmed OG1 shortfall reaches the fallthrough
			// alone, which is where it used to become a go.
			name: "a blocking shortfall on an unconfirmed extraction is not a go",
			reqs: []Requirement{
				soaReq("OG2", ClassificaI, false),
				docExcerpt(false),
			},
			dossier: Dossier{SOA: []SOACategory{
				{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
				{Category: "OG2", Classifica: ClassificaI, Attribution: stated()},
			}},
			want: VerdictInsufficientData,
		},
		{
			// Rule 4. The product's answer to an unmodelled gara is a QUESTION.
			name:    "a blocking unknown gap is insufficient data, not a guess",
			reqs:    []Requirement{soaReq("OG1", ClassificaIII, true)},
			dossier: Dossier{},
			want:    VerdictInsufficientData,
		},
		{
			// A non-blocking unknown is a criterio premiante, not a barrier.
			name: "a non-blocking unknown gap does not stop a go",
			reqs: []Requirement{
				soaReq("OG1", ClassificaIII, true),
				func() Requirement {
					r := Requirement{Kind: RequirementOther, Source: RequirementUserStated, Text: "Criterio premiante: certificazione ISO 20121"}
					r.Blocking = false
					return r
				}(),
			},
			dossier: og1III,
			want:    VerdictGo,
		},
		{
			name:    "rule 6: every blocking requirement met is a go",
			reqs:    []Requirement{soaReq("OG1", ClassificaIII, true)},
			dossier: og1III,
			want:    VerdictGo,
		},
		{
			// A no_go outranks a pending question: a known blocker is a stronger
			// fact than an unasked one.
			name: "a blocking gap outranks a pending question",
			reqs: []Requirement{
				soaReq("OG1", ClassificaIII, true),
				{Kind: RequirementOther, Source: RequirementUserStated, Blocking: true, Text: "Polizza RCT"},
			},
			dossier: og1I,
			want:    VerdictNoGo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Evaluate(tt.reqs, tt.dossier, &evalDeadline, evalNow)
			if a.Verdict != tt.want {
				t.Errorf("verdict = %q, want %q (gaps: %+v)", a.Verdict, tt.want, a.Gaps)
			}
			if !a.Consistent() {
				t.Errorf("Evaluate produced an inconsistent assessment: verdict %q with %d authoritative requirements and %d blocking gaps",
					a.Verdict, a.AuthoritativeRequirements, a.BlockingGapCount)
			}
		})
	}
}

// TestAssessmentConsistent asserts the invariant directly, on hand-built
// assessments the engine would never produce — the same construction
// document.Availability.Consistent is tested with.
func TestAssessmentConsistent(t *testing.T) {
	blockingAuthoritative := Gap{
		Requirement:       soaReq("OG1", ClassificaIII, true),
		Status:            GapShortfall,
		Blocking:          true,
		factAuthoritative: true,
	}
	blockingInferredFact := blockingAuthoritative
	blockingInferredFact.factAuthoritative = false

	// The other way to fail rule 3's pair of conjuncts: the DOSSIER fact is
	// authoritative but the REQUIREMENT is an unconfirmed extraction, so
	// CanBlock is false. Both halves are covered below because the two are
	// independent, and a Consistent that tested only their conjunction would
	// admit either one alone beside a go.
	blockingUnconfirmedRequirement := blockingAuthoritative
	blockingUnconfirmedRequirement.Requirement = Requirement{
		Kind:     RequirementSOA,
		Source:   RequirementDocumentExcerpt,
		Text:     "SOA OG1 classifica III",
		Blocking: true,
		SOA:      &SOARequirement{Category: "OG1", Classifica: ClassificaIII},
	}

	unknownBlocking := Gap{
		Requirement: soaReq("OG1", ClassificaIII, true),
		Status:      GapUnknown,
	}

	tests := []struct {
		name string
		in   Assessment
		want bool
	}{
		{"no_go on an authoritative blocking gap", Assessment{Verdict: VerdictNoGo, Gaps: []Gap{blockingAuthoritative}, AuthoritativeRequirements: 1}, true},
		{"no_go with no blocking gap at all", Assessment{Verdict: VerdictNoGo, AuthoritativeRequirements: 1}, false},
		{"no_go resting on an inferred dossier fact", Assessment{Verdict: VerdictNoGo, Gaps: []Gap{blockingInferredFact}, AuthoritativeRequirements: 1}, false},
		{"go with authoritative requirements and no unknowns", Assessment{Verdict: VerdictGo, AuthoritativeRequirements: 2}, true},
		{"go with nothing authoritative under it", Assessment{Verdict: VerdictGo}, false},
		{"go with a blocking unknown", Assessment{Verdict: VerdictGo, Gaps: []Gap{unknownBlocking}, AuthoritativeRequirements: 1}, false},
		{"go contradicted by a blocking gap", Assessment{Verdict: VerdictGo, Gaps: []Gap{blockingAuthoritative}, AuthoritativeRequirements: 1}, false},
		{"go contradicted by a blocking gap on an inferred dossier fact", Assessment{Verdict: VerdictGo, Gaps: []Gap{blockingInferredFact}, AuthoritativeRequirements: 1}, false},
		{"go contradicted by a blocking gap on an unconfirmed requirement", Assessment{Verdict: VerdictGo, Gaps: []Gap{blockingUnconfirmedRequirement}, AuthoritativeRequirements: 1}, false},
		{"insufficient data is always consistent", Assessment{Verdict: VerdictInsufficientData}, true},
		{"an unknown verdict string is not", Assessment{Verdict: "maybe"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Consistent(); got != tt.want {
				t.Errorf("Consistent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEvaluateGapOrdering pins the product rule that no consumer may lead with
// the verdict: blocking facts first, then the questions to ask, then warnings,
// then what is already fine.
func TestEvaluateGapOrdering(t *testing.T) {
	blocking := soaReq("OG1", ClassificaIII, true) // shortfall against OG1 I
	expiring := Requirement{
		Kind: RequirementCertification, Source: RequirementUserStated, Blocking: true,
		Text: "ISO 14001", Certification: &CertificationRequirement{Standard: CertISO14001},
	}
	unknown := Requirement{
		Kind: RequirementRegistration, Source: RequirementUserStated, Blocking: true,
		Text: "White list", Registration: &RegistrationRequirement{Kind: RegWhiteList},
	}
	met := Requirement{
		Kind: RequirementCertification, Source: RequirementUserStated, Blocking: true,
		Text: "ISO 9001", Certification: &CertificationRequirement{Standard: CertISO9001},
	}

	d := Dossier{
		SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: stated()}},
		Certifications: []Certification{
			{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(2, 0, 0)), Attribution: stated()},
			{Standard: CertISO14001, ValidUntil: p(evalNow.AddDate(0, 0, 4)), Attribution: stated()},
		},
	}

	// Deliberately fed in the WRONG order — the domain, not the caller, decides
	// the reading order.
	a := Evaluate([]Requirement{met, expiring, unknown, blocking}, d, &evalDeadline, evalNow)
	wantStatuses := []GapStatus{GapShortfall, GapUnknown, GapExpiring, GapMet}
	if len(a.Gaps) != len(wantStatuses) {
		t.Fatalf("got %d gaps, want %d", len(a.Gaps), len(wantStatuses))
	}
	for i, want := range wantStatuses {
		if a.Gaps[i].Status != want {
			t.Errorf("gap %d status = %q, want %q", i, a.Gaps[i].Status, want)
		}
	}
	if a.BlockingGapCount != 1 || a.UnknownGapCount != 1 {
		t.Errorf("counts: blocking=%d unknown=%d, want 1 and 1", a.BlockingGapCount, a.UnknownGapCount)
	}
	if a.RequirementCount != 4 || a.AuthoritativeRequirements != 4 {
		t.Errorf("requirement counts = %d/%d, want 4/4", a.RequirementCount, a.AuthoritativeRequirements)
	}
}

// TestEvaluateIsDeterministic asserts the purity claim: same inputs, byte-equal
// gap ordering, whatever order the repository happened to return them in.
func TestEvaluateIsDeterministic(t *testing.T) {
	reqs := []Requirement{
		{Kind: RequirementOther, Source: RequirementUserStated, Blocking: true, Text: "Requisito B"},
		{Kind: RequirementOther, Source: RequirementUserStated, Blocking: true, Text: "Requisito A"},
		{Kind: RequirementOther, Source: RequirementUserStated, Blocking: true, Text: "Requisito C"},
	}
	first := Evaluate(reqs, Dossier{}, &evalDeadline, evalNow)
	shuffled := []Requirement{reqs[2], reqs[0], reqs[1]}
	second := Evaluate(shuffled, Dossier{}, &evalDeadline, evalNow)

	if len(first.Gaps) != len(second.Gaps) {
		t.Fatalf("gap counts differ: %d vs %d", len(first.Gaps), len(second.Gaps))
	}
	for i := range first.Gaps {
		if first.Gaps[i].Requirement.Text != second.Gaps[i].Requirement.Text {
			t.Fatalf("gap %d differs: %q vs %q", i, first.Gaps[i].Requirement.Text, second.Gaps[i].Requirement.Text)
		}
	}
}

// TestEvaluateNoDeadlineNeverReportsExpiring covers the honest degradation: with
// no submission deadline there is nothing to compare an expiry against, and a
// warning invented from thin air would be worse than none.
func TestEvaluateNoDeadlineNeverReportsExpiring(t *testing.T) {
	req := Requirement{
		Kind: RequirementCertification, Source: RequirementUserStated, Blocking: true,
		Text: "ISO 9001", Certification: &CertificationRequirement{Standard: CertISO9001},
	}
	d := Dossier{Certifications: []Certification{
		{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(0, 0, 3)), Attribution: stated()},
	}}
	a := Evaluate([]Requirement{req}, d, nil, evalNow)
	if got := a.Gaps[0].Status; got != GapMet {
		t.Fatalf("status = %q, want met (valid today, no deadline to compare against)", got)
	}
}

func TestAssessmentAwardGridOnlyAndUnconfirmed(t *testing.T) {
	awardOnly := Evaluate([]Requirement{
		{Kind: RequirementOther, Source: RequirementAwardCriteria, Text: "Offerta tecnica 70 punti"},
	}, Dossier{}, &evalDeadline, evalNow)
	if !awardOnly.AwardGridOnly() {
		t.Error("an assessment built only from award criteria must say so")
	}

	mixed := Evaluate([]Requirement{
		{Kind: RequirementOther, Source: RequirementAwardCriteria, Text: "Offerta tecnica 70 punti"},
		{Kind: RequirementOther, Source: RequirementUserStated, Text: "Polizza RCT"},
	}, Dossier{}, &evalDeadline, evalNow)
	if mixed.AwardGridOnly() {
		t.Error("a mixed assessment is not award-grid-only")
	}

	unconfirmed := Evaluate([]Requirement{
		{Kind: RequirementOther, Source: RequirementDocumentExcerpt, Text: "Polizza RCT"},
	}, Dossier{}, &evalDeadline, evalNow)
	if !unconfirmed.HasUnconfirmedExtraction() {
		t.Error("an unconfirmed document quote must be flagged")
	}
	if confirmedOnly := Evaluate([]Requirement{
		{Kind: RequirementOther, Source: RequirementDocumentExcerpt, Text: "Polizza RCT", ConfirmedBy: "u1", ConfirmedAt: &evalNow},
	}, Dossier{}, &evalDeadline, evalNow); confirmedOnly.HasUnconfirmedExtraction() {
		t.Error("a confirmed quote must not be flagged as unconfirmed")
	}
}
