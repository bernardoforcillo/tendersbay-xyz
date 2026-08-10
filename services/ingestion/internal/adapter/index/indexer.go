// Package index runs the indexing pass that makes ingested tenders
// semantically searchable: for every tender not yet indexed, it builds a
// summary chunk from structured fields, downloads and extracts any
// attached document's text (or reuses previously-extracted text), and
// calls the shared knowledge base to embed and store it.
package index

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/buildwithgo/berrygem/rag"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
)

// batchSize bounds how many unindexed tenders one RunOnce call processes,
// so a large backlog drains over several hourly cycles instead of making
// one run balloon in duration.
const batchSize = 200

// Repo is the subset of postgres.TenderRepo the indexer needs.
type Repo interface {
	ListUnindexed(ctx context.Context, limit int) ([]postgres.UnindexedTender, error)
	DocumentParts(ctx context.Context, documentID int64) ([]tender.DocumentPart, error)
	SaveDocumentParts(ctx context.Context, documentID int64, parts []tender.DocumentPart) error
	MarkIndexed(ctx context.Context, tenderID int64) error
}

// KnowledgeBase is the subset of knowledge.KnowledgeBase the indexer needs.
// IngestWithAttributes rather than plain Ingest: the structured facets it
// carries are what let the search API pre-filter inside the vector store
// instead of retrieving unfiltered candidates and discarding most of them.
type KnowledgeBase interface {
	IngestWithAttributes(ctx context.Context, doc *rag.Document, attrs knowledge.Attributes) error
}

// Fetcher downloads and extracts one document's text, as chunks carrying the
// page/section provenance the extractor observed. The indexer itself embeds
// only the text; the rest is persisted for the retrieval side to cite.
type Fetcher interface {
	FetchAndExtract(ctx context.Context, url string) ([]tender.DocumentPart, error)
}

// Indexer embeds and indexes tenders that haven't been indexed yet.
type Indexer struct {
	repo    Repo
	kb      KnowledgeBase
	fetcher Fetcher
}

// New returns an Indexer.
func New(repo Repo, kb KnowledgeBase, fetcher Fetcher) *Indexer {
	return &Indexer{repo: repo, kb: kb, fetcher: fetcher}
}

// RunOnce indexes up to batchSize unindexed tenders — both freshly
// ingested tenders and previous indexing failures look identical
// (indexed_at IS NULL), so one pass handles both. Per-tender indexing
// failures (Ollama/Qdrant unreachable, download/extraction errors) are
// logged and skipped, not fatal — the tender stays unindexed and is
// retried on a later cycle. RunOnce itself only returns an error for a
// failure that prevents listing candidates at all.
// It returns how many tenders it successfully marked indexed, which is what
// Drain uses to decide whether a pass made progress.
func (idx *Indexer) RunOnce(ctx context.Context) (int, error) {
	tenders, err := idx.repo.ListUnindexed(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("index: list unindexed: %w", err)
	}

	indexed := 0
	for _, t := range tenders {
		if err := idx.indexOne(ctx, t); err != nil {
			slog.ErrorContext(ctx, "failed to index tender", "tender_id", t.ID, "error", err)
			continue
		}
		if err := idx.repo.MarkIndexed(ctx, t.ID); err != nil {
			slog.ErrorContext(ctx, "failed to mark tender indexed", "tender_id", t.ID, "error", err)
			continue
		}
		indexed++
	}
	return indexed, nil
}

// Drain repeats RunOnce until a pass indexes nothing, so one invocation
// clears as much of the backlog as its time budget allows instead of
// stopping at batchSize.
//
// Terminating on "indexed nothing" rather than "listed nothing" is what
// keeps this from spinning: a batch whose every tender fails (Ollama or
// Qdrant down) leaves those same rows unindexed, so ListUnindexed would
// hand back an identical batch forever. No progress ends the pass and the
// rows are retried on the next scheduled run.
//
// Cancellation is not an error: the caller's deadline expiring mid-drain is
// the normal way a large backlog ends a run, and every completed batch is
// already committed.
func (idx *Indexer) Drain(ctx context.Context) error {
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			slog.InfoContext(ctx, "indexing pass stopped early", "reason", err, "indexed_total", total)
			return nil
		}

		indexed, err := idx.RunOnce(ctx)
		total += indexed
		if err != nil {
			return err
		}
		if indexed == 0 {
			slog.InfoContext(ctx, "indexing pass complete", "indexed_total", total)
			return nil
		}
		slog.InfoContext(ctx, "indexing batch complete", "indexed", indexed, "indexed_total", total)
	}
}

func (idx *Indexer) indexOne(ctx context.Context, t postgres.UnindexedTender) error {
	summary := t.Title + "."
	if t.Description != "" {
		summary += " " + t.Description
	}
	summary += fmt.Sprintf(
		" Buyer: %s. CPV: %s. Procedure: %s. Country: %s. Status: %s.",
		t.BuyerName, t.CPV, t.ProcedureType, t.Country, t.Status,
	)
	tenderID := strconv.FormatInt(t.ID, 10)
	chunks := []rag.Chunk{{
		ID:      fmt.Sprintf("%d_chunk_0", t.ID),
		DocID:   tenderID,
		Index:   0,
		Content: summary,
	}}

	// chunkIndex is a running counter across ALL of this tender's
	// documents, not reset per-document — reusing indices across
	// documents (e.g. 1,2,3 for doc A and 1,2,3 again for doc B) would
	// make their Qdrant point IDs collide (points are keyed on
	// tender+index, not tender+document+index), silently dropping one
	// document's chunks when the other's overwrite the same points.
	chunkIndex := 1
	for _, d := range t.Documents {
		parts, err := idx.repo.DocumentParts(ctx, d.ID)
		if err != nil {
			return fmt.Errorf("document parts for document %d: %w", d.ID, err)
		}
		if len(parts) == 0 {
			parts, err = idx.fetcher.FetchAndExtract(ctx, d.URL)
			if err != nil {
				return fmt.Errorf("fetch/extract document %d: %w", d.ID, err)
			}
			if err := idx.repo.SaveDocumentParts(ctx, d.ID, parts); err != nil {
				return fmt.Errorf("save document parts for document %d: %w", d.ID, err)
			}
		}
		for _, p := range parts {
			chunks = append(chunks, rag.Chunk{
				ID:      fmt.Sprintf("%d_chunk_%d", t.ID, chunkIndex),
				DocID:   tenderID,
				Index:   chunkIndex,
				Content: p.Text,
			})
			chunkIndex++
		}
	}

	doc := &rag.Document{
		ID:      tenderID,
		Content: summary,
		Chunks:  chunks,
		Metadata: map[string]string{
			"source":     t.Source,
			"source_ref": t.SourceRef,
		},
	}
	return idx.kb.IngestWithAttributes(ctx, doc, attributesFor(t))
}

// attributesFor projects a tender's structured columns onto the vector store's
// filterable payload. These deliberately duplicate what Postgres already
// holds: the vector search has to be able to narrow itself, and it can only do
// that from its own payload.
func attributesFor(t postgres.UnindexedTender) knowledge.Attributes {
	return knowledge.Attributes{
		Title:        t.Title,
		Country:      t.Country,
		Status:       t.Status,
		CPV:          t.CPV,
		CPVSecondary: t.CPVSecondary,
		NUTS:         t.NUTS,
		Value:        t.Value,
		PublishedAt:  t.PublishedAt,
		Deadline:     t.Deadline,
	}
}
