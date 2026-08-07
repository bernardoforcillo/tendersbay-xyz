// Package document owns reading the text we extracted from a tender's
// attached documents, and owning the honest answer to "how much of this
// tender can we actually read, and why not more".
//
// It is a separate package from core/tender deliberately. Its consumers are
// core/agent (the read_tender_documents tool) and, later, core/bid's
// requirement extraction; core/tender is already the largest package in this
// service and carries the retrieval eval harness, so folding a second,
// unrelated read model into it would couple two things that change for
// different reasons.
//
// Like core/bid's ports, this package imports nothing from internal/adapter/*
// — it defines the Reader port in its own vocabulary and an adapter implements
// it. That direction is what lets Phase 3's buyer-portal fetcher arrive behind
// the same port without a single type here moving.
package document

import "errors"

// ErrTenderNotFound is returned when no tender has the given id. It is a
// distinct answer from "this tender has no documents": an unknown id is a
// caller error, whereas an empty coverage answer is a fact about a real
// tender, and collapsing the two would be the same conflation Coverage and
// Reason exist to prevent, one level up.
var ErrTenderNotFound = errors.New("document: tender not found")

// Coverage says HOW MUCH of a tender's document set we hold extracted text
// for. It answers a quantity question and nothing else.
type Coverage string

const (
	// CoverageNone — no extracted text at all.
	CoverageNone Coverage = "none"
	// CoverageNotice — text from the notice PDF only. This is what TED and
	// PL BZP attach; it is the notice restated, not the specification, so it
	// answers "what is being bought" and never "what must a bid contain".
	CoverageNotice Coverage = "notice_only"
	// CoverageFull — text from at least one non-notice document (a capitolato,
	// disciplinare, PCAP/PCTP). Only ES PLACSP reaches this today.
	CoverageFull Coverage = "full"
)

// Reason says WHY coverage is below full. It is a SEPARATE field from
// Coverage, not a wider enum over it, because the two answer different
// questions and drive different behaviour: coverage decides what the agent may
// assert, cause decides what the UI must say and what prioritises the Phase 3
// portal backlog. Collapsing them is how "we could not reach it" silently
// becomes "there is nothing there".
type Reason string

const (
	// ReasonNone — coverage is full; there is nothing to explain.
	ReasonNone Reason = ""
	// ReasonNoDocumentsPublished — we successfully read the notice and it
	// published no document link. This is the ONLY reason that asserts
	// absence, and Availability.Consistent gates when it may be used.
	ReasonNoDocumentsPublished Reason = "no_documents_published"
	// ReasonNoticeNotRead — we have not successfully read this tender's
	// structured notice, so we do not know what it publishes. Absence of links
	// here is ignorance, not a fact about the tender.
	ReasonNoticeNotRead Reason = "notice_not_read"
	// ReasonBodyNotRetrieved — the notice publishes a document link we hold,
	// and we have not retrieved what is behind it. This is the bucket most
	// Italian tenders land in, and it is exactly the Phase 3 queue.
	ReasonBodyNotRetrieved Reason = "body_not_retrieved"
	// ReasonNotYetExtracted — a document row exists and the indexing pass has
	// not reached this tender yet. Transient by construction; it resolves
	// itself on the next indexer cycle, so a caller should say "not yet",
	// never "not available".
	ReasonNotYetExtracted Reason = "not_yet_extracted"
	// ReasonExtractionFailed — the indexer ran over this tender and produced
	// no text. A scanned PDF with no OCR path is the expected cause.
	ReasonExtractionFailed Reason = "extraction_failed"
)

// Availability is one tender's document-coverage answer plus the evidence the
// answer was derived from.
//
// The evidence fields are carried rather than discarded so that Consistent is
// checkable rather than merely intended: an Availability that has forgotten
// why it says what it says cannot be audited by anything except the code that
// built it.
type Availability struct {
	TenderID int64
	Coverage Coverage
	Reason   Reason
	// NoticeRead is true only when the structured notice was read
	// SUCCESSFULLY. A fetch that 404'd, timed out, or failed to parse leaves
	// this false. That collapse is deliberate and one-directional: several
	// distinct transport outcomes all mean "we do not know what this buyer
	// published", and only the successful one licenses a claim about absence.
	NoticeRead bool
	// KnownDocumentLinks counts buyer document links we HOLD — the notice's
	// own documents_url plus any per-lot ones — whose body we have not
	// retrieved. A non-zero count is proof that documents exist, which is what
	// makes ReasonNoDocumentsPublished indefensible while it is non-zero.
	KnownDocumentLinks int
	// ExtractedDocuments and ExtractedParts count what we did extract, so a
	// caller can distinguish "nothing at all" from "a little".
	ExtractedDocuments int
	ExtractedParts     int
}

// Consistent encodes the one invariant this package exists to protect: an
// inaccessible document is never reported as an absent one.
//
// Asserting ReasonNoDocumentsPublished is a positive claim about the buyer,
// and it is only defensible when we actually read the notice and it named no
// link. Every other state is ignorance, and ignorance has its own reasons —
// which is why a sub-full coverage with ReasonNone is also inconsistent: it
// would be a gap with no explanation attached, and a gap with no explanation
// reads to every downstream consumer as an absence.
func (a Availability) Consistent() bool {
	if a.Reason == ReasonNoDocumentsPublished {
		return a.NoticeRead && a.KnownDocumentLinks == 0
	}
	if a.Coverage == CoverageFull {
		return a.Reason == ReasonNone
	}
	return a.Reason != ReasonNone
}

// Citation is where an excerpt came from.
//
// Every field below the document URL is optional, and that is a fact about the
// corpus rather than a modelling preference: the ingestion pass populates page
// and section metadata GOING FORWARD ONLY, so every document extracted before
// those columns existed keeps NULL and re-extracting the corpus is deferred. A
// renderer must therefore degrade all the way to the document URL alone.
type Citation struct {
	DocumentURL  string
	DocumentType string // "notice" | "spec" | "corrigendum" | ...
	// PartIndex is the part's ordinal within its document. Unlike the fields
	// below it is always present — it is the part's own key — so it is the one
	// piece of provenance that never degrades.
	PartIndex    int
	PageStart    *int
	PageEnd      *int
	SectionPath  []string // e.g. ["7", "7.2"]; nil when unknown
	SectionTitle string
	HasTable     bool
}

// Excerpt is one retrieved passage. Text is already clamped to
// MaxExcerptRunes by Service — no caller ever receives an unclamped one.
type Excerpt struct {
	Text string
	// Truncated reports that Text was clamped and the underlying part is
	// longer. It exists so a consumer can say "the passage continues" instead
	// of presenting a sentence that stops mid-clause as if it ended there.
	Truncated      bool
	RelevanceScore float64
	Citation       Citation
}

// ExcerptQuery asks for the passages of one tender most relevant to one
// question.
//
// TenderID is mandatory and is what keeps this cheap: retrieval is always
// scoped to a single tender's parts, never to the corpus. A corpus-wide part
// search would need a full-text index this service cannot create — it does not
// migrate the ingestion schema — so the scoping is a structural constraint,
// not a policy that a later caller may relax.
type ExcerptQuery struct {
	TenderID int64
	Question string
	Limit    int // clamped to [1, MaxExcerpts]
}

// ExcerptResult pairs the passages with the coverage answer, so a caller can
// never see "zero excerpts" without also seeing why. "We found no passage
// about penalties" and "we hold nothing but the notice PDF" are different
// answers, and a bare empty slice conflates them.
type ExcerptResult struct {
	Availability Availability
	Excerpts     []Excerpt
}
