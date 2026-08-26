package domain

import "fmt"

type BatchStatus string

const (
	StatusDraft                BatchStatus = "draft"
	StatusPendingSampling      BatchStatus = "pending_sampling"
	StatusAnnotating           BatchStatus = "annotating"
	StatusPendingAdjudication  BatchStatus = "pending_adjudication"
	StatusPendingQualityReview BatchStatus = "pending_quality_review"
	StatusPublished            BatchStatus = "published"
)

func (s BatchStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusPendingSampling, StatusAnnotating,
		StatusPendingAdjudication, StatusPendingQualityReview, StatusPublished:
		return true
	default:
		return false
	}
}

func (s BatchStatus) EnsureMutable() error {
	if s == StatusPublished {
		return ErrPublished
	}
	if !s.Valid() {
		return fmt.Errorf("未知批次状态 %q", s)
	}
	return nil
}

func CanTransition(from, to BatchStatus) bool {
	allowed := map[BatchStatus]map[BatchStatus]bool{
		StatusDraft:                {StatusPendingSampling: true},
		StatusPendingSampling:      {StatusAnnotating: true},
		StatusAnnotating:           {StatusPendingAdjudication: true, StatusPendingQualityReview: true},
		StatusPendingAdjudication:  {StatusAnnotating: true, StatusPendingQualityReview: true},
		StatusPendingQualityReview: {StatusAnnotating: true, StatusPendingAdjudication: true, StatusPublished: true},
	}
	return allowed[from][to]
}

func (b *CorpusBatch) transition(to BatchStatus) error {
	if b.Status == to {
		return nil
	}
	if !CanTransition(b.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidState, b.Status, to)
	}
	b.Status = to
	return nil
}
