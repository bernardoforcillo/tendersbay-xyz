package tender

import (
	"context"
	"strings"
	"testing"
	"time"
)

var parseNow = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func TestParseQuery_ExtractsCPVCodes(t *testing.T) {
	got := ParseQuery("45233120 riasfaltatura strade", parseNow)
	if len(got.Filters.CPVPrefixes) != 1 || got.Filters.CPVPrefixes[0] != "45233120" {
		t.Errorf("CPVPrefixes = %v, want [45233120]", got.Filters.CPVPrefixes)
	}
	// The code stays in the text: it is indexed as text too, so leaving it in
	// lets a tender that only mentions it in prose still match.
	if got.Text != "45233120 riasfaltatura strade" {
		t.Errorf("Text = %q, want the code left in place", got.Text)
	}
}

func TestParseQuery_AcceptsCPVCheckDigit(t *testing.T) {
	got := ParseQuery("cpv 45233120-9", parseNow)
	if len(got.Filters.CPVPrefixes) != 1 || got.Filters.CPVPrefixes[0] != "45233120" {
		t.Errorf("CPVPrefixes = %v, want [45233120] with the check digit dropped", got.Filters.CPVPrefixes)
	}
}

func TestParseQuery_DedupesRepeatedCPVCodes(t *testing.T) {
	got := ParseQuery("45233120 oppure 45233120-9", parseNow)
	if len(got.Filters.CPVPrefixes) != 1 {
		t.Errorf("CPVPrefixes = %v, want the repeat deduped", got.Filters.CPVPrefixes)
	}
}

// A shorter run of digits is far more likely to be a year, a quantity or a lot
// number than a code, and a false CPV filter empties the results.
func TestParseQuery_IgnoresNonEightDigitNumbers(t *testing.T) {
	for _, q := range []string{"bando 2026", "lotto 12345", "riferimento 123456789012"} {
		if got := ParseQuery(q, parseNow); len(got.Filters.CPVPrefixes) != 0 {
			t.Errorf("ParseQuery(%q) extracted %v, want no CPV", q, got.Filters.CPVPrefixes)
		}
	}
}

func TestParseQuery_ExtractsUpperBound(t *testing.T) {
	tests := map[string]int64{
		"pulizie sotto 100k":          100_000,
		"cleaning under 100k":         100_000,
		"nettoyage moins de 250000 €": 250_000,
		"reinigung unter 1 mln":       1_000_000,
		"limpieza hasta 50.000 euro":  50_000,
	}
	for query, want := range tests {
		got := ParseQuery(query, parseNow)
		if got.Filters.ValueMax == nil {
			t.Errorf("ParseQuery(%q): ValueMax = nil, want %d", query, want)
			continue
		}
		if *got.Filters.ValueMax != want {
			t.Errorf("ParseQuery(%q): ValueMax = %d, want %d", query, *got.Filters.ValueMax, want)
		}
	}
}

func TestParseQuery_ExtractsLowerBound(t *testing.T) {
	got := ParseQuery("lavori oltre 500k", parseNow)
	if got.Filters.ValueMin == nil || *got.Filters.ValueMin != 500_000 {
		t.Errorf("ValueMin = %v, want 500000", got.Filters.ValueMin)
	}
	if got.Filters.ValueMax != nil {
		t.Errorf("ValueMax = %v, want nil for a one-sided bound", *got.Filters.ValueMax)
	}
}

func TestParseQuery_ExtractsRange(t *testing.T) {
	got := ParseQuery("forniture tra 50k e 200k", parseNow)
	if got.Filters.ValueMin == nil || got.Filters.ValueMax == nil {
		t.Fatalf("range = [%v %v], want both bounds", got.Filters.ValueMin, got.Filters.ValueMax)
	}
	if *got.Filters.ValueMin != 50_000 || *got.Filters.ValueMax != 200_000 {
		t.Errorf("range = [%d %d], want [50000 200000]", *got.Filters.ValueMin, *got.Filters.ValueMax)
	}
}

func TestParseQuery_NormalisesAnInvertedRange(t *testing.T) {
	got := ParseQuery("between 200k and 50k", parseNow)
	if *got.Filters.ValueMin != 50_000 || *got.Filters.ValueMax != 200_000 {
		t.Errorf("range = [%d %d], want the bounds swapped into order", *got.Filters.ValueMin, *got.Filters.ValueMax)
	}
}

