package canceled_lock_wait_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

func TestCanceledCommandDoesNotEnterRepositoryAfterLockWait(t *testing.T) {
	repository := &blockingRepository{
		firstFindEntered:  make(chan struct{}),
		releaseFirstFind:  make(chan struct{}),
		secondFindEntered: make(chan struct{}),
	}
	svc := service.New(repository, func() time.Time {
		return time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	})
	firstResult := make(chan error, 1)
	go func() {
		_, err := svc.CreateBatch(context.Background(), createInput("req-first"))
		firstResult <- err
	}()
	<-repository.firstFindEntered

	waitContext, cancel := context.WithCancel(context.Background())
	observed := &observedContext{Context: waitContext, checked: make(chan struct{})}
	secondResult := make(chan error, 1)
	go func() {
		_, err := svc.CreateBatch(observed, createInput("req-second"))
		secondResult <- err
	}()
	<-observed.checked
	cancel()
	close(repository.releaseFirstFind)

	if err := <-firstResult; err != nil {
		t.Fatalf("占锁命令失败: %v", err)
	}
	if err := <-secondResult; err != context.Canceled {
		t.Fatalf("取消命令返回 %v，期望 context.Canceled", err)
	}
	select {
	case <-repository.secondFindEntered:
		t.Fatal("已取消的命令在等待批次锁后仍进入了 Repository.FindCommand")
	default:
	}
}

func createInput(requestID string) service.CreateBatchInput {
	return service.CreateBatchInput{
		Meta: service.CommandMeta{RequestID: requestID, ActorID: "admin-one"},
		ID:   "batch-lock-context", Title: "锁等待取消复现", SamplingSeed: "seed-lock-context",
		QualityThresholds: domain.QualityThresholds{},
	}
}

type observedContext struct {
	context.Context
	once    sync.Once
	checked chan struct{}
}

func (c *observedContext) Err() error {
	first := false
	c.once.Do(func() {
		first = true
		close(c.checked)
	})
	if first {
		return nil
	}
	return c.Context.Err()
}

type blockingRepository struct {
	mu                sync.Mutex
	findCalls         int
	firstFindEntered  chan struct{}
	releaseFirstFind  chan struct{}
	secondFindEntered chan struct{}
}

func (r *blockingRepository) FindCommand(ctx context.Context, _, _ string) (*store.IdempotencyRecord, error) {
	r.mu.Lock()
	r.findCalls++
	call := r.findCalls
	r.mu.Unlock()
	if call == 1 {
		close(r.firstFindEntered)
		<-r.releaseFirstFind
		return nil, domain.ErrNotFound
	}
	close(r.secondFindEntered)
	return nil, ctx.Err()
}

func (r *blockingRepository) CreateCommand(context.Context, store.CommandWrite) error {
	return nil
}

func (r *blockingRepository) UpdateCommand(context.Context, store.CommandWrite) error {
	return nil
}

func (r *blockingRepository) GetBatch(context.Context, string) (*domain.CorpusBatch, error) {
	return nil, domain.ErrNotFound
}

func (r *blockingRepository) ListAudit(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}

func (r *blockingRepository) Close() error {
	return nil
}
