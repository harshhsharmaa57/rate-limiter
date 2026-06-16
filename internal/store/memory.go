package store

import (
	"context"
	"sync"
	"time"
)

// MemoryStore implements Store using an in-memory map.
// It is safe to use from multiple goroutines (thread-safe).
// It does NOT implement sliding window — it just counts.
// RedisStore (Part 8) will implement the real sliding window.
type MemoryStore struct {
	mu     sync.Mutex       // protects the counts map from race conditions
	counts map[string]int64 // key → number of requests
}

// NewMemoryStore creates a properly initialized MemoryStore.
// Note: we use make() to initialize the map. Without this, writing to counts will panic.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		counts: make(map[string]int64),
	}
}

// Increment increments the count for the given key and checks against the limit.
// YOUR TASK: implement this.
//
// You must:
//  1. Lock the mutex (and defer unlock)
//  2. Increment counts[key] by 1
//  3. Read the new value
//  4. If the value is <= limit: return (true, newValue, nil)   ← allowed
//     If the value is >  limit: return (false, newValue, nil)  ← rejected
//
// Note: we ignore the window parameter for now. MemoryStore doesn't expire counts.
// RedisStore will handle the real sliding window.
//
// Note: we ignore the ctx parameter for now. Pass it to other functions if needed,
// but MemoryStore doesn't need it.
func (ms *MemoryStore) Increment(ctx context.Context, key string, window time.Duration, limit int64) (bool, int64, error) {
	// YOUR CODE HERE
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.counts[key]++
	r := ms.counts[key]
	if r <= limit {
		return true, r, nil
	} else {
		return false, r, nil
	}
}

// Reset clears the count for the given key.
// YOUR TASK: implement this.
//
// You must:
//  1. Lock the mutex (and defer unlock)
//  2. Use delete(ms.counts, key) to remove the key
//  3. Return nil
func (ms *MemoryStore) Reset(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.counts, key)
	return nil
}
