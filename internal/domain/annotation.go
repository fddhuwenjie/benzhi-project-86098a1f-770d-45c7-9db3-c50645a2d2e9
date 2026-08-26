package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Conflict struct {
	ClipID      string                  `json:"clip_id"`
	Annotations []IndependentAnnotation `json:"annotations"`
	Priority    float64                 `json:"priority"`
	Stratum     string                  `json:"stratum"`
}

type AnnotationDeliveryResult struct {
	CreatedCount   int      `json:"created_count"`
	CorrectedCount int      `json:"corrected_count"`
	ClipIDs        []string `json:"clip_ids"`
	ClipSetSHA256  string   `json:"clip_set_sha256"`
}

func labelsEqual(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func (b *CorpusBatch) AnnotationsFor(clipID string) []IndependentAnnotation {
	out := make([]IndependentAnnotation, 0, 2)
	for _, a := range b.Annotations {
		if a.ClipID == clipID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AnnotatorID < out[j].AnnotatorID })
	return out
}

func (b *CorpusBatch) VisibleAnnotations(clipID, actorID string) ([]IndependentAnnotation, error) {
	clip, ok := b.ClipByID(clipID)
	if !ok || !clip.Sampled {
		return nil, ErrNotFound
	}
	all := b.AnnotationsFor(clipID)
	if len(all) >= 2 || b.Status == StatusPendingAdjudication || b.Status == StatusPendingQualityReview || b.Status == StatusPublished {
		return all, nil
	}
	visible := make([]IndependentAnnotation, 0, 1)
	for _, a := range all {
		if a.AnnotatorID == actorID {
			visible = append(visible, a)
		}
	}
	return visible, nil
}

func (b *CorpusBatch) SubmitAnnotation(a IndependentAnnotation, now time.Time) error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusAnnotating && b.Status != StatusPendingAdjudication && b.Status != StatusPendingQualityReview {
		return fmt.Errorf("%w: 当前阶段不可提交标注", ErrInvalidState)
	}
	clip, ok := b.ClipByID(a.ClipID)
	if !ok || !clip.Sampled {
		return Invalid("clip_id", "片段不存在或未入样")
	}
	a.BatchID = b.ID
	a.SubmittedAt = now.UTC()
	if err := ValidateAnnotation(a); err != nil {
		return err
	}
	position := -1
	count := 0
	for i, current := range b.Annotations {
		if current.ClipID != a.ClipID {
			continue
		}
		count++
		if current.AnnotatorID == a.AnnotatorID {
			position = i
		}
	}
	if position < 0 && count >= 2 {
		return Invalid("annotator_id", "该片段已有两名标注员")
	}
	if position >= 0 {
		a.ID = b.Annotations[position].ID
		b.Annotations[position] = a
		b.removeAdjudication(a.ClipID)
	} else {
		if a.ID == "" {
			a.ID = a.ClipID + "-" + a.AnnotatorID
		}
		b.Annotations = append(b.Annotations, a)
	}
	b.recalculateAnnotationState()
	b.touch(now)
	return nil
}