// The regression this floor exists for: "fino a" is a budget keyword, so
// without a plausibility check a duration becomes "value <= 5" and the search
// silently returns nothing.
func TestParseQuery_DoesNotReadDurationsAsBudgets(t *testing.T) {
	for _, q := range []string{
		"contratti fino a 5 anni",
		"framework up to 4 lots",
		"almeno 3 referenze",
	} {
		got := ParseQuery(q, parseNow)
		if got.Filters.ValueMin != nil || got.Filters.ValueMax != nil {
			t.Errorf("ParseQuery(%q) read a budget [%v %v], want none", q, got.Filters.ValueMin, got.Filters.ValueMax)
		}
	}
}

// A small number IS money when something says so.
func TestParseQuery_AcceptsSmallAmountsMarkedAsMoney(t *testing.T) {
	for _, q := range []string{"under 500 €", "sotto 500 euro", "under 5k"} {
		if got := ParseQuery(q, parseNow); got.Filters.ValueMax == nil {
			t.Errorf("ParseQuery(%q): ValueMax = nil, want the marked amount accepted", q)
		}
	}
}

func TestParseQuery_RemovesTheBudgetPhraseFromTheText(t *testing.T) {
	got := ParseQuery("servizi di pulizia sotto 100k", parseNow)
	if got.Text != "servizi di pulizia" {
		t.Errorf("Text = %q, want the budget phrase removed — it is a constraint, not subject matter", got.Text)
	}
}

func TestParseQuery_ExtractsRelativeDeadline(t *testing.T) {
	tests := map[string]time.Time{
		"pulizie entro 30 giorni":      parseNow.AddDate(0, 0, 30),
		"cleaning next 2 weeks":        parseNow.AddDate(0, 0, 14),
		"reinigung innerhalb 3 monate": parseNow.AddDate(0, 3, 0),
	}
	for query, want := range tests {
		got := ParseQuery(query, parseNow)
		if got.Filters.DeadlineTo == nil {
			t.Errorf("ParseQuery(%q): DeadlineTo = nil, want %v", query, want)
			continue
		}
		if !got.Filters.DeadlineTo.Equal(want) {
			t.Errorf("ParseQuery(%q): DeadlineTo = %v, want %v", query, *got.Filters.DeadlineTo, want)
		}
		// The window opens at now: "closing within 30 days" means tenders one
		// can still bid on, so an already-expired deadline is not a match.
		if got.Filters.DeadlineFrom == nil || !got.Filters.DeadlineFrom.Equal(parseNow) {
			t.Errorf("ParseQuery(%q): DeadlineFrom = %v, want now", query, got.Filters.DeadlineFrom)
		}
	}
}

func TestParseQuery_ExtractsOpenStatus(t *testing.T) {
	for _, q := range []string{
		"bandi aperti pulizie", "open tenders cleaning", "ausschreibungen offen",
		// Italian and Iberian tenders are as often referred to by a feminine
		// noun (gara, licitación/licitação) as by the masculine one already
		// covered above (bando/bandi) — the adjective must agree in gender.
		"gara aperta pulizie", "licitaciones abiertas limpieza", "licitações abertas limpeza",
	} {
		got := ParseQuery(q, parseNow)
		if len(got.Filters.Statuses) != 1 || got.Filters.Statuses[0] != "open" {
			t.Errorf("ParseQuery(%q): Statuses = %v, want [open]", q, got.Filters.Statuses)
		}
	}
}

// German only carried the uninflected predicate form ("die Ausschreibung ist
// offen") in openWords, not the attributive one that agrees with the noun in
// front of it ("laufende Ausschreibungen") — which is the ordinary way a
// German speaker writes "ongoing tenders". openStatusPattern matches on word
// boundaries, so "laufende" (missing) extracted nothing even though "laufend"
// (present) would have.
func TestParseQuery_ExtractsGermanInflectedOpenStatus(t *testing.T) {
	got := ParseQuery("laufende Ausschreibungen für Bauarbeiten", parseNow)
	if len(got.Filters.Statuses) != 1 || got.Filters.Statuses[0] != "open" {
		t.Fatalf("Statuses = %v, want [open]", got.Filters.Statuses)
	}
	if strings.Contains(strings.ToLower(got.Text), "laufende") {
		t.Errorf("Text = %q, want \"laufende\" stripped from the residual text", got.Text)
	}
}

