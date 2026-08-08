// Package retrieve is the core of the portal-document retrieval pass: for every
// enriched tender whose own notice carried no usable award grid, follow the
// buyer's documents link, enumerate the files that page publishes, extract
// their text, and record both how much of it we got and — when we got none —
// why not. It defines the ports the adapters implement (fetching, enumeration,
// extraction, persistence) and the orchestrator that drives them; it imports
// nothing from internal/adapter, and nothing here knows what HTTP, HTML or
// Postgres are.
//
// # The trigger, and why it cannot run early
//
// The queue is grid_usable = false: a notice we HAVE read, whose award grid
// carries no weights. grid_usable is three-valued and NULL means "not enriched
// yet", so a row the enricher has not looked at cannot enter this queue at all.
// The ordering between the two passes is enforced by the data rather than by a
// scheduler, and it survives one job being paused for a day. Do not widen the
// predicate to IS NOT TRUE.
//
// # Coverage and cause are different questions
//
// This pass answers two things about every tender, and it must never let one
// answer stand in for the other:
//
//   - COVERAGE — how many files the listing published, and how many of them we
//     could read. Two counts, reported as counts, because "full coverage" is
//     not observable: nothing on a listing page says whether it showed
//     everything the buyer published.
//   - CAUSE — one status out of a closed vocabulary of eight, saying what the
//     last look actually produced.
//
// The invariant that ties them together is inherited from the fetching adapter
// and is the reason this pass exists rather than a "download the PDF" helper:
// AN INACCESSIBLE FILE IS NEVER MISTAKEN FOR AN ABSENT ONE. A portal that
// forbids us, one that demands a login, and one that genuinely publishes
// nothing are three different sentences to a bid manager, actionable by three
// different people, and collapsing any of them into "no documents" is a lie
// this service would then repeat.
//
// StatusNoFiles carries the whole of that rule. It is a POSITIVE CLAIM about
// absence, and only a named platform parser is allowed to produce it; the
// generic fallback finding nothing means the fallback could not read the page,
// which is StatusTraversalFailed. The distinction is asserted mechanically in
// this package's tests, not merely described here.
//
// # The rule that keeps the unguarded fetcher away from buyer portals
//
// A tender-document row is written IF AND ONLY IF its text was successfully
// extracted. That is not tidiness. The indexer reuses previously-extracted
// parts and, finding none, fetches the document's URL itself — with the fetcher
// in internal/adapter/document, which has no robots handling, no per-host
// pacing and no dial guard. A document row with zero parts is therefore a buyer
// portal URL handed to that fetcher on every indexing cycle, forever. A scanned
// PDF with no text layer would produce exactly one, and there will be many:
// this service builds with CGO_ENABLED=0, so there is no OCR path.
//
// So this pass extracts text itself rather than enqueueing URLs for the indexer,
// and a file that yields nothing is counted and dropped, never persisted.
//
// # The bytes are not stored, anywhere
//
// There is no object storage in this design and no blob column. A retrieved
// file exists as one Document value and, for the length of one extraction, one
// temp file on the adapter side; then it is gone. What survives is its URL, so
// a human can open it, and its extracted parts, so an answer can cite it. The
// decision is expressed in the Document type below rather than as a comment on
// a storage layer that deliberately does not exist.
package retrieve

