package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// The mapping from persisted ingestion state to a domain coverage answer is
// the part of this phase most likely to be quietly wrong, so it is asserted
// over the COMPLETE input cross-product rather than a sample:
//
//	extraction state {none, pending, notice-only, with-spec}
//	  x notice read  {true, false}
//	  x held links   {0, 1}
//	  x indexed      {true, false}   = 32 cases
//
// The expectations are written out literally instead of being recomputed from
// the same predicates the implementation uses — a generated expectation would
// pass no matter what mapAvailability said. These tests need no database:
// mapAvailability is pure, and CI has no Postgres, so anything only a DB-gated
// test checks gates nothing (the same reasoning tender_search.go's SQL-string
// tests are written under).
func TestMapAvailability_CompleteCrossProduct(t *testing.T) {
	// Extraction states, expressed as the row counts availabilitySQL returns.
	const (
		// none: no document row at all.
		noneDocs, nonePartsSpec = 0, 0
		// pending / failed: a document row exists, no parts came out of it.
		pendingDocs = 1
		// notice-only: parts exist, none from a non-notice document.
		noticeDocs, noticeParts = 1, 3
		// with-spec: parts from a real specification document.
		specDocs, specParts, specSpecParts = 2, 6, 3
	)

	cases := []struct {
		name         string
		noticeRead   bool
		links        int
		indexed      bool
		docRows      int
		partRows     int
		specPartRows int
		wantCoverage document.Coverage
		wantReason   document.Reason
	}{
		// ── No document row at all. The answer is entirely about the notice. ──
		{"none/unread/nolink/unindexed", false, 0, false, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonNoticeNotRead},
		{"none/unread/nolink/indexed", false, 0, true, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonNoticeNotRead},
		{"none/unread/link/unindexed", false, 1, false, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonBodyNotRetrieved},
		{"none/unread/link/indexed", false, 1, true, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonBodyNotRetrieved},
		{"none/read/nolink/unindexed", true, 0, false, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonNoDocumentsPublished},
		{"none/read/nolink/indexed", true, 0, true, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonNoDocumentsPublished},
		{"none/read/link/unindexed", true, 1, false, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonBodyNotRetrieved},
		{"none/read/link/indexed", true, 1, true, noneDocs, 0, nonePartsSpec, document.CoverageNone, document.ReasonBodyNotRetrieved},

		// ── A document row with no parts. This is a fact about OUR pipeline,
		// so neither the notice nor the links may influence the answer: all
		// eight rows below depend on `indexed` alone.
		{"pending/unread/nolink/unindexed", false, 0, false, pendingDocs, 0, 0, document.CoverageNone, document.ReasonNotYetExtracted},
		{"pending/unread/nolink/indexed", false, 0, true, pendingDocs, 0, 0, document.CoverageNone, document.ReasonExtractionFailed},
		{"pending/unread/link/unindexed", false, 1, false, pendingDocs, 0, 0, document.CoverageNone, document.ReasonNotYetExtracted},
		{"pending/unread/link/indexed", false, 1, true, pendingDocs, 0, 0, document.CoverageNone, document.ReasonExtractionFailed},
		{"pending/read/nolink/unindexed", true, 0, false, pendingDocs, 0, 0, document.CoverageNone, document.ReasonNotYetExtracted},
		{"pending/read/nolink/indexed", true, 0, true, pendingDocs, 0, 0, document.CoverageNone, document.ReasonExtractionFailed},
		{"pending/read/link/unindexed", true, 1, false, pendingDocs, 0, 0, document.CoverageNone, document.ReasonNotYetExtracted},
		{"pending/read/link/indexed", true, 1, true, pendingDocs, 0, 0, document.CoverageNone, document.ReasonExtractionFailed},

		// ── Parts, but only from the notice PDF. Coverage rises to
		// notice_only; the reason for the REST of the gap is unchanged.
		{"notice/unread/nolink/unindexed", false, 0, false, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonNoticeNotRead},
		{"notice/unread/nolink/indexed", false, 0, true, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonNoticeNotRead},
		{"notice/unread/link/unindexed", false, 1, false, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonBodyNotRetrieved},
		{"notice/unread/link/indexed", false, 1, true, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonBodyNotRetrieved},
		{"notice/read/nolink/unindexed", true, 0, false, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonNoDocumentsPublished},
		{"notice/read/nolink/indexed", true, 0, true, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonNoDocumentsPublished},
		{"notice/read/link/unindexed", true, 1, false, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonBodyNotRetrieved},
		{"notice/read/link/indexed", true, 1, true, noticeDocs, noticeParts, 0, document.CoverageNotice, document.ReasonBodyNotRetrieved},

		// ── Text from a real specification. Full coverage carries no cause,
		// even when links remain unfollowed — there is no gap to explain.
		{"spec/unread/nolink/unindexed", false, 0, false, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/unread/nolink/indexed", false, 0, true, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/unread/link/unindexed", false, 1, false, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/unread/link/indexed", false, 1, true, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/read/nolink/unindexed", true, 0, false, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/read/nolink/indexed", true, 0, true, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/read/link/unindexed", true, 1, false, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
		{"spec/read/link/indexed", true, 1, true, specDocs, specParts, specSpecParts, document.CoverageFull, document.ReasonNone},
	}

	if len(cases) != 32 {
		t.Fatalf("cross-product has %d cases, want all 32 — a state is missing", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapAvailability(42, tc.noticeRead, tc.indexed, tc.links, tc.docRows, tc.partRows, tc.specPartRows)

			if got.Coverage != tc.wantCoverage || got.Reason != tc.wantReason {
				t.Errorf("(coverage, reason) = (%q, %q), want (%q, %q)",
					got.Coverage, got.Reason, tc.wantCoverage, tc.wantReason)
			}
			// The invariant, on every single output. This is the assertion
			// that must never be relaxed: an inaccessible document may never
			// be reported as an absent one.
			if !got.Consistent() {
				t.Errorf("Consistent() = false for %+v", got)
			}
			// The evidence must survive the mapping, or Consistent is
			// checking something other than the answer it accompanies.
			if got.TenderID != 42 || got.NoticeRead != tc.noticeRead ||
				got.KnownDocumentLinks != tc.links ||
				got.ExtractedDocuments != tc.docRows || got.ExtractedParts != tc.partRows {
				t.Errorf("evidence = %+v, want it to carry the inputs through", got)
			}
		})
	}
}

