package pdf_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/pdf"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

var fixedNow = time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func human() company.Attribution {
	return company.Attribution{
		Provenance: company.ProvenanceUserStated, StatedBy: "u1",
		StatedAt: fixedNow.Add(-48 * time.Hour), PromptedBy: company.PromptOnboarding,
	}
}

func response(t *testing.T) espd.Response {
	t.Helper()
	attr := map[company.FieldKey]company.Attribution{}
	for _, k := range []company.FieldKey{
		company.FieldLegalName, company.FieldVATNumber, company.FieldCountry, company.FieldIsSME,
	} {
		attr[k] = human()
	}
	d := company.Dossier{
		Identity: company.Identity{
			LegalName: "Acme Costruzioni Srl", VATNumber: "IT01234567890",
			Country: "IT", IsSME: true, Attribution: attr,
		},
		Representatives: []company.Representative{{
			ID: "rep-1", Role: "legale rappresentante", GivenName: "Anna", FamilyName: "Rossi",
			BirthPlace: "Milano", Attribution: human(),
		}},
		FinancialYears: []company.FinancialYear{{
			Year: 2025, TurnoverMinor: ptr(int64(2_100_000_00)), Currency: "EUR",
			Headcount: ptr(int32(18)), Attribution: human(),
		}},
	}
	for _, k := range espd.ExclusionCriteria() {
		dec := company.Declaration{ID: "dec", Criterion: string(k), Attribution: human()}
		if k == espd.CritPaymentTaxes {
			dec.Answer = true
			dec.SelfCleaning = "Rateizzazione concessa dall'Agenzia delle Entrate; pagamenti regolari."
		}
		d.Declarations = append(d.Declarations, dec)
	}
	in := espd.BidInput{
		Bid:       bid.Bid{ID: "bid-1"},
		Procedure: espd.Procedure{BuyerName: "Comune di Milano", Title: "Manutenzione strade", Reference: "A0B1", Country: "IT"},
		Confirmation: &bid.DeclarationConfirmation{
			BidID: "bid-1", ConfirmedAt: fixedNow.Add(-time.Hour),
			DeclarationsHash: espd.HashDeclarations(d.Declarations),
		},
	}
	return espd.Compose(d, in, nil, fixedNow)
}

// TestRenderProducesAReadablePDF checks the file is a PDF a reader will open:
// the header, a cross-reference table, a trailer and the EOF marker. A PDF that
// renders as a broken file is the one failure mode a person cannot work around.
func TestRenderProducesAReadablePDF(t *testing.T) {
	out, err := pdf.New().Render(response(t), espd.RenderOptions{Locale: "it-it", Version: espd.EDM211})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-1.")) {
		t.Fatalf("not a PDF: %q", out[:min(16, len(out))])
	}
	for _, marker := range []string{"/Type /Catalog", "/Type /Pages", "/Type /Page ", "xref", "trailer", "startxref", "%%EOF"} {
		if !bytes.Contains(out, []byte(marker)) {
			t.Errorf("the document has no %q", marker)
		}
	}
	// The cross-reference offsets must point at real objects, or a reader
	// rejects the file.
	if err := checkXref(out); err != nil {
		t.Error(err)
	}
}

// checkXref verifies every offset in the table lands on an "N 0 obj" header.
func checkXref(doc []byte) error {
	i := bytes.LastIndex(doc, []byte("startxref"))
	if i < 0 {
		return errString("no startxref")
	}
	var start int
	if _, err := fmtSscan(string(doc[i+len("startxref"):]), &start); err != nil {
		return err
	}
	table := doc[start:]
	lines := strings.Split(string(table), "\n")
	for n, line := range lines {
		if len(line) < 18 || !strings.HasSuffix(strings.TrimRight(line, " "), "n") {
			continue
		}
		var off int
		if _, err := fmtSscan(line[:10], &off); err != nil {
			continue
		}
		if off <= 0 || off >= len(doc) {
			return errString("xref line " + itoa(n) + " points outside the file")
		}
		if !bytes.Contains(doc[off:min(off+24, len(doc))], []byte(" 0 obj")) {
			return errString("xref line " + itoa(n) + " does not point at an object header")
		}
	}
	return nil
}

// TestRenderCarriesEveryValue: the PDF is the artefact a human signs, so a
// value the response holds and the document omits is a value the signatory
// never saw.
func TestRenderCarriesEveryValue(t *testing.T) {
	out, err := pdf.New().Render(response(t), espd.RenderOptions{Locale: "it-it", Version: espd.EDM211})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"Acme Costruzioni Srl", "IT01234567890", // Part II.A
		"Anna", "Rossi", "Milano", // Part II.B
		"Comune di Milano", "Manutenzione strade", "A0B1", // Part I
		"2100000.00 EUR",         // Part IV.B — minor units rendered as an amount
		"Rateizzazione concessa", // the Art. 57(6) measures
	} {
		if !strings.Contains(text, escaped(want)) {
			t.Errorf("the document does not carry %q", want)
		}
	}
}

// TestRenderIsLocalised pins that the Italian copy is Italian: a DGUE handed to
// an Italian commission in English is a document nobody asked for.
func TestRenderIsLocalised(t *testing.T) {
	r := response(t)
	it, err := pdf.New().Render(r, espd.RenderOptions{Locale: "it-it", Version: espd.EDM211})
	if err != nil {
		t.Fatal(err)
	}
	en, err := pdf.New().Render(r, espd.RenderOptions{Locale: "en-ie", Version: espd.EDM211})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(it), escaped("Documento di gara unico europeo")) {
		t.Error("the Italian document is not in Italian")
	}
	if !strings.Contains(string(en), escaped("European Single Procurement Document")) {
		t.Error("the English document is not in English")
	}
	// Part VI must state, in both, that the file is not signed. It is the one
	// sentence that stops someone submitting an unsigned DGUE.
	if !strings.Contains(string(it), escaped("non è firmato")) && !strings.Contains(string(it), `non \350 firmato`) {
		t.Error("the Italian document does not say it is unsigned")
	}
	if !strings.Contains(string(en), escaped("is not signed")) {
		t.Error("the English document does not say it is unsigned")
	}
}

// TestRenderIsDeterministic: same response, same bytes — the property the
// export audit's content hash rests on.
func TestRenderIsDeterministic(t *testing.T) {
	r := response(t)
	opts := espd.RenderOptions{Locale: "it-it", Version: espd.EDM211}
	first, err := pdf.New().Render(r, opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := pdf.New().Render(r, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two renders of one response differ")
	}
}

// TestRenderPaginates: a dossier long enough to overflow one page must produce
// several, each with its own footer, rather than one page with text drawn off
// the bottom edge.
func TestRenderPaginates(t *testing.T) {
	out, err := pdf.New().Render(response(t), espd.RenderOptions{Locale: "it-it", Version: espd.EDM211})
	if err != nil {
		t.Fatal(err)
	}
	pages := bytes.Count(out, []byte("/Type /Page "))
	if pages < 2 {
		t.Fatalf("a full DGUE fits on %d page(s); the fixture answers 23 exclusion grounds", pages)
	}
	if got := bytes.Count(out, []byte("Preparato con tendersbay")); got != pages {
		t.Errorf("%d footers for %d pages", got, pages)
	}
}

// escaped mirrors the writer's PDF-literal escaping for the characters our
// assertions actually contain, so a test can look for text in the stream.
func escaped(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	out := r.Replace(s)
	var b strings.Builder
	for _, c := range out {
		if c > 127 && c <= 255 {
			b.WriteString("\\" + octal(byte(c)))
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}
