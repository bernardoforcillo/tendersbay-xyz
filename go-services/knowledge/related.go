package knowledge

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/drops/qdrant"
)

// relatedBuffer over-fetches recommend hits so self-exclusion and per-tender
// dedupe still leave `limit` distinct tenders.
const relatedBuffer = 10

// RelatedByDocID returns tenders semantically similar to docID, using Qdrant's
// recommend API seeded with docID's summary-chunk point (index 0) as the sole
// positive example — the tender's own stored embedding, no re-embedding. The
// docID itself and duplicate chunks of the same tender are removed; the
// best-scoring chunk per tender wins. Returns up to limit results.
func (kb *KnowledgeBase) RelatedByDocID(ctx context.Context, docID string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	hits, err := kb.qdrant.Recommend(ctx, kb.collection, qdrant.RecommendRequest{
		Positive:    []any{pointID(docID, 0)},
		Limit:       limit + relatedBuffer,
		WithPayload: true,
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge: recommend: %w", err)
	}
	seen := map[string]bool{docID: true}
	out := make([]SearchResult, 0, limit)
	for _, h := range hits {
		chunk := chunkFromHit(h)
		if chunk.DocID == "" || seen[chunk.DocID] {
			continue
		}
		seen[chunk.DocID] = true
		out = append(out, SearchResult{Chunk: chunk, Score: h.Score})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
