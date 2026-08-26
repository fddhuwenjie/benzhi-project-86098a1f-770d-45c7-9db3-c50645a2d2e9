package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"bioacoustic-corpus-release/internal/domain"
)

func (s *Service) GetBatch(ctx context.Context, batchID string) (*BatchDetail, error) {
	if err := domain.ValidateID("batch_id", batchID); err != nil {
		return nil, err
	}
	batch, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	clips := append([]domain.RecordingClip(nil), batch.Clips...)
	sort.Slice(clips, func(i, j int) bool { return clips[i].ID < clips[j].ID })
	return &BatchDetail{Batch: summarize(batch), Clips: clips, LastQualityResult: batch.LastQualityResult}, nil
}

func (s *Service) QualityHistory(ctx context.Context, batchID string, filter QualityHistoryFilter) (*QualityHistoryPage, error) {
	if err := domain.ValidateID("batch_id", batchID); err != nil {
		return nil, err
	}
	if filter.Offset < 0 || filter.Limit <= 0 || filter.Limit > 500 {
		return nil, domain.Invalid("pagination", "offset 须非负且 limit 须为 1 至 500")
	}
	if filter.MinRevision < 0 || filter.MaxRevision < 0 || (filter.MaxRevision > 0 && filter.MinRevision > filter.MaxRevision) {
		return nil, domain.Invalid("revision", "修订区间无效")
	}
	filter.IssueCode = strings.TrimSpace(filter.IssueCode)
	filter.ClipID = strings.TrimSpace(filter.ClipID)
	filter.Stratum = strings.TrimSpace(filter.Stratum)
	if filter.ClipID != "" {
		if err := domain.ValidateID("clip_id", filter.ClipID); err != nil {
			return nil, err
		}
	}
	if len(filter.IssueCode) > 100 || len(filter.Stratum) > 400 {
		return nil, domain.Invalid("filter", "问题代码或分层筛选值过长")
	}
	release := s.locks.acquire(batchID)
	defer release()
	batch, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	events, err := s.auditReader.Timeline(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := validateQualityHistory(batch, events); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrIntegrity, err)
	}
	filtered := make([]domain.QualityCheckRecord, 0, len(batch.QualityChecks))
	for _, record := range batch.QualityChecks {
		if filter.Passed != nil && record.Passed != *filter.Passed {
			continue
		}
		if filter.MinRevision > 0 && record.CheckedRevision < filter.MinRevision {
			continue
		}
		if filter.MaxRevision > 0 && record.CheckedRevision > filter.MaxRevision {
			continue
		}
		if !qualityRecordMatches(record, filter) {
			continue
		}
		filtered = append(filtered, record)
	}
	if filter.Offset > len(filtered) {
		return nil, domain.Invalid("offset", "超出质量检查历史范围")
	}
	end := filter.Offset + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := &QualityHistoryPage{Records: filtered[filter.Offset:end], Total: len(filtered), Offset: filter.Offset, Limit: filter.Limit}
	if end < len(filtered) {
		next := end
		page.NextOffset = &next
	}
	return page, nil
}

