package webdoc

// The fallback enumerator: what we read on a portal nobody has integrated.
//
// It exists because the platform backlog is a long tail with a fat head. Six
// platforms cover most Italian volume and the rest is dozens of one-off
// installations, each too small to justify an integration and collectively too
// large to ignore — so the choice for the tail is between a conservative
// general parser and nothing at all.
//
// # What it will catch
//
// Server-rendered listings whose files are ordinary anchors: a table or list of
// <a href="...pdf">, which is the shape of essentially every Italian
// amministrazione-trasparente page and of most small municipal portals. It also
// catches extension-less download endpoints, but only when the link's own words
// and the URL agree that it is a download — two independent signals, because
// either one alone is a site menu about as often as it is a file.
//
// # What it will NOT catch, stated plainly
//
//   - Listings rendered by JavaScript. There is no headless browser in this
//     design and adding one is a separate and much larger decision, so a page
//     whose file table arrives by XHR reads here as a page with no files on it.
//     eSOP/i-Faber, 8 notices in the measured sample, is exactly this.
//   - Files behind a form POST or a javascript:__doPostBack link, the ASP.NET
//     WebForms shape several Italian portals still use. There is no href to
//     resolve, so there is nothing to enumerate.
//   - Files one click further in, on a per-document detail page. This parser
//     reads one page and does not crawl; a listing of links to landing pages
//     that each hold one file yields nothing.
//   - Files served from a different host. Cross-host links are dropped by
//     scanLinks, because on these pages they are the transparency portal and
//     the AgID banners. A same-host link that REDIRECTS to a file CDN is fine —
//     that is resolved at fetch time, not here.
//   - Extension-less download endpoints whose anchor text is an icon and whose
//     URL says only /download/8842. One signal is not enough for the fallback,
//     and this is where a named platform parser earns its place: knowing the
//     platform is what makes that one signal sufficient.
//
// Every one of those is a page this parser reports as empty. THAT IS WHY AN
// EMPTY RESULT FROM THIS PARSER IS NOT A CLAIM THAT THE LISTING PUBLISHES
// NOTHING. It is a statement about this parser, and the caller is required to
// keep the two apart — see registry.go.

import (
	"net/url"
	"strings"
)

// genericParser is the ListingParser of last resort.
type genericParser struct{}

func (genericParser) Name() string { return PlatformGeneric }

// Detect always claims the page. The registry only ever reaches the fallback
// after every named parser has declined, so a conditional here would create a
// fourth outcome — "no parser at all" — that nothing downstream has a word for.
func (genericParser) Detect(Document) bool { return true }

func (g genericParser) Parse(page Document) ([]FileLink, error) {
	doc, base, err := parsePage(page)
	if err != nil {
		return nil, err
	}
	return scanLinks(doc, base, g.accept), nil
}

// accept is the fallback's acceptance rule, and it is deliberately asymmetric:
// one signal is enough when it is the strong one, and two are required
// otherwise.
//
// PRECISION OVER RECALL, and the asymmetry between the two failure modes is why.
// A missed document is a gap that shows up as a count and gets fixed by writing
// a parser for that platform. A fabricated one is worse in kind: its text is
// extracted, persisted against the tender, indexed, and eventually quoted back
// to a bid manager as that tender's requirements. Returning the privacy policy
// as a capitolato is not a smaller version of returning nothing.
func (genericParser) accept(u *url.URL, label string) bool {
	if documentExtensions[extensionOf(u)] {
		return true
	}
	return hasDocumentWord(label) && hasDownloadShape(u)
}

// documentWords are the words an Italian listing uses for the things we want,
// lowercased and cut to stems so that singular, plural and the compound forms
// ("allegato", "allegati", "allegato-a") all match one entry.
//
// They are matched against the assembled label — link text plus the rest of its
// table row — which is what makes the download-column portals reachable at all:
// the anchor there says only "Scarica", and the words that identify the file
// are in the sibling cell.
var documentWords = []string{
	"disciplinare",
	"capitolato",
	"bando",
	"avviso",
	"allegat",
	"documentazione",
	"documenti di gara",
	"modulistica",
	"modello",
	"modulo",
	"dgue",
	"elaborat",
	"planimetr",
	"contratto",
	"chiarimenti",
	"lettera di invito",
	"relazione",
	"computo",
	"elenco prezzi",
	"cronoprogramma",
	"duvri",
	"progetto",
	"determina",
	"scarica",
	"download",
}

// downloadTokens are the URL fragments that mean "this address returns a file".
//
// The list is short on purpose. "documento" and "file" were candidates and are
// deliberately absent: /gare/id42/documenti is the LISTING on several of these
// portals, not a file, so accepting it would spend a request per page fetching
// the page we are already on — and "file" is a substring of "profile".
var downloadTokens = []string{
	"download",
	"scarica",
	"getfile",
	"getdoc",
	"estraifile",
	"attach",
	"allegato/",
	"allegati/",
}

func hasDocumentWord(label string) bool {
	lower := strings.ToLower(label)
	for _, w := range documentWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func hasDownloadShape(u *url.URL) bool {
	target := strings.ToLower(u.Path)
	if u.RawQuery != "" {
		target += "?" + strings.ToLower(u.RawQuery)
	}
	for _, tok := range downloadTokens {
		if strings.Contains(target, tok) {
			return true
		}
	}
	return false
}