import (
	"context"
	"errors"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// The docs_status vocabulary, and the whole of it. A row's status says what the
// last look at its buyer portal produced; combined with docs_fetched_at
// (NULL = still queued) it is the complete record of what this pass knows.
//
// Only StatusUnreachable is normally written while a row stays queued — it
// means "the last attempt failed transiently" and becomes terminal once the
// attempt budget is spent. The one exception is an address this service refuses
// to dial at all, which is written as StatusUnreachable and parked immediately;
// docs_attempts tells the two apart, since a parked refusal shows 1 attempt
// where a spent budget shows maxAttempts.
const (
	// StatusOK — at least one file's text was extracted and persisted.
	StatusOK = "ok"

	// StatusNoFiles — a NAMED PLATFORM PARSER read the listing and it publishes
	// no files. The only status in this list that is a positive claim about
	// absence, and the only one the generic fallback may never produce. See the
	// package comment.
	StatusNoFiles = "no_files"

	// StatusRobotsDenied — the host's robots.txt disallows this path for our
	// agent. Terminal, and deliberately never retried on a schedule: asking
	// again is itself the impoliteness. robots.txt files do change, so
	// re-checking is a manual operation.
	StatusRobotsDenied = "robots_denied"

	// StatusCaptcha — a 200 that is a human-check interstitial rather than the
	// document. Terminal.
	StatusCaptcha = "captcha"

	// StatusAuthRequired — 401/403, or a login form served in place of the
	// listing. Terminal, and an honest, useful answer: "log in here" is
	// actionable where "no documents" is a lie. Expected for the
	// marketplace-shaped Italian platforms.
	StatusAuthRequired = "auth_required"

	// StatusTraversalFailed — the page was fetched but no parser could
	// enumerate it. An ADMISSION, not a claim: we know nothing about what that
	// listing publishes. Terminal, because the same page parses the same way on
	// every pass — what changes it is a new parser shipping, not a retry.
	StatusTraversalFailed = "traversal_failed"

	// StatusExtractFailed — files were found and fetched but none yielded
	// usable text. Terminal for the same determinism reason, and the population
	// that decides whether .p7m and .zip unwrapping is worth building: it is
	// the one non-ok status that still carries both counts.
	StatusExtractFailed = "extract_failed"

	// StatusUnreachable — a transport error, a 5xx, a 429, a 404 on the entry
	// page, a robots.txt we could not read, or an address this service refuses
	// to dial. Transient by default; becomes terminal once the attempt budget
	// is spent.
	StatusUnreachable = "unreachable"
)

// PlatformGeneric is the name the fallback enumerator reports.
//
// It is a value in docs_platform like any other, and deliberately not an empty
// string, because "we did not recognise this portal" is the single most useful
// thing a platform-backlog query can be told. Grouping by platform then answers
// "where is the fallback already working, and where is it finding nothing" —
// the ranked, measured list of what to integrate next, rather than a sampled
// guess.
const PlatformGeneric = "generic"

// The error contract between the fetching adapter and this package's
// bookkeeping, and it is the whole interface to that machinery.
//
// Every sentinel here except ErrRobotsUnknown is TERMINAL for the address it
// was raised on: a decision a later pass cannot improve on without something
// changing on the buyer's side, so retrying costs a third party requests for an
// answer we already have. Everything else — ErrRobotsUnknown, a transport
// error, an unclassified failure — is TRANSIENT.
//
// Unclassified meaning transient is the safe way round, and it is the same way
// round as enrich.NoticeFetcher: a new failure mode nobody has named costs a
// bounded number of retries, where defaulting to terminal would silently park
// rows that were only briefly unreachable.
//
// The sentinels live here rather than in the adapter because which of them is
// terminal, and which docs_status each one becomes, are product decisions about
// what a human is told. The adapter maps HTTP-level outcomes onto them and
// stops there.
var (
	// ErrRobotsDenied — robots.txt disallows this path for our user agent.
	ErrRobotsDenied = errors.New("retrieve: robots.txt denies this path for our agent")

	// ErrRobotsUnknown — robots.txt could not be read: a 5xx, a timeout, a TLS
	// failure, a file over the size bound. THE ONE THAT MUST NOT BE FOLDED INTO
	// ErrRobotsDenied. A robots.txt we could not read is not permission and it
	// is not a refusal; it is an absence of information, and it is transient.
	// Merging the two would file a struggling server under "this portal forbids
	// us", which is the most misleading row the platform-backlog query could
	// contain.
	ErrRobotsUnknown = errors.New("retrieve: robots.txt could not be read")

	// ErrCaptcha — a 200 carrying a human-check interstitial.
	ErrCaptcha = errors.New("retrieve: the page is behind a human check")

	// ErrAuthRequired — 401/403, or a 200 that is a login form.
	ErrAuthRequired = errors.New("retrieve: the resource requires authentication")

	// ErrNotFound — 404 or 410. Terminal for the reason a 404 is terminal
	// everywhere in this service: it is a final answer, and treating absence as
	// transient is the shortest path to an infinite loop against a server that
	// has already answered.
	ErrNotFound = errors.New("retrieve: the resource is not published")

	// ErrUnusable — a response this pass can never use: over the byte bound, or
	// a redirect chain that never terminated. Terminal, and kept apart from a
	// plain fetch failure so an oversized file is downloaded once and parked
	// rather than downloaded three times.
	ErrUnusable = errors.New("retrieve: the response is over the size bound or did not terminate")

	// ErrAddressRefused — the URL's scheme is not http/https, or the address it
	// resolved to is not public unicast. Terminal. In production it should stay
	// at zero, because this queue's addresses come from TED's own register, so
	// any occurrence is a signal worth reading.
	ErrAddressRefused = errors.New("retrieve: the URL's scheme or resolved address is not permitted")
)

// Terminal reports whether err is one no later pass will resolve by trying
// again. It exists so the queue bookkeeping has one place to ask, rather than
// re-deriving the classification from a list of sentinels that will grow.
//
// Note what is NOT here: ErrRobotsUnknown. That omission is the contract.
func Terminal(err error) bool {
	switch {
	case errors.Is(err, ErrRobotsDenied),
		errors.Is(err, ErrCaptcha),
		errors.Is(err, ErrAuthRequired),
		errors.Is(err, ErrNotFound),
		errors.Is(err, ErrUnusable),
		errors.Is(err, ErrAddressRefused):
		return true
	default:
		return false
	}
}

// StatusFor maps a fetch failure onto the docs_status a human reads.
//
// It is exhaustive over the sentinels above and defaults to StatusUnreachable,
// which is the same safe-way-round default as Terminal: an unclassified failure
// reads as "we could not reach it", which is true of every failure this package
// has not named.
//
// Three distinct terminal sentinels — ErrNotFound, ErrUnusable and
// ErrAddressRefused — all land on StatusUnreachable, and that is a deliberate
// consequence of the vocabulary being closed at eight rather than an oversight.
// Each of them means "no usable answer came back from this address, and none
// ever will"; the difference between them is a detail for the failure log, not
// a different sentence to a bid manager. Their permanence is carried by
// docs_fetched_at, which Terminal stamps immediately.
func StatusFor(err error) string {
	switch {
	case errors.Is(err, ErrRobotsDenied):
		return StatusRobotsDenied
	case errors.Is(err, ErrCaptcha):
		return StatusCaptcha
	case errors.Is(err, ErrAuthRequired):
		return StatusAuthRequired
	default:
		return StatusUnreachable
	}
}

// PendingTender is one row of the retrieval queue.
type PendingTender struct {
	// ID is the tender's surrogate key, the handle every Repo call takes.
	ID int64

	// DocumentsURL is the buyer's own documents page, as the notice published
	// it. Never empty: the queue predicate excludes rows with nowhere to start,
	// because a row that can never advance would otherwise burn a batch slot on
	// every pass to re-derive the same answer.
	DocumentsURL string

	// Country is the buyer's country as ISO alpha-2. It decides nothing and is
	// carried purely so the per-tender observation can be broken down by it —
	// portal coverage varies enormously by country, and an aggregate that
	// cannot be split is an average of populations nobody wants averaged.
	//
	// It is one low-cardinality categorical, and that is the ceiling on what
	// this struct will ever carry for the observation. Buyer name, contact
	// details and the URL itself stay out of it by construction; see
	// observation.go.
	Country string

	// Attempts is how many attempts have ALREADY failed against this row,
	// before the one about to be made. The orchestrator adds one to it to
	// decide whether this failure exhausts the budget, so a repository
	// implementation must expose the stored counter here unmodified rather than
	// pre-incrementing it.
	Attempts int
}

// Document is one retrieved resource.
//
// Body is the bytes, and nothing in this package or below it ever persists
// them: text is extracted and the bytes are dropped when this value goes out of
// scope. That is this pass's central storage decision, and it is expressed here
// — in the type, where it is visible to everyone who touches a retrieved file —
// rather than as a comment on a storage layer that deliberately does not exist.
type Document struct {
	// URL is the address the bytes actually came from, after any redirects —
	// not the address that was requested. Enumerated links have to be resolved
	// against this one, so a listing that redirected from /gare to /gare/ would
	// otherwise produce a set of links off by one path segment.
	//
	// The orchestrator overwrites it with the stable, normalised link before
	// persisting a file; see retriever.go for why the post-redirect address is
	// the wrong thing to store.
	URL string

	// ContentType is the server's own claim, verbatim, parameters included.
	ContentType string

	// Filename is the name the server stated in Content-Disposition, falling
	// back to the last path segment. It feeds ranking — a file called
	// "disciplinare.pdf" is the one that matters — and is never persisted.
	Filename string

	// Body is the response, whole. A truncated body never reaches here: the
	// read that produced it refuses an overflow rather than cutting it short,
	// because a truncated HTML listing parses cleanly into a SHORTER file list
	// and that silent undercount is exactly what this pass must not produce.
	Body []byte
}

// IsHTML reports whether this document is a page to enumerate rather than a
// file to extract. The distinction decides the whole next step, and it is asked
// of the CONTENT TYPE rather than of the URL because a portal's
// "/documenti/12345" with no extension is a listing about as often as it is a
// PDF.
func (d Document) IsHTML() bool {
	ct := strings.ToLower(d.ContentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "text/html" || ct == "application/xhtml+xml"
}

// FileLink is one downloadable document a listing page publishes.
type FileLink struct {
	// URL is absolute, resolved against the page it was found on, and stripped
	// of its fragment — a fragment is never part of what gets fetched, and
	// leaving it on would make "#top" and "" two different documents.
	//
	// It is otherwise verbatim as the page published it. Session-parameter
	// normalisation happens in the orchestrator, once, immediately before the
	// URL is either fetched or stored; doing it in the enumerator as well would
	// mean two places had to agree on the rule forever.
	URL string

	// Label is the text the page gives this file, assembled from the link and,
	// where the link sits in a table row, that row's other cells. It feeds
	// ranking and is never persisted.
	Label string

	// Ext is the lowercased extension from the URL path, WITHOUT the leading
	// dot, and empty when the path has none. Empty is common and is not a
	// defect: a large share of these portals serve every attachment from an
	// extension-less download endpoint.
	Ext string
}

// Listing is what one page yielded: the files, and the platform whose parser
// produced them.
type Listing struct {
	// Platform is a named parser's id, or PlatformGeneric. It is what decides
	// whether an empty Files is a claim or an admission, which is why it is
	// reported rather than inferred.
	Platform string

	// Files are the links the page published, in published order.
	Files []FileLink
}

// PortalFetcher retrieves one URL from a buyer portal. Implementations own
// everything this package deliberately does not know about: robots.txt
// retrieval and obedience, the per-host rate limit, the private-address dial
// guard, the redirect cap, the byte cap, retry/backoff and Retry-After.
//
// The error contract is the sentinel set above, and it is the whole interface
// between that machinery and this package's bookkeeping.
type PortalFetcher interface {
	Fetch(ctx context.Context, url string) (Document, error)
}

// ListingEnumerator turns one fetched page into the files it publishes, and
// names the platform it recognised. Pure: no I/O, no context, no HTTP
// vocabulary — the same shape, and for the same reason, as
// enrich.NoticeDetailParser. Adding a platform is a change behind this
// interface and nowhere else.
//
// It returns an error only when the page could not be read as a document at
// all. A page it does not recognise is NOT an error: it reports PlatformGeneric
// with whatever the fallback found, and what an empty result from an
// unrecognised page MEANS is decided here rather than there.
type ListingEnumerator interface {
	Enumerate(page Document) (Listing, error)
}

// TextExtractor turns one file's bytes into citable parts. Taking bytes rather
// than a path keeps every filesystem detail on the adapter side, which is what
// lets this package state the no-blob-persistence rule as a property of its own
// types instead of a convention someone has to remember.
//
// A format this phase cannot read returns zero parts and NO error: those
// formats are out of scope, their absence is a measurement rather than a
// per-row failure, and an error for each would log the same known limitation
// fifteen times per tender while making a tender of unreadable files
// indistinguishable from a tender whose portal was down.
type TextExtractor interface {
	Extract(doc Document) ([]tender.DocumentPart, error)
}

// Outcome is what one tender's retrieval produced, as persisted.
type Outcome struct {
	// Status is one of the eight values above.
	Status string

	// Platform is the enumerator that read the listing, or "" for an outcome
	// reached before any page was parsed. It is the backlog-prioritisation
	// column: grouping by it separates "the fallback already works here" from
	// "this is what to integrate next".
	Platform string

	// FilesFound is how many files the listing published, BEFORE the per-tender
	// cap. A listing of three hundred records three hundred, so the truncation
	// is a SQL fact rather than an inference.
	FilesFound int

	// FilesExtracted is how many of them yielded text this pass could persist.
	FilesExtracted int

	// PartsWritten is the total number of citable parts persisted across those
	// files. It feeds the observation only; nothing is decided on it.
	PartsWritten int

	// BytesFetched is what the tender cost the buyer's server, for the
	// observation's bucket.
	BytesFetched int

	// KeptURLs are exactly the document URLs that were persisted, normalised.
	// FinishRetrieval deletes every other tender-document row for this tender,
	// which is what stops a corrected notice's superseded attachments from
	// outliving it.
	KeptURLs []string
}

// Counts renders the two coverage columns the way the repository must write
// them: a nil pointer is a NULL column.
//
// The rule is status-driven and mechanical, and it is the invariant of this
// whole pass expressed where it can be tested. A count is written only when
// this pass ACTUALLY READ A LISTING and the number it came back with is a
// measurement of that listing:
//
//   - ok, no_files, extract_failed — a listing was enumerated. no_files records
//     0/0, which is a real measurement because only a named platform parser can
//     produce that status; extract_failed records found > 0 with extracted = 0,
//     which is the population that decides whether .p7m and .zip unwrapping is
//     worth building.
//   - everything else — NULL. A robots-denied, captcha'd, login-walled or
//     unreachable tender has no file count because we never saw the listing,
//     and neither has a traversal_failed one: an empty result from the generic
//     fallback is a statement about the fallback, so writing 0 there would say
//     the buyer published nothing. That is precisely the
//     inaccessible-mistaken-for-absent conflation this pass exists to remove.
func (o Outcome) Counts() (found, extracted *int) {
	switch o.Status {
	case StatusOK, StatusNoFiles, StatusExtractFailed:
		f, e := o.FilesFound, o.FilesExtracted
		return &f, &e
	default:
		return nil, nil
	}
}

// Repo is the persistence port for the retrieval pass.
//
// The two terminal write methods are the only ways a row leaves the queue, and
// between them they encode the pass's progress rule: FinishRetrieval always
// takes a row out; MarkDocumentsFailed takes it out only when the failure is
// permanent.
type Repo interface {
	// ListPendingDocuments returns up to limit queued tenders. Ordering is the
	// implementation's choice, but it must be stable and must not starve any
	// row, or a permanently-unlucky tail is never attempted.
	ListPendingDocuments(ctx context.Context, limit int) ([]PendingTender, error)

	// SaveRetrievedDocument persists ONE file — its tender-document row and all
	// of its parts — in one transaction.
	//
	// It is called per file rather than per tender so that a run killed at its
	// deadline mid-tender keeps the files it already read: those are
	// third-party requests that must not be spent twice. The tender stays
	// queued regardless, because only FinishRetrieval stamps docs_fetched_at.
	//
	// It MUST NOT be called for a file that yielded no parts. See the package
	// comment for what a partless document row does to the indexer.
	SaveRetrievedDocument(ctx context.Context, tenderID int64, doc Document, parts []tender.DocumentPart) error

	// FinishRetrieval takes the tender out of the queue, in one transaction:
	// writes the status, the platform, the two counts and docs_fetched_at,
	// deletes every tender-document row for this tender whose URL is not in
	// KeptURLs, and resets indexed_at so the retrieved text is embedded.
	FinishRetrieval(ctx context.Context, tenderID int64, o Outcome) error

	// MarkDocumentsFailed records a failed attempt: always increments
	// docs_attempts and writes docs_status; permanent additionally stamps
	// docs_fetched_at, which is what removes the row from the queue for good.
	//
	// Leaving docs_fetched_at NULL for a transient failure is what makes the
	// queue self-healing — the row is simply attempted again next pass — and
	// incrementing the counter on every attempt is what stops that from being
	// forever.
	MarkDocumentsFailed(ctx context.Context, tenderID int64, status string, permanent bool) error
}
