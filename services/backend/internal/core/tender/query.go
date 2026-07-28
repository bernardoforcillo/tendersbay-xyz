package tender

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedQuery is what ParseQuery makes of a line of free text: the part worth
// retrieving on, and the part that was really a filter in disguise.
type ParsedQuery struct {
	// Text is the query with structured phrases removed, to be handed to the
	// retrievers.
	Text string
	// Filters holds the constraints lifted out of the text.
	Filters Filters
}

// ParseQuery lifts structured constraints out of a free-text search.
//
// People type one line and expect it to mean everything in it: "pulizie sotto
// 100k in scadenza entro 30 giorni" is a topic AND a budget AND a deadline.
// Handing that whole string to a retriever asks it to match "sotto" and
// "100k" as if they were subject matter, which is both worse at finding the
// right tenders and worse at excluding the wrong ones — a budget is a hard
// constraint, not a hint.
//
// Deliberately NOT extracted:
//
//   - Country names. Doing it properly needs a per-locale name table across
//     24 languages, and the cheap version — matching bare alpha-2 codes — is
//     actively harmful: "it" is an English word, "de" is Spanish, Portuguese
//     and French. A false country filter silently empties the results, which
//     is far worse than not having the feature. The country control in the UI
//     covers this facet without the ambiguity.
//   - Quoted phrases. They already work: the lexical retriever uses
//     websearch_to_tsquery, which handles "..." as a phrase and - as negation
//     natively. Re-implementing that here would only be able to get it wrong.
//
// now anchors relative dates and is passed in rather than read from the clock
// so the result is testable and reproducible.
func ParseQuery(raw string, now time.Time) ParsedQuery {
	out := ParsedQuery{Text: raw}

	// Order matters: ranges must be tried before the open-ended comparisons,
	// or "tra 50k e 200k" would match the "min" arm on its first number and
	// leave the rest of the phrase behind as noise.
	out.Text, out.Filters.ValueMin, out.Filters.ValueMax = extractValueRange(out.Text)
	out.Text = extractDeadline(out.Text, now, &out.Filters)
	out.Text = extractStatus(out.Text, &out.Filters)
	out.Filters.CPVPrefixes = extractCPVCodes(out.Text)

	out.Text = collapseSpaces(out.Text)
	return out
}

// mergeParsedFilters combines filters the caller set explicitly with ones
// parsed out of the query text.
//
// Explicit always wins, per facet. A filter set in the UI is a deliberate
// decision the user can see and undo; one inferred from their prose is a
// guess. When the guess disagrees with the decision, the decision is right.
func mergeParsedFilters(explicit, parsed Filters) Filters {
	out := explicit
	if len(out.CPVPrefixes) == 0 {
		out.CPVPrefixes = parsed.CPVPrefixes
	}
	if len(out.Statuses) == 0 {
		out.Statuses = parsed.Statuses
	}
	if len(out.Countries) == 0 {
		out.Countries = parsed.Countries
	}
	if out.ValueMin == nil && out.ValueMax == nil {
		out.ValueMin, out.ValueMax = parsed.ValueMin, parsed.ValueMax
	}
	if out.DeadlineFrom == nil && out.DeadlineTo == nil {
		out.DeadlineFrom, out.DeadlineTo = parsed.DeadlineFrom, parsed.DeadlineTo
	}
	return out
}

// ── CPV codes ──

// cpvPattern matches a full 8-digit CPV code, with or without its optional
// check digit ("45233120" / "45233120-9").
//
// Exactly eight digits, and only as a whole word: a shorter run of digits is
// far more likely to be a quantity, a year, or a lot number than a code, and
// a false CPV filter would silently narrow the search to nothing.
var cpvPattern = regexp.MustCompile(`\b(\d{8})(?:-\d)?\b`)

