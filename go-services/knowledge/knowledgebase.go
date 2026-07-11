package knowledge

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/drops/qdrant"
	"github.com/buildwithgo/berrygem/rag"
)

// Ingest embeds and upserts every chunk of doc as a Qdrant point. If
// doc.Chunks is empty, doc.Content is used as a single chunk (index 0) —
// so a document with no pre-computed chunking is still searchable. Each
// point's payload carries "content" (so Search can reconstruct
// rag.Chunk.Content from a hit) and "tender_id" (doc.ID — what Delete
// filters on), plus every key from doc.Metadata passed through verbatim.
// On success, the computed embeddings are written back onto doc.Chunks,
// mirroring berrygem's own InMemoryKB.Ingest behavior.
func (kb *KnowledgeBase) Ingest(ctx context.Context, doc *rag.Document) error {
	chunks := doc.Chunks
	if len(chunks) == 0 {
		chunks = []rag.Chunk{{ID: fmt.Sprintf("%s_chunk_0", doc.ID), DocID: doc.ID, Index: 0, Content: doc.Content}}
	}

	points := make([]qdrant.Point, len(chunks))
	for i, c := range chunks {
		vec, err := kb.embedder.Embed(ctx, c.Content)
		if err != nil {
			return fmt.Errorf("knowledge: embed chunk %s: %w", c.ID, err)
		}

		payload := map[string]any{
			"content":     c.Content,
			"tender_id":   doc.ID,
			"chunk_index": c.Index,
		}
		for k, v := range doc.Metadata {
			payload[k] = v
		}

		points[i] = qdrant.Point{
			ID:      fmt.Sprintf("%s_chunk_%d", doc.ID, c.Index),
			Vector:  vec,
			Payload: payload,
		}

		if doc.Chunks != nil {
			embedding := make([]float64, len(vec))
			for j, f := range vec {
				embedding[j] = float64(f)
			}
			doc.Chunks[i].Embedding = embedding
		}
	}

	return kb.qdrant.Upsert(ctx, kb.collection, points, qdrant.WriteOptions{Wait: true})
}

// Search embeds query and returns the limit nearest chunks by cosine
// similarity, reconstructed from each hit's payload (see Ingest — payload
// always carries "content", "tender_id", "chunk_index").
func (kb *KnowledgeBase) Search(ctx context.Context, query string, limit int) ([]rag.Chunk, error) {
	vec, err := kb.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("knowledge: embed query: %w", err)
	}

	hits, err := kb.qdrant.Search(ctx, kb.collection, qdrant.SearchRequest{
		Vector:      vec,
		Limit:       limit,
		WithPayload: true,
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge: search: %w", err)
	}

	chunks := make([]rag.Chunk, len(hits))
	for i, h := range hits {
		content, _ := h.Payload["content"].(string)
		docID, _ := h.Payload["tender_id"].(string)
		index := 0
		if idx, ok := h.Payload["chunk_index"].(float64); ok {
			index = int(idx)
		}
		chunks[i] = rag.Chunk{
			ID:      fmt.Sprint(h.ID),
			DocID:   docID,
			Index:   index,
			Content: content,
		}
	}
	return chunks, nil
}

// Delete removes every chunk belonging to docID.
func (kb *KnowledgeBase) Delete(ctx context.Context, docID string) error {
	return kb.qdrant.DeleteByFilter(ctx, kb.collection, qdrant.Must(qdrant.Eq("tender_id", docID)))
}

// scrollPageSize is the page size used internally by List when scrolling
// through the full collection.
const scrollPageSize = 100

// List returns every indexed document, reconstructed by grouping Qdrant
// points on their "tender_id" payload key. This is a low-usage admin/debug
// path, not on the hot ingest or search path — it pages through the entire
// collection via Scroll.
func (kb *KnowledgeBase) List(ctx context.Context) ([]rag.Document, error) {
	docs := map[string]*rag.Document{}
	var offset any
	for {
		page, err := kb.qdrant.Scroll(ctx, kb.collection, qdrant.ScrollRequest{
			Limit:       scrollPageSize,
			Offset:      offset,
			WithPayload: true,
		})
		if err != nil {
			return nil, fmt.Errorf("knowledge: scroll: %w", err)
		}
		for _, p := range page.Points {
			docID, _ := p.Payload["tender_id"].(string)
			content, _ := p.Payload["content"].(string)
			d, ok := docs[docID]
			if !ok {
				d = &rag.Document{ID: docID}
				docs[docID] = d
			}
			d.Chunks = append(d.Chunks, rag.Chunk{ID: fmt.Sprint(p.ID), DocID: docID, Content: content})
		}
		if page.NextPageOffset == nil {
			break
		}
		offset = page.NextPageOffset
	}

	result := make([]rag.Document, 0, len(docs))
	for _, d := range docs {
		result = append(result, *d)
	}
	return result, nil
}

var _ rag.KnowledgeBase = (*KnowledgeBase)(nil)
