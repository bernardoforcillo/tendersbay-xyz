package eforms_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
)

// The parse boundary is a port owned by core, and this is the only place the
// two sides are named together: the adapter satisfies the interface
// structurally, so nothing in the production build depends on this line holding
// — which is exactly why it is asserted here rather than left to be discovered
// at the wiring site.
var _ enrich.NoticeDetailParser = eforms.DetailParser{}

// The corpus below is chosen for the shapes that break a parser, not for the
// happy path: a notice whose criteria carry no weights at all, a notice with no
// criteria, a multilingual notice where first-wins would silently pick the
// wrong language, and a deadline whose UTC offset must survive. A parser that
// only ever sees well-formed multi-lot notices with a complete weighted grid
// passes while being wrong about most of the real corpus.
//
// PROVENANCE — read this before trusting a fixture.
//
//   - notice_548186-2026_single_lot_weighted.xml is a real TED notice, stored
//     byte-for-byte as published (30 KB, retrieved via the TED MCP server;
//     direct HTTPS to ted.europa.eu is blocked from this environment). Every
//     structural claim the other fixtures make is copied from it.
//   - The derived_*.xml fixtures are hand-built. They are structurally faithful
//     — same namespaces, same element nesting, same attribute spellings, same
//     eForms idioms as the real notice — but their content is invented, and a
//     hand-built fixture cannot falsify an assumption the author already holds.
//     They should be replaced with real notices of the same shape before merge;
//     see the spec's golden-corpus gate, which calls for ~20 real documents
//     including ES, FR, DE and PL notices that this corpus does not yet cover.
func loadXML(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return data
}

func parseDetail(t *testing.T, name string) tender.NoticeDetail {
	t.Helper()
	d, err := eforms.ParseDetail(loadXML(t, name))
	if err != nil {
		t.Fatalf("ParseDetail(%s): %v", name, err)
	}
	return d
}

// wantLot is the part of a lot that distinguishes one notice shape from
// another. Value is a pointer for the same reason the domain's is: absent and
// zero are different answers.
type wantLot struct {
	ref           string
	title         string
	cpv           string
	cpvSecondary  []string
	value         *int64
	currency      string
	deadline      string // RFC3339, offset included; "" means nil
	documentsURL  string
	submissionURL string
	criteria      int
}

