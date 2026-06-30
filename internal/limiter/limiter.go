package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/harshhsharmaa57/rate-limiter.git/internal/store"
)

// Rule is the shared rule model used by the limiter and store packages.
type Rule = store.Rule

// Result is returned by the Limiter after checking a key.
type Result struct {
	Allowed   bool
	Remaining int64
	Limit     int64
	ResetAt   time.Time
}

// Limiter is the main rate limiting engine.
type Limiter struct {
	store     store.Store
	ruleCache *store.RuleCache
	quotas    *QuotaManager
}

// NewWithCache creates a Limiter with a RuleCache.
func NewWithCache(s store.Store, rc *store.RuleCache) *Limiter {
	return &Limiter{
		store:     s,
		ruleCache: rc,
		quotas:    NewQuotaManager(),
	}
}

// getRule looks up a rule by ID.
func (l *Limiter) getRule(ruleID string) (Rule, error) {
	rule, ok := l.ruleCache.Get(ruleID)
	if !ok {
		return Rule{}, fmt.Errorf("rule %q not found", ruleID)
	}
	return rule, nil
}

// Consume checks whether a request is allowed and records it.
func (l *Limiter) Consume(ctx context.Context, key string, ruleID string) (Result, error) {
	rule, err := l.getRule(ruleID)
	if err != nil {
		return Result{}, err
	}

	redisKey := fmt.Sprintf("rl:%s:%s", ruleID, key)

	allowed, count, err := l.store.Increment(ctx, redisKey, rule.Window(), rule.LimitCount)
	if err != nil {
		return Result{}, err
	}

	remaining := rule.LimitCount - count
	if remaining < 0 {
		remaining = 0
	}

	l.quotas.Publish(key, QuotaEvent{
		Key:       key,
		Used:      count,
		Remaining: remaining,
		Exceeded:  !allowed,
	})

	return Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     rule.LimitCount,
		ResetAt:   time.Now().Add(rule.Window()),
	}, nil
}

// Check checks without recording.
func (l *Limiter) Check(ctx context.Context, key string, ruleID string) (Result, error) {
	return l.Consume(ctx, key, ruleID)
}

// Subscribe wraps QuotaManager.Subscribe.
func (l *Limiter) Subscribe(key string) <-chan QuotaEvent {
	return l.quotas.Subscribe(key)
}

// Unsubscribe wraps QuotaManager.Unsubscribe.
func (l *Limiter) Unsubscribe(key string, ch <-chan QuotaEvent) {
	l.quotas.Unsubscribe(key, ch)
}