func (s *Service) QualityCheck(ctx context.Context, batchID string, sequence int) (*domain.QualityCheckRecord, error) {
	if sequence < 1 {
		return nil, domain.Invalid("sequence", "检查序号须为正整数")
	}
	page, err := s.QualityHistory(ctx, batchID, QualityHistoryFilter{Offset: sequence - 1, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(page.Records) == 1 && page.Records[0].Sequence == sequence {
		record := page.Records[0]
		return &record, nil
	}
	return nil, domain.ErrNotFound
}

func qualityRecordMatches(record domain.QualityCheckRecord, filter QualityHistoryFilter) bool {
	if filter.IssueCode == "" && filter.ClipID == "" && filter.Stratum == "" {
		return true
	}
	for _, issue := range record.Issues {
		if filter.IssueCode != "" && issue.Code != filter.IssueCode {
			continue
		}
		if filter.ClipID != "" && issue.ClipID != filter.ClipID {
			continue
		}
		if filter.Stratum != "" && issue.Stratum != filter.Stratum {
			continue
		}
		return true
	}
	return false
}

func validateQualityHistory(batch *domain.CorpusBatch, events []domain.AuditEvent) error {
	qualityEvents := make([]domain.AuditEvent, 0, len(batch.QualityChecks))
	for _, event := range events {
		if event.EventType == "quality.checked" {
			qualityEvents = append(qualityEvents, event)
		}
	}
	if len(qualityEvents) != len(batch.QualityChecks) {
		return fmt.Errorf("检查记录数与 quality.checked 审计事件数不一致")
	}
	for i, record := range batch.QualityChecks {
		if record.Sequence != i+1 {
			return fmt.Errorf("检查记录序号不连续")
		}
		for j := range record.Issues {
			if record.Issues[j].Key != domain.QualityIssueKey(record.Issues[j]) {
				return fmt.Errorf("检查记录 %d 的问题业务键无效", record.Sequence)
			}
			if j > 0 && record.Issues[j-1].Key >= record.Issues[j].Key {
				return fmt.Errorf("检查记录 %d 的问题顺序无效", record.Sequence)
			}
		}
		var previous *domain.QualityCheckRecord
		if i > 0 {
			previous = &batch.QualityChecks[i-1]
		}
		expectedComparison := qualityComparisonForValidation(record, previous)
		if !reflect.DeepEqual(record.Comparison, expectedComparison) {
			return fmt.Errorf("检查记录 %d 的问题对比无效", record.Sequence)
		}
		event := qualityEvents[i]
		if event.RequestID != record.RequestID || event.ActorID != record.ActorID || event.Revision != record.CheckedRevision+1 {
			return fmt.Errorf("检查记录 %d 与审计事件关联不一致", record.Sequence)
		}
		var payload CommandResponse
		if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Quality == nil {
			return fmt.Errorf("检查记录 %d 的审计载荷无效", record.Sequence)
		}
		if payload.Batch.Revision != event.Revision || !qualityResultMatchesRecord(payload.Quality, record) {
			return fmt.Errorf("检查记录 %d 与审计载荷内容不一致", record.Sequence)
		}
	}
	if batch.Status == domain.StatusPublished && len(batch.QualityChecks) > 0 {
		last := batch.QualityChecks[len(batch.QualityChecks)-1]
		if !last.Passed || batch.Manifest == nil || !reflect.DeepEqual(last.Metrics, batch.Manifest.QualityMetrics) {
			return fmt.Errorf("最终质量检查与发布清单不一致")
		}
	}
	return nil
}

func qualityResultMatchesRecord(result *domain.QualityResult, record domain.QualityCheckRecord) bool {
	if result == nil || result.Passed != record.Passed || result.Metrics != record.Metrics ||
		result.CheckedRevision != record.CheckedRevision || result.Thresholds != record.Thresholds ||
		!result.CheckedAt.Equal(record.CheckedAt) || len(result.Issues) != len(record.Issues) {
		return false
	}
	for i := range result.Issues {
		if result.Issues[i] != record.Issues[i] {
			return false
		}
	}
	return true
}

func qualityComparisonForValidation(record domain.QualityCheckRecord, previous *domain.QualityCheckRecord) domain.QualityComparison {
	comparison := domain.QualityComparison{Added: []domain.QualityIssue{}, Ongoing: []domain.QualityIssue{}, Resolved: []domain.QualityIssue{}}
	old := make(map[string]domain.QualityIssue)
	if previous != nil {
		for _, issue := range previous.Issues {
			old[issue.Key] = issue
		}
	}
	for _, issue := range record.Issues {
		if _, ok := old[issue.Key]; ok {
			comparison.Ongoing = append(comparison.Ongoing, issue)
			delete(old, issue.Key)
		} else {
			comparison.Added = append(comparison.Added, issue)
		}
	}
	for _, issue := range old {
		comparison.Resolved = append(comparison.Resolved, issue)
	}
	sort.Slice(comparison.Resolved, func(i, j int) bool { return comparison.Resolved[i].Key < comparison.Resolved[j].Key })
	if previous != nil {
		comparison.MetricChanges = domain.QualityMetricChanges{
			SampleCount:     record.Metrics.SampleCount - previous.Metrics.SampleCount,
			CompleteCount:   record.Metrics.CompleteCount - previous.Metrics.CompleteCount,
			AgreementCount:  record.Metrics.AgreementCount - previous.Metrics.AgreementCount,
			TotalStrata:     record.Metrics.TotalStrata - previous.Metrics.TotalStrata,
			CoveredStrata:   record.Metrics.CoveredStrata - previous.Metrics.CoveredStrata,
			UnresolvedCount: record.Metrics.UnresolvedCount - previous.Metrics.UnresolvedCount,
			Completeness:    record.Metrics.Completeness - previous.Metrics.Completeness,
			Agreement:       record.Metrics.Agreement - previous.Metrics.Agreement,
			StratumCoverage: record.Metrics.StratumCoverage - previous.Metrics.StratumCoverage,
		}
	}
	return comparison
}

func (s *Service) GetAnnotations(ctx context.Context, batchID, clipID, actorID string) ([]domain.IndependentAnnotation, error) {
	if err := domain.ValidateID("actor_id", actorID); err != nil {
		return nil, err
	}
	batch, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return batch.VisibleAnnotations(clipID, actorID)
}

func (s *Service) AnnotationProgress(ctx context.Context, batchID, actorID string) (*AnnotationProgress, error) {
	if err := domain.ValidateID("actor_id", actorID); err != nil {
		return nil, err
	}
	s.progressMu.RLock()
	cached := cloneAnnotationProgress(s.progress[batchID])
	s.progressMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	b, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	p := &AnnotationProgress{ActorID: actorID, SampleCount: len(b.SampledClips()), PendingClipIDs: []string{}}
	total := 0.0
	for _, c := range b.SampledClips() {
		anns := b.AnnotationsFor(c.ID)
		found := false
		for _, a := range anns {
			if a.AnnotatorID == actorID {
				found = true
				p.SubmittedCount++
				total += a.Confidence
			}
		}
		if !found {
			p.PendingClipIDs = append(p.PendingClipIDs, c.ID)
		}
	}
	if p.SubmittedCount > 0 {
		p.AverageConfidence = total / float64(p.SubmittedCount)
	}
	if p.SampleCount > 0 {
		p.DoubleLabelRate = float64(len(b.Annotations)) / float64(p.SampleCount*2)
		p.OverallDoubleLabelRate = p.DoubleLabelRate
	}
	s.progressMu.Lock()
	s.progress[batchID] = cloneAnnotationProgress(p)
	s.progressMu.Unlock()
	return cloneAnnotationProgress(p), nil
}

func cloneAnnotationProgress(progress *AnnotationProgress) *AnnotationProgress {
	if progress == nil {
		return nil
	}
	cloned := *progress
	cloned.PendingClipIDs = append([]string(nil), progress.PendingClipIDs...)
	return &cloned
}

func (s *Service) PendingConflicts(ctx context.Context, batchID string) ([]domain.Conflict, error) {
	batch, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return batch.PendingConflicts(), nil
}

func (s *Service) PreviewSample(ctx context.Context, batchID string, quota map[string]int) (*SamplingPreview, error) {
	b, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	strata, err := b.PreviewSample(quota)
	if err != nil {
		return nil, err
	}
	return &SamplingPreview{SamplingSeed: b.SamplingSeed, Strata: strata}, nil
}

func (s *Service) GetManifest(ctx context.Context, batchID string) (*domain.ReleaseManifest, error) {
	batch, err := s.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if batch.Manifest == nil || batch.Status != domain.StatusPublished {
		return nil, domain.ErrNotFound
	}
	manifest := *batch.Manifest
	manifest.ClipEntries = append([]domain.ManifestClip(nil), batch.Manifest.ClipEntries...)
	digest, err := domain.ManifestDigest(&manifest)
	if err != nil {
		return nil, err
	}
	if digest != manifest.SHA256Digest {
		return nil, fmt.Errorf("%w: 清单摘要校验失败", domain.ErrIntegrity)
	}
	return &manifest, nil
}

func (s *Service) GetManifestPage(ctx context.Context, batchID string, offset, limit int) (*ManifestPage, error) {
	if offset < 0 || limit <= 0 || limit > 1000 {
		return nil, domain.Invalid("pagination", "offset 须非负且 limit 须为 1 至 1000")
	}
	m, err := s.GetManifest(ctx, batchID)
	if err != nil {
		return nil, err
	}
	total := len(m.ClipEntries)
	if offset > total {
		return nil, domain.Invalid("offset", "超出清单范围")
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return &ManifestPage{BatchID: m.BatchID, ReleaseVersion: m.ReleaseVersion, GeneratedAt: m.GeneratedAt, SHA256Digest: m.SHA256Digest, Total: total, Offset: offset, Limit: limit, ClipEntries: m.ClipEntries[offset:end]}, nil
}

func (s *Service) AuditTimeline(ctx context.Context, batchID string) ([]domain.AuditEvent, error) {
	return s.auditReader.Timeline(ctx, batchID)
}

func (s *Service) AuditPage(ctx context.Context, batchID, actor, eventType string, minRev, maxRev, offset, limit int) (*AuditPage, error) {
	if offset < 0 || limit <= 0 || limit > 1000 {
		return nil, domain.Invalid("pagination", "offset 须非负且 limit 须为 1 至 1000")
	}
	if minRev < 0 || maxRev < 0 || (maxRev > 0 && minRev > maxRev) {
		return nil, domain.Invalid("revision", "修订区间无效")
	}
	all, err := s.AuditTimeline(ctx, batchID)
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.AuditEvent, 0)
	for _, e := range all {
		if actor != "" && e.ActorID != actor {
			continue
		}
		if eventType != "" && e.EventType != eventType {
			continue
		}
		if minRev > 0 && e.Revision < int64(minRev) {
			continue
		}
		if maxRev > 0 && e.Revision > int64(maxRev) {
			continue
		}
		filtered = append(filtered, e)
	}
	if offset > len(filtered) {
		return nil, domain.Invalid("offset", "超出审计范围")
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := &AuditPage{Events: filtered[offset:end], Total: len(filtered), Offset: offset, Limit: limit, ContinuityProof: ""}
	if len(all) > 0 {
		page.FirstRevision = all[0].Revision
		page.LastRevision = all[len(all)-1].Revision
	}
	raw := fmt.Sprintf("%d:%d:%d", page.FirstRevision, page.LastRevision, len(all))
	sum := sha256.Sum256([]byte(raw))
	page.ContinuityProof = hex.EncodeToString(sum[:])
	if end < len(filtered) {
		n := end
		page.NextOffset = &n
		page.NextCursor = fmt.Sprintf("%d", n)
	}
	return page, nil
}