func (b *CorpusBatch) SubmitAnnotations(deliveries []IndependentAnnotation, actorID string, now time.Time) (*AnnotationDeliveryResult, error) {
	if err := b.Status.EnsureMutable(); err != nil {
		return nil, err
	}
	if b.Status != StatusAnnotating && b.Status != StatusPendingAdjudication && b.Status != StatusPendingQualityReview {
		return nil, fmt.Errorf("%w: 当前阶段不可批量交付标注", ErrInvalidState)
	}
	if len(deliveries) == 0 || len(deliveries) > 500 {
		return nil, Invalid("annotations", "每次须交付 1 至 500 个片段")
	}
	if err := ValidateID("actor_id", actorID); err != nil {
		return nil, err
	}
	working := append([]IndependentAnnotation(nil), b.Annotations...)
	decisions := append([]AdjudicationDecision(nil), b.Adjudications...)
	sorted := append([]IndependentAnnotation(nil), deliveries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ClipID < sorted[j].ClipID })
	seen := make(map[string]bool, len(sorted))
	result := &AnnotationDeliveryResult{ClipIDs: make([]string, 0, len(sorted))}
	for _, delivery := range sorted {
		if seen[delivery.ClipID] {
			return nil, Invalid("annotations["+delivery.ClipID+"].clip_id", "请求内片段标识重复")
		}
		seen[delivery.ClipID] = true
		clip, ok := b.ClipByID(delivery.ClipID)
		if !ok || !clip.Sampled {
			return nil, Invalid("annotations["+delivery.ClipID+"].clip_id", "片段不存在或未入样")
		}
		delivery.BatchID = b.ID
		delivery.AnnotatorID = actorID
		delivery.SubmittedAt = now.UTC()
		if err := ValidateAnnotation(delivery); err != nil {
			if validation, ok := err.(*ValidationError); ok {
				return nil, Invalid("annotations["+delivery.ClipID+"]."+validation.Field, validation.Message)
			}
			return nil, err
		}
		position, count := -1, 0
		for i := range working {
			if working[i].ClipID != delivery.ClipID {
				continue
			}
			count++
			if working[i].AnnotatorID == actorID {
				position = i
			}
		}
		if position < 0 && count >= 2 {
			return nil, Invalid("annotations["+delivery.ClipID+"].clip_id", "该片段没有可供此标注员占用的席位")
		}
		if position >= 0 {
			delivery.ID = working[position].ID
			working[position] = delivery
			result.CorrectedCount++
			kept := decisions[:0]
			for _, decision := range decisions {
				if decision.ClipID != delivery.ClipID {
					kept = append(kept, decision)
				}
			}
			decisions = kept
		} else {
			delivery.ID = delivery.ClipID + "-" + actorID
			working = append(working, delivery)
			result.CreatedCount++
		}
		result.ClipIDs = append(result.ClipIDs, delivery.ClipID)
	}
	sort.Slice(working, func(i, j int) bool {
		if working[i].ClipID == working[j].ClipID {
			return working[i].AnnotatorID < working[j].AnnotatorID
		}
		return working[i].ClipID < working[j].ClipID
	})
	b.Annotations = working
	b.Adjudications = decisions
	b.recalculateAnnotationState()
	b.touch(now)
	encoded, _ := json.Marshal(result.ClipIDs)
	digest := sha256.Sum256(encoded)
	result.ClipSetSHA256 = hex.EncodeToString(digest[:])
	return result, nil
}

func (b *CorpusBatch) removeAdjudication(clipID string) {
	kept := b.Adjudications[:0]
	for _, decision := range b.Adjudications {
		if decision.ClipID != clipID {
			kept = append(kept, decision)
		}
	}
	b.Adjudications = kept
}

func (b *CorpusBatch) Conflicts() []Conflict {
	conflicts := make([]Conflict, 0)
	for _, clip := range b.SampledClips() {
		annotations := b.AnnotationsFor(clip.ID)
		if len(annotations) == 2 && !labelsEqual(annotations[0].TaxonLabel, annotations[1].TaxonLabel) {
			diff := annotations[0].Confidence - annotations[1].Confidence
			if diff < 0 {
				diff = -diff
			}
			conflicts = append(conflicts, Conflict{ClipID: clip.ID, Annotations: annotations, Priority: diff, Stratum: StratumKey(clip)})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Priority != conflicts[j].Priority {
			return conflicts[i].Priority > conflicts[j].Priority
		}
		return conflicts[i].ClipID < conflicts[j].ClipID
	})
	return conflicts
}

func (b *CorpusBatch) hasDecision(clipID string) bool {
	for _, d := range b.Adjudications {
		if d.ClipID == clipID {
			return true
		}
	}
	return false
}

func (b *CorpusBatch) recalculateAnnotationState() {
	complete := true
	for _, clip := range b.SampledClips() {
		if len(b.AnnotationsFor(clip.ID)) != 2 {
			complete = false
			break
		}
	}
	if !complete {
		b.Status = StatusAnnotating
		return
	}
	for _, conflict := range b.Conflicts() {
		if !b.hasDecision(conflict.ClipID) {
			b.Status = StatusPendingAdjudication
			return
		}
	}
	b.Status = StatusPendingQualityReview
}