func TestParseQuery_CombinesEveryFacetInOneLine(t *testing.T) {
	got := ParseQuery("bandi aperti pulizie sotto 100k entro 30 giorni", parseNow)

	if len(got.Filters.Statuses) != 1 || got.Filters.Statuses[0] != "open" {
		t.Errorf("Statuses = %v, want [open]", got.Filters.Statuses)
	}
	if got.Filters.ValueMax == nil || *got.Filters.ValueMax != 100_000 {
		t.Errorf("ValueMax = %v, want 100000", got.Filters.ValueMax)
	}
	if got.Filters.DeadlineTo == nil {
		t.Error("DeadlineTo = nil, want the 30-day window")
	}
	if got.Text != "pulizie" {
		t.Errorf("Text = %q, want only the subject matter left", got.Text)
	}
}

// The whole query being constraints is a legitimate browse, not an empty
// search — the caller routes on this.
func TestParseQuery_CanConsumeTheEntireQuery(t *testing.T) {
	got := ParseQuery("bandi aperti sotto 100k", parseNow)
	if got.Text != "" {
		t.Errorf("Text = %q, want empty when the query was nothing but constraints", got.Text)
	}
}

func TestParseQuery_LeavesOrdinaryProseAlone(t *testing.T) {
	const q = "servizi di manutenzione impianti elettrici"
	got := ParseQuery(q, parseNow)
	if got.Text != q {
		t.Errorf("Text = %q, want %q unchanged", got.Text, q)
	}
	if !got.Filters.IsZero() {
		t.Errorf("Filters = %+v, want none extracted from plain prose", got.Filters)
	}
}

func TestParseQuery_HandlesEmptyInput(t *testing.T) {
	got := ParseQuery("", parseNow)
	if got.Text != "" || !got.Filters.IsZero() {
		t.Errorf("ParseQuery(\"\") = %+v, want an empty result", got)
	}
}

// A filter set in the UI is a decision the user can see and undo; one inferred
// from prose is a guess. The decision wins.
func TestMergeParsedFilters_ExplicitWinsPerFacet(t *testing.T) {
	explicitMax := int64(999)
	explicit := Filters{
		Countries: []string{"IT"},
		ValueMax:  &explicitMax,
	}
	parsedMax := int64(100_000)
	parsed := Filters{
		Countries:   []string{"DE"},
		CPVPrefixes: []string{"45"},
		ValueMax:    &parsedMax,
	}

	got := mergeParsedFilters(explicit, parsed)

	if len(got.Countries) != 1 || got.Countries[0] != "IT" {
		t.Errorf("Countries = %v, want the explicit [IT]", got.Countries)
	}
	if got.ValueMax == nil || *got.ValueMax != 999 {
		t.Errorf("ValueMax = %v, want the explicit 999", got.ValueMax)
	}
	// A facet the user left unset is where the parsed value belongs.
	if len(got.CPVPrefixes) != 1 || got.CPVPrefixes[0] != "45" {
		t.Errorf("CPVPrefixes = %v, want the parsed [45] to fill the gap", got.CPVPrefixes)
	}
}

// A one-sided explicit bound must not be completed by a parsed one from the
// other side: the two describe different ranges, and mixing them invents a
// constraint neither the user nor the text asked for.
func TestMergeParsedFilters_TreatsAValueRangeAsOneFacet(t *testing.T) {
	explicitMin := int64(10_000)
	parsedMax := int64(100_000)
	got := mergeParsedFilters(Filters{ValueMin: &explicitMin}, Filters{ValueMax: &parsedMax})

	if got.ValueMax != nil {
		t.Errorf("ValueMax = %d, want nil — the explicit bound owns the whole range facet", *got.ValueMax)
	}
	if got.ValueMin == nil || *got.ValueMin != 10_000 {
		t.Errorf("ValueMin = %v, want the explicit 10000", got.ValueMin)
	}
}

