// Package knowledgekb adapts go-services/knowledge's Qdrant/Ollama-backed
// KnowledgeBase to internal/core/tender's narrow KnowledgeBase port (see
// tender.go's package doc for why core/tender doesn't depend on
// go-services/knowledge directly), and provides a stub that always errors
// so tender.Service's filters-only fallback engages when no knowledge base
// is configured or its construction failed at startup.
package knowledgekb

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// KB adapts a *knowledge.KnowledgeBase to tender.KnowledgeBase.
type KB struct {
	kb *knowledge.KnowledgeBase
}

// New wraps kb as a tender.KnowledgeBase.
func New(kb *knowledge.KnowledgeBase) *KB {
	return &KB{kb: kb}
}

// SearchWithScores satisfies tender.KnowledgeBase, mapping each
// knowledge.SearchResult (a rag.Chunk plus its Qdrant score) down to
// tender's own minimal ScoredChunk — just the chunk's DocID and Score,
// which is all Search needs to rank and dedupe hits per tender.
func (a *KB) SearchWithScores(ctx context.Context, query string, limit int) ([]tender.ScoredChunk, error) {
	hits, err := a.kb.SearchWithScores(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(hits))
	for i, h := range hits {
		out[i] = tender.ScoredChunk{DocID: h.DocID, Score: h.Score}
	}
	return out, nil
}

var _ tender.KnowledgeBase = (*KB)(nil)

// errUnavailable is Unavailable.SearchWithScores's static error.
var errUnavailable = errors.New("knowledgekb: knowledge base unavailable")

// Unavailable is a tender.KnowledgeBase that always errors, so
// tender.Service's filters-only fallback engages — main.go substitutes
// this when knowledge.NewKnowledgeBase fails at startup (a non-fatal
// condition; see main.go's construction block).
type Unavailable struct{}

// SearchWithScores always returns errUnavailable.
func (Unavailable) SearchWithScores(context.Context, string, int) ([]tender.ScoredChunk, error) {
	return nil, errUnavailable
}

var _ tender.KnowledgeBase = Unavailable{}