func TestParseDetail(t *testing.T) {
	tests := []struct {
		name string
		file string

		language      string
		documentsURL  string
		submissionURL string
		deadline      string // RFC3339, offset included; "" means nil

		lots           []wantLot
		noticeCriteria int // entries with LotRef ""
		criteriaCount  int // every entry, at any level
		weightedCount  int
		gridUsable     bool
		orgCount       int
	}{
		{
			// The anchor: a real notice. Its single lot is collapsed onto the
			// notice, so its twelve criteria are notice-level and its portal
			// URLs — published only on the lot — surface at notice level too.
			name:           "real single-lot notice with a fully weighted grid",
			file:           "notice_548186-2026_single_lot_weighted.xml",
			language:       "ITA",
			documentsURL:   "https://portaleappalti.amiu.genova.it/PortaleAppalti/it/procedure/codice/G06630",
			submissionURL:  "https://portaleappalti.amiu.genova.it/PortaleAppalti/it/procedure/codice/G06630",
			deadline:       "2026-09-10T12:30:00+02:00",
			lots:           nil,
			noticeCriteria: 12,
			criteriaCount:  12,
			weightedCount:  12,
			gridUsable:     true,
			orgCount:       2,
		},
		{
			// Per-lot values are the Phase 1a claim; note that everything else
			// is shared across the lots, which is what real multi-lot notices
			// look like. LOT-0003 is the exception that proves the override
			// semantics: it publishes its own documents URL, so it keeps one
			// while the two lots that agree with the notice are cleared.
			name:          "multi-lot with distinct per-lot values",
			file:          "derived_multilot_distinct_values.xml",
			language:      "ITA",
			documentsURL:  "https://portale.esempio.it/procedure/codice/G00749",
			submissionURL: "https://portale.esempio.it/offerte",
			deadline:      "2026-09-10T12:30:00+02:00",
			lots: []wantLot{
				{
					ref: "LOT-0001", title: "Lotto 1 — area nord", cpv: "90511000",
					value: int64p(5010851464), currency: "EUR",
					deadline: "2026-09-10T12:30:00+02:00", criteria: 2,
				},
				{
					ref: "LOT-0002", title: "Lotto 2 — area sud", cpv: "90511000",
					value: int64p(2220517740), currency: "EUR",
					deadline: "2026-09-10T12:30:00+02:00", criteria: 2,
				},
				{
					ref: "LOT-0003", title: "Lotto 3 — isole ecologiche", cpv: "90511000",
					cpvSecondary: []string{"90512000"},
					value:        int64p(100000000), currency: "EUR",
					deadline:     "2026-09-24T09:00:00+02:00",
					documentsURL: "https://portale.esempio.it/procedure/codice/G00750",
					criteria:     1,
				},
			},
			noticeCriteria: 0,
			criteriaCount:  5,
			weightedCount:  4,
			gridUsable:     true,
			orgCount:       2,
		},
		{
			// No ProcurementProjectLot at all: terms, deadline and criteria all
			// sit on the notice element. The +03:00 offset is the point — see
			// TestParseDetail_DeadlineKeepsOffset.
			name:           "no lot element, terms at notice level",
			file:           "derived_no_lot_element.xml",
			language:       "RON",
			documentsURL:   "https://e-licitatie.ro/pub/notices/SEAP-2026-118",
			submissionURL:  "https://e-licitatie.ro/pub/offers/SEAP-2026-118",
			deadline:       "2026-08-11T16:00:00+03:00",
			lots:           nil,
			noticeCriteria: 1,
			criteriaCount:  1,
			weightedCount:  1,
			gridUsable:     true,
			orgCount:       2,
		},
		{
			// The 40% trap: criteria are published, so a naive "has criteria"
			// count calls this covered, but not one of them carries a weight
			// and no grid can be rebuilt from it. The third criterion element
			// is empty and must be dropped rather than inflate the count.
			name:           "criteria published with no weights",
			file:           "derived_criteria_no_weights.xml",
			language:       "ITA",
			documentsURL:   "https://portale.asl.esempio.it/gare/2026-DM-07",
			submissionURL:  "https://portale.asl.esempio.it/gare/2026-DM-07/offerte",
			deadline:       "2026-09-28T23:59:00+02:00",
			lots:           nil,
			noticeCriteria: 2,
			criteriaCount:  2,
			weightedCount:  0,
			gridUsable:     false,
			orgCount:       1,
		},
		{
			// No award criteria at all. GridUsable must be false — "we read the
			// notice and there is no grid" — never confused with the NULL that
			// means "not enriched".
			name:           "no criteria at all",
			file:           "derived_no_criteria.xml",
			language:       "ITA",
			documentsURL:   "https://portale.ctmcagliari.it/gare/2026-PAL-01",
			submissionURL:  "",
			deadline:       "2026-09-16T10:00:00+01:00",
			lots:           nil,
			noticeCriteria: 0,
			criteriaCount:  0,
			weightedCount:  0,
			gridUsable:     false,
			orgCount:       1,
		},
		{
			name:           "multilingual MUL notice",
			file:           "derived_multilang_mul.xml",
			language:       "NLD",
			documentsURL:   "https://enot.publicprocurement.be/BRU-2026-441",
			submissionURL:  "https://enot.publicprocurement.be/BRU-2026-441/submit",
			deadline:       "2026-10-01T11:45:00+01:00",
			lots:           nil,
			noticeCriteria: 3,
			criteriaCount:  3,
			weightedCount:  2,
			gridUsable:     true,
			orgCount:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDetail(t, tt.file)

			if got.Language != tt.language {
				t.Errorf("Language = %q, want %q", got.Language, tt.language)
			}
			if got.DocumentsURL != tt.documentsURL {
				t.Errorf("DocumentsURL = %q, want %q", got.DocumentsURL, tt.documentsURL)
			}
			if got.SubmissionURL != tt.submissionURL {
				t.Errorf("SubmissionURL = %q, want %q", got.SubmissionURL, tt.submissionURL)
			}
			assertInstant(t, "Deadline", got.Deadline, tt.deadline)

			if len(got.Criteria) != tt.noticeCriteria {
				t.Errorf("notice-level criteria = %d, want %d", len(got.Criteria), tt.noticeCriteria)
			}
			if n := got.CriteriaCount(); n != tt.criteriaCount {
				t.Errorf("CriteriaCount() = %d, want %d", n, tt.criteriaCount)
			}
			if n := got.WeightedCount(); n != tt.weightedCount {
				t.Errorf("WeightedCount() = %d, want %d", n, tt.weightedCount)
			}
			if u := got.GridUsable(); u != tt.gridUsable {
				t.Errorf("GridUsable() = %v, want %v", u, tt.gridUsable)
			}
			if n := len(got.AllCriteria()); n != tt.criteriaCount {
				t.Errorf("len(AllCriteria()) = %d, want %d — the notice-level and lot-level sets must partition, not overlap", n, tt.criteriaCount)
			}
			if len(got.Organizations) != tt.orgCount {
				t.Errorf("Organizations = %d, want %d", len(got.Organizations), tt.orgCount)
			}
			assertLots(t, got.Lots, tt.lots)
		})
	}
}

