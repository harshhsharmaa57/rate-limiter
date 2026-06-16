package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harshhsharmaa57/rate-limiter.git/internal/store"
)

// Rule defines how rate limiting works for one plan tier.
// For example: "free-tier" allows 100 requests per 60 seconds.
type Rule struct {
	ID         string // unique identifier, e.g. "free-tier"
	LimitCount int64  // max requests allowed in the window
	WindowMs   int64  // window size in milliseconds
}

// Window converts WindowMs (an int64) to a time.Duration.
// YOUR TASK: implement this.
// Hint: time.Duration is in nanoseconds. time.Millisecond is 1ms as a Duration.
//
//	Multiply r.WindowMs (int64) by time.Millisecond and return.
//	You'll need to convert r.WindowMs to time.Duration first.
func (r *Rule) Window() time.Duration {
	return time.Duration(r.LimitCount) * time.Second
}

// Result is returned by the Limiter after checking a key.
type Result struct {
	Allowed   bool      // was this request allowed?
	Remaining int64     // how many requests left in the current window
	Limit     int64     // the configured limit
	ResetAt   time.Time // when the window resets
}

// Keep your Rule and Result types from before, then add:

// Limiter is the main rate limiting engine.
// It uses a Store to track counts and a rule map to know the limits.
type Limiter struct {
	store  store.Store     // the backing store (Memory or Redis)
	rules  map[string]Rule // ruleID → Rule configuration
	mu     sync.RWMutex    // protects the rules map
	quotas *QuotaManager   // publishes quota events (add this in Part 9)
}

// New creates a Limiter with the given store.
func New(s store.Store) *Limiter {
	return &Limiter{
		store: s,
		rules: make(map[string]Rule),
	}
}

// AddRule adds or replaces a rule. Safe to call from multiple goroutines.
// YOUR TASK: implement this.
// Requirements:
//   - Use a write lock (mu.Lock, not mu.RLock)
//   - Store the rule: l.rules[rule.ID] = rule
func (l *Limiter) AddRule(rule Rule) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rules[rule.ID] = rule
}

// getRule looks up a rule by ID. Returns an error if the rule doesn't exist.
// YOUR TASK: implement this.
// Requirements:
//   - Use a read lock (mu.RLock)
//   - Look up l.rules[ruleID]
//   - If the rule doesn't exist (ok == false from the map lookup), return:
//     return Rule{}, fmt.Errorf("rule %q not found", ruleID)
//   - Otherwise return the rule and nil error
func (l *Limiter) getRule(ruleID string) (Rule, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rule, err := l.rules[ruleID]
	if !err {
		return Rule{}, fmt.Errorf("Rule %s not found", ruleID)
	} else {

		return rule, nil

	}
}

// Consume checks whether a request is allowed and records it.
// This is the main function your gRPC server will call.
// YOUR TASK: implement this.
// Requirements:
//  1. Look up the rule using l.getRule(ruleID)
//     If error: return zero Result and the error
//  2. Build the Redis key: fmt.Sprintf("rl:%s:%s", ruleID, key)
//     Example: "rl:free-tier:user:harsh123"
//  3. Call l.store.Increment(ctx, redisKey, rule.Window(), rule.LimitCount)
//     If error: return zero Result and the error
//  4. Calculate remaining: rule.LimitCount - count  (but not less than 0)
//  5. Return Result{Allowed: allowed, Remaining: remaining, Limit: rule.LimitCount}
func (l *Limiter) Consume(ctx context.Context, key string, ruleID string) (Result, error) {
	rule, err := l.getRule(ruleID)
	if err != nil {
		return Result{}, err
	}
	rediskey := fmt.Sprintf("rl:%s:%s", ruleID, key)
	allow, count, err := l.store.Increment(ctx, rediskey, rule.Window(), rule.LimitCount)
	
}
