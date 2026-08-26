package domain

import (
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func NormalizeDigest(value string) string {
	return "sha256:" + strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "sha256:"))
}

var (
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$`)
	regionPattern = regexp.MustCompile(`^[A-Z]{2,8}(-[A-Z0-9]{1,12})*$`)
	taxonPattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 ._-]{1,119}$`)
)

func ValidateID(field, value string) error {
	if !idPattern.MatchString(value) {
		return Invalid(field, "须为 3 至 64 位字母、数字、点、下划线或连字符")
	}
	return nil
}

func ValidateThresholds(q QualityThresholds) error {
	if q.MinCompleteness < 0 || q.MinCompleteness > 1 {
		return Invalid("quality_thresholds.min_completeness", "须在 0 到 1 之间")
	}
	if q.MinAgreement < 0 || q.MinAgreement > 1 {
		return Invalid("quality_thresholds.min_agreement", "须在 0 到 1 之间")
	}
	if q.MinStratumCoverage < 0 || q.MinStratumCoverage > 1 {
		return Invalid("quality_thresholds.min_stratum_coverage", "须在 0 到 1 之间")
	}
	return nil
}

func ValidateNewBatch(batch *CorpusBatch) error {
	if err := ValidateID("id", batch.ID); err != nil {
		return err
	}
	if strings.TrimSpace(batch.Title) == "" || len(batch.Title) > 160 {
		return Invalid("title", "不能为空且不能超过 160 字节")
	}
	if len(batch.Description) > 2000 {
		return Invalid("description", "不能超过 2000 字节")
	}
	if strings.TrimSpace(batch.SamplingSeed) == "" || len(batch.SamplingSeed) > 128 {
		return Invalid("sampling_seed", "不能为空且不能超过 128 字节")
	}
	return ValidateThresholds(batch.QualityThresholds)
}

func ValidateClip(clip RecordingClip, now time.Time) error {
	if err := ValidateID("clip.id", clip.ID); err != nil {
		return err
	}
	parsed, err := url.ParseRequestURI(clip.SourceURI)
	if err != nil || parsed.Scheme == "" {
		return Invalid("clip.source_uri", "须为包含 scheme 的合法 URI")
	}
	digest := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(clip.ContentDigest)), "sha256:")
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != 32 {
		return Invalid("clip.content_digest", "须为 SHA-256 十六进制摘要")
	}
	if !regionPattern.MatchString(clip.RegionCode) {
		return Invalid("clip.region_code", "须为大写区域编码，例如 CN-YN")
	}
	if clip.RecordedAt.IsZero() || clip.RecordedAt.After(now.Add(5*time.Minute)) {
		return Invalid("clip.recorded_at", "须为不晚于当前时间的有效时间")
	}
	if clip.DurationMS < 100 || clip.DurationMS > int64((24*time.Hour)/time.Millisecond) {
		return Invalid("clip.duration_ms", "须在 100 毫秒到 24 小时之间")
	}
	if !taxonPattern.MatchString(clip.CandidateTaxon) {
		return Invalid("clip.candidate_taxon", "须为 2 至 120 位候选类群名称")
	}
	return nil
}

func ValidateAnnotation(a IndependentAnnotation) error {
	if err := ValidateID("annotator_id", a.AnnotatorID); err != nil {
		return err
	}
	if !taxonPattern.MatchString(a.TaxonLabel) {
		return Invalid("taxon_label", "须为 2 至 120 位类群名称")
	}
	if a.Confidence < 0 || a.Confidence > 1 {
		return Invalid("confidence", "须在 0 到 1 之间")
	}
	if strings.TrimSpace(a.EvidenceNote) == "" || len(a.EvidenceNote) > 2000 {
		return Invalid("evidence_note", "不能为空且不能超过 2000 字节")
	}
	return nil
}

func ValidateAdjudication(d AdjudicationDecision) error {
	if err := ValidateID("adjudicator_id", d.AdjudicatorID); err != nil {
		return err
	}
	if !taxonPattern.MatchString(d.FinalLabel) {
		return Invalid("final_label", "须为合法类群名称")
	}
	if strings.TrimSpace(d.Reason) == "" || len(d.Reason) > 2000 {
		return Invalid("reason", "不能为空且不能超过 2000 字节")
	}
	if len(d.EvidenceRefs) == 0 || len(d.EvidenceRefs) > 20 {
		return Invalid("evidence_refs", "须包含 1 至 20 条证据引用")
	}
	for _, ref := range d.EvidenceRefs {
		if strings.TrimSpace(ref) == "" || len(ref) > 300 {
			return Invalid("evidence_refs", "证据引用不能为空且不能超过 300 字节")
		}
	}
	return nil
}
