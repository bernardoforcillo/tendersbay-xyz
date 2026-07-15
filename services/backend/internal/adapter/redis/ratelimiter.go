// Package redis implements a fixed-window rate limiter backed by a
// dedicated Redis client (github.com/redis/go-redis/v9), used only for
// atomic per-key request counting. This is deliberately NOT built on the
// repo's shared drops/cache library — that library has no atomic
// increment (only Get/Set/Delete/Exists/TTL), and a counter built from
// Get+Set would race under concurrent requests from the same key,
// defeating the purpose of a cross-replica-correct rate limit.
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// keyPrefix namespaces every key this package touches. redis-release-master
// (namespace redis) may be shared with other workloads on the same DB 0; an
// unprefixed bare IP/user-ID key could collide with an unrelated workload's
// key and silently cross-contaminate counters. Applied inside Allow/Reset
// themselves so every caller gets it for free.
const keyPrefix = "tb:backend:ratelimit:"

// RateLimiter enforces a fixed-window request cap per key.
type RateLimiter struct {
	client *goredis.Client
}

// NewRateLimiter parses redisURL (e.g. "redis://localhost:6379") and
// returns a RateLimiter backed by it.
func NewRateLimiter(redisURL string) (*RateLimiter, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	return &RateLimiter{client: goredis.NewClient(opts)}, nil
}

// Allow reports whether one more request under key is permitted within
// window, given a maximum of limit requests per window. The first call
// for a fresh key starts a new window (TTL = window, set only once, so a
// client hammering just under the limit can't indefinitely extend it);
// later calls within the same window share its remaining TTL.
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	key = keyPrefix + key
	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis: incr %s: %w", key, err)
	}
	if count == 1 {
		if err := rl.client.Expire(ctx, key, window).Err(); err != nil {
			return false, fmt.Errorf("redis: expire %s: %w", key, err)
		}
	}
	return count <= limit, nil
}

// Reset removes key's counter entirely — used by tests to isolate runs;
// production callers never need this (windows expire on their own).
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.client.Del(ctx, keyPrefix+key).Err()
}

// Ping verifies Redis is reachable.
func (rl *RateLimiter) Ping(ctx context.Context) error {
	return rl.client.Ping(ctx).Err()
}

// Close releases the underlying connection pool. Idempotent.
func (rl *RateLimiter) Close() error {
	return rl.client.Close()
}
