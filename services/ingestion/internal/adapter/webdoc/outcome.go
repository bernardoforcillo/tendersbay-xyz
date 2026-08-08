package webdoc

import (
	"mime"
	"net/url"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"
)

// The error contract, and it is the whole interface between this package's
// machinery and the caller's bookkeeping.
//
// Every name below is an ALIAS of the corresponding value in
// internal/core/retrieve, not a second definition of it, and the direction is
// the house dependency rule: the core layer owns which failures are terminal
// and which docs_status a human is shown, because those are decisions about
// what this product tells a bid manager; this package owns recognising them in
// an HTTP response. Two independent sets of sentinels would mean a translation
// table that has to be kept in step forever, and the first time it slipped, a
// portal's refusal would be filed as an outage.
//
// They are re-exported rather than merely used because they ARE this package's
// contract: a reader of the fetcher needs to find the whole error vocabulary
// where the machinery that raises it lives, and every one of the reasons below
// is a statement about HTTP.
//
// Every sentinel below except ErrRobotsUnknown is TERMINAL for the address it
// was raised on: a decision a later pass cannot improve on without something
// changing on the buyer's side, so retrying it costs a third party requests for
// an answer we already have. Everything else — ErrRobotsUnknown, a transport
// error, a 5xx that outlived its retries, an unclassified status — is
// TRANSIENT.
//
// Unclassified meaning transient is the safe way round, and it is the same way
// round as enrich.NoticeFetcher: a new failure mode nobody has named yet costs
// a bounded number of retries, where defaulting to terminal would silently park
// rows that were only briefly unreachable.
//
// The sentinels are not interchangeable and must not be collapsed into a
// generic "could not fetch". Each one is a different sentence to a human — "the
// portal forbids automated clients", "log in here", "solve a captcha", "the
// buyer withdrew the file", "that address is not on the public internet" — and
// the package comment's invariant is exactly the rule that they stay apart.
var (
	// ErrRobotsDenied — the host's robots.txt disallows this path for our user
	// agent. Terminal, and deliberately not retried: asking again is itself the
	// impoliteness. robots.txt files do change, so re-checking is a deliberate
	// manual operation rather than a schedule.
	ErrRobotsDenied = retrieve.ErrRobotsDenied

	// ErrRobotsUnknown — the robots.txt could not be read: a 5xx, a timeout, a
	// TLS failure, or a file over the size bound. THE ONE THAT MUST NOT BE
	// FOLDED INTO ErrRobotsDenied. A robots.txt we could not read is not
	// permission and it is not a refusal; it is an absence of information, and
	// it is transient. Merging the two would file a struggling server under
	// "this portal forbids us", which is the most misleading row the
	// platform-backlog query could contain.
	ErrRobotsUnknown = retrieve.ErrRobotsUnknown

	// ErrCaptcha — the server answered 200 with a human-check interstitial
	// rather than the document. Terminal. A 200 that is a captcha is not the
	// document, and reporting it as one is the exact confusion this package
	// exists to remove.
	ErrCaptcha = retrieve.ErrCaptcha

	// ErrAuthRequired — 401 or 403, or a 200 that is a login form. Terminal,
	// and an honest, useful answer: "log in here" is actionable where "no
	// documents" is a lie. Expected for the marketplace-shaped portals.
	ErrAuthRequired = retrieve.ErrAuthRequired

	// ErrNotFound — 404 or 410. Terminal, for the reason document.Fetch already
	// treats a 404 as terminal: it is a final answer, and treating absence as
	// transient is the shortest path to an infinite loop against a server that
	// has already answered.
	ErrNotFound = retrieve.ErrNotFound

	// ErrUnusable — a response arrived that this pass can never use: over the
	// byte bound, or a redirect chain that never terminated. Terminal, and kept
	// separate from a plain fetch failure so an oversized file is downloaded
	// once and parked rather than downloaded three times.
	ErrUnusable = retrieve.ErrUnusable

	// ErrAddressRefused — the URL's scheme is not http/https, or the address it
	// resolved to is not public unicast. Terminal. It is a separate sentinel
	// from ErrUnusable on purpose: "we refused to dial this" and "the file is
	// too big" are different facts, and a column that conflates them cannot be
	// used to tell a misconfigured notice from a hostile one. In production it
	// should stay at zero — the queue's addresses come from TED's own register
	// — so any occurrence is a signal worth reading.
	ErrAddressRefused = retrieve.ErrAddressRefused
)

// Terminal reports whether err is one this package will never resolve by trying
// again. It exists so the caller's queue bookkeeping has one place to ask,
// rather than re-deriving the classification from a list of sentinels that will
// grow.
//
// Note what is NOT here: ErrRobotsUnknown. That omission is the contract.
func Terminal(err error) bool { return retrieve.Terminal(err) }

// Document is one retrieved resource, and it is the core layer's type rather
// than a parallel one.
//
// Body is the bytes, and nothing in this package or below it ever persists
// them: text is extracted (extract.go) and the bytes are dropped when this
// value goes out of scope. That is this feature's central storage decision, and
// it is expressed in the type — where it is visible to everyone who touches a
// retrieved file — rather than as a comment on a storage layer that
// deliberately does not exist.
//
// An alias rather than a struct of the same shape, for the reason the sentinels
// above are aliases: it is what makes Client, Registry and Extractor satisfy the
// core ports by construction, with no translation layer that could drift. The
// field documentation lives on retrieve.Document.
type Document = retrieve.Document

// isTextual reports whether a content type carries text a marker scan can be
// run over. Forked from tedmcp along with the human-check detection it gates.
func isTextual(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "json") ||
		strings.Contains(ct, "html")
}

// filenameFrom picks a name for the document, preferring the one the server
// states and falling back to the last path segment. Forked from tedmcp.
func filenameFrom(disposition string, u *url.URL) string {
	if disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if name := params["filename"]; name != "" {
				return name
			}
		}
	}
	path := u.EscapedPath()
	if i := strings.LastIndexByte(path, '/'); i >= 0 && i+1 < len(path) {
		if name, err := url.PathUnescape(path[i+1:]); err == nil {
			return name
		}
		return path[i+1:]
	}
	return ""
}

// requestPath is the path robots.txt rules are matched against: always rooted,
// and including the query string, which some rules key on. Forked from tedmcp.
func requestPath(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path
}
