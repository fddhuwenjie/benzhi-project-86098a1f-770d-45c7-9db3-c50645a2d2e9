package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/store"
)

func (s *Service) CreateBatch(ctx context.Context, input CreateBatchInput) (*CommandResponse, error) {
	if err := validateMeta(input.Meta, true); err != nil {
		return nil, err
	}
	if err := domain.ValidateID("id", input.ID); err != nil {
		return nil, err
	}
	fp, err := fingerprint("batch.created", input)
	if err != nil {
		return nil, err
	}
	release, err := s.locks.acquire(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	defer release()
	if response, found, err := s.existingResponse(ctx, input.ID, input.Meta.RequestID, fp); found || err != nil {
		return response, err
	}
	now := s.now().UTC()
	batch, err := domain.NewBatch(input.ID, input.Title, input.Description, input.SamplingSeed, input.QualityThresholds, now)
	if err != nil {
		return nil, err
	}
	response := &CommandResponse{Batch: summarize(batch)}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("编码创建响应: %w", err)
	}
	event := s.auditor.Build(batch.ID, batch.Revision, input.Meta.RequestID, input.Meta.ActorID, "batch.created", encoded)
	err = s.repository.CreateCommand(ctx, store.CommandWrite{
		Batch: batch, ExpectedRevision: 0, RequestID: input.Meta.RequestID,
		Fingerprint: fp, Response: encoded, Event: event,
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *Service) CorrectClips(ctx context.Context, batchID string, input CorrectClipsInput) (*CommandResponse, error) {
	canonical := input
	canonical.Clips = append([]ClipPatchInput(nil), input.Clips...)
	sort.SliceStable(canonical.Clips, func(i, j int) bool { return canonical.Clips[i].ID < canonical.Clips[j].ID })
	return s.update(ctx, batchID, "clips.corrected", canonical.Meta, canonical, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		patches := make([]domain.ClipPatch, len(canonical.Clips))
		for i, patch := range canonical.Clips {
			patches[i] = domain.ClipPatch{
				ID: patch.ID, SourceURI: patch.SourceURI, ContentDigest: patch.ContentDigest,
				RegionCode: patch.RegionCode, RecordedAt: patch.RecordedAt,
				DurationMS: patch.DurationMS, CandidateTaxon: patch.CandidateTaxon,
			}
		}
		result, err := batch.CorrectClips(patches, s.now())
		return &mutationOutcome{clipMutation: result}, err
	})
}

func (s *Service) WithdrawClips(ctx context.Context, batchID string, input WithdrawClipsInput) (*CommandResponse, error) {
	canonical := input
	canonical.ClipIDs = append([]string(nil), input.ClipIDs...)
	sort.Strings(canonical.ClipIDs)
	return s.update(ctx, batchID, "clips.withdrawn", canonical.Meta, canonical, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		result, err := batch.WithdrawClips(canonical.ClipIDs, s.now())
		return &mutationOutcome{clipMutation: result}, err
	})
}

func (s *Service) AddClips(ctx context.Context, batchID string, input AddClipsInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "clips.registered", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		clips := make([]domain.RecordingClip, len(input.Clips))
		for i, clip := range input.Clips {
			clips[i] = domain.RecordingClip{
				ID: clip.ID, SourceURI: clip.SourceURI, ContentDigest: clip.ContentDigest,
				RegionCode: clip.RegionCode, RecordedAt: clip.RecordedAt,
				DurationMS: clip.DurationMS, CandidateTaxon: clip.CandidateTaxon,
			}
		}
		return &mutationOutcome{}, batch.AddClips(clips, s.now())
	})
}

func (s *Service) SubmitDraft(ctx context.Context, batchID string, input MetaInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "batch.submitted", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		return &mutationOutcome{}, batch.SubmitDraft(s.now())
	})
}

func (s *Service) LockSample(ctx context.Context, batchID string, input SampleInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "sample.locked", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		return &mutationOutcome{}, batch.LockSample(input.Quota, s.now())
	})
}
