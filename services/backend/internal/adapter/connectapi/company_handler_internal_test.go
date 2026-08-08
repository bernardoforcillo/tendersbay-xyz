package connectapi

import (
	"testing"
	"time"

	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// TestToProtoAttribution_ConfidenceIsAbsentNotZero is the point of the paired
// _set bool. A human-stated fact carries no model confidence — that is
// not-applicable, not 0.0 and not 1.0 — and a wire shape that could not express
// the difference would let a later scoring pass treat a typed fact and a model
// guess as the same kind of thing.
func TestToProtoAttribution_ConfidenceIsAbsentNotZero(t *testing.T) {
	human := toProtoAttribution(company.Attribution{
		Provenance: company.ProvenanceUserStated,
		StatedBy:   "user-1",
		StatedAt:   mustTime(t, "2026-08-01T10:00:00Z"),
		PromptedBy: company.PromptOnboarding,
	})
	if human.ConfidenceSet {
		t.Fatalf("confidence_set = true for a user_stated fact, want false")
	}
	if human.Confidence != 0 {
		t.Fatalf("confidence = %v, want the zero value alongside confidence_set=false", human.Confidence)
	}

	// A genuine zero confidence must be distinguishable from "no confidence".
	zero := 0.0
	inferred := toProtoAttribution(company.Attribution{
		Provenance: company.ProvenanceAgentInferred,
		Confidence: &zero,
	})
	if !inferred.ConfidenceSet || inferred.Confidence != 0 {
		t.Fatalf("confidence=%v set=%v, want 0 with confidence_set=true", inferred.Confidence, inferred.ConfidenceSet)
	}
}

func TestToProtoAttribution_PromptedByTenderIDStringifies(t *testing.T) {
	id := int64(545620)
	got := toProtoAttribution(company.Attribution{
		Provenance:         company.ProvenanceUserConfirmed,
		PromptedBy:         company.PromptTender,
		PromptedByTenderID: &id,
	})
	if got.PromptedByTenderId != "545620" {
		t.Fatalf("prompted_by_tender_id = %q, want \"545620\"", got.PromptedByTenderId)
	}
	if none := toProtoAttribution(company.Attribution{}); none.PromptedByTenderId != "" {
		t.Fatalf("prompted_by_tender_id = %q for an unprompted fact, want \"\"", none.PromptedByTenderId)
	}
}

// TestFromProtoAttribution_RejectsUnparseableTenderID keeps a malformed id a
// client error rather than a silently dropped one: an attribution that lost the
// gara it was prompted by is a provenance record that cannot answer "why did we
// ask this".
func TestFromProtoAttribution_RejectsUnparseableTenderID(t *testing.T) {
	_, err := fromProtoAttribution(&companyv1.Attribution{
		Provenance:         "user_confirmed",
		PromptedBy:         "tender",
		PromptedByTenderId: "gara-545620",
	})
	if err == nil {
		t.Fatal("fromProtoAttribution accepted a non-numeric prompted_by_tender_id")
	}
}

// TestToProtoSOA_CarriesBothExpiryDates is the single most load-bearing mapping
// in this file. The triennial verification lapses THREE YEARS before the
// attestation itself, and a wire shape that carried only valid_until would show
// a lapsed attestation as valid for two more years.
func TestToProtoSOA_CarriesBothExpiryDates(t *testing.T) {
	validUntil := mustTime(t, "2029-04-30T00:00:00Z")
	verifiedUntil := mustTime(t, "2026-04-30T00:00:00Z")
	got := toProtoSOA(company.SOACategory{
		ID: "soa-1", Category: "OG1", Classifica: company.ClassificaIIIBis,
		ValidUntil: &validUntil, VerifiedUntil: &verifiedUntil,
	})
	if got.ValidUntil == "" || got.VerifiedUntil == "" {
		t.Fatalf("valid_until=%q verified_until=%q, want both populated", got.ValidUntil, got.VerifiedUntil)
	}
	// The classifica travels as its published string, "bis" tiers included.
	if got.Classifica != "III-bis" {
		t.Fatalf("classifica = %q, want \"III-bis\"", got.Classifica)
	}
}

// TestFromProtoSOA_RoundTripsThroughTheDomainParser pins the wire and the engine
// to one definition of what a classifica label names.
func TestFromProtoSOA_RoundTripsThroughTheDomainParser(t *testing.T) {
	in := toProtoSOA(company.SOACategory{Category: "OS30", Classifica: company.ClassificaIVBis})
	out, err := fromProtoSOA(in)
	if err != nil {
		t.Fatalf("fromProtoSOA: %v", err)
	}
	if out.Classifica != company.ClassificaIVBis {
		t.Fatalf("classifica = %v, want ClassificaIVBis", out.Classifica)
	}
}

func TestFromProtoSOA_RejectsUnknownClassifica(t *testing.T) {
	_, err := fromProtoSOA(&companyv1.SoaCategory{Category: "OG1", Classifica: "IX"})
	if err == nil {
		t.Fatal("fromProtoSOA accepted classifica \"IX\", which names no tier")
	}
}

// TestFromProtoSOA_RejectsMalformedDate keeps a stated expiry from being
// silently dropped. A lapsed attestation whose date vanished reads as one that
// never expires — the single most likely wrong "go" this wire could produce.
func TestFromProtoSOA_RejectsMalformedDate(t *testing.T) {
	fixture := &companyv1.SoaCategory{Category: "OG1", Classifica: "III", ValidUntil: "30/04/2029"}
	if _, err := fromProtoSOA(fixture); err == nil {
		t.Fatal("fromProtoSOA accepted a non-RFC3339 valid_until")
	}
}

// TestFinancialYearOptionalScalarsRoundTrip covers the difference this dossier
// turns on: a turnover of zero (the company invoiced nothing) and an unknown
// turnover (nobody asked) are different facts, and only the paired _set bools
// keep them apart across the wire.
func TestFinancialYearOptionalScalarsRoundTrip(t *testing.T) {
	zero := int64(0)
	in := company.FinancialYear{Year: 2024, TurnoverMinor: &zero, Currency: "EUR"}
	wire := toProtoFinancialYear(in)
	if !wire.TurnoverSet || wire.TurnoverMinor != 0 {
		t.Fatalf("turnover=%d set=%v, want 0 with turnover_set=true", wire.TurnoverMinor, wire.TurnoverSet)
	}
	if wire.HeadcountSet || wire.SpecificTurnoverSet {
		t.Fatalf("an unstated headcount/specific turnover must not be marked set")
	}
	out, err := fromProtoFinancialYear(wire)
	if err != nil {
		t.Fatalf("fromProtoFinancialYear: %v", err)
	}
	if out.TurnoverMinor == nil || *out.TurnoverMinor != 0 {
		t.Fatalf("turnover = %v, want a stated 0", out.TurnoverMinor)
	}
	if out.Headcount != nil {
		t.Fatalf("headcount = %v, want nil for an unstated value", *out.Headcount)
	}
}

// TestFromProtoPastContract_UnsetSharePctStaysNil guards the direction of
// caution SharePct exists for: defaulting an unstated share to 100% is how an
// SME's dossier overstates its own track record.
func TestFromProtoPastContract_UnsetSharePctStaysNil(t *testing.T) {
	value := int64(500_000_00)
	wire := toProtoPastContract(company.PastContract{
		Role: company.RoleRTIMandante, ValueMinor: &value, Currency: "EUR",
	})
	out, err := fromProtoPastContract(wire)
	if err != nil {
		t.Fatalf("fromProtoPastContract: %v", err)
	}
	if out.SharePct != nil {
		t.Fatalf("share_pct = %v, want nil", *out.SharePct)
	}
	if out.EffectiveValueMinor() != nil {
		t.Fatal("a shared role with no stated share must contribute no effective value")
	}
}

// TestToProtoRequirement_FlattensCitation checks the three scalars the wire
// carries in place of document.Citation, including the degradation the corpus
// forces: page and section metadata exist going forward only.
func TestToProtoRequirement_FlattensCitation(t *testing.T) {
	start, end := 14, 16
	got := toProtoRequirement(company.Requirement{
		ID: "req-1", TenderID: 545620, Kind: company.RequirementSOA,
		Text: "SOA OG1 classifica III", Blocking: true,
		Source: company.RequirementDocumentExcerpt,
		Citation: &document.Citation{
			DocumentURL: "https://example.test/disciplinare.pdf",
			PageStart:   &start, PageEnd: &end,
			SectionPath: []string{"7", "7.2"}, SectionTitle: "Requisiti di capacità tecnica",
		},
	})
	if got.TenderId != "545620" {
		t.Fatalf("tender_id = %q, want the stringified int64", got.TenderId)
	}
	if got.Page != "14-16" {
		t.Fatalf("page = %q, want \"14-16\"", got.Page)
	}
	if got.Section != "7.2 Requisiti di capacità tecnica" {
		t.Fatalf("section = %q", got.Section)
	}
	if got.ConfirmedBy != "" {
		t.Fatalf("confirmed_by = %q, want \"\" for an unconfirmed extraction", got.ConfirmedBy)
	}

	bare := toProtoRequirement(company.Requirement{Kind: company.RequirementOther, Text: "leggi §12"})
	if bare.DocumentUrl != "" || bare.Page != "" || bare.Section != "" {
		t.Fatalf("a requirement with no citation must render empty provenance, got %+v", bare)
	}
}

func TestCitationPage_SinglePageIsNotADegenerateRange(t *testing.T) {
	start := 14
	if got := citationPage(document.Citation{PageStart: &start}); got != "14" {
		t.Fatalf("page = %q, want \"14\"", got)
	}
	end := 14
	if got := citationPage(document.Citation{PageStart: &start, PageEnd: &end}); got != "14" {
		t.Fatalf("page = %q, want \"14\" for a single-page span", got)
	}
	if got := citationPage(document.Citation{}); got != "" {
		t.Fatalf("page = %q, want \"\" when no page metadata was captured", got)
	}
}

// TestToProtoAssessment_PreservesDomainGapOrder is the wire half of the
// "never lead with the verdict" rule: the domain sorts gaps blocking-first, and
// a mapper that re-ordered them would let the panel disagree with the engine
// about what the reader must see first.
func TestToProtoAssessment_PreservesDomainGapOrder(t *testing.T) {
	blocking := company.Requirement{
		Kind: company.RequirementSOA, Text: "SOA OG1 III", Blocking: true,
		Source: company.RequirementUserStated, ConfirmedBy: "user-1",
	}
	met := company.Requirement{
		Kind: company.RequirementCertification, Text: "ISO 9001", Blocking: true,
		Source: company.RequirementUserStated, ConfirmedBy: "user-1",
	}
	a := company.Assessment{
		TenderID: 545620,
		Gaps: []company.Gap{
			{Requirement: blocking, Status: company.GapShortfall, Blocking: true, Held: "OG1 I",
				Remedy: company.Remedy{Kind: company.RemedyAvvalimento}},
			{Requirement: met, Status: company.GapMet, Held: "iso_9001"},
		},
		Verdict:          company.VerdictNoGo,
		DocumentCoverage: document.CoverageNotice,
		DocumentReason:   document.ReasonBodyNotRetrieved,
	}
	got := toProtoAssessment(a)
	if len(got.Gaps) != 2 {
		t.Fatalf("gaps = %d, want 2", len(got.Gaps))
	}
	if got.Gaps[0].Status != "shortfall" || !got.Gaps[0].Blocking {
		t.Fatalf("first gap = %+v, want the blocking shortfall", got.Gaps[0])
	}
	if got.Gaps[0].Held != "OG1 I" {
		t.Fatalf("held = %q, want the dossier's own words", got.Gaps[0].Held)
	}
	if got.DocumentCoverage != "notice_only" || got.DocumentReason != "body_not_retrieved" {
		t.Fatalf("coverage=%q reason=%q, want both carried verbatim", got.DocumentCoverage, got.DocumentReason)
	}
	if got.Verdict != "no_go" {
		t.Fatalf("verdict = %q, want \"no_go\"", got.Verdict)
	}
}

// TestToProtoGap_UnknownCarriesItsQuestion keeps the third answer distinguishable
// on the wire: an unknown gap is an ACTION (ask this), not a quiet pass.
func TestToProtoGap_UnknownCarriesItsQuestion(t *testing.T) {
	got := toProtoGap(company.Gap{
		Requirement: company.Requirement{Kind: company.RequirementSOA, Text: "SOA OG1 III"},
		Status:      company.GapUnknown,
		Remedy:      company.Remedy{Kind: company.RemedyProvideFact},
		Question: &company.CaptureQuestion{
			Prompt:  "Questa gara richiede SOA OG1 classifica III. La possiedi?",
			Options: []string{"Sì", "No", "Non lo so"},
		},
	})
	if got.Status != "unknown" || got.Question == "" || len(got.QuestionOptions) != 3 {
		t.Fatalf("gap = %+v, want an unknown status carrying its question and options", got)
	}
	if got.Blocking {
		t.Fatal("an unknown gap is not a blocking one — it is a question")
	}
}

// TestToProtoAssessment_CarriesTheCaveats is the wire half of the honesty
// block. The verdict alone is not the answer: an assessment computed with no
// document coverage at all, rendered with nothing beside it, reads as a clean
// bill of health. The caveats are what say "and here is what we did not check".
func TestToProtoAssessment_CarriesTheCaveats(t *testing.T) {
	unconfirmed := company.Requirement{
		Kind: company.RequirementSOA, Text: "SOA OG1 III", Blocking: true,
		Source: company.RequirementDocumentExcerpt,
	}
	a := company.Assessment{
		TenderID:         545620,
		Gaps:             []company.Gap{{Requirement: unconfirmed, Status: company.GapUnknown}},
		RequirementCount: 1,
		UnknownGapCount:  1,
		Verdict:          company.VerdictInsufficientData,
		DocumentCoverage: document.CoverageNone,
		DocumentReason:   document.ReasonBodyNotRetrieved,
	}
	got := toProtoAssessment(a)

	want := map[string]bool{}
	for _, c := range a.Recommendation().Caveats {
		want[string(c)] = true
	}
	if len(want) == 0 {
		t.Fatal("the fixture produced no caveats — it no longer exercises the mapping")
	}
	if len(got.Caveats) != len(want) {
		t.Fatalf("caveats = %v, want the domain's %d", got.Caveats, len(want))
	}
	for i, c := range got.Caveats {
		if !want[c] {
			t.Errorf("caveats[%d] = %q, not a caveat the domain produced", i, c)
		}
	}
	// Order is the domain's, fixed so two runs over the same inputs agree.
	for i, c := range a.Recommendation().Caveats {
		if got.Caveats[i] != string(c) {
			t.Errorf("caveats[%d] = %q, want %q — the order is part of the answer", i, got.Caveats[i], c)
		}
	}
}

// TestToProtoAssessment_CaveatsAreEmptyNotNil holds the house rule for a
// repeated field that reaches JSON: nil and [] read differently to a client, and
// "we have no caveats" must not arrive as "the field is missing".
func TestToProtoAssessment_CaveatsAreEmptyNotNil(t *testing.T) {
	met := company.Requirement{
		Kind: company.RequirementCertification, Text: "ISO 9001", Blocking: true,
		Source: company.RequirementUserStated,
	}
	got := toProtoAssessment(company.Assessment{
		TenderID:                  1,
		Gaps:                      []company.Gap{{Requirement: met, Status: company.GapMet}},
		RequirementCount:          1,
		AuthoritativeRequirements: 1,
		Verdict:                   company.VerdictGo,
		DocumentCoverage:          document.CoverageFull,
	})
	if got.Caveats == nil {
		t.Fatal("caveats = nil, want an empty slice")
	}
	if len(got.Caveats) != 0 {
		t.Errorf("caveats = %v, want none under full coverage with nothing open", got.Caveats)
	}
}
