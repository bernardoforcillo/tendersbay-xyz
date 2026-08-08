package company

import (
	"slices"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// certReq builds a certification requirement whose Blocking flag the caller
// chooses — the one axis that separates a requisito di partecipazione from a
// criterio premiante, and therefore the one axis the lead sentence must not
// blur.
func certReq(std CertificationStandard, blocking bool) Requirement {
	return Requirement{
		Kind:          RequirementCertification,
		Source:        RequirementUserStated,
		Text:          "certificazione " + string(std),
		Blocking:      blocking,
		Certification: &CertificationRequirement{Standard: std},
	}
}

// fullCoverage is the availability of a tender whose own documents we hold, so
// a test that is not about coverage does not accidentally assert the
// documents-unread caveat.
func fullCoverage(a Assessment) Assessment {
	a.DocumentCoverage = document.CoverageFull
	a.DocumentReason = document.ReasonNone
	return a
}

// ── The lead sentence ───────────────────────────────────────────────────────

// TestRecommendationLeadsWithTheFact is the product rule this whole file
// protects: the recommendation names what the gara demands and what the dossier
// answers BEFORE it names a verdict. A bare "non partecipare" is a loss-framed
// instruction from a machine, and the person carrying the liability discounts it.
func TestRecommendationLeadsWithTheFact(t *testing.T) {
	dossier := Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
	}}
	a := fullCoverage(Evaluate([]Requirement{soaReq("OG1", ClassificaIII, true)}, dossier, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if a.Verdict != VerdictNoGo {
		t.Fatalf("verdict = %q, want no_go (the fixture is a blocking shortfall on authoritative evidence)", a.Verdict)
	}
	// Both halves of the fact, required first.
	if !strings.Contains(rec.Lead, "SOA OG1 classifica III") {
		t.Errorf("lead does not state what the gara requires: %q", rec.Lead)
	}
	// Then held.
	if !strings.Contains(rec.Lead, "OG1 I") {
		t.Errorf("lead does not state what the dossier holds: %q", rec.Lead)
	}
	// And the verdict is nowhere in the sentence a reader meets first.
	for _, forbidden := range []string{"no_go", "non partecipare", "insufficient_data"} {
		if strings.Contains(strings.ToLower(rec.Lead), forbidden) {
			t.Errorf("the lead must not carry the verdict, got %q", rec.Lead)
		}
	}
	if rec.Verdict != a.Verdict {
		t.Errorf("recommendation verdict = %q, want %q", rec.Verdict, a.Verdict)
	}
}

// TestRecommendationSaysWhatWouldChangeIt — a blocking gap with no remedy line
// reads as unsolvable, which for an Italian SOA gap is simply false: avvalimento
// (art. 104) exists precisely for it, and omitting it turns a solvable gap into
// a lost bid.
func TestRecommendationSaysWhatWouldChangeIt(t *testing.T) {
	dossier := Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
	}}
	rec := fullCoverage(Evaluate([]Requirement{soaReq("OG1", ClassificaIII, true)}, dossier, &evalDeadline, evalNow)).Recommendation()

	if len(rec.Changes) != 1 {
		t.Fatalf("changes = %v, want exactly one remedy line", rec.Changes)
	}
	line := rec.Changes[0]
	if !strings.Contains(line, "avvalimento") {
		t.Errorf("a prevalente SOA shortfall must offer avvalimento, got %q", line)
	}
	// The delta is what makes the line worth reading: "get a higher classifica"
	// is what the user already knew.
	if !strings.Contains(line, "classifica III") {
		t.Errorf("remedy line must name the classifica needed, got %q", line)
	}
	if !strings.Contains(line, "1033000.00 EUR") {
		t.Errorf("remedy line must name what classifica III covers, got %q", line)
	}
}

