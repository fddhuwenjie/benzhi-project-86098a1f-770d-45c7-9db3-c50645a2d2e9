package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/store"
)

func TestCreateIdempotencySurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "idempotency.db")
	fixed := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	input := CreateBatchInput{
		Meta: CommandMeta{RequestID: "request-create", ActorID: "admin-user", ExpectedRevision: 0},
		ID:   "batch-persist", Title: "持久化批次", SamplingSeed: "persist-seed",
		QualityThresholds: domain.QualityThresholds{MinCompleteness: 1, MinAgreement: .8, MinStratumCoverage: 1},
	}
	repository, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := New(repository, func() time.Time { return fixed }).CreateBatch(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	repository, err = store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	svc := New(repository, func() time.Time { return fixed.Add(time.Hour) })
	replayed, err := svc.CreateBatch(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Batch.Revision != first.Batch.Revision || !replayed.Batch.CreatedAt.Equal(first.Batch.CreatedAt) {
		t.Fatalf("幂等重放未返回原始响应: %#v %#v", first, replayed)
	}
	input.Title = "不同命令"
	if _, err := svc.CreateBatch(ctx, input); !errors.Is(err, domain.ErrIdempotency) {
		t.Fatalf("相同 request_id 的不同命令应冲突，得到 %v", err)
	}
}
