package tender

import (
	"context"
	"errors"
	"testing"
)

// stubCache records what the service asks of it and can be made to fail in
// each of the ways a real cache does.
type stubCache struct {
	stored  map[string][]float32
	getErr  error
	setErr  error
	gets    int
	sets    int
	corrupt bool
}

func newStubCache() *stubCache { return &stubCache{stored: map[string][]float32{}} }

func (c *stubCache) GetEmbedding(_ context.Context, query string) ([]float32, bool, error) {
	c.gets++
	if c.getErr != nil {
		return nil, false, c.getErr
	}
	if c.corrupt {
		// The shape a decoder rejects: reported as a hit but unusable. A real
		// cache signals this as a miss, and so must this stub's contract.
		return nil, false, nil
	}
	vec, ok := c.stored[query]
	return vec, ok, nil
}

func (c *stubCache) SetEmbedding(_ context.Context, query string, vec []float32) error {
	c.sets++
	if c.setErr != nil {
		return c.setErr
	}
	c.stored[query] = vec
	return nil
}

// cacheTestService wires just enough of a Service for embedQuery.
func cacheTestService(kb KnowledgeBase, cache EmbeddingCache) *Service {
	s := &Service{kb: kb}
	if cache != nil {
		s = s.WithEmbeddingCache(cache)
	}
	return s
}

// countingKB counts how often the (expensive, network-bound) embedding call
// actually reaches the model server.
type countingKB struct {
	calls int
	err   error
}

func (k *countingKB) EmbedQuery(context.Context, string) ([]float32, error) {
	k.calls++
	if k.err != nil {
		return nil, k.err
	}
	return []float32{0.1, 0.2}, nil
}

func (k *countingKB) SearchByVector(context.Context, []float32, int, Filters) ([]ScoredChunk, error) {
	return nil, nil
}

func (k *countingKB) RelatedByDocID(context.Context, string, int) ([]ScoredChunk, error) {
	return nil, nil
}

// The point of the cache: embedding is an HTTP round trip on the critical path
// of every search, including every page of the same search.
func TestEmbedQuery_ReusesACachedEmbedding(t *testing.T) {
	kb := &countingKB{}
	cache := newStubCache()
	s := cacheTestService(kb, cache)
	ctx := context.Background()

	first, err := s.embedQuery(ctx, "lavori stradali")
	if err != nil {
		t.Fatalf("embedQuery: %v", err)
	}
	second, err := s.embedQuery(ctx, "lavori stradali")
	if err != nil {
		t.Fatalf("embedQuery (second): %v", err)
	}

	if kb.calls != 1 {
		t.Errorf("model server called %d times, want 1 — the second query should hit the cache", kb.calls)
	}
	if len(second) != len(first) || second[0] != first[0] {
		t.Errorf("cached vector = %v, want the same as the computed one %v", second, first)
	}
}

func TestEmbedQuery_DifferentQueriesDoNotShareAnEntry(t *testing.T) {
	kb := &countingKB{}
	s := cacheTestService(kb, newStubCache())
	ctx := context.Background()

	if _, err := s.embedQuery(ctx, "lavori stradali"); err != nil {
		t.Fatalf("embedQuery: %v", err)
	}
	if _, err := s.embedQuery(ctx, "servizi di pulizia"); err != nil {
		t.Fatalf("embedQuery: %v", err)
	}
	if kb.calls != 2 {
		t.Errorf("model server called %d times, want 2 — distinct queries need distinct embeddings", kb.calls)
	}
}

// Correctness must never depend on the cache: recomputing is always available
// and always right, so no cache failure may fail a search.
func TestEmbedQuery_ReadFailureFallsBackToComputing(t *testing.T) {
	kb := &countingKB{}
	cache := newStubCache()
	cache.getErr = errors.New("redis unreachable")
	s := cacheTestService(kb, cache)

	vec, err := s.embedQuery(context.Background(), "q")
	if err != nil {
		t.Fatalf("embedQuery: want the read failure absorbed, got %v", err)
	}
	if len(vec) == 0 {
		t.Error("embedQuery returned an empty vector")
	}
	if kb.calls != 1 {
		t.Errorf("model server called %d times, want 1", kb.calls)
	}
}

func TestEmbedQuery_WriteFailureDoesNotFailTheSearch(t *testing.T) {
	kb := &countingKB{}
	cache := newStubCache()
	cache.setErr = errors.New("redis read-only")
	s := cacheTestService(kb, cache)

	if _, err := s.embedQuery(context.Background(), "q"); err != nil {
		t.Fatalf("embedQuery: want the write failure absorbed, got %v", err)
	}
}

func TestEmbedQuery_CorruptEntryIsTreatedAsAMiss(t *testing.T) {
	kb := &countingKB{}
	cache := newStubCache()
	cache.corrupt = true
	s := cacheTestService(kb, cache)

	if _, err := s.embedQuery(context.Background(), "q"); err != nil {
		t.Fatalf("embedQuery: %v", err)
	}
	if kb.calls != 1 {
		t.Errorf("model server called %d times, want 1 — a corrupt entry must recompute", kb.calls)
	}
}

// The cache is optional; without one the service is fully correct, just slower.
func TestEmbedQuery_WorksWithoutACache(t *testing.T) {
	kb := &countingKB{}
	s := cacheTestService(kb, nil)

	if _, err := s.embedQuery(context.Background(), "q"); err != nil {
		t.Fatalf("embedQuery: %v", err)
	}
	if kb.calls != 1 {
		t.Errorf("model server called %d times, want 1", kb.calls)
	}
}

// A model-server failure is a real failure — the cache must not paper over it
// with a stale or empty vector.
func TestEmbedQuery_PropagatesModelServerErrors(t *testing.T) {
	kb := &countingKB{err: errors.New("ollama down")}
	cache := newStubCache()
	s := cacheTestService(kb, cache)

	if _, err := s.embedQuery(context.Background(), "q"); err == nil {
		t.Fatal("embedQuery: want the model server error surfaced, got nil")
	}
	if cache.sets != 0 {
		t.Error("a failed embedding was written to the cache")
	}
}
