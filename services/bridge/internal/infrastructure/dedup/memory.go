// Package dedup implementa a deduplicação por id de mensagem.
package dedup

import (
	"context"
	"sync"
	"time"
)

// Memory guarda ids em memória por uma janela; serve para uma instância só ou para testes.
type Memory struct {
	ttl  time.Duration
	now  func() time.Time
	mu   sync.Mutex
	seen map[string]time.Time
	last time.Time
}

func NewMemory(ttl time.Duration) *Memory {
	return &Memory{ttl: ttl, now: time.Now, seen: map[string]time.Time{}}
}

func (m *Memory) Claim(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.evict(now)
	if exp, ok := m.seen[id]; ok && exp.After(now) {
		return false, nil
	}
	m.seen[id] = now.Add(m.ttl)
	return true, nil
}

func (m *Memory) Release(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.seen, id)
	return nil
}

// evict varre no máximo uma vez por janela/10 para não pesar em cada Claim.
func (m *Memory) evict(now time.Time) {
	if now.Sub(m.last) < m.ttl/10 {
		return
	}
	m.last = now
	for id, exp := range m.seen {
		if !exp.After(now) {
			delete(m.seen, id)
		}
	}
}
