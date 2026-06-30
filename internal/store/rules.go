package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Rule defines how rate limiting works for one plan tier.
type Rule struct {
	ID         string
	LimitCount int64
	WindowMs   int64
}

// Window converts WindowMs to a time.Duration.
func (r Rule) Window() time.Duration {
	return time.Duration(r.WindowMs) * time.Millisecond
}

// RuleCache loads rules from PostgreSQL and caches them in memory.
// Rules are refreshed every 30 seconds in the background.
// Using RWMutex: reads (every request) are concurrent, writes (refresh) are exclusive.
type RuleCache struct {
	mu    sync.RWMutex
	rules map[string]Rule
	db    *sql.DB
}

// NewRuleCache creates a RuleCache with a database connection.
func NewRuleCache(db *sql.DB) *RuleCache {
	return &RuleCache{
		rules: make(map[string]Rule),
		db:    db,
	}
}

// OpenDB opens a PostgreSQL connection.
func OpenDB(connStr string) (*sql.DB, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return db, nil
}

// Get returns the rule for a given ID.
// Returns false if the rule doesn't exist.
func (rc *RuleCache) Get(id string) (Rule, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	rule, ok := rc.rules[id]
	return rule, ok
}

// Refresh loads all rules from the database and updates the cache.
func (rc *RuleCache) Refresh(ctx context.Context) error {
	rows, err := rc.db.QueryContext(ctx, "SELECT id, limit_count, window_ms FROM rules")
	if err != nil {
		return fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	newRules := make(map[string]Rule)
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.LimitCount, &rule.WindowMs); err != nil {
			return fmt.Errorf("scan rule: %w", err)
		}
		newRules[rule.ID] = rule
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration: %w", err)
	}

	rc.mu.Lock()
	rc.rules = newRules
	rc.mu.Unlock()

	return nil
}

// StartRefreshLoop calls Refresh immediately, then every interval.
// Runs in a background goroutine — this function returns immediately.
func (rc *RuleCache) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	if err := rc.Refresh(ctx); err != nil {
		fmt.Printf("initial rule refresh failed: %v\n", err)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := rc.Refresh(ctx); err != nil {
					fmt.Printf("rule refresh failed: %v\n", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
