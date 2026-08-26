package service

import "sync"

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

type batchLocks struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}

func newBatchLocks() *batchLocks {
	return &batchLocks{entries: make(map[string]*lockEntry)}
}

func (l *batchLocks) acquire(batchID string) func() {
	l.mu.Lock()
	entry := l.entries[batchID]
	if entry == nil {
		entry = &lockEntry{}
		l.entries[batchID] = entry
	}
	entry.refs++
	l.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.entries, batchID)
		}
		l.mu.Unlock()
	}
}
