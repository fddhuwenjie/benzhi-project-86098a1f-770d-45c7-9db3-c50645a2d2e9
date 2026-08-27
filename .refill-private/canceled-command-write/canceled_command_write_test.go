package canceledcommandwrite

import (
	"context"
	"errors"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

func TestCanceledCommandDoesNotPersistMutation(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	fixed := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	svc := service.New(repository, func() time.Time { return fixed })
	create := service.CreateBatchInput{
		Meta: service.CommandMeta{RequestID: "create-cancel", ActorID: "admin-user", ExpectedRevision: 0},
		ID:   "batch-cancel-write", Title: "取消写入测试", SamplingSeed: "seed-cancel",
		QualityThresholds: domain.QualityThresholds{MinCompleteness: 1, MinAgreement: 1, MinStratumCoverage: 1},
	}
	if _, err := svc.CreateBatch(ctx, create); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = svc.AddClips(canceled, create.ID, service.AddClipsInput{
		Meta: service.CommandMeta{RequestID: "add-cancel", ActorID: "admin-user", ExpectedRevision: 1},
		Clips: []service.ClipInput{{
			ID: "clip-cancel", SourceURI: "s3://audio/cancel.wav",
			ContentDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			RegionCode:    "CN-YN", RecordedAt: fixed.Add(-time.Minute), DurationMS: 1000,
			CandidateTaxon: "Strix aluco",
		}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的写命令应返回 context.Canceled，得到 %v", err)
	}
	batch, err := repository.GetBatch(ctx, create.ID)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Revision != 1 || len(batch.Clips) != 0 {
		t.Fatalf("取消后的写命令不应持久化，修订=%d 片段数=%d", batch.Revision, len(batch.Clips))
	}
}
