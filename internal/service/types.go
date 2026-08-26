package service

import (
	"time"

	"bioacoustic-corpus-release/internal/domain"
)

type CommandMeta struct {
	RequestID        string `json:"request_id"`
	ActorID          string `json:"actor_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateBatchInput struct {
	Meta              CommandMeta              `json:"meta"`
	ID                string                   `json:"id"`
	Title             string                   `json:"title"`
	Description       string                   `json:"description"`
	SamplingSeed      string                   `json:"sampling_seed"`
	QualityThresholds domain.QualityThresholds `json:"quality_thresholds"`
}

type ClipInput struct {
	ID             string    `json:"id"`
	SourceURI      string    `json:"source_uri"`
	ContentDigest  string    `json:"content_digest"`
	RegionCode     string    `json:"region_code"`
	RecordedAt     time.Time `json:"recorded_at"`
	DurationMS     int64     `json:"duration_ms"`
	CandidateTaxon string    `json:"candidate_taxon"`
}

type AddClipsInput struct {
	Meta  CommandMeta `json:"meta"`
	Clips []ClipInput `json:"clips"`
}

type ClipPatchInput struct {
	ID             string     `json:"id"`
	SourceURI      *string    `json:"source_uri,omitempty"`
	ContentDigest  *string    `json:"content_digest,omitempty"`
	RegionCode     *string    `json:"region_code,omitempty"`
	RecordedAt     *time.Time `json:"recorded_at,omitempty"`
	DurationMS     *int64     `json:"duration_ms,omitempty"`
	CandidateTaxon *string    `json:"candidate_taxon,omitempty"`
}

type CorrectClipsInput struct {
	Meta  CommandMeta      `json:"meta"`
	Clips []ClipPatchInput `json:"clips"`
}

type WithdrawClipsInput struct {
	Meta    CommandMeta `json:"meta"`
	ClipIDs []string    `json:"clip_ids"`
}

type SampleInput struct {
	Meta  CommandMeta    `json:"meta"`
	Quota map[string]int `json:"quota,omitempty"`
}

type AnnotationInput struct {
	Meta         CommandMeta `json:"meta"`
	ClipID       string      `json:"clip_id"`
	TaxonLabel   string      `json:"taxon_label"`
	Confidence   float64     `json:"confidence"`
	EvidenceNote string      `json:"evidence_note"`
}

type AnnotationItemInput struct {
	ClipID       string  `json:"clip_id"`
	TaxonLabel   string  `json:"taxon_label"`
	Confidence   float64 `json:"confidence"`
	EvidenceNote string  `json:"evidence_note"`
}

type AnnotationDeliveryInput struct {
	Meta        CommandMeta           `json:"meta"`
	Annotations []AnnotationItemInput `json:"annotations"`
}

type AnnotationHTTPInput struct {
	Meta         CommandMeta           `json:"meta"`
	ClipID       string                `json:"clip_id,omitempty"`
	TaxonLabel   string                `json:"taxon_label,omitempty"`
	Confidence   float64               `json:"confidence,omitempty"`
	EvidenceNote string                `json:"evidence_note,omitempty"`
	Annotations  []AnnotationItemInput `json:"annotations,omitempty"`
}

type AdjudicationInput struct {
	Meta         CommandMeta `json:"meta"`
	ClipID       string      `json:"clip_id"`
	FinalLabel   string      `json:"final_label"`
	Reason       string      `json:"reason"`
	EvidenceRefs []string    `json:"evidence_refs"`
}

type MetaInput struct {
	Meta CommandMeta `json:"meta"`
}

type BatchSummary struct {
	ID                   string                   `json:"id"`
	Title                string                   `json:"title"`
	Description          string                   `json:"description"`
	Status               domain.BatchStatus       `json:"status"`
	Revision             int64                    `json:"revision"`
	SamplingSeed         string                   `json:"sampling_seed"`
	SamplingQuota        map[string]int           `json:"sampling_quota,omitempty"`
	QualityThresholds    domain.QualityThresholds `json:"quality_thresholds"`
	ClipCount            int                      `json:"clip_count"`
	SampleCount          int                      `json:"sample_count"`
	AnnotationCount      int                      `json:"annotation_count"`
	PendingConflictCount int                      `json:"pending_conflict_count"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
	PublishedAt          *time.Time               `json:"published_at,omitempty"`
	DuplicateChecked     bool                     `json:"duplicate_checked"`
	DuplicateCheck       DuplicateCheckSummary    `json:"duplicate_check"`
}

