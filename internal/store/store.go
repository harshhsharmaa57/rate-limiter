package store

import (
	"context"
	"time"
)

// Store is the contract any rate limit backend must fulfill.
type Store interface {
	Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error)
	Reset(ctx context.Context, key string) error
}
