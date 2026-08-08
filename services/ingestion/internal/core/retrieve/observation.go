package retrieve

import (
	"context"
	"log/slog"
)

// eventTenderDocumentsRetrieved is the name of the per-tender observation this
// pass emits, and it is the log message itself so the event is greppable and so
// a later move to a product-analytics capture is a change of sink rather than a
// change of vocabulary.
//
// It is a structured log line rather than an analytics event on purpose, and
// the reason is stronger here than it was for the enrichment pass: every
// acceptance gate for this phase — coverage by country and status, the platform
// distribution that decides which parser to write next, whether anything is
// stuck in the queue, and the ratio that actually matters (how many retrieved
// tenders contain scoring language at all) — is a SQL query over the rows this
// pass writes. Making the measurement depend on an external analytics sink
// would put an authorization dependency in front of a phase that does not need
// one. The event is the ongoing production read, not the gate.
const eventTenderDocumentsRetrieved = "tender_documents_retrieved"

// eventLocation names the code path the observation came from, in the house
// dotted form.
const eventLocation = "ingestion.retriever"

// noCount marks a count that was not measured, distinguishing it from a
// measured zero — the same technique, and for the same reason, as the
// enrichment pass's noXMLBytes.
//
// It is the whole coverage-versus-cause rule carried into the event stream. A
// robots-denied tender did not find zero files; we never saw its listing. If
// this emitted 0 there, every chart summing files_found would report the
// buyer's portal as empty rather than as closed, which is precisely the
// conflation this pass exists to remove — and it would do it in the most
// quotable form available, a single number with nothing adjacent to contradict
// it.
const noCount = -1

// observation is one tender's worth of what this pass learned, in the shape it
// is reported: counts, booleans and low-cardinality categoricals, and nothing
// else. No free text, no URL, no host, no buyer name.
//
// That restriction is enforced by the type rather than by care at the call
// site, exactly as the enrichment pass's observation is. Retrieved tender
// documents name the RUP and frequently name individuals, and a documents link
// carries the buyer's own procedure reference and sometimes a session token.
// Since this struct has nowhere to put such a value, no future edit to the emit
// path can leak one without first widening the struct, which is a change a
// reviewer can see.
//
// # Why there is no has_documents
//
// The obvious property to add — "did this tender have documents" — is
// deliberately absent, for the same reason has_criteria is absent from the
// enrichment pass's event. It would report what we SAW rather than what we can
// READ, and the gap between those two numbers is the entire finding of this
// phase: a corpus of .p7m archives and scanned PDFs answers "yes" to
// has_documents and delivers nothing. filesFound and filesExtracted are both
// emitted instead, which puts the gap in front of anyone who looks, by
// construction.
type observation struct {
	country  string
	platform string
	status   string

	// filesFound and filesExtracted are noCount when no listing was read. See
	// Outcome.Counts, which is the same rule applied to the database columns.
	filesFound     int
	filesExtracted int

	// filesCapped reports that the listing published more files than one tender
	// is allowed to fetch, so filesExtracted is bounded by our own budget
	// rather than by the portal. Without it a capped tender is indistinguishable
	// from a fully-read one in every aggregate.
	filesCapped bool

	partsWritten int

	// bytesFetched is what this tender cost the buyer's server. Reported only
	// as a bucket: the exact size of one tender's downloads is not a number
	// anyone acts on, and the buckets are what make "are we asking too much of
	// these hosts" answerable without keeping a continuous measure.
	bytesFetched int
}

// observe reports what one tender's retrieval produced.
//
// It is called exactly once per row that LEAVES the queue — on FinishRetrieval,
// and on a permanent park — and never for a transient failure that will be
// retried. That rule is what makes the event stream one observation per tender,
// which in turn is what makes counting over it a coverage measurement rather
// than an attempt-weighted one: a row that fails twice before succeeding must
// not appear three times, or every population number is inflated by exactly the
// portals that are slowest. Transient attempts are already visible in the
// per-attempt failure log.
func observe(ctx context.Context, n PendingTender, out Outcome) {
	newObservation(n, out).emit(ctx)
}

// newObservation builds the payload. It is separate from observe so a test can
// assert on the value rather than on a log line, and so the coverage rule below
// has one expression.
//
// The counts are taken from Outcome.Counts rather than from the Outcome's
// fields directly, which is what keeps the event stream and the database
// columns saying the same thing: an unmeasured count is absent in both, by the
// same rule, evaluated once.
func newObservation(n PendingTender, out Outcome) observation {
	o := observation{
		country:        n.Country,
		platform:       out.Platform,
		status:         out.Status,
		filesFound:     noCount,
		filesExtracted: noCount,
		partsWritten:   out.PartsWritten,
		bytesFetched:   out.BytesFetched,
	}
	if found, extracted := out.Counts(); found != nil {
		o.filesFound, o.filesExtracted = *found, *extracted
		o.filesCapped = *found > MaxFilesPerTender
	}
	return o
}

// emit writes the observation as one structured log line.
func (o observation) emit(ctx context.Context) {
	slog.InfoContext(ctx, eventTenderDocumentsRetrieved, o.attrs()...)
}

// attrs renders the observation's properties. It is separate from emit so a
// test can assert on the exact property set — both that everything expected is
// present, and that nothing identifying has been added to it.
func (o observation) attrs() []any {
	return []any{
		"location", eventLocation,
		"country", o.country,
		"platform", o.platform,
		"status", o.status,
		"files_found", o.filesFound,
		"files_extracted", o.filesExtracted,
		"files_capped", o.filesCapped,
		"parts_written", o.partsWritten,
		"bytes_bucket", bytesBucket(o.bytesFetched),
	}
}

// Bucket boundaries for one tender's total downloaded bytes. A whole procedure's
// document set is normally a few megabytes; the interesting signal is the tail
// that approaches MaxTenderBytes, not the distribution near the bottom.
const (
	bucketSmallMax  = 1 << 20
	bucketMediumMax = 10 << 20
	bucketLargeMax  = 50 << 20
)

// bytesBucket coarsens a tender's download volume into the reported bucket.
// "none" is a value of its own rather than the smallest bucket: a tender whose
// portal refused us did not download a small amount, and folding the two
// together would make the refused cases look like tiny successful ones in
// exactly the chart meant to spot them.
func bytesBucket(n int) string {
	switch {
	case n <= 0:
		return "none"
	case n < bucketSmallMax:
		return "<1m"
	case n < bucketMediumMax:
		return "1-10m"
	case n < bucketLargeMax:
		return "10-50m"
	default:
		return ">50m"
	}
}
