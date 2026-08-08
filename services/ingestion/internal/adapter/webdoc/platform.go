package webdoc

// The table-driven listing parser every named platform is built from.
//
// A platform integration here is a DESCRIPTION, not a program: the fingerprints
// that recognise the page, an optional hint at the region of the page that
// holds the file table, and the shapes of that platform's download URLs. The
// walking, labelling, resolving and deduplicating are shared and are exercised
// by this package's tests against fixtures; a platform entry adds no control
// flow of its own.
//
// That split is deliberate and it is about who has to be right about what. The
// shared half is verifiable offline and stays verified. The per-platform half
// is a claim about markup nobody in this repository has captured yet — see the
// warning at the head of sintel.go and tuttogare.go — and when the first real
// capture contradicts it, correcting the platform is editing a table rather
// than rewriting a parser.

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// platformSpec describes one white-label portal platform.
type platformSpec struct {
	// name is the stable platform id, persisted and grouped by. It does not
	// change once shipped.
	name string

	// markers are lowercase markup fingerprints, any one of which identifies
	// the platform. These are the PRIMARY signal, and the ordering is the whole
	// point of the sample: 33 of the 60 measured document URLs sit on buyers'
	// own domains, so a platform that can only be recognised by hostname is a
	// platform we would fail to recognise on more than half its pages.
	markers []string

	// pathPatterns are URL path shapes this platform serves its procedure pages
	// under. They are matched against the decoded path only, never the query,
	// because a session parameter is not a shape.
	pathPatterns []*regexp.Regexp

	// hostSuffixes are the hosts the platform's operator runs itself. This is
	// the weakest signal and it is last: it identifies the single-tenant
	// installations (one operator, one address, many buyers) and says nothing
	// about the white-label ones.
	hostSuffixes []string

	// container narrows the scan to the region of the page holding the file
	// table. It is an optimisation for precision and it can never cost recall:
	// when it matches nothing, or matches a region with no files in it, Parse
	// re-scans the whole page (see Parse).
	container nodeMatch

	// downloadPatterns are the shapes of this platform's EXTENSION-LESS
	// download endpoints, matched against path and query together. They are the
	// reason a named parser beats the fallback on these portals at all: the
	// generic parser refuses a link with no file extension unless two
	// independent signals agree, and a great many portals serve every
	// attachment from /download/8842 with the anchor text "Scarica". Knowing
	// the platform is what makes one signal enough.
	downloadPatterns []*regexp.Regexp
}

// nodeMatch selects an element by tag and by a substring of its id or class.
//
// It matches on SUBSTRINGS rather than exact class tokens on purpose. An exact
// token is a fact about one release of one platform's templates, and pinning
// one from an uncaptured page would be inventing a constant; "the element whose
// class or id mentions documenti or allegati" is a claim about how these
// portals are written that survives a template change and is honest about being
// a heuristic.
type nodeMatch struct {
	// tags are the element names to consider, any of. Empty means any element.
	tags []string
	// tokens are lowercase substrings, any of which matching the element's id
	// or class attribute selects it. Empty means the match is unused.
	tokens []string
}

func (m nodeMatch) unused() bool { return len(m.tokens) == 0 }

func (m nodeMatch) matches(n *html.Node) bool {
	if m.unused() {
		return false
	}
	if len(m.tags) > 0 {
		name := strings.ToLower(n.Data)
		if !containsString(m.tags, name) {
			return false
		}
	}
	attrs := strings.ToLower(attrValue(n, "id") + " " + attrValue(n, "class"))
	for _, tok := range m.tokens {
		if strings.Contains(attrs, tok) {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// platformParser is a ListingParser built from one platformSpec.
type platformParser struct{ spec platformSpec }

func (p platformParser) Name() string { return p.spec.name }

// Detect reports whether this page belongs to the platform, checking the three
// signals in decreasing order of how well each generalises across buyers.
//
// Any one signal is enough. The lists are each narrow enough that a false
// positive is unlikely, and the consequence of one is contained a layer up:
// Registry.Enumerate refuses to let a parser that claimed a page and then found
// nothing on it turn that into a claim that the page publishes nothing. Without
// that guard an over-eager fingerprint here would produce the exact lie this
// whole package exists to prevent, so the two are designed together.
func (p platformParser) Detect(page Document) bool {
	for _, marker := range p.spec.markers {
		if containsFold(page.Body, marker) {
			return true
		}
	}

	u, err := url.Parse(page.URL)
	if err != nil {
		return false
	}
	for _, re := range p.spec.pathPatterns {
		if re.MatchString(u.Path) {
			return true
		}
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range p.spec.hostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// Parse enumerates the platform's file links.
//
// The container is tried first and the whole page second, and that order is a
// precision-then-recall ladder rather than a fallback for a bug. A page whose
// file table was found gives up only the links in it; a page whose markup has
// moved since this entry was written degrades to the broader scan instead of to
// silence. Degrading toward finding MORE is the safe direction here: acceptance
// is still governed by the extension list and this platform's download shapes,
// so the broader scan cannot invent a link, whereas silence would be reported
// as a listing that publishes nothing.
func (p platformParser) Parse(page Document) ([]FileLink, error) {
	doc, base, err := parsePage(page)
	if err != nil {
		return nil, err
	}

	if !p.spec.container.unused() {
		if node := findNode(doc, p.spec.container.matches); node != nil {
			if files := scanLinks(node, base, p.accept); len(files) > 0 {
				return files, nil
			}
		}
	}
	return scanLinks(doc, base, p.accept), nil
}

// accept is this platform's acceptance rule: a known document extension, or a
// URL matching one of its download endpoint shapes.
func (p platformParser) accept(u *url.URL, _ string) bool {
	if documentExtensions[extensionOf(u)] {
		return true
	}
	target := u.Path
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	for _, re := range p.spec.downloadPatterns {
		if re.MatchString(target) {
			return true
		}
	}
	return false
}

// parsePage parses a listing's markup and the address to resolve its links
// against.
//
// The base is page.URL, which by contract is the address the bytes actually
// came FROM rather than the one that was requested. That distinction is load
// bearing here and nowhere else: a listing reached through a 302 from /gare to
// /gare/ resolves every relative link one path segment differently, so using
// the requested address would produce a full set of plausible, absolute,
// entirely wrong URLs.
func parsePage(page Document) (*html.Node, *url.URL, error) {
	if !page.IsHTML() {
		return nil, nil, fmt.Errorf("webdoc: enumerate: %s is %s, which is a file to extract rather than a page to enumerate",
			page.Filename, page.ContentType)
	}
	base, err := url.Parse(page.URL)
	if err != nil {
		// The parse error text would quote the whole address, session query
		// string included; this package's errors carry a host and never a full
		// URL.
		return nil, nil, fmt.Errorf("webdoc: enumerate: the page's own address is unparseable")
	}
	doc, err := html.Parse(bytes.NewReader(page.Body))
	if err != nil {
		return nil, nil, fmt.Errorf("webdoc: enumerate %s: %w", base.Host, err)
	}
	return doc, base, nil
}