// extractCPVCodes returns the CPV codes in text.
//
// Unlike the other extractors this one does NOT remove what it matched: the
// code is also indexed as text, so leaving it in lets a tender whose code
// appears in its description still match, while the filter narrows the search
// to the right sector. Removing it would throw that away for nothing.
func extractCPVCodes(text string) []string {
	matches := cpvPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// ── Monetary values ──

// Comparison keywords, per direction. Covering the languages that carry most
// of the traffic rather than pretending to cover all 24: an unrecognised
// phrase simply stays in the query text and is retrieved on, which is the
// behaviour before this existed. Adding a language means adding words here,
// nothing else.
var (
	belowWords = []string{
		"sotto", "meno di", "fino a", "massimo", "max", // it
		"under", "below", "less than", "up to", "at most", // en
		"moins de", "jusqu'a", "jusqu'à", "au maximum", // fr
		"unter", "weniger als", "bis zu", "hochstens", "höchstens", // de
		"menos de", "hasta", "como maximo", "como máximo", // es
		"menos que", "ate", "até", // pt
		"ponizej", "poniżej", "do", // pl
		"onder", "minder dan", "tot", // nl
	}
	aboveWords = []string{
		"sopra", "oltre", "piu di", "più di", "almeno", "minimo", "min", // it
		"over", "above", "more than", "at least", "starting from", // en
		"plus de", "au moins", "a partir de", "à partir de", // fr
		"uber", "über", "mehr als", "mindestens", "ab", // de
		"mas de", "más de", "al menos", "desde", // es
		"mais de", "pelo menos", // pt
		"powyzej", "powyżej", "co najmniej", "od", // pl
		"boven", "meer dan", "minstens", "vanaf", // nl
	}
	betweenWords = []string{
		"tra", "fra", // it
		"between",         // en
		"entre",           // fr, es, pt
		"zwischen", "von", // de
		"miedzy", "między", // pl
		"tussen", // nl
	}
	andWords = []string{"e", "and", "et", "und", "y", "i", "en", "a", "-", "to", "bis"}
)

var (
	valueBelowPattern   = buildComparisonPattern(belowWords)
	valueAbovePattern   = buildComparisonPattern(aboveWords)
	valueBetweenPattern = buildBetweenPattern()
)

// amountPattern is the number itself: digits with optional thousands/decimal
// separators, an optional magnitude suffix, and an optional currency mark.
// Written once and embedded in each comparison pattern so all of them accept
// exactly the same number formats. All three parts are captured because
// acceptAmount needs to know which were present — see minBareAmount.
// The number group is greedy but bounded: digits with in-word separators,
// plus space-separated groups of exactly three digits ("1 250 000"). It must
// not accept arbitrary whitespace, or a lazy/greedy match would run past the
// number into the following words. The currency alternation lists longer
// forms first, since Go's regexp picks the leftmost-FIRST alternative — with
// "eur" first, "euro" would only ever match its prefix.
const amountPattern = `([0-9][0-9.,]*(?:[ \x{00a0}][0-9]{3})*)\s*(k|m|mln|mio|mila|milioni|millions|million|millionen|millones|milhões|milhoes|tysięcy|tysiecy|tys)?\s*(€|euros|euro|eur)?`

// minBareAmount is the smallest number accepted as money when nothing marks it
// as money — no magnitude suffix, no currency.
//
// Without this floor the comparison keywords fire on ordinary prose: "fino a"
// is one of them, so "contratti fino a 5 anni" would become "value <= 5" and
// silently return nothing. Procurement values written in full are thousands of
// euros at minimum, so a bare number below this is far more likely to be a
// duration, a quantity, or a lot count than a budget.
const minBareAmount = 1000

func buildComparisonPattern(words []string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(?:` + alternation(words) + `)\s+` + amountPattern)
}

func buildBetweenPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(?:` + alternation(betweenWords) + `)\s+` + amountPattern +
		`\s*(?:` + alternation(andWords) + `)\s+` + amountPattern)
}

// alternation renders words as a regex alternation, longest first so that
// "più di" wins over a bare "di"-style prefix of itself, and each word escaped
// so punctuation in the list stays literal.
func alternation(words []string) string {
	sorted := make([]string, len(words))
	copy(sorted, words)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j]) > len(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	escaped := make([]string, len(sorted))
	for i, w := range sorted {
		escaped[i] = regexp.QuoteMeta(w)
	}
	return strings.Join(escaped, "|")
}

// extractValueRange pulls a budget constraint out of text and returns the text
// with that phrase removed, since "sotto 100k" is a constraint rather than
// subject matter and only adds noise to retrieval.
func extractValueRange(text string) (string, *int64, *int64) {
	if m := valueBetweenPattern.FindStringSubmatch(text); m != nil {
		lo, okLo := parseAmount(m[1], m[2], m[3])
		hi, okHi := parseAmount(m[4], m[5], m[6])
		if okLo && okHi {
			if lo > hi {
				lo, hi = hi, lo
			}
			return strings.Replace(text, m[0], " ", 1), &lo, &hi
		}
	}
	var (
		minV, maxV *int64
		out        = text
	)
	if m := valueBelowPattern.FindStringSubmatch(out); m != nil {
		if v, ok := parseAmount(m[1], m[2], m[3]); ok {
			maxV = &v
			out = strings.Replace(out, m[0], " ", 1)
		}
	}
	if m := valueAbovePattern.FindStringSubmatch(out); m != nil {
		if v, ok := parseAmount(m[1], m[2], m[3]); ok {
			minV = &v
			out = strings.Replace(out, m[0], " ", 1)
		}
	}
	return out, minV, maxV
}

// magnitudes maps a suffix to its multiplier.
var magnitudes = map[string]int64{
	"k": 1_000, "mila": 1_000, "tys": 1_000, "tysiecy": 1_000, "tysięcy": 1_000,
	"m": 1_000_000, "mln": 1_000_000, "mio": 1_000_000,
	"milioni": 1_000_000, "million": 1_000_000, "millions": 1_000_000,
	"millionen": 1_000_000, "millones": 1_000_000,
	"milhoes": 1_000_000, "milhões": 1_000_000,
}

