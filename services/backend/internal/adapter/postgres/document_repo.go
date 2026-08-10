// Package postgres — this file implements core/document's Reader over the
// document text services/ingestion extracted. Like tender_repo.go it is a
// READ-ONLY reference into the `tenders` schema: this service never writes
// those tables and never migrates them.
//
// The SQL here is raw and parameterised for the same reason tender_search.go's
// is — full-text ranking (websearch_to_tsquery, ts_rank_cd) and array
// projection (array_to_string) have no DSL in the drops builder.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// jsonTextArray decodes the JSON rendering of a text[] column (produced by
// to_jsonb, see searchPartsSQL) back into a slice, mapping a NULL column, the
// JSON null and an empty array all onto nil so callers get one representation
// of "no section path". It is lossless for headings containing any delimiter,
// which is the whole reason the column crosses as JSON rather than joined text.
func jsonTextArray(encoded string) ([]string, error) {
	if encoded == "" || encoded == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(encoded), &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// DocumentRepo answers "how much of this tender can we read, and what does it
// say about X" out of the extracted-text tables.
type DocumentRepo struct{ db *pg.DB }

// NewDocumentRepo builds a DocumentRepo over db.
func NewDocumentRepo(db *pg.DB) *DocumentRepo { return &DocumentRepo{db: db} }

var _ document.Reader = (*DocumentRepo)(nil)

// availabilitySQL gathers, in ONE statement, everything mapAvailability needs.
//
// The counts are aggregates rather than a per-document loop on purpose: a
// tender with forty attached documents costs exactly the same query as one
// with none. Each subquery is an indexed lookup — ingested_tender_documents is
// unique on (tender_id, url) and ingested_tender_document_parts on
// (document_id, index) — so the whole statement is a handful of index probes.
//
// spec_part_rows is the one that decides `full` versus `notice_only`: a part
// belonging to a document whose type is not 'notice' is text from an actual
// specification (a capitolato, a disciplinare), which is a different kind of
// answer from the notice restated as a PDF.
//
// notice_read is coalesced because both columns it reads are nullable and
// `TRUE AND NULL` is NULL, not false: a row with xml_fetched_at set and
// xml_status NULL would scan a SQL NULL into a Go bool and fail the whole call —
// i.e. the agent's read_tender_documents tool would error rather than answer.
// No writer produces that pair today, but the query should not depend on the
// ingestion service's writers staying disciplined to avoid a hard failure.
const availabilitySQL = `
SELECT
  coalesce(t.xml_fetched_at IS NOT NULL AND t.xml_status = $2, false) AS notice_read,
  t.indexed_at IS NOT NULL                                   AS indexed,
  coalesce(t.documents_url, '') <> ''                        AS has_tender_link,
  (SELECT count(*) FROM tenders.ingested_tender_lots l
     WHERE l.tender_id = t.id AND coalesce(l.documents_url,'') <> '')::int AS lot_links,
  (SELECT count(*) FROM tenders.ingested_tender_documents d
     WHERE d.tender_id = t.id)::int                          AS doc_rows,
  (SELECT count(*) FROM tenders.ingested_tender_documents d
     JOIN tenders.ingested_tender_document_parts p ON p.document_id = d.id
     WHERE d.tender_id = t.id)::int                          AS part_rows,
  (SELECT count(*) FROM tenders.ingested_tender_documents d
     JOIN tenders.ingested_tender_document_parts p ON p.document_id = d.id
     WHERE d.tender_id = t.id AND d.type <> 'notice')::int   AS spec_part_rows
FROM tenders.ingested_tenders t
WHERE t.id = $1`

// Availability satisfies document.Reader.
func (r *DocumentRepo) Availability(ctx context.Context, tenderID int64) (document.Availability, error) {
	rows, err := r.db.Query(ctx, availabilitySQL, tenderID, xmlStatusOK)
	if err != nil {
		return document.Availability{}, fmt.Errorf("postgres: document availability: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return document.Availability{}, fmt.Errorf("postgres: document availability: %w", err)
		}
		return document.Availability{}, document.ErrTenderNotFound
	}
	var (
		noticeRead, indexed, hasTenderLink        bool
		lotLinks, docRows, partRows, specPartRows int
	)
	if err := rows.Scan(&noticeRead, &indexed, &hasTenderLink,
		&lotLinks, &docRows, &partRows, &specPartRows); err != nil {
		return document.Availability{}, fmt.Errorf("postgres: scan document availability: %w", err)
	}
	if err := rows.Err(); err != nil {
		return document.Availability{}, fmt.Errorf("postgres: document availability: %w", err)
	}

	links := lotLinks
	if hasTenderLink {
		links++
	}
	return mapAvailability(tenderID, noticeRead, indexed, links, docRows, partRows, specPartRows), nil
}

// mapAvailability turns persisted ingestion state into a domain answer. It is
// pure and total: every input combination produces an Availability satisfying
// Consistent, which is what document_repo_internal_test.go asserts exhaustively.
//
// The load-bearing input is noticeRead. The ingestion service's xml_status is a
// FETCHER's vocabulary of four values — 'ok' | 'absent' | 'parse_error' |
// 'unavailable' — and three of them are ways of NOT having read the notice.
// They collapse to one domain bit here, and the collapse is deliberately
// one-directional and conservative: 'parse_error' stamps xml_fetched_at (it is
// terminal) yet proves nothing whatever about what the buyer published, so it
// must never be allowed to reach ReasonNoDocumentsPublished.
//
// Passing xml_status through as a Reason instead would put the fetcher's own
// vocabulary into core, and from there into the proto contract, making a
// fetcher swap a wire-format change. Stopping that leak is what this function
// is for, and it is why Phase 3's buyer-portal fetcher can arrive behind the
// same Reader port without a single domain type moving.
func mapAvailability(tenderID int64, noticeRead, indexed bool, links, docRows, partRows, specPartRows int) document.Availability {
	av := document.Availability{
		TenderID:           tenderID,
		NoticeRead:         noticeRead,
		KnownDocumentLinks: links,
		ExtractedDocuments: docRows,
		ExtractedParts:     partRows,
	}

	switch {
	case specPartRows > 0:
		// Text from at least one real specification. Nothing to explain.
		av.Coverage, av.Reason = document.CoverageFull, document.ReasonNone
	case partRows > 0:
		// We hold text, but all of it is the notice restated. Why we hold no
		// more is the same question as the no-text case below, so it gets the
		// same answer — the gap is identical, only its size differs.
		av.Coverage, av.Reason = document.CoverageNotice, missingBodyReason(noticeRead, links)
	case docRows > 0:
		// A document row exists but produced no parts. That is a fact about
		// OUR pipeline, not about the buyer, so it never consults links or
		// noticeRead: indexed_at tells us whether the extraction pass has run.
		av.Coverage = document.CoverageNone
		if indexed {
			av.Reason = document.ReasonExtractionFailed
		} else {
			av.Reason = document.ReasonNotYetExtracted
		}
	default:
		av.Coverage, av.Reason = document.CoverageNone, missingBodyReason(noticeRead, links)
	}
	return av
}

// missingBodyReason explains why we hold no document body, in the order the
// evidence licenses.
//
// The order is the whole point. A held link is positive proof that documents
// exist, so it outranks everything: whatever else is true, we are not entitled
// to say none were published. Only with no link AND a successfully read notice
// is the absence claim defensible. Everything else is ignorance, and ignorance
// is reported as such.
func missingBodyReason(noticeRead bool, links int) document.Reason {
	switch {
	case links > 0:
		return document.ReasonBodyNotRetrieved
	case noticeRead:
		return document.ReasonNoDocumentsPublished
	default:
		return document.ReasonNoticeNotRead
	}
}

// searchPartsSQL ranks one tender's extracted parts against a question.
//
// Three deliberate choices, each with a precedent elsewhere in this package:
//
//   - 'simple', matching the ingestion migration that built the tender-level
//     search vector: this corpus spans 24 languages, so any single stemmer is
//     wrong for most rows, and a configuration mismatch silently stops matching.
//   - ts_rank_cd's normalisation flag 32, matching tender_search.go: it maps an
//     unbounded rank into (0,1), which is comparable across questions where a
//     raw rank is not.
//   - to_jsonb(section_path), because drops has no array-typed column and no
//     array scan path, so the array has to cross as a scalar. JSON rather than
//     a delimiter-joined string: a document heading routinely contains any
//     separator one might pick ("Capo 1, Disposizioni generali", "Allegato A >
//     B"), so joining and re-splitting would silently invent extra path
//     elements, and array_to_string drops NULL elements on top of that. The
//     ingestion adapter encodes the same column the same way for the same
//     reason — this is the read side of one convention, not two.
//
// The tie-break is (d.id, p.index) so the order is total: without it two
// equally-ranked parts could swap places between calls.
//
// WHY THIS NEEDS NO INDEX, and why a corpus-wide version would:
// ingested_tender_document_parts carries no tsvector and no GIN index, and this
// service cannot add one — it does not migrate the `tenders` schema. It does
// not need one, because `d.tender_id = $1` narrows to a single tender's parts
// (a few hundred rows even for a 150-page PDF) through an indexed join BEFORE
// any to_tsvector is computed. That is categorically different from the
// unindexed expression-left-of-@@ sequential scan the ingestion service's 0009
// migration had to revert. A cross-tender part search would need the GIN index,
// and therefore an ingestion migration — do not generalise this query into one.
//
// No ts_headline: the part IS the excerpt, and <mark> markers in an agent tool
// result are noise to a model rather than an affordance.
const searchPartsSQL = `
SELECT d.url, d.type, p.index, p.content,
       p.page_start, p.page_end,
       coalesce(p.section_title, ''),
       coalesce(to_jsonb(p.section_path)::text, ''),
       coalesce(p.has_table, false),
       ts_rank_cd(to_tsvector('simple', p.content),
                  websearch_to_tsquery('simple', $2), 32) AS relevance
FROM tenders.ingested_tender_document_parts p
JOIN tenders.ingested_tender_documents d ON d.id = p.document_id
WHERE d.tender_id = $1
  AND to_tsvector('simple', p.content) @@ websearch_to_tsquery('simple', $2)
ORDER BY relevance DESC, d.id ASC, p.index ASC
LIMIT $3`

// SearchParts satisfies document.Reader.
func (r *DocumentRepo) SearchParts(ctx context.Context, tenderID int64, question string, limit int) ([]document.Excerpt, error) {
	question = strings.TrimSpace(question)
	if question == "" || limit <= 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, searchPartsSQL, tenderID, question, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: search document parts: %w", err)
	}
	defer rows.Close()

	var out []document.Excerpt
	for rows.Next() {
		var (
			ex          document.Excerpt
			sectionPath string
		)
		// page_start/page_end stay pointers: the ingestion pass populates them
		// going forward only, so NULL here means "extracted before those
		// columns existed", not "this passage has no page". A zero would be a
		// lie a citation renderer could not detect.
		if err := rows.Scan(&ex.Citation.DocumentURL, &ex.Citation.DocumentType,
			&ex.Citation.PartIndex, &ex.Text,
			&ex.Citation.PageStart, &ex.Citation.PageEnd,
			&ex.Citation.SectionTitle, &sectionPath, &ex.Citation.HasTable,
			&ex.RelevanceScore); err != nil {
			return nil, fmt.Errorf("postgres: scan document part: %w", err)
		}
		if ex.Citation.SectionPath, err = jsonTextArray(sectionPath); err != nil {
			return nil, fmt.Errorf("postgres: decode section path for tender %d: %w", tenderID, err)
		}
		out = append(out, ex)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: search document parts: %w", err)
	}
	return out, nil
}
