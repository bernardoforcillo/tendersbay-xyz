// Package enrich is the core of the notice-enrichment pass: for every ingested
// tender whose machine-readable notice document has not been read yet, fetch
// that document, turn it into domain detail, and persist the result. It defines
// the ports the adapters implement (fetching, parsing, persistence) and the
// orchestrator that drives them; it imports nothing from internal/adapter, and
// nothing here knows what HTTP, XML or Postgres are.
//
// # Why the ports are shaped this way
//
// NoticeDetailParser takes bytes and returns domain types. It has no context,
// no URL and no HTTP vocabulary, and that is deliberate: parsing TED's eForms
// XML is a pure function of the document, so the open question of whether that
// parser is vendored here or eventually imported from the tedmcp project is a
// one-file swap behind this interface. Nothing in this package, and nothing
// that calls it, changes when the answer changes.
//
// # The scope fence
//
// NoticeFetcher fetches ted.europa.eu and nothing else. This pass reads the
// EU's own machine-readable register; it deliberately does not follow the buyer
// portal links it stores (DocumentsURL, SubmissionURL). Fetching a buyer portal
// carries robots-compliance and SSRF obligations that this path does not have
// and is not audited for, so the implementing adapter rejects any URL whose
// host is not ted.europa.eu. That host check is one `if` today and is the
// difference between a scoped fetcher and an unaudited egress primitive later —
// do not relax it to "make the fetcher reusable".
package enrich

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// The xml_status vocabulary, and the whole of it. A row's status says what the
// last look at its notice document produced; combined with xml_fetched_at
// (NULL = still queued) it is the complete record of what this pass knows about
// a notice.
//
// Only StatusUnavailable is ever written while the row stays queued — it means
// "the last attempt failed transiently", and becomes terminal once the attempt
// budget is spent. The other three are terminal the moment they are written.
const (
	// StatusOK — the document was fetched, parsed, and its detail persisted.
	StatusOK = "ok"

	// StatusAbsent — the notice has no XML document and never will (a legacy
	// pre-eForms notice, or a notice type TED does not publish XML for). This
	// is TERMINAL. Treating a 404 as retryable is the shortest path to an
	// infinite loop against TED, which is why absence is a status rather than
	// a failure that leaves the row queued.
	StatusAbsent = "absent"

	// StatusParseError — a document was retrieved but could not be turned into
	// detail: the decoder rejected it, or it was unusable on its face
	// (oversized, truncated). TERMINAL, because it is deterministic — the same
	// bytes will fail the same way on every retry, so retrying only costs a
	// download.
	StatusParseError = "parse_error"

	// StatusUnavailable — the notice's detail could not be made available for a
	// reason that might not persist: the document was not retrieved (throttling,
	// a 5xx, a transport error), or it was retrieved and parsed but the store
	// rejected the write. Written on every such attempt; becomes terminal once
	// maxAttempts attempts have failed, at which point the row stops being
	// re-listed and is visible as a parked failure rather than as an
	// eternally-pending one.
	//
	// The persistence half of that is deliberately not given a status of its
	// own. A reader of this column wants to know whether we have the notice's
	// detail, and "we could not fetch it" and "we could not store it" are the
	// same answer to that question, differing only in whose fault it is — which
	// the error log already records. The status vocabulary stays closed at four
	// values.
	StatusUnavailable = "unavailable"
)

// ErrNoticeAbsent reports that the upstream register answered, definitively,
// that this notice has no XML document — an HTTP 404 in the current adapter.
// Adapters wrap it (fmt.Errorf("...: %w", enrich.ErrNoticeAbsent)) so that the
// orchestrator can tell "there is nothing to fetch" from "we could not fetch
// it", without this package knowing a status code exists.
//
// This distinction is the difference between a row that parks after one attempt
// and a row that is retried five times, so an adapter returning a bare error
// where it means absence turns a legacy-notice-heavy corpus into five times the
// traffic against TED for the same result.
var ErrNoticeAbsent = errors.New("enrich: notice document is absent")

// ErrNoticeUnusable reports that the upstream register did answer with a
// document, but one this pass will never be able to use: over the size bound,
// or truncated at it. It is deterministic, so it is terminal — separated from a
// plain fetch error precisely so an 8 MiB notice is downloaded once and parked,
// not downloaded five times before being parked with a status that implies TED
// was at fault.
//
// It maps to StatusParseError rather than a status of its own: from a reader's
// point of view "we got the bytes and could not derive detail from them" is one
// outcome, and the status vocabulary is deliberately closed at four values.
var ErrNoticeUnusable = errors.New("enrich: notice document is unusable")

