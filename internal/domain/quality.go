package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func (b *CorpusBatch) CalculateQuality() (QualityMetrics, []QualityIssue) {
	sampled := b.SampledClips()
	metrics := QualityMetrics{SampleCount: len(sampled), TotalStrata: len(b.AllStrata())}
	covered := make(map[string]bool)
	issues := make([]QualityIssue, 0)
	for _, clip := range sampled {
		covered[StratumKey(clip)] = true
		annotations := b.AnnotationsFor(clip.ID)
		if len(annotations) != 2 {
			issues = append(issues, QualityIssue{Code: "incomplete_annotation", Message: "入样片段尚未完成双人标注", ClipID: clip.ID})
			continue
		}
		metrics.CompleteCount++
		if labelsEqual(annotations[0].TaxonLabel, annotations[1].TaxonLabel) {
			metrics.AgreementCount++
		} else if !b.hasDecision(clip.ID) {
			metrics.UnresolvedCount++
			issues = append(issues, QualityIssue{Code: "unresolved_conflict", Message: "双标分歧尚未完成仲裁", ClipID: clip.ID})
		}
	}
	for _, stratum := range b.AllStrata() {
		count, complete := 0, 0
		for _, clip := range sampled {
			if StratumKey(clip) != stratum {
				continue
			}
			count++
			if len(b.AnnotationsFor(clip.ID)) == 2 {
				complete++
			}
		}
		if count == 0 {
			issues = append(issues, QualityIssue{Code: "stratum_gap", Message: "分层没有入样片段", Stratum: stratum})
		}
		if complete < count {
			for _, clip := range sampled {
				if StratumKey(clip) == stratum && len(b.AnnotationsFor(clip.ID)) != 2 {
					issues = append(issues, QualityIssue{Code: "stratum_incomplete", Message: "分层存在未完成双标", ClipID: clip.ID, Stratum: stratum})
				}
			}
		}
	}
	metrics.CoveredStrata = len(covered)
	metrics.Completeness = ratio(metrics.CompleteCount, metrics.SampleCount)
	metrics.Agreement = ratio(metrics.AgreementCount, metrics.CompleteCount)
	metrics.StratumCoverage = ratio(metrics.CoveredStrata, metrics.TotalStrata)
	if metrics.SampleCount == 0 {
		issues = append(issues, QualityIssue{Code: "empty_sample", Message: "质控样本为空"})
	}
	if metrics.Completeness < b.QualityThresholds.MinCompleteness {
		issues = append(issues, QualityIssue{Code: "completeness_below_threshold", Message: fmt.Sprintf("完整率 %.4f 低于阈值 %.4f", metrics.Completeness, b.QualityThresholds.MinCompleteness)})
	}
	if metrics.Agreement < b.QualityThresholds.MinAgreement {
		issues = append(issues, QualityIssue{Code: "agreement_below_threshold", Message: fmt.Sprintf("双标一致率 %.4f 低于阈值 %.4f，可复核并修正标注", metrics.Agreement, b.QualityThresholds.MinAgreement)})
	}
	if metrics.StratumCoverage < b.QualityThresholds.MinStratumCoverage {
		issues = append(issues, QualityIssue{Code: "coverage_below_threshold", Message: fmt.Sprintf("分层覆盖率 %.4f 低于阈值 %.4f", metrics.StratumCoverage, b.QualityThresholds.MinStratumCoverage)})
	}
	if metrics.UnresolvedCount > 0 {
		issues = append(issues, QualityIssue{Code: "conflicts_remaining", Message: fmt.Sprintf("仍有 %d 个未决冲突", metrics.UnresolvedCount)})
	}
	for i := range issues {
		issues[i].Key = QualityIssueKey(issues[i])
	}
	sortQualityIssues(issues)
	return metrics, issues
}

func QualityIssueKey(issue QualityIssue) string {
	return strings.Join([]string{issue.Code, issue.ClipID, issue.Stratum}, "|")
}

func sortQualityIssues(issues []QualityIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].Key < issues[j].Key
	})
}

func compareQuality(current QualityMetrics, issues []QualityIssue, previous *QualityCheckRecord) QualityComparison {
	comparison := QualityComparison{Added: []QualityIssue{}, Ongoing: []QualityIssue{}, Resolved: []QualityIssue{}}
	if previous == nil {
		comparison.Added = append(comparison.Added, issues...)
	} else {
		old := make(map[string]QualityIssue, len(previous.Issues))
		for _, issue := range previous.Issues {
			old[issue.Key] = issue
		}
		for _, issue := range issues {
			if _, exists := old[issue.Key]; exists {
				comparison.Ongoing = append(comparison.Ongoing, issue)
				delete(old, issue.Key)
			} else {
				comparison.Added = append(comparison.Added, issue)
			}
		}
		for _, issue := range old {
			comparison.Resolved = append(comparison.Resolved, issue)
		}
		comparison.MetricChanges = qualityMetricChanges(current, previous.Metrics)
	}
	sortQualityIssues(comparison.Added)
	sortQualityIssues(comparison.Ongoing)
	sortQualityIssues(comparison.Resolved)
	return comparison
}

func qualityMetricChanges(current, previous QualityMetrics) QualityMetricChanges {
	return QualityMetricChanges{
		SampleCount:     current.SampleCount - previous.SampleCount,
		CompleteCount:   current.CompleteCount - previous.CompleteCount,
		AgreementCount:  current.AgreementCount - previous.AgreementCount,
		TotalStrata:     current.TotalStrata - previous.TotalStrata,
		CoveredStrata:   current.CoveredStrata - previous.CoveredStrata,
		UnresolvedCount: current.UnresolvedCount - previous.UnresolvedCount,
		Completeness:    current.Completeness - previous.Completeness,
		Agreement:       current.Agreement - previous.Agreement,
		StratumCoverage: current.StratumCoverage - previous.StratumCoverage,
	}
}

func (b *CorpusBatch) RunQualityGate(actorID, requestID string, now time.Time) (*QualityResult, error) {
	if err := b.Status.EnsureMutable(); err != nil {
		return nil, err
	}
	if b.Status != StatusPendingQualityReview {
		return nil, fmt.Errorf("%w: 仅待质量核验状态可运行门禁", ErrInvalidState)
	}
	metrics, issues := b.CalculateQuality()
	result := &QualityResult{Passed: len(issues) == 0, Metrics: metrics, Issues: issues, CheckedAt: now.UTC(), CheckedRevision: b.Revision, Thresholds: b.QualityThresholds}
	var previous *QualityCheckRecord
	if len(b.QualityChecks) > 0 {
		previous = &b.QualityChecks[len(b.QualityChecks)-1]
	}
	record := QualityCheckRecord{
		Sequence: len(b.QualityChecks) + 1, ActorID: actorID, RequestID: requestID,
		CheckedRevision: b.Revision, Thresholds: b.QualityThresholds, Metrics: metrics,
		Issues: append([]QualityIssue{}, issues...), Passed: result.Passed, CheckedAt: now.UTC(),
		Comparison: compareQuality(metrics, issues, previous),
	}
	b.LastQualityResult = result
	if result.Passed {
		manifest, err := b.buildManifest(metrics, now)
		if err != nil {
			return nil, err
		}
		b.Manifest = manifest
		if err := b.transition(StatusPublished); err != nil {
			return nil, err
		}
		published := now.UTC()
		b.PublishedAt = &published
	}
	b.QualityChecks = append(b.QualityChecks, record)
	b.Revision++
	b.UpdatedAt = now.UTC()
	return result, nil
}
