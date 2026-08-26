package domain

import (
	"fmt"
	"sort"
	"time"
)

func (b *CorpusBatch) PendingConflicts() []Conflict {
	all := b.Conflicts()
	out := make([]Conflict, 0, len(all))
	for _, conflict := range all {
		if !b.hasDecision(conflict.ClipID) {
			out = append(out, conflict)
		}
	}
	return out
}

func (b *CorpusBatch) Adjudicate(d AdjudicationDecision, now time.Time) error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusPendingAdjudication {
		return fmt.Errorf("%w: 仅待仲裁状态可裁定", ErrInvalidState)
	}
	d.BatchID = b.ID
	d.DecidedAt = now.UTC()
	if err := ValidateAdjudication(d); err != nil {
		return err
	}
	found := false
	for _, conflict := range b.PendingConflicts() {
		if conflict.ClipID == d.ClipID {
			found = true
			for _, annotation := range conflict.Annotations {
				if annotation.AnnotatorID == d.AdjudicatorID {
					return Invalid("adjudicator_id", "仲裁员不得是该片段的标注员")
				}
			}
			break
		}
	}
	if !found {
		return Invalid("clip_id", "片段不在待仲裁队列")
	}
	if d.ID == "" {
		d.ID = d.ClipID + "-decision"
	}
	d.EvidenceRefs = append([]string(nil), d.EvidenceRefs...)
	b.Adjudications = append(b.Adjudications, d)
	sort.Slice(b.Adjudications, func(i, j int) bool { return b.Adjudications[i].ClipID < b.Adjudications[j].ClipID })
	b.recalculateAnnotationState()
	b.touch(now)
	return nil
}

func (b *CorpusBatch) FinalLabel(clipID string) (label, resolution string, ok bool) {
	annotations := b.AnnotationsFor(clipID)
	if len(annotations) != 2 {
		return "", "", false
	}
	if labelsEqual(annotations[0].TaxonLabel, annotations[1].TaxonLabel) {
		return annotations[0].TaxonLabel, "agreement", true
	}
	for _, decision := range b.Adjudications {
		if decision.ClipID == clipID {
			return decision.FinalLabel, "adjudication", true
		}
	}
	return "", "", false
}
