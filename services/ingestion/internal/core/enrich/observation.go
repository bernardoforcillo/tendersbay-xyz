package enrich

import (
	"context"
	"log/slog"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// eventNoticeXMLIngested is the name of the per-notice observation this pass
// emits, and it is the log message itself so the event is greppable and so a
// later move to a product-analytics capture is a change of sink rather than a
// change of vocabulary.
//
// It is a structured log line rather than an analytics event on purpose: the
// acceptance gates for this pass — coverage by country, grid_usable
// consistency, nothing stuck in the queue — are all answerable in SQL against
// the rows it writes. Making the measurement depend on an external analytics
// sink would put an authorization dependency in front of a phase that does not
// need one. The event is the ongoing production read, not the gate.
const eventNoticeXMLIngested = "notice_xml_ingested"

// eventLocation names the code path the observation came from, in the house
// dotted form.
const eventLocation = "ingestion.enricher"

// eventSource is constant because the enrichment queue is TED-only by
// definition — no other provider publishes a machine-readable notice document
// this pass knows how to read. It is emitted anyway so that the property
// exists from day one, rather than every historical event needing a default
// the first time a second source appears.
const eventSource = "ted"

// noXMLBytes marks an observation for a notice whose document never arrived,
// distinguishing it from a zero-length one.
const noXMLBytes = -1

// observation is one notice's worth of what this pass learned, in the shape it
// is reported: counts, booleans and low-cardinality categoricals, and nothing
// else. No free text, no URL, no buyer name, no contact detail.
//
// That restriction is enforced by the type rather than by care at the call
// site. Notice organizations routinely carry an email or a phone number, and
// some notices name an individual rather than a mailbox; the data is public —
// it is published in the EU's own register — but publishing it there is not a
// reason to copy it into a second system that exists to be sliced and charted.
// Since this struct has nowhere to put such a value, no future edit to the
// emit path can leak one without first widening the struct, which is a change
// a reviewer can see.
//
// # Why there is no has_criteria
//
// The obvious property to add here — "did this notice publish any criteria" —
// is deliberately absent, and its absence is the point.
//
// Across 50 sampled Italian cn-standard notices, 38 (76%) published at least
// one criterion entry, but only 18 (36%) attached a weight to any of them. The
// gap is notices that name the award METHOD ("offerta economicamente più
// vantaggiosa") and stop there: present, and useless for reconstructing a
// scoring grid. A has_criteria property would therefore report roughly twice
// the coverage this pass actually delivers, and it would do so in the most
// quotable form available — a single boolean, in every chart, with no adjacent
// number contradicting it.
//
// The reliable way to stop that number being reported is to make it
// unavailable, so it is not computed here at all. criteriaCount and
// weightedCriteriaCount are both emitted instead, which puts the gap in front
// of anyone who looks, by construction. Do not "helpfully" add has_criteria
// back as a convenience for a dashboard filter.
type observation struct {
	country     string
	noticeType  string
	parseStatus string

	lotCount          int
	hasDocumentsURL   bool
	hasSubmissionURL  bool
	criteriaCount     int
	weightedCount     int
	gridUsable        bool
	organizationCount int

	// xmlBytes is the size of the document that was read, or noXMLBytes when
	// none arrived. Reported only as a bucket — the exact size of one notice is
	// not a number anyone acts on, and the buckets are what make "are documents
	// getting bigger" answerable without keeping a continuous measure.
	xmlBytes int
}

// observeNotice reports what one notice's enrichment produced.
//
// It is called exactly once per row that LEAVES the queue — on success, and on
// a permanent park — and never for a transient failure that will be retried.
// That rule is what makes the event stream one observation per notice, which
// in turn is what makes counting over it a coverage measurement rather than an
// attempt-weighted one: a row that fails four times before succeeding must not
// appear five times, or every population number is inflated by exactly the
// notices TED was slowest to serve. Transient attempts are already visible in
// the per-attempt failure log.
//
// d is the zero NoticeDetail for a failure, which is correct rather than
// merely convenient: a notice whose document never parsed genuinely published
// no lots and no criteria as far as this pass is concerned, and parseStatus
// says which case a reader is looking at.
func observeNotice(ctx context.Context, n PendingNotice, status string, d tender.NoticeDetail, xmlBytes int) {
	observation{
		country:           n.Country,
		noticeType:        n.NoticeType,
		parseStatus:       status,
		lotCount:          len(d.Lots),
		hasDocumentsURL:   d.DocumentsURL != "",
		hasSubmissionURL:  d.SubmissionURL != "",
		criteriaCount:     d.CriteriaCount(),
		weightedCount:     d.WeightedCount(),
		gridUsable:        d.GridUsable(),
		organizationCount: len(d.Organizations),
		xmlBytes:          xmlBytes,
	}.emit(ctx)
}

// emit writes the observation as one structured log line.
func (o observation) emit(ctx context.Context) {
	slog.InfoContext(ctx, eventNoticeXMLIngested, o.attrs()...)
}

// attrs renders the observation's properties. It is separate from emit so a
// test can assert on the exact property set — both that everything expected is
// present, and that has_criteria is not.
func (o observation) attrs() []any {
	return []any{
		"location", eventLocation,
		"source", eventSource,
		"country", o.country,
		"notice_type", o.noticeType,
		"parse_status", o.parseStatus,
		"lot_count", o.lotCount,
		"has_documents_url", o.hasDocumentsURL,
		"has_submission_url", o.hasSubmissionURL,
		"criteria_count", o.criteriaCount,
		"weighted_criteria_count", o.weightedCount,
		"grid_usable", o.gridUsable,
		"organization_count", o.organizationCount,
		"xml_bytes_bucket", bytesBucket(o.xmlBytes),
	}
}

// Bucket boundaries for document size, in decimal thousands to match how the
// buckets read. Real notices sit almost entirely in the first bucket, so the
// interesting signal is any drift out of it — not the distribution within it.
const (
	bucketSmallMax  = 50_000
	bucketMediumMax = 200_000
	bucketLargeMax  = 1_000_000
)

// bytesBucket coarsens a document size into the reported bucket. "none" is a
// value of its own rather than the smallest bucket: a notice whose document
// never arrived did not produce a small document, and folding the two together
// would make the absent cases look like tiny successful ones in exactly the
// chart meant to spot them.
func bytesBucket(n int) string {
	switch {
	case n < 0:
		return "none"
	case n < bucketSmallMax:
		return "<50k"
	case n < bucketMediumMax:
		return "50-200k"
	case n < bucketLargeMax:
		return "200k-1m"
	default:
		return ">1m"
	}
}
