package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/store"
)

func TestExtendedCommandsPersistIdempotentlyAndExposeQualityHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "extensions.db")
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(repository, func() time.Time { return now })
	create := CreateBatchInput{
		Meta: CommandMeta{RequestID: "create-extended", ActorID: "admin-user", ExpectedRevision: 0},
		ID:   "batch-extended", Title: "扩展流程批次", SamplingSeed: "seed-extended",
		QualityThresholds: domain.QualityThresholds{MinCompleteness: 1, MinAgreement: 1, MinStratumCoverage: 1},
	}
	if _, err := svc.CreateBatch(ctx, create); err != nil {
		t.Fatal(err)
	}
	clips := AddClipsInput{Meta: CommandMeta{RequestID: "add-extended", ActorID: "admin-user", ExpectedRevision: 1}}
	for i, id := range []string{"clip-one", "clip-two", "clip-three"} {
		digestByte := byte('a' + i)
		clips.Clips = append(clips.Clips, ClipInput{
			ID: id, SourceURI: "s3://extended/" + id + ".wav",
			ContentDigest: repeatHex(digestByte), RegionCode: "CN-YN",
			RecordedAt: now.Add(-time.Hour), DurationMS: 1000, CandidateTaxon: "Strix aluco",
		})
	}
	if _, err := svc.AddClips(ctx, create.ID, clips); err != nil {
		t.Fatal(err)
	}
	duration := int64(1500)
	correction := CorrectClipsInput{Meta: CommandMeta{RequestID: "correct-extended", ActorID: "admin-user", ExpectedRevision: 2}, Clips: []ClipPatchInput{{ID: "clip-two", DurationMS: &duration}}}
	corrected, err := svc.CorrectClips(ctx, create.ID, correction)
	if err != nil || corrected.Batch.Revision != 3 || len(corrected.ClipMutation.Changes) != 1 {
		t.Fatalf("纠错命令结果无效: %#v %v", corrected, err)
	}
	withdrawal := WithdrawClipsInput{Meta: CommandMeta{RequestID: "withdraw-extended", ActorID: "admin-user", ExpectedRevision: 3}, ClipIDs: []string{"clip-three"}}
	withdrawn, err := svc.WithdrawClips(ctx, create.ID, withdrawal)
	if err != nil || withdrawn.Batch.Revision != 4 {
		t.Fatalf("撤销命令结果无效: %#v %v", withdrawn, err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	repository, err = store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	svc = New(repository, func() time.Time { return now.Add(time.Minute) })
	replayed, err := svc.WithdrawClips(ctx, create.ID, withdrawal)
	if err != nil || replayed.Batch.Revision != withdrawn.Batch.Revision {
		t.Fatalf("撤销命令跨重启重放失败: %#v %v", replayed, err)
	}
	if _, err := svc.SubmitDraft(ctx, create.ID, MetaInput{Meta: CommandMeta{RequestID: "submit-extended", ActorID: "admin-user", ExpectedRevision: 4}}); err != nil {
		t.Fatal(err)
	}
	stratum := "CN-YN|2026-08|strix aluco"
	if _, err := svc.LockSample(ctx, create.ID, SampleInput{Meta: CommandMeta{RequestID: "sample-extended", ActorID: "admin-user", ExpectedRevision: 5}, Quota: map[string]int{stratum: 2}}); err != nil {
		t.Fatal(err)
	}
	items := []AnnotationItemInput{
		{ClipID: "clip-two", TaxonLabel: "Strix aluco", Confidence: .8, EvidenceNote: "批量交付证据"},
		{ClipID: "clip-one", TaxonLabel: "Strix aluco", Confidence: .9, EvidenceNote: "批量交付证据"},
	}
	first, err := svc.SubmitAnnotations(ctx, create.ID, AnnotationDeliveryInput{Meta: CommandMeta{RequestID: "annotations-a", ActorID: "annotator-a", ExpectedRevision: 6}, Annotations: items})
	if err != nil || first.Batch.Revision != 7 || first.AnnotationDelivery.CreatedCount != 2 {
		t.Fatalf("首份批量标注失败: %#v %v", first, err)
	}
	visible, err := svc.GetAnnotations(ctx, create.ID, "clip-one", "annotator-b")
	if err != nil || len(visible) != 0 {
		t.Fatal("批量交付后盲标隔离失效")
	}
	second, err := svc.SubmitAnnotations(ctx, create.ID, AnnotationDeliveryInput{Meta: CommandMeta{RequestID: "annotations-b", ActorID: "annotator-b", ExpectedRevision: 7}, Annotations: items})
	if err != nil || second.Batch.Revision != 8 || second.Batch.Status != domain.StatusPendingQualityReview {
		t.Fatalf("第二份批量标注失败: %#v %v", second, err)
	}
	qualityInput := MetaInput{Meta: CommandMeta{RequestID: "quality-extended", ActorID: "quality-owner", ExpectedRevision: 8}}
	quality, err := svc.RunQualityGate(ctx, create.ID, qualityInput)
	if err != nil || quality.Batch.Status != domain.StatusPublished {
		t.Fatalf("质量发布失败: %#v %v", quality, err)
	}
	history, err := svc.QualityHistory(ctx, create.ID, QualityHistoryFilter{Passed: boolPointer(true), Limit: 10})
	if err != nil || history.Total != 1 || history.Records[0].RequestID != qualityInput.Meta.RequestID {
		t.Fatalf("质量历史查询失败: %#v %v", history, err)
	}
	events, err := svc.AuditTimeline(ctx, create.ID)
	if err != nil || len(events) != 9 || len(events[2].Payload) == 0 {
		t.Fatalf("扩展命令审计证据不完整: %d %v", len(events), err)
	}
}

func repeatHex(value byte) string {
	buffer := make([]byte, 64)
	for i := range buffer {
		buffer[i] = value
	}
	return string(buffer)
}

func boolPointer(value bool) *bool { return &value }
