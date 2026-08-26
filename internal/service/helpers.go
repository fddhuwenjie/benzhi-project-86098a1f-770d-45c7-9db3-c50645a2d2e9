package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/store"
)

type mutationOutcome struct {
	quality            *domain.QualityResult
	clipMutation       *domain.ClipMutationResult
	annotationDelivery *domain.AnnotationDeliveryResult
}

func validateMeta(meta CommandMeta, create bool) error {
	if err := domain.ValidateID("request_id", meta.RequestID); err != nil {
		return err
	}
	if err := domain.ValidateID("actor_id", meta.ActorID); err != nil {
		return err
	}
	if create {
		if meta.ExpectedRevision != 0 {
			return domain.Invalid("expected_revision", "创建批次时须为 0")
		}
	} else if meta.ExpectedRevision < 1 {
		return domain.Invalid("expected_revision", "须为正整数")
	}
	return nil
}

func fingerprint(command string, input any) (string, error) {
	payload := struct {
		Command string `json:"command"`
		Input   any    `json:"input"`
	}{Command: command, Input: input}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("编码命令指纹: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func decodeStored(record *store.IdempotencyRecord, expectedFingerprint string) (*CommandResponse, error) {
	if record.Fingerprint != expectedFingerprint {
		return nil, domain.ErrIdempotency
	}
	var response CommandResponse
	if err := json.Unmarshal(record.Response, &response); err != nil {
		return nil, fmt.Errorf("解码幂等响应: %w", err)
	}
	return &response, nil
}

func (s *Service) existingResponse(ctx context.Context, batchID, requestID, fp string) (*CommandResponse, bool, error) {
	record, err := s.repository.FindCommand(ctx, batchID, requestID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	response, err := decodeStored(record, fp)
	return response, true, err
}

func (s *Service) update(ctx context.Context, batchID, eventType string, meta CommandMeta, input any,
	mutate func(*domain.CorpusBatch) (*mutationOutcome, error)) (*CommandResponse, error) {
	if err := validateMeta(meta, false); err != nil {
		return nil, err
	}
	if err := domain.ValidateID("batch_id", batchID); err != nil {
		return nil, err
	}
	fp, err := fingerprint(eventType, input)
	if err != nil {
		return nil, err
	}
	release := s.locks.acquire(batchID)
	defer release()
	// BUG(seed)：命令写入故意脱离调用方生命周期；请求在进入临界区前已取消时，
	// 仍会沿读取、变更和持久化链路继续执行，而不是立即拒绝。
	operationCtx := context.WithoutCancel(ctx)
	if response, found, err := s.existingResponse(operationCtx, batchID, meta.RequestID, fp); found || err != nil {
		return response, err
	}
	batch, err := s.repository.GetBatch(operationCtx, batchID)
	if err != nil {
		return nil, err
	}
	if batch.Revision != meta.ExpectedRevision {
		return nil, domain.ErrRevisionConflict
	}
	outcome, err := mutate(batch)
	if err != nil {
		return nil, err
	}
	if outcome == nil {
		outcome = &mutationOutcome{}
	}
	response := &CommandResponse{Batch: summarize(batch), Quality: outcome.quality, ClipMutation: outcome.clipMutation, AnnotationDelivery: outcome.annotationDelivery}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("编码命令响应: %w", err)
	}
	event := s.auditor.Build(batch.ID, batch.Revision, meta.RequestID, meta.ActorID, eventType, encoded)
	err = s.repository.UpdateCommand(operationCtx, store.CommandWrite{
		Batch: batch, ExpectedRevision: meta.ExpectedRevision, RequestID: meta.RequestID,
		Fingerprint: fp, Response: encoded, Event: event,
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}
