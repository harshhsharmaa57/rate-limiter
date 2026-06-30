package limiter

import "sync"

// QuotaEvent is sent to subscribers whenever a key's usage changes.
type QuotaEvent struct {
	Key       string
	Used      int64
	Remaining int64
	Exceeded  bool
}

// QuotaManager tracks subscriptions and broadcasts quota events.
type QuotaManager struct {
	mu          sync.RWMutex
	subscribers map[string][]chan QuotaEvent
}

// NewQuotaManager creates an empty QuotaManager.
func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		subscribers: make(map[string][]chan QuotaEvent),
	}
}

// Subscribe registers interest in events for a key.
func (qm *QuotaManager) Subscribe(key string) <-chan QuotaEvent {
	ch := make(chan QuotaEvent, 10)

	qm.mu.Lock()
	qm.subscribers[key] = append(qm.subscribers[key], ch)
	qm.mu.Unlock()

	return ch
}

// Unsubscribe removes a channel from the subscriber list and closes it.
func (qm *QuotaManager) Unsubscribe(key string, ch <-chan QuotaEvent) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	subs := qm.subscribers[key]
	newSubs := make([]chan QuotaEvent, 0, len(subs))
	for _, c := range subs {
		if c != ch {
			newSubs = append(newSubs, c)
		}
	}
	qm.subscribers[key] = newSubs
}

// Publish sends an event to all subscribers of a key.
func (qm *QuotaManager) Publish(key string, event QuotaEvent) {
	qm.mu.RLock()
	subs := qm.subscribers[key]
	qm.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}
