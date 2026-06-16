package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNoRaceCondition(t *testing.T) {
	ms := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	const numGoroutines = 1000
	const highLimit = numGoroutines + 1 // limit high enough that all should be allowed

	// Launch 1000 goroutines, each incrementing the same key once
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ms.Increment(ctx, "test-key", time.Minute, highLimit)
		}()
	}

	wg.Wait()

	// After 1000 goroutines each incrementing once, count must be exactly 1000
	finalCount := ms.counts["test-key"]
	if finalCount != numGoroutines {
		t.Errorf("expected count %d, got %d — you have a race condition", numGoroutines, finalCount)
	}
}
