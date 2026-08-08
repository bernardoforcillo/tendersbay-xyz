package retrieve

import (
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// Which of a listing's files are worth the cap, when there are more than
// MaxFilesPerTender of them.
//
// This is in core rather than in the enumerator because "which of these forty
// files answers where the points are won" is a product decision, not an HTML
// detail. The enumerator reports what a page links to; what matters among those
// links is the same question the whole pass exists to answer, and it is tuned
// against the same measurement — the share of retrieved tenders whose text
// actually contains scoring language, not the share that retrieved something.

// fileClass is a link's rank band. Lower sorts first.
type fileClass int

const (
	// classGoverning — the document that carries the scoring grid in the
	// overwhelming majority of Italian open procedures. If exactly one file can
	// be read, this is the one.
	classGoverning fileClass = iota

	// classCriteria — a file whose own name says it is about the criteria or
	// the technical offer. Rarer than the disciplinare and more specific, but
	// second because a buyer who publishes a separate criteria annex has
	// usually kept the grid out of the disciplinare.
	classCriteria

	// classNotice — the bando or the letter of invitation: reliably present,
	// reliably procedural, and only sometimes carrying weights.
	classNotice

	// classContract — the contract schema and the requirement documents. Real
	// content, rarely the grid.
	classContract

	// classOther — everything else, in published order.
	classOther
)

// rankKeywords are the keyword sets for the four named classes, in rank order.
//
// Italian first because the measured corpus is Italian, and untranslated
// because these are the words the documents are actually titled with — a
// portal's file is called "Disciplinare_di_gara.pdf", never "tender rules". A
// second country's corpus gets its own set appended here rather than a
// translation layer.
//
// Every entry is matched as a whole word (or whole phrase) against the link's
// label and its filename, folded to lowercase with punctuation, underscores and
// separators treated as spaces — which is what makes "Disciplinare_di_gara.pdf"
// and "DISCIPLINARE DI GARA" the same match. Short entries like "csa" (the
// standard abbreviation of capitolato speciale d'appalto) are why whole-word
// matching is used rather than plain substring: as a substring it would fire on
// any word that happens to contain those three letters.
var rankKeywords = [][]string{
	classGoverning: {"disciplinare", "capitolato", "csa", "cds"},
	classCriteria:  {"criteri", "criterio", "valutazione", "punteggi", "punteggio", "offerta tecnica"},
	classNotice:    {"bando", "avviso", "lettera di invito", "lettera invito"},
	classContract:  {"schema di contratto", "contratto", "condizioni", "requisiti"},
}

// rankFiles orders a listing's files by how likely each is to answer "where are
// the points won", then truncates to limit.
//
// The sort is STABLE, so within a class the published order is preserved —
// a buyer lists the governing document first far more often than not, and the
// page's own ordering is real information that a keyword list should refine
// rather than discard.
//
// Ranking is deliberately NOT a filter. An unmatched file is still fetched if
// there is room under the cap, so a listing whose labels are all "Allegato
// 1..8" degrades to published order rather than to nothing. The keyword list
// decides the ORDER when the cap binds, and nothing else.
func rankFiles(files []FileLink, limit int) []FileLink {
	ranked := make([]FileLink, len(files))
	copy(ranked, files)

	classes := make(map[string]fileClass, len(ranked))
	for _, f := range ranked {
		classes[f.URL] = classify(f)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return classes[ranked[i].URL] < classes[ranked[j].URL]
	})

	if limit >= 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// classify assigns one link its rank band, reading its label and its filename
// together.
//
// Both are needed. The label is what the page calls the file and is often the
// only place the document type is written ("Documentazione di gara" in one
// table cell, "Scarica" in the anchor); the filename is what survives on a
// portal whose every link says only "Download". Either one matching is enough,
// because a false promotion costs a slot in a fifteen-file budget while a
// missed one can cost the only file that mattered.
func classify(f FileLink) fileClass {
	haystack := foldWords(f.Label) + foldWords(filenameOf(f.URL))
	for class, keywords := range rankKeywords {
		for _, kw := range keywords {
			if strings.Contains(haystack, " "+kw+" ") {
				return fileClass(class)
			}
		}
	}
	return classOther
}

// foldWords lowercases s and reduces everything that is not a letter or a digit
// to a single space, returning the result padded with a space at each end so a
// caller can test for a whole word with a plain Contains.
//
// Padding rather than tokenising keeps multi-word phrases ("offerta tecnica")
// matchable by exactly the same test as single words, which is why the keyword
// table can hold both without a second code path.
func foldWords(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte(' ')
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}

// filenameOf returns the last path segment of a URL, which is the half of the
// ranking signal the page's own words do not carry.
func filenameOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := u.Path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	if unescaped, err := url.PathUnescape(path); err == nil {
		return unescaped
	}
	return path
}

// dedupeFiles drops links that name the same document once session parameters
// are stripped, keeping the first published occurrence of each.
//
// It runs on the NORMALISED form but keeps the link VERBATIM, because those are
// the two different addresses this pass deals in: the normalised one is what
// identifies a document across runs, and the published one is what actually has
// to be requested. Listings duplicate links routinely — the same file appears
// as a row in the table and again as an icon in a sidebar — and every duplicate
// that survives here is a request spent twice and a slot taken from a file that
// was never fetched at all.
func dedupeFiles(files []FileLink) []FileLink {
	seen := make(map[string]bool, len(files))
	out := make([]FileLink, 0, len(files))
	for _, f := range files {
		key := normaliseURL(f.URL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