func assertLots(t *testing.T, got []tender.Lot, want []wantLot) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Lots = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Ref != w.ref {
			t.Errorf("Lots[%d].Ref = %q, want %q", i, g.Ref, w.ref)
		}
		if g.Title != w.title {
			t.Errorf("Lots[%d].Title = %q, want %q", i, g.Title, w.title)
		}
		if g.CPV != w.cpv {
			t.Errorf("Lots[%d].CPV = %q, want %q", i, g.CPV, w.cpv)
		}
		if !equalStrings(g.CPVSecondary, w.cpvSecondary) {
			t.Errorf("Lots[%d].CPVSecondary = %v, want %v", i, g.CPVSecondary, w.cpvSecondary)
		}
		switch {
		case g.Value == nil && w.value != nil, g.Value != nil && w.value == nil:
			t.Errorf("Lots[%d].Value = %v, want %v", i, g.Value, w.value)
		case g.Value != nil && *g.Value != *w.value:
			t.Errorf("Lots[%d].Value = %d, want %d (minor units)", i, *g.Value, *w.value)
		}
		if g.Currency != w.currency {
			t.Errorf("Lots[%d].Currency = %q, want %q", i, g.Currency, w.currency)
		}
		assertInstant(t, "Lots["+g.Ref+"].Deadline", g.Deadline, w.deadline)
		if g.DocumentsURL != w.documentsURL {
			t.Errorf("Lots[%d].DocumentsURL = %q, want %q (\"\" means it agrees with the notice's)", i, g.DocumentsURL, w.documentsURL)
		}
		if g.SubmissionURL != w.submissionURL {
			t.Errorf("Lots[%d].SubmissionURL = %q, want %q (\"\" means it agrees with the notice's)", i, g.SubmissionURL, w.submissionURL)
		}
		if len(g.Criteria) != w.criteria {
			t.Errorf("Lots[%d].Criteria = %d, want %d", i, len(g.Criteria), w.criteria)
		}
		for j, c := range g.Criteria {
			if c.LotRef != g.Ref {
				t.Errorf("Lots[%d].Criteria[%d].LotRef = %q, want %q", i, j, c.LotRef, g.Ref)
			}
			if c.Ordinal != j+1 {
				t.Errorf("Lots[%d].Criteria[%d].Ordinal = %d, want %d", i, j, c.Ordinal, j+1)
			}
		}
	}
}

