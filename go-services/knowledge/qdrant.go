package knowledge

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/drops/qdrant"
)

const (
	collectionName = "tenders"
	vectorSize     = 768
)

// KnowledgeBase implements berrygem/rag.KnowledgeBase, backed by Qdrant for
// vector storage/search and Ollama for embeddings.
type KnowledgeBase struct {
	qdrant     *qdrant.Client
	embedder   *Embedder
	collection string
}

// NewKnowledgeBase connects to Qdrant at qdrantURL, ensures the "tenders"
// collection exists (768-dim, cosine distance — EmbeddingGemma's verified
// default output size), and returns a KnowledgeBase ready for
// Ingest/Search/Delete/List. Embeddings are computed via Ollama at
// ollamaBaseURL using embeddingModel.
func NewKnowledgeBase(ctx context.Context, qdrantURL, ollamaBaseURL, embeddingModel string) (*KnowledgeBase, error) {
	q, err := qdrant.NewClient(qdrantURL)
	if err != nil {
		return nil, fmt.Errorf("knowledge: qdrant client: %w", err)
	}

	exists, err := q.CollectionExists(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("knowledge: check collection: %w", err)
	}
	if !exists {
		if err := q.CreateCollection(ctx, collectionName, qdrant.CollectionConfig{
			Vectors: qdrant.VectorParams{Size: vectorSize, Distance: qdrant.DistanceCosine},
		}); err != nil {
			return nil, fmt.Errorf("knowledge: create collection: %w", err)
		}
	}

	return &KnowledgeBase{
		qdrant:     q,
		embedder:   NewEmbedder(ollamaBaseURL, embeddingModel),
		collection: collectionName,
	}, nil
}
