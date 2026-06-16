package store

import (
	"context"
	"time"
)

// Store is the contract any rate limit backend must fulfill.
// Both MemoryStore (testing) and RedisStore (production) implement this.
type Store interface {
	// Increment checks whether to allow a request and records it.
	// Returns: (allowed, currentCount, error)
	// - allowed: true if this request is within the limit
	// - currentCount: how many requests have been made in this window (including this one)
	// - error: any error from the backend (Redis down, etc.)
	Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error)

	// Reset clears all recorded requests for a key. Useful for testing.
	Reset(ctx context.Context, key string) error
}