// TestRecommendationScorporabileOffersSubcontract pins the one distinction that
// turns a solvable gap into a false no_go: a scorporabile category may be
// covered in subappalto, a prevalente one may not.
func TestRecommendationScorporabileOffersSubcontract(t *testing.T) {
	dossier := Dossier{SOA: []SOACategory{
		{Category: "OS30", Classifica: ClassificaI, Attribution: stated()},
	}}
	rec := fullCoverage(Evaluate([]Requirement{soaReq("OS30", ClassificaIII, false)}, dossier, &evalDeadline, evalNow)).Recommendation()
	if len(rec.Changes) != 1 || !strings.Contains(rec.Changes[0], "subappalto") {
		t.Fatalf("changes = %v, want a subappalto remedy for a scorporabile category", rec.Changes)
	}
}

// TestRecommendationNonBlockingGapSaysSo — an unmet criterio premiante means
// "you score lower", not "you are out". Reporting the two the same way is the
// difference between a bid abandoned and a bid submitted.
func TestRecommendationNonBlockingGapSaysSo(t *testing.T) {
	dossier := Dossier{Certifications: []Certification{
		{Standard: CertISO14001, Attribution: stated()},
	}}
	a := fullCoverage(Evaluate([]Requirement{certReq(CertISO9001, false)}, dossier, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if len(a.Gaps) != 1 || a.Gaps[0].Status != GapMissing {
		t.Fatalf("gaps = %+v, want one missing gap", a.Gaps)
	}
	if a.Gaps[0].Blocking {
		t.Fatal("a requirement captured as non-blocking must never produce a blocking gap")
	}
	if a.BlockingGapCount != 0 {
		t.Fatalf("blocking gap count = %d, want 0", a.BlockingGapCount)
	}
	// A non-blocking gap alone cannot reach rule 3, so the verdict is a go.
	if a.Verdict != VerdictGo {
		t.Fatalf("verdict = %q, want go — nothing blocking is unmet", a.Verdict)
	}
	if !strings.Contains(rec.Lead, "Non è un requisito di partecipazione bloccante") {
		t.Errorf("the lead must say the gap is not blocking, got %q", rec.Lead)
	}
	if !strings.Contains(rec.Lead, "iso_14001") {
		t.Errorf("the lead must still state what the dossier holds, got %q", rec.Lead)
	}
}

// TestRecommendationUnknownRequirementAsksRatherThanGuesses — the dossier holds
// no fact either way, so the honest output is a question and an
// insufficient_data verdict. A "go" here would be "we checked nothing and found
// nothing wrong", which is the silent pass this package exists to prevent.
func TestRecommendationUnknownRequirementAsksRatherThanGuesses(t *testing.T) {
	a := fullCoverage(Evaluate([]Requirement{soaReq("OG1", ClassificaIII, true)}, Dossier{}, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if len(a.Gaps) != 1 || a.Gaps[0].Status != GapUnknown {
		t.Fatalf("gaps = %+v, want one unknown gap", a.Gaps)
	}
	if a.Gaps[0].Question == nil {
		t.Fatal("an unknown gap must carry the question that would resolve it")
	}
	if a.Verdict != VerdictInsufficientData {
		t.Fatalf("verdict = %q, want insufficient_data — ask, do not guess", a.Verdict)
	}
	if !strings.Contains(rec.Lead, "Non sappiamo") {
		t.Errorf("the lead must say we do not know, got %q", rec.Lead)
	}
	if !slices.Contains(rec.Caveats, CaveatOpenQuestions) {
		t.Errorf("caveats = %v, want open_questions", rec.Caveats)
	}
	if len(rec.Changes) != 1 || !strings.Contains(rec.Changes[0], "rispondi") {
		t.Fatalf("changes = %v, want the answer-the-question remedy", rec.Changes)
	}
}

// TestRecommendationNoRequirementsIsNotAPass is the single most important
// property in this package, and the one most likely to be quietly lost.
//
// Phase 1a persists AWARD criteria — how an admitted offer is scored. SELECTION
// criteria (requisiti di partecipazione) live in the disciplinare, and the
// eForms path that would carry them is parsed nowhere in this repo. So a tender
// with no captured requirement is not a tender the company qualifies for; it is
// a tender we know nothing about, and the two must never render the same.
func TestRecommendationNoRequirementsIsNotAPass(t *testing.T) {
	a := Evaluate(nil, Dossier{
		SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaVIII, Attribution: stated()}},
	}, &evalDeadline, evalNow)
	a.DocumentCoverage = document.CoverageNotice
	a.DocumentReason = document.ReasonBodyNotRetrieved
	rec := a.Recommendation()

	if a.Verdict != VerdictInsufficientData {
		t.Fatalf("verdict = %q, want insufficient_data even with a maximal dossier", a.Verdict)
	}
	if len(rec.Changes) != 0 {
		t.Errorf("nothing is remediable when nothing was asked, got %v", rec.Changes)
	}
	for _, want := range []string{"criteri di aggiudicazione", "disciplinare"} {
		if !strings.Contains(rec.Lead, want) {
			t.Errorf("the lead must name %q so the gap in the DATA is visible, got %q", want, rec.Lead)
		}
	}
	for _, want := range []Caveat{CaveatNoRequirements, CaveatDocumentsUnread} {
		if !slices.Contains(rec.Caveats, want) {
			t.Errorf("caveats = %v, want %q", rec.Caveats, want)
		}
	}
}

// TestRecommendationAwardGridOnlyIsFlagged — holding the award grid is not
// holding the requisiti, and presenting one as the other is a wrong answer
// rather than a partial one.
func TestRecommendationAwardGridOnlyIsFlagged(t *testing.T) {
	grid := Requirement{
		Kind:     RequirementOther,
		Source:   RequirementAwardCriteria,
		Text:     "Offerta tecnica — 70 punti",
		Blocking: false,
	}
	a := fullCoverage(Evaluate([]Requirement{grid}, Dossier{}, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if a.Verdict != VerdictInsufficientData {
		t.Fatalf("verdict = %q, want insufficient_data — an award grid is not a participation requirement", a.Verdict)
	}
	if a.AuthoritativeRequirements != 0 {
		t.Fatalf("authoritative requirements = %d, want 0", a.AuthoritativeRequirements)
	}
	if !slices.Contains(rec.Caveats, CaveatAwardGridOnly) {
		t.Errorf("caveats = %v, want award_grid_only", rec.Caveats)
	}
}

// TestRecommendationUnconfirmedExtractionIsFlagged — a quote nobody checked is
// a claim about a PDF, not about the gara, and the reader has to be told.
func TestRecommendationUnconfirmedExtractionIsFlagged(t *testing.T) {
	quoted := soaReq("OG1", ClassificaIII, true)
	quoted.Source = RequirementDocumentExcerpt
	quoted.Excerpt = "è richiesta attestazione SOA OG1 classifica III"

	dossier := Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
	}}
	a := fullCoverage(Evaluate([]Requirement{quoted}, dossier, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if a.Verdict != VerdictInsufficientData {
		t.Fatalf("verdict = %q, want insufficient_data — an unconfirmed quote can never produce a verdict", a.Verdict)
	}
	if !slices.Contains(rec.Caveats, CaveatUnconfirmedExtraction) {
		t.Errorf("caveats = %v, want unconfirmed_extraction", rec.Caveats)
	}
	// The gap itself still stands and still leads with the fact — the caveat
	// caps the VERDICT, it does not hide the finding.
	if !strings.Contains(rec.Lead, "OG1 I") {
		t.Errorf("lead = %q, want the held attestation stated even under an unconfirmed requirement", rec.Lead)
	}
}

// TestRecommendationAllMet renders the only shape that may open positively, and
// only because rule 5 was actually reached.
func TestRecommendationAllMet(t *testing.T) {
	dossier := Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaIV, ValidUntil: p(evalNow.AddDate(3, 0, 0)), Attribution: stated()},
	}}
	a := fullCoverage(Evaluate([]Requirement{soaReq("OG1", ClassificaIII, true)}, dossier, &evalDeadline, evalNow))
	rec := a.Recommendation()

	if a.Verdict != VerdictGo {
		t.Fatalf("verdict = %q, want go", a.Verdict)
	}
	if !strings.Contains(rec.Lead, "soddisfatti") {
		t.Errorf("lead = %q, want the met-requirements sentence", rec.Lead)
	}
	if len(rec.Changes) != 0 {
		t.Errorf("nothing to change when everything is met, got %v", rec.Changes)
	}
	if len(rec.Caveats) != 0 {
		t.Errorf("caveats = %v, want none under full coverage with no open questions", rec.Caveats)
	}
}

// ── Ordering, determinism and bounds ────────────────────────────────────────

// TestRecommendationLeadsWithTheBlockingGap — gaps arrive pre-sorted by the
// domain, and the lead must speak about the FIRST one. A recommendation that
// opened on a met requirement while a blocking gap sat below it would be
// technically complete and practically a lie.
func TestRecommendationLeadsWithTheBlockingGap(t *testing.T) {
	dossier := Dossier{
		SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: stated()}},
		Certifications: []Certification{
			{Standard: CertISO9001, ValidUntil: p(evalNow.AddDate(2, 0, 0)), Attribution: stated()},
		},
	}
	reqs := []Requirement{
		certReq(CertISO9001, true),         // met
		soaReq("OG1", ClassificaIII, true), // blocking shortfall
		certReq(CertISO14001, false),       // non-blocking, missing
	}
	rec := fullCoverage(Evaluate(reqs, dossier, &evalDeadline, evalNow)).Recommendation()

	if !strings.Contains(rec.Lead, "SOA OG1 classifica III") {
		t.Fatalf("lead = %q, want the blocking SOA shortfall", rec.Lead)
	}
}

