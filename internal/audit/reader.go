package audit

import (
	"context"
	"fmt"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/store"
)

type Reader struct {
	repository store.Repository
}

func NewReader(repository store.Repository) *Reader {
	return &Reader{repository: repository}
}

func (r *Reader) Timeline(ctx context.Context, batchID string) ([]domain.AuditEvent, error) {
	batch, err := r.repository.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	events, err := r.repository.ListAudit(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := ValidateTimeline(batch, events); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrIntegrity, err)
	}
	return events, nil
}