// parseAmount turns a matched number, magnitude suffix and currency mark into
// a value, reporting ok=false when the match doesn't look like money at all.
//
// Separators are stripped rather than interpreted: "100.000" means a hundred
// thousand across most of Europe and a hundred-point-zero in the anglophone
// convention, and there is no way to tell which from the string alone. Reading
// them as thousands separators is right far more often for procurement values,
// which are whole euros.
func parseAmount(number, suffix, currency string) (int64, bool) {
	cleaned := strings.NewReplacer(".", "", ",", "", " ", "", " ", "").Replace(number)
	if cleaned == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, false
	}
	mult, hasMagnitude := magnitudes[strings.ToLower(suffix)]
	if hasMagnitude {
		// Guard the multiply: a pathological "99999999999999999999k" must not
		// wrap around into a small or negative bound.
		if n > (1<<62)/mult {
			return 0, false
		}
		n *= mult
	}
	// Nothing marks this as money, so only a plausibly monetary magnitude
	// counts — see minBareAmount.
	if !hasMagnitude && currency == "" && n < minBareAmount {
		return 0, false
	}
	return n, true
}

// ── Deadlines ──

var (
	dayWords   = []string{"giorni", "giorno", "days", "day", "jours", "jour", "tage", "tagen", "dias", "días", "dni", "dagen"}
	weekWords  = []string{"settimane", "settimana", "weeks", "week", "semaines", "semaine", "wochen", "woche", "semanas", "tygodni", "tygodnie", "weken"}
	monthWords = []string{"mesi", "mese", "months", "month", "mois", "monate", "monat", "meses", "miesiecy", "miesięcy", "maanden", "maand"}

	// relativeDeadlinePattern matches "entro 30 giorni", "next 2 weeks",
	// "in den nächsten 3 Monaten" — a number followed by a time unit. The
	// leading preposition is optional because the number+unit pair is the part
	// that carries the meaning, and its wording varies far more by language.
	relativeDeadlinePattern = regexp.MustCompile(`(?i)\b(?:entro|next|prossim\w+|within|in|dans|innerhalb|binnen|proximos|próximos|en los proximos|en los próximos|w ciagu|w ciągu|nas|nos)?\s*(\d{1,3})\s*(` +
		alternation(append(append(append([]string{}, dayWords...), weekWords...), monthWords...)) + `)\b`)
)

// extractDeadline turns a relative time window into a deadline range and
// removes the phrase from the query text.
//
// The window starts at now, never earlier: "closing within 30 days" means
// tenders one can still bid on, so an already-expired deadline is not a match
// however well it fits the arithmetic.
func extractDeadline(text string, now time.Time, filters *Filters) string {
	m := relativeDeadlinePattern.FindStringSubmatch(text)
	if m == nil {
		return text
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return text
	}

	unit := strings.ToLower(m[2])
	var until time.Time
	switch {
	case containsWord(dayWords, unit):
		until = now.AddDate(0, 0, n)
	case containsWord(weekWords, unit):
		until = now.AddDate(0, 0, 7*n)
	case containsWord(monthWords, unit):
		until = now.AddDate(0, n, 0)
	default:
		return text
	}

	from := now
	filters.DeadlineFrom = &from
	filters.DeadlineTo = &until
	return strings.Replace(text, m[0], " ", 1)
}

// ── Status ──

var openWords = []string{
	"aperti", "aperto", "in corso", // it
	"open", "ongoing", "current", "live", // en
	"ouverts", "ouvert", "en cours", // fr
	"offen", "offene", "laufend", // de
	"abiertos", "abierto", "en curso", // es
	"abertos", "aberto", // pt
	"otwarte", "otwarty", // pl
	"lopend", "lopende", // nl
}

var openStatusPattern = regexp.MustCompile(`(?i)\b(?:bandi|gare|tenders|notices|appels|ausschreibungen|licitaciones|concursos|przetargi|aanbestedingen)?\s*\b(?:` + alternation(openWords) + `)\b`)

// extractStatus recognises a request for tenders that can still be bid on.
//
// Only the "open" direction is recognised. Asking for awarded or cancelled
// notices is a research task that the status control expresses precisely,
// whereas "open" is what people type in prose constantly — and the words for
// the other states overlap far more with ordinary subject matter ("closed
// circuit", "cancelled flights").
func extractStatus(text string, filters *Filters) string {
	m := openStatusPattern.FindStringIndex(text)
	if m == nil {
		return text
	}
	filters.Statuses = []string{"open"}
	return text[:m[0]] + " " + text[m[1]:]
}

// ── helpers ──

func containsWord(words []string, w string) bool {
	for _, candidate := range words {
		if candidate == w {
			return true
		}
	}
	return false
}

var multiSpace = regexp.MustCompile(`\s+`)

// collapseSpaces tidies the gaps left where extracted phrases were removed.
func collapseSpaces(s string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(s, " "))
}