// The magnitude alternation must be longest-first: Go's regexp picks the
// leftmost alternative that still lets the rest of the pattern match, not the
// longest one, so an unsorted list lets a short prefix ("m", 1,000,000)
// shadow a longer word that starts with it ("mila", 1,000) — a silent
// factor-of-1000 error on the budget filter.
func TestParseQuery_ItalianThousandsSuffixNotShadowedByBareM(t *testing.T) {
	got := ParseQuery("pulizie sotto 100 mila", parseNow)
	if got.Filters.ValueMax == nil || *got.Filters.ValueMax != 100_000 {
		t.Errorf("ValueMax = %v, want 100000 — \"mila\" must not be read as \"m\" (1000x too large)", got.Filters.ValueMax)
	}
	// "m" shadowing "mila" also leaves "ila" behind as unconsumed query text,
	// polluting what gets handed to the retrievers.
	if strings.Contains(got.Text, "ila") {
		t.Errorf("Text = %q, want no leftover suffix from a partially-matched magnitude word", got.Text)
	}
}

// "milioni" happens to share "mila"'s multiplier's sibling (1,000,000) with
// the bare "m" that would otherwise shadow it, so the value comes out right
// either way — but the match still only consumes "m", leaving "ilioni" as
// residual noise in the retrieval text. This only catches the residual-text
// half of the bug; TestParseQuery_ItalianThousandsSuffixNotShadowedByBareM
// above catches the value half.
func TestParseQuery_ItalianMillionsLeavesNoResidualSuffix(t *testing.T) {
	got := ParseQuery("sotto 5 milioni", parseNow)
	if got.Filters.ValueMax == nil || *got.Filters.ValueMax != 5_000_000 {
		t.Errorf("ValueMax = %v, want 5000000", got.Filters.ValueMax)
	}
	if strings.Contains(got.Text, "ilioni") {
		t.Errorf("Text = %q, want \"milioni\" consumed whole, not shadowed down to \"m\"", got.Text)
	}
}

// Same residual-text hazard as the Italian case above, for the French form.
func TestParseQuery_FrenchMillionsLeavesNoResidualSuffix(t *testing.T) {
	got := ParseQuery("plus de 50 millions", parseNow)
	if got.Filters.ValueMin == nil || *got.Filters.ValueMin != 50_000_000 {
		t.Errorf("ValueMin = %v, want 50000000", got.Filters.ValueMin)
	}
	if strings.Contains(got.Text, "illions") {
		t.Errorf("Text = %q, want \"millions\" consumed whole, not shadowed down to \"m\"", got.Text)
	}
}

func TestParseAmount_RejectsOverflow(t *testing.T) {
	if _, ok := parseAmount("99999999999999999999", "k", ""); ok {
		t.Error("parseAmount accepted a value that would overflow when multiplied")
	}
}

func TestParseAmount_StripsThousandsSeparators(t *testing.T) {
	got, ok := parseAmount("1.250.000", "", "€")
	if !ok || got != 1_250_000 {
		t.Errorf("parseAmount = (%d, %v), want (1250000, true)", got, ok)
	}
}

// Longest-first alternation is what keeps a short keyword from shadowing a
// longer one that starts with it.
func TestAlternation_OrdersLongestFirst(t *testing.T) {
	got := alternation([]string{"a", "abc", "ab"})
	if got != "abc|ab|a" {
		t.Errorf("alternation = %q, want longest-first", got)
	}
}

// Keyword lists are hand-maintained, so a word containing regex punctuation
// must stay a literal rather than silently becoming a wildcard.
func TestAlternation_EscapesRegexMetacharacters(t *testing.T) {
	if got := alternation([]string{"a.b"}); got != `a\.b` {
		t.Errorf("alternation = %q, want the dot escaped", got)
	}
}

