package canceledquerycontext

import (
	"context"
	"errors"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

type contextCheckingRepository struct {
	seen context.Context
}

func (r *contextCheckingRepository) CreateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unused")
}
func (r *contextCheckingRepository) UpdateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unused")
}
func (r *contextCheckingRepository) FindCommand(context.Context, string, string) (*store.IdempotencyRecord, error) {
	return nil, domain.ErrNotFound
}
func (r *contextCheckingRepository) GetBatch(ctx context.Context, _ string) (*domain.CorpusBatch, error) {
	r.seen = ctx
	if ctx.Err() == nil {
		return &domain.CorpusBatch{ID: "batch-cancel", Clips: []domain.RecordingClip{}}, nil
	}
	return nil, ctx.Err()
}
func (r *contextCheckingRepository) ListAudit(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unused")
}
func (r *contextCheckingRepository) Close() error { return nil }

func TestCanceledQueryPropagatesContextToRepository(t *testing.T) {
	repo := &contextCheckingRepository{}
	svc := service.New(repo, func() time.Time { return time.Unix(0, 0) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetBatch(ctx, "batch-cancel")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消的查询应返回 context.Canceled，得到 %v", err)
	}
	if repo.seen == nil || !errors.Is(repo.seen.Err(), context.Canceled) {
		t.Fatalf("仓储未收到已取消的 request context: %v", repo.seen)
	}
}
