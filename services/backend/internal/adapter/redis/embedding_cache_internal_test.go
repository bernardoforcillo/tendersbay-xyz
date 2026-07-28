package redis

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestVectorRoundTripsExactly(t *testing.T) {
	// Values chosen to catch a lossy encoding: a JSON round trip through
	// float64 and back would not reproduce these bit-for-bit.
	want := []float32{0, -1, 0.1, 1e-8, 3.4028235e+38, float32(math.Pi)}

	got, err := decodeVector(encodeVector(want))
	if err != nil {
		t.Fatalf("decodeVector: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("vec[%d] = %v, want %v (exact round trip)", i, got[i], want[i])
		}
	}
}

func TestEncodeVector_UsesFourBytesPerDimension(t *testing.T) {
	// A 768-dim embedding must stay around 3 KiB; anything larger would make
	// caching cost more memory than it saves.
	if got := len(encodeVector(make([]float32, 768))); got != 3072 {
		t.Errorf("encoded size = %d bytes, want 3072", got)
	}
}

func TestDecodeVector_RejectsMalformedBlobs(t *testing.T) {
	for _, blob := range [][]byte{nil, {}, {1}, {1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := decodeVector(blob); err == nil {
			t.Errorf("decodeVector(%v) = nil error, want a rejection", blob)
		}
	}
}

// The model is part of the key so that switching EMBEDDING_MODEL can never
// serve a vector computed by the previous model — which would be silently
// wrong rather than visibly broken.
func TestKey_DependsOnBothModelAndQuery(t *testing.T) {
	a := &EmbeddingCache{model: "embeddinggemma:latest"}
	b := &EmbeddingCache{model: "nomic-embed-text:v1.5"}

	if a.key("appalto") == b.key("appalto") {
		t.Error("the same query under two models produced the same key")
	}
	if a.key("appalto") == a.key("appalti") {
		t.Error("two different queries produced the same key")
	}
	if a.key("appalto") != a.key("appalto") {
		t.Error("key is not deterministic")
	}
}

func TestKey_IsNamespacedAndFixedLength(t *testing.T) {
	c := &EmbeddingCache{model: "m"}
	short, long := c.key("a"), c.key(strings.Repeat("x", 100_000))

	if !strings.HasPrefix(short, embeddingKeyPrefix) {
		t.Errorf("key %q is not namespaced — Redis may be shared with other workloads", short)
	}
	// Hashing is what keeps arbitrarily long user input from producing an
	// arbitrarily long key.
	if len(short) != len(long) {
		t.Errorf("key lengths differ (%d vs %d) — the query should be hashed, not embedded", len(short), len(long))
	}
}

// The separator prevents a model/query boundary collision: without it,
// ("ab", "c") and ("a", "bc") would hash identically.
func TestKey_SeparatesModelFromQuery(t *testing.T) {
	ab := (&EmbeddingCache{model: "ab"}).key("c")
	a := (&EmbeddingCache{model: "a"}).key("bc")
	if ab == a {
		t.Error("model and query concatenate ambiguously")
	}
}

func TestNewEmbeddingCache_DefaultsANonPositiveTTL(t *testing.T) {
	c, err := NewEmbeddingCache("redis://localhost:6379", "m", 0)
	if err != nil {
		t.Fatalf("NewEmbeddingCache: %v", err)
	}
	defer c.Close()
	if c.ttl != time.Hour {
		t.Errorf("ttl = %v, want the one-hour default — a zero TTL means 'never expire' in Redis", c.ttl)
	}
}

func TestNewEmbeddingCache_RejectsAMalformedURL(t *testing.T) {
	if _, err := NewEmbeddingCache("not-a-redis-url", "m", time.Hour); err == nil {
		t.Fatal("NewEmbeddingCache: want an error for a malformed URL, got nil")
	}
}
