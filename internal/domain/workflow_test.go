package domain

import (
	"reflect"
	"testing"
	"time"
)

func testBatch(t *testing.T) *CorpusBatch {
	t.Helper()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	batch, err := NewBatch("batch-test", "测试批次", "领域流程", "seed-001", QualityThresholds{1, 1, 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	clips := []RecordingClip{
		{ID: "clip-one", SourceURI: "s3://bucket/one.wav", ContentDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RegionCode: "CN-YN", RecordedAt: now.Add(-time.Hour), DurationMS: 1000, CandidateTaxon: "Strix aluco"},
		{ID: "clip-two", SourceURI: "s3://bucket/two.wav", ContentDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RegionCode: "CN-YN", RecordedAt: now.Add(-2 * time.Hour), DurationMS: 1000, CandidateTaxon: "Strix aluco"},
	}
	if err := batch.AddClips(clips, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SubmitDraft(now); err != nil {
		t.Fatal(err)
	}
	key := StratumKey(clips[0])
	if err := batch.LockSample(map[string]int{key: 2}, now); err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestDraftClipCorrectionAndWithdrawalAreAtomic(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	batch, err := NewBatch("batch-clips", "片段纠错", "", "seed-001", QualityThresholds{1, 1, 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	clips := []RecordingClip{
		{ID: "clip-one", SourceURI: "s3://bucket/one.wav", ContentDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RegionCode: "CN-YN", RecordedAt: now.Add(-time.Hour), DurationMS: 1000, CandidateTaxon: "Strix aluco"},
		{ID: "clip-two", SourceURI: "s3://bucket/two.wav", ContentDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RegionCode: "CN-YN", RecordedAt: now.Add(-time.Hour), DurationMS: 1100, CandidateTaxon: "Strix aluco"},
		{ID: "clip-three", SourceURI: "s3://bucket/three.wav", ContentDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", RegionCode: "CN-YN", RecordedAt: now.Add(-time.Hour), DurationMS: 1200, CandidateTaxon: "Strix aluco"},
	}
	if err := batch.AddClips(clips, now); err != nil {
		t.Fatal(err)
	}
	original := append([]RecordingClip(nil), batch.Clips...)
	originalRevision := batch.Revision
	duration := int64(2500)
	duplicate := clips[2].ContentDigest
	_, err = batch.CorrectClips([]ClipPatch{{ID: "clip-one", DurationMS: &duration}, {ID: "clip-two", ContentDigest: &duplicate}}, now)
	if err == nil {
		t.Fatal("摘要冲突的批量纠错应失败")
	}
	if batch.Revision != originalRevision || !reflect.DeepEqual(batch.Clips, original) {
		t.Fatal("失败的批量纠错不应产生部分修改")
	}
	region := "CN-GX"
	result, err := batch.CorrectClips([]ClipPatch{{ID: "clip-two", RegionCode: &region}, {ID: "clip-one", DurationMS: &duration}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Revision != originalRevision+1 || len(result.Changes) != 2 || result.AffectedClipIDs[0] != "clip-one" {
		t.Fatalf("纠错应只增加一次修订并稳定返回片段: %#v", result)
	}
	result, err = batch.WithdrawClips([]string{"clip-three"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Revision != originalRevision+2 || len(batch.Clips) != 2 || len(result.Changes) != 1 {
		t.Fatal("撤销应原子移除片段并只增加一次修订")
	}
}

func TestBulkAnnotationsAndQualityHistory(t *testing.T) {
	batch := testBatch(t)
	now := time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)
	deliveriesA := []IndependentAnnotation{
		{ClipID: "clip-two", TaxonLabel: "Strix aluco", Confidence: .8, EvidenceNote: "标注员甲证据"},
		{ClipID: "clip-one", TaxonLabel: "Strix aluco", Confidence: .9, EvidenceNote: "标注员甲证据"},
	}
	startRevision := batch.Revision
	result, err := batch.SubmitAnnotations(deliveriesA, "annotator-a", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 2 || batch.Revision != startRevision+1 {
		t.Fatal("批量交付应新增两项且只增加一次修订")
	}
	visible, err := batch.VisibleAnnotations("clip-one", "annotator-b")
	if err != nil || len(visible) != 0 {
		t.Fatal("第二名标注员不应看到首份批量交付")
	}
	before := append([]IndependentAnnotation(nil), batch.Annotations...)
	_, err = batch.SubmitAnnotations([]IndependentAnnotation{
		{ClipID: "clip-one", TaxonLabel: "Strix aluco", Confidence: .7, EvidenceNote: "标注员乙证据"},
		{ClipID: "not-sampled", TaxonLabel: "Strix aluco", Confidence: .7, EvidenceNote: "标注员乙证据"},
	}, "annotator-b", now)
	if err == nil || batch.Revision != startRevision+1 || !reflect.DeepEqual(batch.Annotations, before) {
		t.Fatal("含未入样片段的批量交付应整批拒绝")
	}
	_, err = batch.SubmitAnnotations([]IndependentAnnotation{
		{ClipID: "clip-one", TaxonLabel: "Asio otus", Confidence: .7, EvidenceNote: "标注员乙证据"},
		{ClipID: "clip-two", TaxonLabel: "Strix aluco", Confidence: .7, EvidenceNote: "标注员乙证据"},
	}, "annotator-b", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.Adjudicate(AdjudicationDecision{ClipID: "clip-one", AdjudicatorID: "taxonomist", FinalLabel: "Strix aluco", Reason: "复核声谱图", EvidenceRefs: []string{"evidence://one"}}, now); err != nil {
		t.Fatal(err)
	}
	failed, err := batch.RunQualityGate("quality-owner", "quality-first", now)
	if err != nil || failed.Passed || len(batch.QualityChecks) != 1 {
		t.Fatalf("首次质量门禁应保留失败记录: %#v %v", failed, err)
	}
	_, err = batch.SubmitAnnotations([]IndependentAnnotation{{ClipID: "clip-one", TaxonLabel: "Strix aluco", Confidence: .95, EvidenceNote: "修正后的声谱图证据"}}, "annotator-b", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	passed, err := batch.RunQualityGate("quality-owner", "quality-second", now.Add(2*time.Minute))
	if err != nil || !passed.Passed || len(batch.QualityChecks) != 2 {
		t.Fatalf("修正后的重检应通过并追加记录: %#v %v", passed, err)
	}
	if len(batch.QualityChecks[1].Comparison.Resolved) == 0 || batch.QualityChecks[0].Passed {
		t.Fatal("重检对比应保留首次失败并列出已解决问题")
	}
}

func TestAnnotationIsolationAndEquivalentLabels(t *testing.T) {
	batch := testBatch(t)
	now := time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)
	first := IndependentAnnotation{ClipID: "clip-one", AnnotatorID: "annotator-a", TaxonLabel: "Strix aluco", Confidence: .9, EvidenceNote: "第一份独立证据"}
	if err := batch.SubmitAnnotation(first, now); err != nil {
		t.Fatal(err)
	}
	visible, err := batch.VisibleAnnotations("clip-one", "annotator-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatalf("第二名标注员不应看到第一份结果: %#v", visible)
	}
	second := IndependentAnnotation{ClipID: "clip-one", AnnotatorID: "annotator-b", TaxonLabel: "strix ALUCO", Confidence: .8, EvidenceNote: "第二份独立证据"}
	if err := batch.SubmitAnnotation(second, now); err != nil {
		t.Fatal(err)
	}
	if len(batch.Conflicts()) != 0 {
		t.Fatal("大小写差异不应形成类群冲突")
	}
	label, resolution, ok := batch.FinalLabel("clip-one")
	if !ok || label != "Strix aluco" || resolution != "agreement" {
		t.Fatalf("等价标签未形成一致结论: %q %q %v", label, resolution, ok)
	}
}

func TestSamplingIsReproducible(t *testing.T) {
	first := testBatch(t)
	second := testBatch(t)
	if len(first.Clips) != len(second.Clips) {
		t.Fatal("测试批次片段数不一致")
	}
	for i := range first.Clips {
		if first.Clips[i].Sampled != second.Clips[i].Sampled || first.Clips[i].SampleReason != second.Clips[i].SampleReason {
			t.Fatalf("相同种子抽样结果不稳定: %#v %#v", first.Clips[i], second.Clips[i])
		}
	}
}
