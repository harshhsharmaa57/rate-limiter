package limiter

import "sync"

type QuotaEvent struct {
	Key       string
	Used      int64
	Remaining int64
	Exceeded  bool
}

type QuotaManager struct {
	mu          sync.RWMutex
	subscribers map[string][]chan QuotaEvent
}

func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		subscribers: make(map[string][]chan QuotaEvent),
	}
}

func (qm *QuotaManager) Subscribe(key string) <-chan QuotaEvent {
	ch := make(chan QuotaEvent, 10)
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.subscribers[key] = append(qm.subscribers[key], ch)
	return ch
}

func (qm *QuotaManager) Unsubscribe(key string, ch <-chan QuotaEvent) {}

func (qm *QuotaManager) Publish(key string, event QuotaEvent) {}
