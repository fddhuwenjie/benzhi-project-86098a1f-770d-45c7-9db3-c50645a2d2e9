package service

import (
	"context"
	"sync"
)

type lockEntry struct {
	sem  chan struct{} // 容量为 1 的信号量：空表示可用，有令牌表示已占用
	refs int
}

type batchLocks struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}

func newBatchLocks() *batchLocks {
	return &batchLocks{entries: make(map[string]*lockEntry)}
}

func (l *batchLocks) acquire(ctx context.Context, batchID string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	entry := l.entries[batchID]
	if entry == nil {
		entry = &lockEntry{sem: make(chan struct{}, 1)}
		l.entries[batchID] = entry
	}
	entry.refs++
	l.mu.Unlock()

	// 通过 select 在「取得信号量」与「context 取消」之间竞争，使等待方在
	// context 取消时立即返回，不再持有锁或访问仓储。
	select {
	case entry.sem <- struct{}{}:
		// 极端情况下取得信号量与 context 取消几乎同时发生：取得后仍需复检
		// context，确保取消的请求不会继续执行仓储写事务，并把锁交接给下一
		// 个等待者。
		if err := ctx.Err(); err != nil {
			<-entry.sem
			l.decRef(batchID, entry)
			return nil, err
		}
		return func() {
			<-entry.sem
			l.decRef(batchID, entry)
		}, nil
	case <-ctx.Done():
		// 等待期间 context 取消：不占用锁，仅回收引用计数后返回。
		l.decRef(batchID, entry)
		return nil, ctx.Err()
	}
}

func (l *batchLocks) decRef(batchID string, entry *lockEntry) {
	l.mu.Lock()
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, batchID)
	}
	l.mu.Unlock()
}
