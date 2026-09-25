package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type memoryItem struct {
	value   string
	expires time.Time
}

// Memory — кэш в памяти процесса: map + RWMutex + TTL.
type Memory struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

func NewMemory() *Memory {
	return &Memory{items: make(map[string]memoryItem)}
}

func (m *Memory) Get(_ context.Context, key string) (string, bool) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(item.expires) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return "", false
	}
	return item.value, true
}

func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = memoryItem{value: value, expires: time.Now().Add(ttl)}
}

func (m *Memory) Delete(_ context.Context, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

func (m *Memory) DeletePrefix(_ context.Context, prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.items {
		if strings.HasPrefix(key, prefix) {
			delete(m.items, key)
		}
	}
}