// The absence claim is the only one that asserts something about the buyer, so
// it must be unreachable from every state where we could not have known.
func TestMapAvailability_NeverClaimsAbsenceWithoutHavingRead(t *testing.T) {
	for _, links := range []int{0, 1, 7} {
		for _, indexed := range []bool{true, false} {
			for _, docRows := range []int{0, 1} {
				for _, partRows := range []int{0, 3} {
					got := mapAvailability(1, false /* notice NOT read */, indexed, links, docRows, partRows, 0)
					if got.Reason == document.ReasonNoDocumentsPublished {
						t.Errorf("claimed absence without having read the notice: %+v", got)
					}
				}
			}
		}
	}
}

// A held link is positive proof that documents exist; it must outrank a
// successfully read notice, not lose to it.
func TestMissingBodyReason_HeldLinkOutranksASuccessfulRead(t *testing.T) {
	if got := missingBodyReason(true, 1); got != document.ReasonBodyNotRetrieved {
		t.Errorf("reason = %q with a held link, want body_not_retrieved", got)
	}
	if got := missingBodyReason(true, 0); got != document.ReasonNoDocumentsPublished {
		t.Errorf("reason = %q with no link and a read notice, want no_documents_published", got)
	}
	if got := missingBodyReason(false, 0); got != document.ReasonNoticeNotRead {
		t.Errorf("reason = %q with no link and an unread notice, want notice_not_read", got)
	}
}

// ── SQL shape ──────────────────────────────────────────────────────────────