// TestRecommendationIsDeterministic — the same evidence must render the same
// answer every time. A recommendation that varied between two renders is
// indefensible to the person who has to act on it, and the same argument the
// engine already makes for its own gap ordering.
func TestRecommendationIsDeterministic(t *testing.T) {
	dossier := Dossier{
		SOA:            []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: stated()}},
		Certifications: []Certification{{Standard: CertISO14001, Attribution: stated()}},
	}
	reqs := []Requirement{
		soaReq("OG1", ClassificaIII, true),
		certReq(CertISO9001, true),
		certReq(CertISO45001, false),
	}
	first := fullCoverage(Evaluate(reqs, dossier, &evalDeadline, evalNow)).Recommendation()
	for i := range 5 {
		got := fullCoverage(Evaluate(reqs, dossier, &evalDeadline, evalNow)).Recommendation()
		if got.Lead != first.Lead || got.Verdict != first.Verdict ||
			!slices.Equal(got.Changes, first.Changes) || !slices.Equal(got.Caveats, first.Caveats) {
			t.Fatalf("run %d differs:\n got %+v\nwant %+v", i, got, first)
		}
	}
}

// TestRecommendationBoundsVerbatimText — Requirement.Text may carry a whole
// disciplinare clause (4000 characters). These lines land in a model prompt and
// a log record, and an unbounded quote costs tokens in every turn that follows.
func TestRecommendationBoundsVerbatimText(t *testing.T) {
	long := soaReq("OG1", ClassificaIII, true)
	long.Text = strings.Repeat("è richiesta l'attestazione SOA ", 200)

	dossier := Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
	}}
	rec := fullCoverage(Evaluate([]Requirement{long}, dossier, &evalDeadline, evalNow)).Recommendation()

	if len([]rune(rec.Lead)) > maxLeadTextLen+200 {
		t.Fatalf("lead is %d runes; the verbatim text was not bounded", len([]rune(rec.Lead)))
	}
	if !strings.HasSuffix(strings.SplitN(rec.Lead, ". Il profilo", 2)[0], "…") {
		t.Errorf("a truncated quote must be marked as truncated, got %q", rec.Lead)
	}
	// A byte-sliced accented character would have produced U+FFFD here.
	if strings.ContainsRune(rec.Lead, '�') {
		t.Error("truncation must cut on a rune boundary — Italian requisiti are full of accents")
	}
}