// TestParseDetail_DeadlineKeepsOffset is the regression test for the single
// most dangerous thing this parser could have inherited from the tedmcp code
// it was forked from: a helper that keeps only the wall-clock part of an
// eForms time, discarding the UTC offset. A submission deadline is legally
// binding, and there is no way to tell a truncated one from a correct one by
// looking at it.
//
// The instant and the offset are both asserted. Comparing instants alone would
// accept a deadline re-expressed in UTC, which is the same moment but loses the
// local time a bidder actually reads; comparing wall clocks alone would accept
// a two-hour error.
func TestParseDetail_DeadlineKeepsOffset(t *testing.T) {
	d := parseDetail(t, "derived_no_lot_element.xml")
	if d.Deadline == nil {
		t.Fatal("Deadline = nil, want 2026-08-11T16:00:00+03:00")
	}

	want := time.Date(2026, 8, 11, 16, 0, 0, 0, time.FixedZone("", 3*3600))
	if !d.Deadline.Equal(want) {
		t.Errorf("Deadline = %s, want %s", d.Deadline.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if _, off := d.Deadline.Zone(); off != 3*3600 {
		t.Errorf("Deadline offset = %ds, want %ds — the published UTC offset must survive", off, 3*3600)
	}
	if h, m := d.Deadline.Hour(), d.Deadline.Minute(); h != 16 || m != 0 {
		t.Errorf("Deadline wall clock = %02d:%02d, want 16:00 — a truncated time silently becomes midnight", h, m)
	}
}

// TestParseDetail_WeightsAndLanguages pins the two deviations that cost the
// most if they regress: a weight must never be invented, and the language of a
// text must be the one that was chosen rather than the one that happened to be
// first in the document.
func TestParseDetail_WeightsAndLanguages(t *testing.T) {
	d := parseDetail(t, "derived_multilang_mul.xml")
	if len(d.Criteria) != 3 {
		t.Fatalf("Criteria = %d, want 3", len(d.Criteria))
	}

	tests := []struct {
		ordinal     int
		typ         string
		name        string
		description string
		weight      *float64
		weightRaw   string
		lang        string
		why         string
	}{
		{
			ordinal: 1, typ: "quality",
			name: "Technische waarde", description: "Kwaliteit van het voorgestelde onderhoudsplan",
			weight: float64p(30.5), weightRaw: "30,5", lang: "NLD",
			why: "the notice's own declared language wins over the ENG translation that follows it",
		},
		{
			ordinal: 2, typ: "price",
			name: "Price", description: "",
			weight: float64p(40), weightRaw: "40%", lang: "ENG",
			why: "no NLD text is published, so ENG is the documented fallback — and the raw value keeps its percent sign",
		},
		{
			ordinal: 3, typ: "cost",
			name: "Coût du cycle de vie", description: "",
			weight: nil, weightRaw: "nader te bepalen", lang: "FRA",
			why: "the NLD element is blank and there is no ENG one, so the only published text wins; an unparseable weight is nil but never lost",
		},
	}

	for _, tt := range tests {
		c := d.Criteria[tt.ordinal-1]
		if c.LotRef != "" {
			t.Errorf("criterion %d LotRef = %q, want \"\" (single-lot notices carry criteria at notice level)", tt.ordinal, c.LotRef)
		}
		if c.Ordinal != tt.ordinal {
			t.Errorf("criterion %d Ordinal = %d, want %d", tt.ordinal, c.Ordinal, tt.ordinal)
		}
		if c.Type != tt.typ {
			t.Errorf("criterion %d Type = %q, want %q", tt.ordinal, c.Type, tt.typ)
		}
		if c.Name != tt.name {
			t.Errorf("criterion %d Name = %q, want %q — %s", tt.ordinal, c.Name, tt.name, tt.why)
		}
		if c.Description != tt.description {
			t.Errorf("criterion %d Description = %q, want %q", tt.ordinal, c.Description, tt.description)
		}
		if c.Lang != tt.lang {
			t.Errorf("criterion %d Lang = %q, want %q — %s", tt.ordinal, c.Lang, tt.lang, tt.why)
		}
		if c.WeightRaw != tt.weightRaw {
			t.Errorf("criterion %d WeightRaw = %q, want %q", tt.ordinal, c.WeightRaw, tt.weightRaw)
		}
		switch {
		case tt.weight == nil && c.Weight != nil:
			t.Errorf("criterion %d Weight = %v, want nil — %s", tt.ordinal, *c.Weight, tt.why)
		case tt.weight != nil && c.Weight == nil:
			t.Errorf("criterion %d Weight = nil, want %v", tt.ordinal, *tt.weight)
		case tt.weight != nil && *c.Weight != *tt.weight:
			t.Errorf("criterion %d Weight = %v, want %v", tt.ordinal, *c.Weight, *tt.weight)
		}
		if c.HasWeight() != (tt.weight != nil) {
			t.Errorf("criterion %d HasWeight() = %v, want %v", tt.ordinal, c.HasWeight(), tt.weight != nil)
		}
	}
}

// TestParseDetail_WeightlessCriteriaStayNil guards the distinction the whole
// coverage measurement rests on. A criterion naming the award *method* with
// nothing attached to it is present but useless for scoring; flattening its
// absent weight to zero would make this notice look like it publishes a grid.
func TestParseDetail_WeightlessCriteriaStayNil(t *testing.T) {
	d := parseDetail(t, "derived_criteria_no_weights.xml")

	for _, c := range d.AllCriteria() {
		if c.Weight != nil {
			t.Errorf("criterion %d (%q) Weight = %v, want nil", c.Ordinal, c.Description, *c.Weight)
		}
		if c.WeightRaw != "" {
			t.Errorf("criterion %d (%q) WeightRaw = %q, want \"\"", c.Ordinal, c.Description, c.WeightRaw)
		}
	}
	if d.GridUsable() {
		t.Error("GridUsable() = true, want false — criteria are published but none is weighted")
	}

	// The second criterion carries a number-threshold parameter, which is a
	// minimum score rather than a weight. Reading it as one would fabricate a
	// scoring grid out of a pass/fail gate.
	if want := "Soglia di sbarramento sull'offerta tecnica"; d.Criteria[1].Description != want {
		t.Fatalf("Criteria[1].Description = %q, want %q", d.Criteria[1].Description, want)
	}
	if d.Criteria[1].Weight != nil {
		t.Error("a number-threshold parameter was read as a weight")
	}
}

// TestParseDetail_WeightGrammar pins exactly which published forms become a
// number and which are left to WeightRaw.
//
// The refusals are the interesting half. A fabricated weight is worse than an
// absent one, and every case below is a way of fabricating one silently:
// substituting a comma for a point turns the English "1,000" into 1; a value
// the numeric column rejects (an infinity) turns an odd string into a failed
// write for the whole notice, which is a far more expensive outcome than a nil.
// nil costs comparability on one criterion and nothing else — WeightRaw still
// carries the datum verbatim, so any of these can be audited after the fact.
func TestParseDetail_WeightGrammar(t *testing.T) {
	tests := []struct {
		raw  string
		want *float64
	}{
		{"30", ptr(30)},
		{"30%", ptr(30)},
		{" 30 % ", ptr(30)},
		{"30,5", ptr(30.5)},  // comma decimal separator, the European form
		{"30.5", ptr(30.5)},  // point decimal separator
		{"0", ptr(0)},        // a published zero is a weight, not an absence
		{"", nil},            // no weight published — the common case
		{"1,000", nil},       // grouping or decimal? the element does not say
		{"1.234,5", nil},     // two separators: one of them groups
		{"1,234.5", nil},     // the same ambiguity the other way round
		{"Infinity", nil},    // accepted by ParseFloat, rejected by numeric
		{"NaN", nil},         // likewise
		{"trenta", nil},      // spelled out
		{"30-40", nil},       // a range
		{"see annex A", nil}, // a footnote
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			d, err := eforms.ParseDetail([]byte(weightNotice(tt.raw)))
			if err != nil {
				t.Fatalf("ParseDetail: %v", err)
			}
			all := d.AllCriteria()
			if len(all) != 1 {
				t.Fatalf("criteria = %d, want 1", len(all))
			}
			c := all[0]
			switch {
			case tt.want == nil && c.Weight != nil:
				t.Errorf("Weight = %v, want nil", *c.Weight)
			case tt.want != nil && c.Weight == nil:
				t.Errorf("Weight = nil, want %v", *tt.want)
			case tt.want != nil && *c.Weight != *tt.want:
				t.Errorf("Weight = %v, want %v", *c.Weight, *tt.want)
			}
			// The verbatim string survives every refusal above; that is what
			// makes refusing cheap.
			if c.WeightRaw != strings.TrimSpace(tt.raw) {
				t.Errorf("WeightRaw = %q, want %q", c.WeightRaw, strings.TrimSpace(tt.raw))
			}
		})
	}
}