type DuplicateCheckSummary struct {
	Checked   bool `json:"checked"`
	Unique    bool `json:"unique"`
	ClipCount int  `json:"clip_count"`
}

type CommandResponse struct {
	Batch              BatchSummary                     `json:"batch"`
	Quality            *domain.QualityResult            `json:"quality,omitempty"`
	ClipMutation       *domain.ClipMutationResult       `json:"clip_mutation,omitempty"`
	AnnotationDelivery *domain.AnnotationDeliveryResult `json:"annotation_delivery,omitempty"`
}

type BatchDetail struct {
	Batch             BatchSummary           `json:"batch"`
	Clips             []domain.RecordingClip `json:"clips"`
	LastQualityResult *domain.QualityResult  `json:"last_quality_result,omitempty"`
}

type AnnotationProgress struct {
	ActorID                string   `json:"actor_id"`
	SampleCount            int      `json:"sample_count"`
	SubmittedCount         int      `json:"submitted_count"`
	PendingClipIDs         []string `json:"pending_clip_ids"`
	AverageConfidence      float64  `json:"average_confidence"`
	DoubleLabelRate        float64  `json:"double_label_rate"`
	OverallDoubleLabelRate float64  `json:"overall_double_label_rate"`
}

type ManifestPage struct {
	BatchID        string                `json:"batch_id"`
	ReleaseVersion string                `json:"release_version"`
	GeneratedAt    time.Time             `json:"generated_at"`
	SHA256Digest   string                `json:"sha256_digest"`
	Total          int                   `json:"total"`
	Offset         int                   `json:"offset"`
	Limit          int                   `json:"limit"`
	ClipEntries    []domain.ManifestClip `json:"clip_entries"`
}

type AuditPage struct {
	Events          []domain.AuditEvent `json:"events"`
	Total           int                 `json:"total"`
	FirstRevision   int64               `json:"first_revision"`
	LastRevision    int64               `json:"last_revision"`
	Offset          int                 `json:"offset"`
	Limit           int                 `json:"limit"`
	NextOffset      *int                `json:"next_offset,omitempty"`
	NextCursor      string              `json:"next_cursor,omitempty"`
	ContinuityProof string              `json:"continuity_proof"`
}

type QualityHistoryFilter struct {
	Passed      *bool
	IssueCode   string
	ClipID      string
	Stratum     string
	MinRevision int64
	MaxRevision int64
	Offset      int
	Limit       int
}

type QualityHistoryPage struct {
	Records    []domain.QualityCheckRecord `json:"records"`
	Total      int                         `json:"total"`
	Offset     int                         `json:"offset"`
	Limit      int                         `json:"limit"`
	NextOffset *int                        `json:"next_offset,omitempty"`
}

type SamplingPreview struct {
	SamplingSeed string                          `json:"sampling_seed"`
	Strata       []domain.SamplingPreviewStratum `json:"strata"`
}

func summarize(batch *domain.CorpusBatch) BatchSummary {
	quota := make(map[string]int, len(batch.SamplingQuota))
	for key, value := range batch.SamplingQuota {
		quota[key] = value
	}
	return BatchSummary{
		ID: batch.ID, Title: batch.Title, Description: batch.Description,
		Status: batch.Status, Revision: batch.Revision, SamplingSeed: batch.SamplingSeed,
		SamplingQuota: quota, QualityThresholds: batch.QualityThresholds,
		ClipCount: len(batch.Clips), SampleCount: len(batch.SampledClips()),
		AnnotationCount: len(batch.Annotations), PendingConflictCount: len(batch.PendingConflicts()),
		CreatedAt: batch.CreatedAt, UpdatedAt: batch.UpdatedAt, PublishedAt: batch.PublishedAt,
		DuplicateChecked: len(batch.Clips) > 0,
		DuplicateCheck:   DuplicateCheckSummary{Checked: len(batch.Clips) > 0, Unique: true, ClipCount: len(batch.Clips)},
	}
}
