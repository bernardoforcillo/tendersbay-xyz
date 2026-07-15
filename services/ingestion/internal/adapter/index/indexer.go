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

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
)

// batchSize bounds how many unindexed tenders one RunOnce call processes,
// so a large backlog drains over several hourly cycles instead of making
// one run balloon in duration.
const batchSize = 200

// Repo is the subset of postgres.TenderRepo the indexer needs.
type Repo interface {
	ListUnindexed(ctx context.Context, limit int) ([]postgres.UnindexedTender, error)
	DocumentParts(ctx context.Context, documentID int64) ([]string, error)
	SaveDocumentParts(ctx context.Context, documentID int64, parts []string) error
	MarkIndexed(ctx context.Context, tenderID int64) error
}

// KnowledgeBase is the subset of knowledge.KnowledgeBase the indexer needs.
type KnowledgeBase interface {
	Ingest(ctx context.Context, doc *rag.Document) error
}

// Fetcher downloads and extracts one document's text.
type Fetcher interface {
	FetchAndExtract(ctx context.Context, url string) ([]string, error)
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
func (idx *Indexer) RunOnce(ctx context.Context) error {
	tenders, err := idx.repo.ListUnindexed(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("index: list unindexed: %w", err)
	}

	for _, t := range tenders {
		if err := idx.indexOne(ctx, t); err != nil {
			slog.ErrorContext(ctx, "failed to index tender", "tender_id", t.ID, "error", err)
			continue
		}
		if err := idx.repo.MarkIndexed(ctx, t.ID); err != nil {
			slog.ErrorContext(ctx, "failed to mark tender indexed", "tender_id", t.ID, "error", err)
		}
	}
	return nil
}

func (idx *Indexer) indexOne(ctx context.Context, t postgres.UnindexedTender) error {
	summary := fmt.Sprintf(
		"%s. Buyer: %s. CPV: %s. Procedure: %s. Country: %s. Status: %s.",
		t.Title, t.BuyerName, t.CPV, t.ProcedureType, t.Country, t.Status,
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
				Content: p,
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
	return idx.kb.Ingest(ctx, doc)
}