// weightNotice wraps one published weight in the smallest notice that carries
// a single criterion, so the grammar is exercised through ParseDetail rather
// than against an unexported helper.
func weightNotice(raw string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<ContractNotice xmlns="urn:oasis:names:specification:ubl:schema:xsd:ContractNotice-2">
  <cbc:NoticeLanguageCode>ITA</cbc:NoticeLanguageCode>
  <cac:TenderingTerms><cac:AwardingTerms><cac:AwardingCriterion>
    <cac:SubordinateAwardingCriterion>
      <cbc:Name languageID="ITA">Offerta tecnica</cbc:Name>
      <ext:UBLExtensions><ext:UBLExtension><ext:ExtensionContent><efext:EformsExtension>
        <efac:AwardCriterionParameter>
          <efbc:ParameterCode listName="number-weight">per-cent</efbc:ParameterCode>
          <efbc:ParameterNumeric>` + raw + `</efbc:ParameterNumeric>
        </efac:AwardCriterionParameter>
      </efext:EformsExtension></ext:ExtensionContent></ext:UBLExtension></ext:UBLExtensions>
    </cac:SubordinateAwardingCriterion>
  </cac:AwardingCriterion></cac:AwardingTerms></cac:TenderingTerms>
</ContractNotice>`
}

func ptr(v float64) *float64 { return &v }

// TestParseDetail_Organizations checks the two-pass shape eForms forces:
// parties are declared once in the extension block and given roles from
// elsewhere in the document, including from inside a lot.
func TestParseDetail_Organizations(t *testing.T) {
	d := parseDetail(t, "notice_548186-2026_single_lot_weighted.xml")
	if len(d.Organizations) != 2 {
		t.Fatalf("Organizations = %d, want 2", len(d.Organizations))
	}

	buyer := d.Organizations[0]
	if buyer.Ref != "ORG-0001" || buyer.Name != "Gare Amiu Genova" {
		t.Errorf("Organizations[0] = {%q, %q}, want {ORG-0001, Gare Amiu Genova}", buyer.Ref, buyer.Name)
	}
	if buyer.CompanyID != "03818890109" {
		t.Errorf("buyer CompanyID = %q, want 03818890109 (the codice fiscale that joins to ANAC)", buyer.CompanyID)
	}
	if buyer.Email != "gare@amiu.genova.it" || buyer.Phone != "+39 0105584357" {
		t.Errorf("buyer contact = {%q, %q}, want {gare@amiu.genova.it, +39 0105584357}", buyer.Email, buyer.Phone)
	}
	if buyer.City != "GENOVA (GE)" || buyer.Country != "ITA" {
		t.Errorf("buyer address = {%q, %q}, want {GENOVA (GE), ITA}", buyer.City, buyer.Country)
	}
	// Buyers routinely name themselves as the body to ask about appeals while
	// naming a court as the body that hears them, so a party holding two roles
	// is the normal case rather than an edge one.
	if !equalStrings(buyer.Roles, []string{"buyer", "appeal-information"}) {
		t.Errorf("buyer Roles = %v, want [buyer appeal-information]", buyer.Roles)
	}

	court := d.Organizations[1]
	if court.Ref != "ORG-0002" || court.Name != "TAR Genova" {
		t.Errorf("Organizations[1] = {%q, %q}, want {ORG-0002, TAR Genova}", court.Ref, court.Name)
	}
	if !equalStrings(court.Roles, []string{"appeal-body"}) {
		t.Errorf("court Roles = %v, want [appeal-body] — the role is declared inside the lot's AppealTerms", court.Roles)
	}
}

// TestParseDetail_MultilangOrganizationName is the organization-level half of
// the language rule: a party named in two languages must resolve by the same
// preference order as everything else, and a party named in neither the
// notice's language nor English must still resolve to something.
func TestParseDetail_MultilangOrganizationName(t *testing.T) {
	d := parseDetail(t, "derived_multilang_mul.xml")
	if len(d.Organizations) != 2 {
		t.Fatalf("Organizations = %d, want 2", len(d.Organizations))
	}
	if got, want := d.Organizations[0].Name, "Stad Brussel"; got != want {
		t.Errorf("Organizations[0].Name = %q, want %q (NLD is the notice language; FRA follows it in the document)", got, want)
	}
	if got, want := d.Organizations[1].Name, "Conseil d'État"; got != want {
		t.Errorf("Organizations[1].Name = %q, want %q (only FRA is published)", got, want)
	}
}

// TestParseDetail_RealNoticeCriteria pins the full grid of the one fixture that
// is a real published notice, criterion by criterion. This is the "100% exact
// match" gate: a fabricated criterion is worse than a missing one, so the
// weights are asserted individually rather than as a sum.
func TestParseDetail_RealNoticeCriteria(t *testing.T) {
	d := parseDetail(t, "notice_548186-2026_single_lot_weighted.xml")

	want := []struct {
		typ         string
		description string
		weight      float64
	}{
		{"quality", "Certificazione ISO 9001", 1},
		{"quality", "Certificazione ISO 14001", 1},
		{"quality", "Certificazione ISO 45001", 1},
		{"quality", "Rating di legalità come da delibera AGCM del 12.11.2012, modificata da delibera n. 28361 del 28.07.2020", 3},
		{"quality", "Progetto operativo comprensivo di organizzazione del servizio, percorsi e migliorie", 18},
		{"quality", "Organizzazione del personale impiegato, composizione equipaggi, programma di formazione integrativa", 13},
		{"quality", "Piano di controllo conformità conferimenti e qualità dei materiali raccolti (par. 3.6 - 3.7 Capitolato Speciale d'Appalto)", 5},
		{"quality", "Intervento straordinario di pulizia e ripristino postazioni per sversamenti ingenti", 7},
		{"quality", "Fornitura e installazione di almeno 500 ganci / supporti porta-mastelli", 5},
		{"quality", "Recupero rifiuti non conformi ancora presenti dopo segnalazione all'utente", 9},
		{"quality", "Modalità di gestione ambiente, salute e sicurezza con riferimento ai rischi specifici dei percorsi pedonali e del servizio notturno", 7},
		{"price", "Ribasso percentuale", 30},
	}

	if len(d.Criteria) != len(want) {
		t.Fatalf("Criteria = %d, want %d", len(d.Criteria), len(want))
	}
	for i, w := range want {
		c := d.Criteria[i]
		if c.Ordinal != i+1 {
			t.Errorf("Criteria[%d].Ordinal = %d, want %d", i, c.Ordinal, i+1)
		}
		if c.Type != w.typ {
			t.Errorf("Criteria[%d].Type = %q, want %q", i, c.Type, w.typ)
		}
		// This notice publishes no <cbc:Name> on its criteria at all — only a
		// description. An empty Name here is the notice's shape, not a miss.
		if c.Name != "" {
			t.Errorf("Criteria[%d].Name = %q, want \"\"", i, c.Name)
		}
		if c.Description != w.description {
			t.Errorf("Criteria[%d].Description = %q, want %q", i, c.Description, w.description)
		}
		if c.Lang != "ITA" {
			t.Errorf("Criteria[%d].Lang = %q, want ITA", i, c.Lang)
		}
		if c.Weight == nil || *c.Weight != w.weight {
			t.Errorf("Criteria[%d].Weight = %v, want %v", i, c.Weight, w.weight)
		}
	}
}

// TestParseDetail_Rejects covers the inputs that must fail loudly. The second
// is the important one: encoding/xml decodes an HTML error page without
// complaint into a zero-valued struct, which would then be stored as a notice
// that publishes nothing — indistinguishable from the genuine and frequent case
// of a notice that really publishes nothing, and therefore poison in exactly
// the measurement this parser exists to produce.
// TestParseDetail_KeysAreUniqueAndNonEmpty pins the two invariants storage
// depends on but cannot enforce without failing the whole notice: every
// organization has a distinct, non-empty ref, and so does every lot.
//
// Both keys are UNIQUE in the schema — (tender_id, org_ref) and
// (tender_id, lot_ref, ordinal) — so a duplicate does not lose one row, it
// aborts the transaction that writes the notice. The row then stays queued,
// keeps the head of the id-ordered enrichment queue, and re-downloads its
// document on every pass. Dropping the malformed party or lot is the
// conservative alternative: it can only ever undercount the published grid.
//
// The notice below is authored to be hostile in exactly those two ways — two
// unidentified parties, a duplicate party ref, a lot with no id and a repeated
// lot id — none of which a real notice should contain and all of which the
// parser must survive.
func TestParseDetail_KeysAreUniqueAndNonEmpty(t *testing.T) {
	const notice = `<?xml version="1.0" encoding="UTF-8"?>
<ContractNotice xmlns="urn:oasis:names:specification:ubl:schema:xsd:ContractNotice-2">
  <cbc:NoticeLanguageCode>ITA</cbc:NoticeLanguageCode>
  <ext:UBLExtensions><ext:UBLExtension><ext:ExtensionContent><efext:EformsExtension>
    <efac:Organizations>
      <efac:Organization><efac:Company>
        <cac:PartyIdentification><cbc:ID>ORG-0001</cbc:ID></cac:PartyIdentification>
        <cac:PartyName><cbc:Name languageID="ITA">Comune di Roma</cbc:Name></cac:PartyName>
      </efac:Company></efac:Organization>
      <efac:Organization><efac:Company>
        <cac:PartyName><cbc:Name languageID="ITA">Unidentified A</cbc:Name></cac:PartyName>
      </efac:Company></efac:Organization>
      <efac:Organization><efac:Company>
        <cac:PartyName><cbc:Name languageID="ITA">Unidentified B</cbc:Name></cac:PartyName>
      </efac:Company></efac:Organization>
      <efac:Organization><efac:Company>
        <cac:PartyIdentification><cbc:ID>ORG-0001</cbc:ID></cac:PartyIdentification>
        <cac:PartyName><cbc:Name languageID="ITA">Duplicate Of The First</cbc:Name></cac:PartyName>
      </efac:Company></efac:Organization>
    </efac:Organizations>
  </efext:EformsExtension></ext:ExtensionContent></ext:UBLExtension></ext:UBLExtensions>
  <cac:ContractingParty><cac:Party><cac:PartyIdentification><cbc:ID>ORG-0001</cbc:ID></cac:PartyIdentification></cac:Party></cac:ContractingParty>
  <cac:ProcurementProjectLot>
    <cbc:ID>LOT-0001</cbc:ID>
    <cac:ProcurementProject><cbc:Name languageID="ITA">Primo</cbc:Name></cac:ProcurementProject>
    <cac:TenderingTerms><cac:AwardingTerms><cac:AwardingCriterion>
      <cac:SubordinateAwardingCriterion><cbc:Name languageID="ITA">Prezzo</cbc:Name></cac:SubordinateAwardingCriterion>
    </cac:AwardingCriterion></cac:AwardingTerms></cac:TenderingTerms>
  </cac:ProcurementProjectLot>
  <cac:ProcurementProjectLot>
    <cac:ProcurementProject><cbc:Name languageID="ITA">Senza identificativo</cbc:Name></cac:ProcurementProject>
    <cac:TenderingTerms><cac:AwardingTerms><cac:AwardingCriterion>
      <cac:SubordinateAwardingCriterion><cbc:Name languageID="ITA">Qualità</cbc:Name></cac:SubordinateAwardingCriterion>
    </cac:AwardingCriterion></cac:AwardingTerms></cac:TenderingTerms>
  </cac:ProcurementProjectLot>
  <cac:ProcurementProjectLot>
    <cbc:ID>LOT-0001</cbc:ID>
    <cac:ProcurementProject><cbc:Name languageID="ITA">Ripetuto</cbc:Name></cac:ProcurementProject>
    <cac:TenderingTerms><cac:AwardingTerms><cac:AwardingCriterion>
      <cac:SubordinateAwardingCriterion><cbc:Name languageID="ITA">Tempi</cbc:Name></cac:SubordinateAwardingCriterion>
    </cac:AwardingCriterion></cac:AwardingTerms></cac:TenderingTerms>
  </cac:ProcurementProjectLot>
</ContractNotice>`

	d, err := eforms.ParseDetail([]byte(notice))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}

	if len(d.Organizations) != 1 || d.Organizations[0].Ref != "ORG-0001" {
		t.Fatalf("Organizations = %+v, want only the one identified party", d.Organizations)
	}
	if got := d.Organizations[0].Name; got != "Comune di Roma" {
		t.Errorf("Organizations[0].Name = %q, want the FIRST occurrence of the repeated ref", got)
	}

	if len(d.Lots) != 1 || d.Lots[0].Ref != "LOT-0001" {
		t.Fatalf("Lots = %+v, want only the one identified lot", d.Lots)
	}
	if got := d.Lots[0].Title; got != "Primo" {
		t.Errorf("Lots[0].Title = %q, want the FIRST occurrence of the repeated ref", got)
	}

	seen := map[string]bool{}
	for _, c := range d.AllCriteria() {
		k := fmt.Sprintf("%s/%d", c.LotRef, c.Ordinal)
		if seen[k] {
			t.Errorf("criterion key %q emitted twice — the storage key is not unique", k)
		}
		seen[k] = true
	}
}

// TestParseDetail_AbsentAppealPartiesAssignNoRoles is the same fabrication in
// its most likely form. Most notices publish no AppealTerms at all, so the
// three appeal references are empty on almost every document — and an empty
// reference must match nothing. Matching it would publish the buyer as its own
// appeal body and mediation body, invented out of two absent ids, on precisely
// the field this parse exists to provide.
func TestParseDetail_AbsentAppealPartiesAssignNoRoles(t *testing.T) {
	const notice = `<?xml version="1.0" encoding="UTF-8"?>
<ContractNotice xmlns="urn:oasis:names:specification:ubl:schema:xsd:ContractNotice-2">
  <cbc:NoticeLanguageCode>ITA</cbc:NoticeLanguageCode>
  <ext:UBLExtensions><ext:UBLExtension><ext:ExtensionContent><efext:EformsExtension>
    <efac:Organizations>
      <efac:Organization><efac:Company>
        <cac:PartyIdentification><cbc:ID>ORG-0001</cbc:ID></cac:PartyIdentification>
        <cac:PartyName><cbc:Name languageID="ITA">Comune di Roma</cbc:Name></cac:PartyName>
      </efac:Company></efac:Organization>
    </efac:Organizations>
  </efext:EformsExtension></ext:ExtensionContent></ext:UBLExtension></ext:UBLExtensions>
  <cac:ContractingParty><cac:Party><cac:PartyIdentification><cbc:ID>ORG-0001</cbc:ID></cac:PartyIdentification></cac:Party></cac:ContractingParty>
  <cac:TenderingTerms>
    <cac:AppealTerms>
      <cac:AppealReceiverParty><cac:PartyIdentification><cbc:ID></cbc:ID></cac:PartyIdentification></cac:AppealReceiverParty>
    </cac:AppealTerms>
  </cac:TenderingTerms>
</ContractNotice>`

	d, err := eforms.ParseDetail([]byte(notice))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	if len(d.Organizations) != 1 {
		t.Fatalf("Organizations = %+v, want one", d.Organizations)
	}
	if !equalStrings(d.Organizations[0].Roles, []string{"buyer"}) {
		t.Errorf("Roles = %v, want [buyer] only — an absent appeal reference must assign nothing",
			d.Organizations[0].Roles)
	}
}

func TestParseDetail_Rejects(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{"malformed document", `<ContractNotice><cbc:ID>`},
		{"HTML error page", `<html><head><title>503 Service Unavailable</title></head><body><h1>503</h1></body></html>`},
		{"empty input", ``},
		{
			"XML that is not a UBL notice",
			`<?xml version="1.0"?><rss version="2.0"><channel><title>not a notice</title></channel></rss>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := eforms.ParseDetail([]byte(tt.xml)); err == nil {
				t.Error("ParseDetail returned no error, want one")
			}
		})
	}
}

// assertInstant compares a parsed time against an RFC3339 literal, checking the
// UTC offset as well as the instant. want == "" asserts nil.
func assertInstant(t *testing.T, field string, got *time.Time, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Errorf("%s = %s, want nil", field, got.Format(time.RFC3339))
		}
		return
	}
	if got == nil {
		t.Errorf("%s = nil, want %s", field, want)
		return
	}

	expected, err := time.Parse(time.RFC3339, want)
	if err != nil {
		t.Fatalf("bad want value %q in test table: %v", want, err)
	}
	_, gotOffset := got.Zone()
	_, wantOffset := expected.Zone()
	if !got.Equal(expected) || gotOffset != wantOffset {
		t.Errorf("%s = %s, want %s", field, got.Format(time.RFC3339), want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func int64p(v int64) *int64 { return &v }

func float64p(v float64) *float64 { return &v }
