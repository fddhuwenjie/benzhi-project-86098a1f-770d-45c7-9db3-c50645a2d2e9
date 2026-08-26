package store

import (
	"encoding/json"
	"fmt"

	"bioacoustic-corpus-release/internal/domain"
)

func encodeBatch(batch *domain.CorpusBatch) ([]byte, error) {
	encoded, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("编码批次快照: %w", err)
	}
	return encoded, nil
}

func decodeBatch(encoded []byte) (*domain.CorpusBatch, error) {
	var batch domain.CorpusBatch
	if err := json.Unmarshal(encoded, &batch); err != nil {
		return nil, fmt.Errorf("解码批次快照: %w", err)
	}
	if batch.Clips == nil {
		batch.Clips = []domain.RecordingClip{}
	}
	if batch.Annotations == nil {
		batch.Annotations = []domain.IndependentAnnotation{}
	}
	if batch.Adjudications == nil {
		batch.Adjudications = []domain.AdjudicationDecision{}
	}
	if batch.QualityChecks == nil {
		batch.QualityChecks = []domain.QualityCheckRecord{}
	}
	return &batch, nil
}