// Query parsing is only right for text a person typed into a search box.
// Applied to arbitrary domain prose it misreads it — which is why it is
// opt-in per call site rather than always on.
func TestSearch_DoesNotParseConstraintsUnlessAsked(t *testing.T) {
	repo := &parseFakeRepo{}
	svc := NewService(repo, &parseFakeKB{}, allowAll{}, nil, Config{
		AnonTier:   Tier{MaxResults: 10, RateLimit: 100, RateWindow: time.Minute},
		AuthedTier: Tier{MaxResults: 50, RateLimit: 100, RateWindow: time.Minute},
	})

	// A client profile describing a company, not a search.
	const notes = "Costruiamo strade da trent'anni, con un fatturato oltre 5 milioni"

	if _, err := svc.Search(context.Background(), SearchParams{
		Query: notes, Limit: 10, RateLimitKey: "k",
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if repo.gotFilters.ValueMin != nil {
		t.Errorf("ValueMin = %d, want nil — prose must not become a budget filter",
			*repo.gotFilters.ValueMin)
	}
	if repo.gotLexical.Text != notes {
		t.Errorf("LexicalQuery.Text = %q, want it passed through untouched", repo.gotLexical.Text)
	}

	if _, err := svc.Search(context.Background(), SearchParams{
		Query: notes, ParseQuery: true, Limit: 10, RateLimitKey: "k",
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if repo.gotFilters.ValueMin == nil || *repo.gotFilters.ValueMin != 5_000_000 {
		t.Errorf("ValueMin = %v, want 5000000 once parsing is opted into", repo.gotFilters.ValueMin)
	}
}

func TestSearchHybrid_PassesTheParsedTextAsALexicalQuery(t *testing.T) {
	repo := &parseFakeRepo{}
	svc := NewService(repo, &parseFakeKB{}, allowAll{}, nil, Config{
		AnonTier:   Tier{MaxResults: 10, RateLimit: 100, RateWindow: time.Minute},
		AuthedTier: Tier{MaxResults: 50, RateLimit: 100, RateWindow: time.Minute},
	})

	if _, err := svc.Search(context.Background(), SearchParams{
		Query: "pulizie uffici", ParseQuery: true, Limit: 10, RateLimitKey: "k",
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}

	// The repo must receive the query as a struct carrying the retrievable text,
	// so later work can add fields (query language, index-expansion toggle)
	// without another nine-method interface break.
	if repo.gotLexical.Text != "pulizie uffici" {
		t.Errorf("LexicalQuery.Text = %q, want %q — the struct must carry the parsed text through to the repo call", repo.gotLexical.Text, "pulizie uffici")
	}
}

type parseFakeRepo struct {
	gotLexical LexicalQuery
	gotFilters Filters
}

func (f *parseFakeRepo) LexicalSearch(_ context.Context, q LexicalQuery, filters Filters, _ int) ([]ScoredTender, error) {
	f.gotLexical, f.gotFilters = q, filters
	return nil, nil
}
func (f *parseFakeRepo) FindByCPVPrefixes(context.Context, []string, Filters, int) ([]ScoredTender, error) {
	return nil, nil
}
func (f *parseFakeRepo) SearchTenders(context.Context, Filters, SortOrder, int, int) ([]Tender, error) {
	return nil, nil
}
func (f *parseFakeRepo) FacetCounts(context.Context, Filters) (Facets, error) { return Facets{}, nil }
func (f *parseFakeRepo) EnrichTenders(context.Context, []string, Filters) ([]Tender, error) {
	return nil, nil
}
func (f *parseFakeRepo) FindDetailByID(context.Context, int64) (*TenderDetail, error) {
	return nil, nil
}
func (f *parseFakeRepo) DocumentsByTenderID(context.Context, int64) ([]Document, error) {
	return nil, nil
}
func (f *parseFakeRepo) LotsByTenderID(context.Context, int64) ([]Lot, error) { return nil, nil }
func (f *parseFakeRepo) CriteriaByTenderID(context.Context, int64) ([]AwardCriterion, error) {
	return nil, nil
}
func (f *parseFakeRepo) OrganizationsByTenderID(context.Context, int64) ([]Organization, error) {
	return nil, nil
}

func (f *parseFakeRepo) RecentTenderRefs(context.Context, int) ([]TenderRef, error) { return nil, nil }
func (f *parseFakeRepo) DistinctCountries(context.Context) ([]string, error)        { return nil, nil }

type parseFakeKB struct{}

func (parseFakeKB) EmbedQuery(context.Context, string) ([]float32, error) { return []float32{0.1}, nil }
func (parseFakeKB) SearchByVector(context.Context, []float32, int, Filters) ([]ScoredChunk, error) {
	return nil, nil
}
func (parseFakeKB) RelatedByDocID(context.Context, string, int) ([]ScoredChunk, error) {
	return nil, nil
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}
