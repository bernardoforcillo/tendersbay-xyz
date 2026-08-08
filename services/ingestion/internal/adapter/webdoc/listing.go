package webdoc

// Turning one fetched listing page into the set of files it publishes.
//
// This is the piece the probe found to be the actual work. `documents_url` does
// not land on a capitolato; it lands on a procedure index page, and every file
// that matters is one click further in. So the fetcher above is only half a
// retrieval — the other half is reading a page written for a human and
// recovering the list of documents it offers them.
//
// Everything in this file and the parsers beside it is HTML SHAPE, and nothing
// in it is product judgement. A parser reports what a page links to and which
// platform it recognised. It never decides what an empty result means: an empty
// set from a named platform parser and an empty set from the fallback are
// different facts, and turning either of them into a status is the core layer's
// call, not this one's. See registry.go, where that distinction is preserved
// rather than collapsed.

import (
	"net/url"
	"path"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"
)

// PlatformGeneric is the name the fallback enumerator reports.
//
// It is a value in the platform column like any other, and deliberately not an
// empty string or a NULL, because "we did not recognise this portal" is the
// single most useful thing a platform-backlog query can be told. Grouping by
// platform then answers "where is the fallback already working, and where is it
// finding nothing" — which is the ranked, measured list of what to integrate
// next, instead of a sampled guess.
const PlatformGeneric = retrieve.PlatformGeneric

// FileLink is one downloadable document a listing page publishes, and Listing
// is what one page yielded: the files, and the platform whose parser produced
// them.
//
// Both are the core layer's types rather than parallel ones, so Registry
// satisfies retrieve.ListingEnumerator by construction — see outcome.go for why
// aliases and not a translation layer. Two details of what the parsers here
// produce are worth stating where the parsers are, rather than only on the core
// type:
//
//   - A FileLink URL is left VERBATIM apart from its fragment. Session-parameter
//     normalisation is the orchestrator's, done once immediately before the URL
//     is fetched or stored, because it exists to keep persisted rows stable and
//     doing it here as well would mean two places had to agree on the rule
//     forever.
//   - A Label is assembled from the anchor AND its table row. Italian portals
//     put the file name in one column and the document type in another —
//     "Disciplinare di gara" and "Documentazione di gara" are two different
//     cells — so taking only the anchor text throws away half of what the page
//     said, and on the portals whose download column reads only "Scarica" it
//     throws away all of it.
type (
	FileLink = retrieve.FileLink
	Listing  = retrieve.Listing
)

// ListingParser enumerates the tender-document links one portal PLATFORM
// publishes.
//
// Keyed on platform, never on buyer, and that is the whole reason this is a
// tractable backlog rather than an impossibility. Italian portals are
// white-label software resold to hundreds of authorities: the measured sample
// of 60 notices found Maggioli's Portale Appalti alone under seven distinct
// subdomains, so one parser is one integration covering every buyer that runs
// it. Six platforms cover most of the country's volume.
type ListingParser interface {
	// Name is the stable platform id. It is persisted and it is a grouping key
	// in every backlog query ever written against that column, so it does not
	// change once it has shipped.
	Name() string

	// Detect reports whether this parser recognises the page.
	//
	// It matches on MARKUP FINGERPRINTS FIRST and on hostname only as a
	// corroborating signal, because hostname is precisely what does not
	// generalise here: 33 of the 60 measured document URLs resolve to buyers'
	// own domains and cannot be classified from the host at all.
	Detect(page Document) bool

	// Parse enumerates the files, in the order the page published them.
	//
	// It returns an error only when the page could not be read as a document at
	// all. A page with no documents on it is not an error — it is an empty
	// slice, and what that means is decided a layer up.
	Parse(page Document) ([]FileLink, error)
}

// documentExtensions is the high-precision half of every parser's acceptance
// rule: a link whose path ends in one of these is a file, with no second signal
// needed.
//
// The archive formats are here even though this phase cannot read them.
// Enumerating a .p7m and failing to extract it is a MEASUREMENT — the gap
// between files found and files extracted is the evidence that decides whether
// CAdES unwrapping is worth building — whereas not enumerating it at all would
// make a tender whose disciplinare arrives only as a signed archive
// indistinguishable from a tender that published nothing.
var documentExtensions = map[string]bool{
	"pdf":  true,
	"doc":  true,
	"docx": true,
	"rtf":  true,
	"odt":  true,
	"xls":  true,
	"xlsx": true,
	"ods":  true,
	"zip":  true,
	"7z":   true,
	"p7m":  true,
}

// maxLabelLen bounds an assembled label. A table row can carry a description
// cell of several hundred words, and a label is a ranking signal rather than
// content — the words that decide "is this the disciplinare" are in the first
// few, and carrying the rest would put a paragraph of a page into memory for
// every link on it.
const maxLabelLen = 300

