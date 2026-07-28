package redis

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// embeddingKeyPrefix namespaces this cache's keys, for the same reason
// keyPrefix exists for the rate limiter: the Redis instance may be shared.
const embeddingKeyPrefix = "tb:backend:embedding:"

// EmbeddingCache memoises query embeddings.
//
// Embedding a query is an HTTP round trip to Ollama on the critical path of
// every search — including every page of the same search, and every repeat of
// a popular query. The result is a pure function of (model, text), so caching
// it is safe as long as the model is part of the key; that is what `model`
// below is for. Change the model and every key changes with it, so a stale
// vector from a previous model can never be served.
type EmbeddingCache struct {
	client *goredis.Client
	model  string
	ttl    time.Duration
}

// NewEmbeddingCache parses redisURL and returns a cache keyed for model,
// expiring entries after ttl. A non-positive ttl falls back to one hour.
func NewEmbeddingCache(redisURL, model string, ttl time.Duration) (*EmbeddingCache, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &EmbeddingCache{client: goredis.NewClient(opts), model: model, ttl: ttl}, nil
}

// key derives a fixed-length cache key. The query is hashed rather than
// embedded in the key so that arbitrarily long input, and input containing
// characters Redis would rather not see, still produce a well-formed key.
func (c *EmbeddingCache) key(query string) string {
	sum := sha256.Sum256([]byte(c.model + "\x00" + query))
	return embeddingKeyPrefix + hex.EncodeToString(sum[:])
}

// GetEmbedding returns the cached vector for query. A miss reports ok=false
// with a nil error; only an actual Redis failure returns an error.
func (c *EmbeddingCache) GetEmbedding(ctx context.Context, query string) ([]float32, bool, error) {
	raw, err := c.client.Get(ctx, c.key(query)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis: get embedding: %w", err)
	}
	vec, err := decodeVector(raw)
	if err != nil {
		// A corrupt entry is a miss, not a failure: recomputing is always
		// possible and always correct, so there is nothing to escalate.
		return nil, false, nil
	}
	return vec, true, nil
}

// SetEmbedding stores query's vector.
func (c *EmbeddingCache) SetEmbedding(ctx context.Context, query string, vec []float32) error {
	if len(vec) == 0 {
		return nil
	}
	if err := c.client.Set(ctx, c.key(query), encodeVector(vec), c.ttl).Err(); err != nil {
		return fmt.Errorf("redis: set embedding: %w", err)
	}
	return nil
}

// Close releases the underlying connection pool. Idempotent.
func (c *EmbeddingCache) Close() error { return c.client.Close() }

// encodeVector packs the vector as little-endian float32s — 4 bytes per
// dimension, so a 768-dim embedding costs about 3 KiB. JSON would cost several
// times that and lose exactness in the round trip.
func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(f))
	}
	return buf
}

// decodeVector reverses encodeVector, rejecting anything that isn't a whole
// number of float32s.
func decodeVector(buf []byte) ([]float32, error) {
	if len(buf) == 0 || len(buf)%4 != 0 {
		return nil, fmt.Errorf("redis: embedding blob is %d bytes, not a whole number of float32s", len(buf))
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[4*i:]))
	}
	return vec, nil
}
