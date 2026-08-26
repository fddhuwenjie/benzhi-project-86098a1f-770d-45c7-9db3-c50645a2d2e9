package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func NewBatch(id, title, description, seed string, thresholds QualityThresholds, now time.Time) (*CorpusBatch, error) {
	b := &CorpusBatch{
		ID: id, Title: strings.TrimSpace(title), Description: strings.TrimSpace(description),
		Status: StatusDraft, Revision: 1, SamplingSeed: strings.TrimSpace(seed),
		QualityThresholds: thresholds, Clips: []RecordingClip{},
		Annotations: []IndependentAnnotation{}, Adjudications: []AdjudicationDecision{},
		QualityChecks: []QualityCheckRecord{},
		CreatedAt:     now.UTC(), UpdatedAt: now.UTC(),
	}
	if err := ValidateNewBatch(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *CorpusBatch) AddClips(clips []RecordingClip, now time.Time) error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusDraft {
		return fmt.Errorf("%w: 仅草稿状态可登记片段", ErrInvalidState)
	}
	if len(clips) == 0 || len(clips) > 1000 {
		return Invalid("clips", "每次须登记 1 至 1000 个片段")
	}
	existing := make(map[string]bool, len(b.Clips)+len(clips))
	digests := make(map[string]bool, len(b.Clips)+len(clips))
	sources := make(map[string]bool, len(b.Clips)+len(clips))
	for _, clip := range b.Clips {
		existing[clip.ID] = true
		digests[NormalizeDigest(clip.ContentDigest)] = true
		sources[strings.TrimSpace(clip.SourceURI)] = true
	}
	for i := range clips {
		clips[i].BatchID = b.ID
		clips[i].ContentDigest = NormalizeDigest(clips[i].ContentDigest)
		clips[i].Sampled = false
		clips[i].SampleReason = ""
		if err := ValidateClip(clips[i], now); err != nil {
			if v, ok := err.(*ValidationError); ok {
				return Invalid(fmt.Sprintf("clips[%d].%s", i, strings.TrimPrefix(v.Field, "clip.")), v.Message)
			}
			return fmt.Errorf("clips[%d]: %w", i, err)
		}
		if existing[clips[i].ID] {
			return Invalid(fmt.Sprintf("clips[%d].id", i), "批次内片段标识重复")
		}
		if digests[clips[i].ContentDigest] {
			return Invalid(fmt.Sprintf("clips[%d].content_digest", i), "批次内内容摘要重复")
		}
		if sources[strings.TrimSpace(clips[i].SourceURI)] {
			return Invalid(fmt.Sprintf("clips[%d].source_uri", i), "批次内来源 URI 重复")
		}
		existing[clips[i].ID] = true
		digests[clips[i].ContentDigest] = true
		sources[strings.TrimSpace(clips[i].SourceURI)] = true
	}
	b.Clips = append(b.Clips, clips...)
	b.touch(now)
	return nil
}

func (b *CorpusBatch) SubmitDraft(now time.Time) error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusDraft {
		return fmt.Errorf("%w: 仅草稿可提交", ErrInvalidState)
	}
	if len(b.Clips) < 2 {
		return Invalid("clips", "提交前至少需要两个片段")
	}
	if err := b.transition(StatusPendingSampling); err != nil {
		return err
	}
	b.touch(now)
	return nil
}

func (b *CorpusBatch) touch(now time.Time) {
	b.Revision++
	b.UpdatedAt = now.UTC()
	b.LastQualityResult = nil
}

func StratumKey(clip RecordingClip) string {
	period := clip.RecordedAt.UTC().Format("2006-01")
	return clip.RegionCode + "|" + period + "|" + strings.ToLower(strings.TrimSpace(clip.CandidateTaxon))
}

func (b *CorpusBatch) AllStrata() []string {
	seen := make(map[string]bool)
	for _, clip := range b.Clips {
		seen[StratumKey(clip)] = true
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