// PendingNotice is one row of the enrichment queue: a tender whose notice
// document has not been successfully read yet (xml_fetched_at IS NULL). It
// carries only what the orchestrator needs to make its next decision — the rest
// of the row is the repository's business.
type PendingNotice struct {
	// ID is the tender's surrogate key, the handle every Repo call takes.
	ID int64

	// PublicationNumber is the notice's own immutable publication id
	// ("545620-2026"), distinct from the procedure identifier the tender row is
	// keyed on. It is carried here for logging and correlation: when a notice
	// fails, this is the value that identifies the exact document to a human
	// checking TED by hand.
	PublicationNumber string

	// XMLURL is the notice document to fetch. It can legitimately be empty —
	// the queue is "not yet read", not "readable", and a TED payload that never
	// carried an XML link produces a row with nothing to fetch. The orchestrator
	// parks those as StatusAbsent rather than letting them sit in the queue
	// forever looking like work.
	XMLURL string

	// Country is the buyer's country as ISO alpha-2, and NoticeType is TED's
	// own notice-type code ("cn-standard", "can-standard", …). Neither is used
	// to decide anything: they are carried purely so the per-notice observation
	// this pass emits can be broken down by them, because the whole point of
	// measuring notice-document coverage is that it varies enormously by
	// country and by notice type, and an aggregate that cannot be split is an
	// average of populations nobody wants averaged.
	//
	// They are two low-cardinality categoricals, and that is the ceiling on
	// what this struct will ever carry for the observation. Buyer name, contact
	// details and URLs stay out of it by construction — see observation.go.
	Country    string
	NoticeType string

	// Attempts is how many attempts have ALREADY failed against this row,
	// before the one about to be made. The orchestrator adds one to it to decide
	// whether this failure is the one that exhausts the budget, so a repository
	// implementation must expose the stored counter here unmodified rather than
	// pre-incrementing it.
	Attempts int
}

// NoticeDetailParser turns one notice document into domain detail. Pure: no
// I/O, no context, no knowledge of where the bytes came from — see the package
// comment for why that shape is load-bearing.
//
// An error means the document could not be decoded, which is terminal for the
// row. A clean parse that yields no lots and no criteria is NOT an error: it is
// a real and frequent outcome (most notices publish no usable award grid), and
// reporting it as a per-row failure would hide a population-level fact inside
// an error log.
type NoticeDetailParser interface {
	Parse(xml []byte) (tender.NoticeDetail, error)
}

// NoticeFetcher retrieves one notice document. Implementations own everything
// this package deliberately does not know about: the host allowlist described
// in the package comment, the user agent, the size bound, retry/backoff and
// Retry-After handling for the statuses worth retrying.
//
// The error contract is the whole interface between that machinery and the
// orchestrator's bookkeeping:
//
//   - ErrNoticeAbsent  — the register says there is no document. Terminal.
//   - ErrNoticeUnusable — a document arrived but is unusable on its face. Terminal.
//   - any other error  — transient; the row is retried on a later pass until its
//     attempt budget runs out.
//
// "Any other error" being the retryable default is the safe way round: a new
// failure mode nobody has classified yet costs a bounded number of retries,
// whereas defaulting to terminal would silently park rows that were only
// briefly unreachable.
type NoticeFetcher interface {
	FetchXML(ctx context.Context, url string) ([]byte, error)
}

// Repo is the persistence port for the enrichment pass.
//
// The two write methods are the only ways a row leaves the queue, and between
// them they encode the pass's progress rule: SaveDetail always takes a row out;
// MarkXMLFailed takes it out only when the failure is permanent. See the
// Enricher's Drain for why "left the queue" and not "succeeded" is what
// termination is measured on.
type Repo interface {
	// ListPendingXML returns up to limit queued rows (xml_fetched_at IS NULL).
	// Ordering is the implementation's choice, but it must be stable and must
	// not starve any row, or a permanently-unlucky tail never gets attempted.
	ListPendingXML(ctx context.Context, limit int) ([]PendingNotice, error)

	// SaveDetail persists one notice's detail and takes the row out of the
	// queue, in ONE transaction: the child criteria and organization rows
	// (replaced wholesale, not upserted — a criteria set is a set, not an
	// accumulating log), the derived grid_usable flag, xml_status = StatusOK and
	// xml_fetched_at = now().
	//
	// Two invariants ride on that single transaction. First, grid_usable is a
	// stored denormalisation of the criteria rows, and it is only safe to store
	// because it is written with them — a reader must never see the flag
	// disagree with the rows it summarises. Second, the same transaction resets
	// indexed_at, because enrichment changes the text the search index was built
	// from and the indexer's dirty-check compares only the description; without
	// the reset an enriched tender keeps a stale embedding forever.
	SaveDetail(ctx context.Context, tenderID int64, d tender.NoticeDetail) error

	// MarkXMLFailed records a failed attempt. It always increments xml_attempts
	// and writes status; permanent additionally sets xml_fetched_at, which is
	// what takes the row out of the queue for good.
	//
	// Leaving xml_fetched_at NULL for a transient failure is what makes the
	// queue self-healing — the row is simply attempted again next pass — and
	// incrementing the counter on every attempt is what stops that from being
	// forever.
	//
	// A rejected SaveDetail is recorded through here too, which is what bounds
	// the one failure mode the fetch path cannot produce: a notice whose detail
	// this store will never accept. That it shares an implementation with the
	// fetch failures is also what makes it safe — when the database is down
	// BOTH calls fail, so no attempt is charged for an outage.
	MarkXMLFailed(ctx context.Context, tenderID int64, status string, permanent bool) error
}
