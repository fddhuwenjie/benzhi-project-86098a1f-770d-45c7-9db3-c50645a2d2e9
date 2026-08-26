package service

import (
	"context"
	"sort"

	"bioacoustic-corpus-release/internal/domain"
)

func (s *Service) SubmitAnnotation(ctx context.Context, batchID string, input AnnotationInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "annotation.submitted", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		annotation := domain.IndependentAnnotation{
			ClipID: input.ClipID, AnnotatorID: input.Meta.ActorID,
			TaxonLabel: input.TaxonLabel, Confidence: input.Confidence,
			EvidenceNote: input.EvidenceNote,
		}
		return &mutationOutcome{}, batch.SubmitAnnotation(annotation, s.now())
	})
}

func (s *Service) SubmitAnnotations(ctx context.Context, batchID string, input AnnotationDeliveryInput) (*CommandResponse, error) {
	canonical := input
	canonical.Annotations = append([]AnnotationItemInput(nil), input.Annotations...)
	sort.SliceStable(canonical.Annotations, func(i, j int) bool { return canonical.Annotations[i].ClipID < canonical.Annotations[j].ClipID })
	return s.update(ctx, batchID, "annotations.delivered", canonical.Meta, canonical, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		deliveries := make([]domain.IndependentAnnotation, len(canonical.Annotations))
		for i, item := range canonical.Annotations {
			deliveries[i] = domain.IndependentAnnotation{
				ClipID: item.ClipID, TaxonLabel: item.TaxonLabel,
				Confidence: item.Confidence, EvidenceNote: item.EvidenceNote,
			}
		}
		result, err := batch.SubmitAnnotations(deliveries, canonical.Meta.ActorID, s.now())
		return &mutationOutcome{annotationDelivery: result}, err
	})
}

func (s *Service) Adjudicate(ctx context.Context, batchID string, input AdjudicationInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "conflict.adjudicated", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		decision := domain.AdjudicationDecision{
			ClipID: input.ClipID, AdjudicatorID: input.Meta.ActorID,
			FinalLabel: input.FinalLabel, Reason: input.Reason,
			EvidenceRefs: append([]string(nil), input.EvidenceRefs...),
		}
		return &mutationOutcome{}, batch.Adjudicate(decision, s.now())
	})
}

func (s *Service) RunQualityGate(ctx context.Context, batchID string, input MetaInput) (*CommandResponse, error) {
	return s.update(ctx, batchID, "quality.checked", input.Meta, input, func(batch *domain.CorpusBatch) (*mutationOutcome, error) {
		quality, err := batch.RunQualityGate(input.Meta.ActorID, input.Meta.RequestID, s.now())
		return &mutationOutcome{quality: quality}, err
	})
}
