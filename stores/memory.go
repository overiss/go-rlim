package stores

import (
	"context"
	"sync"
	"time"
)

// memoryEntry holds one fixed-window counter in the process heap.
type memoryEntry struct {
	// count is the number of increments in the current window.
	count int64
	// expires is when the current window ends; after this, the key logically resets.
	expires time.Time
	// lastSeen is updated on each touch for idle-based eviction in cleanup.
	lastSeen time.Time
}

// MemoryStore implements rlim.Store in RAM and runs a background goroutine to delete
// keys that are past their window expiry or idle longer than maxIdle.
type MemoryStore struct {
	mu         sync.Mutex
	items      map[string]*memoryEntry
	cleanEvery time.Duration
	maxIdle    time.Duration
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewMemoryStore starts an in-memory store and its cleaner loop. If cleanEvery <= 0,
// it defaults to one minute; if maxIdle <= 0, it defaults to five minutes.
func NewMemoryStore(cleanEvery, maxIdle time.Duration) *MemoryStore {
	if cleanEvery <= 0 {
		cleanEvery = time.Minute
	}
	if maxIdle <= 0 {
		maxIdle = 5 * time.Minute
	}

	s := &MemoryStore{
		items:      make(map[string]*memoryEntry),
		cleanEvery: cleanEvery,
		maxIdle:    maxIdle,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	go s.cleanLoop()
	return s
}

// Increment implements rlim.Store: creates or rolls the window when expired, else bumps count.
func (s *MemoryStore) Increment(_ context.Context, key string, window time.Duration, now time.Time) (int64, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.items[key]
	if !ok || now.After(entry.expires) {
		entry = &memoryEntry{
			count:    1,
			expires:  now.Add(window),
			lastSeen: now,
		}
		s.items[key] = entry
		return entry.count, entry.expires, nil
	}

	entry.count++
	entry.lastSeen = now
	return entry.count, entry.expires, nil
}

// cleanLoop ticks every cleanEvery until Close, invoking cleanup each tick.
func (s *MemoryStore) cleanLoop() {
	ticker := time.NewTicker(s.cleanEvery)
	defer func() {
		ticker.Stop()
		close(s.doneCh)
	}()

	for {
		select {
		case <-ticker.C:
			s.cleanup(time.Now())
		case <-s.stopCh:
			return
		}
	}
}

// cleanup removes entries past window expiry or idle beyond maxIdle (under mu).
func (s *MemoryStore) cleanup(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.items {
		if now.After(entry.expires) || now.Sub(entry.lastSeen) > s.maxIdle {
			delete(s.items, key)
		}
	}
}

// Close signals the cleaner goroutine to stop and blocks until it has exited.
func (s *MemoryStore) Close() error {
	close(s.stopCh)
	<-s.doneCh
	return nil
}