// The tender scoping is what makes this query safe WITHOUT a full-text index
// on ingested_tender_document_parts — a table this service cannot migrate. If
// the predicate ever widened past one tender, the query would degrade to a
// sequential to_tsvector over the whole corpus.
func TestSearchPartsSQL_IsScopedToOneTender(t *testing.T) {
	if !strings.Contains(searchPartsSQL, "d.tender_id = $1") {
		t.Fatal("searchPartsSQL is not scoped to a single tender — it would scan the corpus")
	}
	if !strings.Contains(searchPartsSQL, "LIMIT $3") {
		t.Error("searchPartsSQL does not bound its result set")
	}
}

// The question is a bound parameter, never interpolated; the ranking flag and
// the text-search configuration must match the rest of this package or results
// silently stop being comparable (flag 32) or stop matching at all ('simple').
func TestSearchPartsSQL_ParameterisesAndRanksLikeTheRestOfThePackage(t *testing.T) {
	if strings.Count(searchPartsSQL, "$2") != 2 {
		t.Errorf("the question should appear exactly twice as $2 (rank + match), got %d", strings.Count(searchPartsSQL, "$2"))
	}
	if !strings.Contains(searchPartsSQL, "websearch_to_tsquery('simple', $2)") {
		t.Error("searchPartsSQL does not use the 'simple' configuration the corpus was indexed with")
	}
	if !strings.Contains(searchPartsSQL, ", 32)") {
		t.Error("searchPartsSQL does not normalise ts_rank_cd with flag 32; scores would not compare across questions")
	}
	// A total order, so two equally-ranked parts cannot swap between calls.
	if !strings.Contains(searchPartsSQL, "ORDER BY relevance DESC, d.id ASC, p.index ASC") {
		t.Error("searchPartsSQL's ordering is not total")
	}
}

// The nullable provenance columns must NOT be coalesced into a number: a zero
// page is indistinguishable from an unknown one to a citation renderer.
func TestSearchPartsSQL_KeepsPageProvenanceNullable(t *testing.T) {
	if strings.Contains(searchPartsSQL, "coalesce(p.page_start") || strings.Contains(searchPartsSQL, "coalesce(p.page_end") {
		t.Error("page_start/page_end are coalesced; an unknown page would be reported as page 0")
	}
}

// One statement, aggregates only — a per-document loop here would be an N+1
// over every attached document on every coverage question.
func TestAvailabilitySQL_IsOneStatementOverAggregates(t *testing.T) {
	if strings.Count(availabilitySQL, "count(*)") != 4 {
		t.Errorf("availabilitySQL has %d aggregate subqueries, want 4", strings.Count(availabilitySQL, "count(*)"))
	}
	if !strings.Contains(availabilitySQL, "d.type <> 'notice'") {
		t.Error("availabilitySQL cannot tell specification text from the notice PDF")
	}
	// xml_status is bound, not inlined, so the one value that means "read"
	// stays a named constant rather than a literal buried in SQL.
	if !strings.Contains(availabilitySQL, "t.xml_status = $2") {
		t.Error("availabilitySQL does not bind the xml_status value")
	}
	if xmlStatusOK != "ok" {
		t.Errorf("xmlStatusOK = %q, want the ingestion service's success value", xmlStatusOK)
	}
}

// The guards fire before the database is touched, which is what makes them
// testable with no database at all — a nil handle here would panic if the
// query ran.
func TestSearchParts_DegenerateInputTouchesNoDatabase(t *testing.T) {
	repo := &DocumentRepo{db: nil}
	for _, tc := range []struct {
		question string
		limit    int
	}{
		{"", 5},
		{"   ", 5},
		{"penali", 0},
		{"penali", -1},
	} {
		got, err := repo.SearchParts(context.Background(), 1, tc.question, tc.limit)
		if err != nil {
			t.Errorf("SearchParts(%q, %d) = error %v, want (nil, nil)", tc.question, tc.limit, err)
		}
		if got != nil {
			t.Errorf("SearchParts(%q, %d) = %v, want nil", tc.question, tc.limit, got)
		}
	}
}