// scanLinks walks the subtree rooted at root and returns every anchor that
// accept claims, in document order, deduplicated by resolved URL.
//
// Three filters are applied before accept ever sees a link, and all three are
// refusals rather than judgements:
//
//   - Site chrome is skipped whole (isChromeRegion). A portal's <nav> and
//     <footer> link to the amministrazione-trasparente section, the privacy
//     policy and the accessibility statement, several of which are PDFs — so
//     without this the highest-precision signal there is, "the path ends in
//     .pdf", would return the site menu as tender documents on every page.
//   - Cross-host links are dropped (resolveLink). On an Italian procurement
//     listing these are the transparency portal, AgID banners and analytics,
//     not tender documents. A same-host link that redirects to a file CDN still
//     works: the redirect is followed at fetch time, where the scheme, robots
//     and dial checks are re-run for the new host.
//   - Non-http(s) schemes are dropped, which is also what removes javascript:
//     and mailto: without either needing to be named.
func scanLinks(root *html.Node, base *url.URL, accept func(u *url.URL, label string) bool) []FileLink {
	var out []FileLink
	seen := map[string]bool{}

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if isChromeRegion(n) {
				return
			}
			if n.DataAtom == atom.A {
				if u, ok := resolveLink(base, attrValue(n, "href")); ok {
					label := labelFor(n)
					if accept(u, label) {
						if s := u.String(); !seen[s] {
							seen[s] = true
							out = append(out, FileLink{URL: s, Label: label, Ext: extensionOf(u)})
						}
					}
				}
				// Anchors do not nest, so there is nothing below this one worth
				// descending into.
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// chromeRoles are the ARIA landmark roles that mean "this region is the site,
// not the page". They are checked alongside the HTML5 tags because portals
// built on older templates mark their chrome with roles on plain divs, and a
// tag-only check would walk straight into those menus.
var chromeRoles = map[string]bool{
	"navigation":  true,
	"banner":      true,
	"contentinfo": true,
	"search":      true,
}

// isChromeRegion reports whether a node roots a region of site furniture whose
// links are never tender documents.
func isChromeRegion(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Nav, atom.Header, atom.Footer, atom.Script, atom.Style, atom.Template:
		return true
	}
	return chromeRoles[strings.ToLower(strings.TrimSpace(attrValue(n, "role")))]
}

// resolveLink turns one href into the absolute address it names, or reports
// that this link is not one this package will follow.
func resolveLink(base *url.URL, href string) (*url.URL, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return nil, false
	}
	u, err := base.Parse(href)
	if err != nil {
		return nil, false
	}
	if !allowedSchemes[strings.ToLower(u.Scheme)] {
		return nil, false
	}
	if !strings.EqualFold(u.Host, base.Host) {
		return nil, false
	}
	u.Fragment, u.RawFragment = "", ""
	return u, true
}

// pageExtensions name a request HANDLER rather than a file, and reporting one
// as a document extension would be reporting the server's implementation as the
// format of a document we have not seen.
//
// This is not a hypothetical tidiness: these portals are Java and .NET
// applications, so their download endpoints are /asset/estraiFile.do and
// /eprocdata/downloadDocument.xhtml, which have a "last dot" and no file
// extension. ".wp" is on the list because it is Maggioli's, recorded in a real
// notice's URL — /PortaleAppalti/it/ppgare_bandi_lista.wp.
var pageExtensions = map[string]bool{
	"do": true, "action": true, "faces": true,
	"jsp": true, "jspx": true, "jspa": true, "xhtml": true,
	"asp": true, "aspx": true, "ashx": true, "axd": true,
	"php": true, "php5": true, "cgi": true, "pl": true,
	"html": true, "htm": true, "shtml": true, "wp": true,
}

// extensionOf returns the document extension the URL path claims, lowercased
// and without its dot, or "".
//
// The length and character bounds matter more than they look: a procedure slug
// like /gare/id42-comune.di.esempio has a "last dot" that path.Ext will happily
// return, and reporting "esempio" as a file extension would put nonsense in a
// field that later gets grouped by.
func extensionOf(u *url.URL) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(u.Path)), ".")
	if ext == "" || len(ext) > 6 || pageExtensions[ext] {
		return ""
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return ext
}

// labelFor assembles the text a page gives one link.
//
// The anchor's own text comes first because it is what a human reads as the
// file's name. The fallbacks after it are for the icon-only download links that
// are common on these portals, where the anchor's text is an <img> or a glyph
// font and the only words are in an attribute.
func labelFor(a *html.Node) string {
	label := nodeText(a)
	if label == "" {
		label = collapse(attrValue(a, "title"))
	}
	if label == "" {
		label = collapse(attrValue(a, "aria-label"))
	}
	return withRowContext(a, label)
}

