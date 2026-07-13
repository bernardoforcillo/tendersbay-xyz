package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/redis"
)

func testRateLimiter(t *testing.T) *redis.RateLimiter {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	rl, err := redis.NewRateLimiter(url)
	if err != nil {
		t.Fatalf("NewRateLimiter: %v", err)
	}
	t.Cleanup(func() { _ = rl.Close() })
	return rl
}

func TestAllow_PermitsUpToLimitThenDenies(t *testing.T) {
	rl := testRateLimiter(t)
	ctx := context.Background()
	key := "test:allow-up-to-limit"
	t.Cleanup(func() { _ = rl.Reset(ctx, key) })

	for i := 0; i < 3; i++ {
		allowed, err := rl.Allow(ctx, key, 3, time.Minute)
		if err != nil {
			t.Fatalf("Allow (request %d): %v", i, err)
		}
		if !allowed {
			t.Fatalf("Allow (request %d) = false, want true (within limit of 3)", i)
		}
	}
	allowed, err := rl.Allow(ctx, key, 3, time.Minute)
	if err != nil {
		t.Fatalf("Allow (4th request): %v", err)
	}
	if allowed {
		t.Error("Allow (4th request) = true, want false (limit of 3 exceeded)")
	}
}

func TestAllow_DifferentKeysHaveIndependentLimits(t *testing.T) {
	rl := testRateLimiter(t)
	ctx := context.Background()
	keyA, keyB := "test:independent-a", "test:independent-b"
	t.Cleanup(func() { _ = rl.Reset(ctx, keyA); _ = rl.Reset(ctx, keyB) })

	for i := 0; i < 2; i++ {
		if allowed, err := rl.Allow(ctx, keyA, 2, time.Minute); err != nil || !allowed {
			t.Fatalf("Allow(keyA, %d): allowed=%v err=%v", i, allowed, err)
		}
	}
	allowed, err := rl.Allow(ctx, keyB, 2, time.Minute)
	if err != nil {
		t.Fatalf("Allow(keyB): %v", err)
	}
	if !allowed {
		t.Error("Allow(keyB) = false, want true (keyA's usage must not affect keyB)")
	}
}

func TestPing_SucceedsAgainstRealRedis(t *testing.T) {
	rl := testRateLimiter(t)
	if err := rl.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