// withRowContext augments a link's label with the rest of its table row.
//
// This is the one piece of shape knowledge that is true of essentially every
// server-rendered Italian listing, and it is why parsers here are row-aware
// rather than anchor-aware. The page presents a table: one column is the file
// name ("Disciplinare_di_gara.pdf"), another is the document type
// ("Documentazione di gara"), often a third is a size or a publication date.
// The words that decide whether this is the file carrying the scoring grid are
// split across those columns, so a label taken from the anchor alone throws
// away the discriminating half — and on the portals whose download column reads
// only "Scarica", it throws away all of it.
func withRowContext(a *html.Node, label string) string {
	row := ancestorWithAtom(a, atom.Tr)
	if row == nil {
		return truncateLabel(label)
	}

	parts := make([]string, 0, 4)
	if label != "" {
		parts = append(parts, label)
	}
	for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
		if cell.Type != html.ElementNode {
			continue
		}
		if cell.DataAtom != atom.Td && cell.DataAtom != atom.Th {
			continue
		}
		// The anchor's own cell is skipped: its text is already the label, and
		// a cell holding nothing but the link would otherwise repeat it.
		if isAncestor(cell, a) {
			continue
		}
		// A cell with no letters and no digits is an icon column — the ⭳ or ▸
		// glyph these tables put beside every row — and carrying it into the
		// label would add a character that no keyword will ever match to every
		// file on the page.
		text := nodeText(cell)
		if text == "" || !hasWordChar(text) || slices.Contains(parts, text) {
			continue
		}
		parts = append(parts, text)
	}
	return truncateLabel(strings.Join(parts, " — "))
}

// hasWordChar reports whether s carries at least one letter or digit in any
// script, which is this package's test for "these are words rather than
// decoration".
func hasWordChar(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// truncateLabel bounds a label on a rune boundary, so a cut never produces the
// invalid UTF-8 that would then have to be handled by everything downstream.
func truncateLabel(s string) string {
	if len(s) <= maxLabelLen {
		return s
	}
	cut := maxLabelLen
	for cut > 0 && !utf8Boundary(s, cut) {
		cut--
	}
	return strings.TrimSpace(s[:cut])
}

// utf8Boundary reports whether index i starts a rune in s.
func utf8Boundary(s string, i int) bool {
	return i >= len(s) || s[i]&0xC0 != 0x80
}

// nodeText returns the visible text of a subtree with its whitespace collapsed,
// skipping the regions whose text is markup rather than content.
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
			return
		case html.ElementNode:
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Template:
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		// A block boundary is a word boundary. Without this, a two-line cell
		// renders as "DisciplinareDocumentazione di gara" and every keyword the
		// ranking looks for straddles a join that is not there on the page.
		if n.Type == html.ElementNode {
			b.WriteByte(' ')
		}
	}
	walk(n)
	return collapse(b.String())
}

// collapse reduces every run of whitespace to a single space and trims the
// ends. strings.Fields splits on unicode.IsSpace, which includes U+00A0 — and
// that matters rather more than it sounds: these pages are built out of
// &nbsp;, so without folding it a label reads as one unsplittable word and
// matches none of the keywords ranking looks for.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// attrValue returns the named attribute's value, or "".
func attrValue(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// ancestorWithAtom returns the nearest ancestor of n with the given tag, or nil.
func ancestorWithAtom(n *html.Node, a atom.Atom) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.DataAtom == a {
			return p
		}
	}
	return nil
}

// isAncestor reports whether ancestor is at or above n in the tree.
func isAncestor(ancestor, n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// findNode returns the first node in document order for which match reports
// true, skipping site chrome so a container search cannot land inside a menu.
func findNode(root *html.Node, match func(*html.Node) bool) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode {
			if isChromeRegion(n) {
				return
			}
			if match(n) {
				found = n
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// containsFold reports whether needle — which must already be lowercase —
// occurs in data under ASCII case folding.
//
// It exists instead of bytes.Contains(bytes.ToLower(data), needle) because
// detection runs every registered parser's fingerprints over a response that
// may be megabytes, and allocating a lowercased copy of the whole page once per
// parser is a cost with no purpose. Portal markup mixes cases freely — class
// names lowercase, "Powered by TuttoGare" in a footer capitalised — so folding
// is required, and only ASCII folding is, because these fingerprints are
// product names and URL fragments.
func containsFold(data []byte, needle string) bool {
	n := len(needle)
	if n == 0 {
		return true
	}
	if len(data) < n {
		return false
	}
	lower := needle[0]
	upper := lower
	if lower >= 'a' && lower <= 'z' {
		upper = lower - 'a' + 'A'
	}
	for i := 0; i+n <= len(data); i++ {
		if c := data[i]; c != lower && c != upper {
			continue
		}
		if equalFoldASCII(data[i:i+n], needle) {
			return true
		}
	}
	return false
}

// equalFoldASCII compares a byte slice with an already-lowercased string.
func equalFoldASCII(data []byte, lowered string) bool {
	for i := 0; i < len(lowered); i++ {
		c := data[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lowered[i] {
			return false
		}
	}
	return true
}
