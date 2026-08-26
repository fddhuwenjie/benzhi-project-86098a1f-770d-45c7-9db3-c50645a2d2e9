package domain

import "time"

type QualityThresholds struct {
	MinCompleteness    float64 `json:"min_completeness"`
	MinAgreement       float64 `json:"min_agreement"`
	MinStratumCoverage float64 `json:"min_stratum_coverage"`
}

type RecordingClip struct {
	ID             string    `json:"id"`
	BatchID        string    `json:"batch_id"`
	SourceURI      string    `json:"source_uri"`
	ContentDigest  string    `json:"content_digest"`
	RegionCode     string    `json:"region_code"`
	RecordedAt     time.Time `json:"recorded_at"`
	DurationMS     int64     `json:"duration_ms"`
	CandidateTaxon string    `json:"candidate_taxon"`
	Sampled        bool      `json:"sampled"`
	SampleReason   string    `json:"sample_reason,omitempty"`
}

type IndependentAnnotation struct {
	ID           string    `json:"id"`
	BatchID      string    `json:"batch_id"`
	ClipID       string    `json:"clip_id"`
	AnnotatorID  string    `json:"annotator_id"`
	TaxonLabel   string    `json:"taxon_label"`
	Confidence   float64   `json:"confidence"`
	EvidenceNote string    `json:"evidence_note"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

type AdjudicationDecision struct {
	ID            string    `json:"id"`
	BatchID       string    `json:"batch_id"`
	ClipID        string    `json:"clip_id"`
	AdjudicatorID string    `json:"adjudicator_id"`
	FinalLabel    string    `json:"final_label"`
	Reason        string    `json:"reason"`
	EvidenceRefs  []string  `json:"evidence_refs"`
	DecidedAt     time.Time `json:"decided_at"`
}

type QualityMetrics struct {
	SampleCount     int     `json:"sample_count"`
	CompleteCount   int     `json:"complete_count"`
	AgreementCount  int     `json:"agreement_count"`
	TotalStrata     int     `json:"total_strata"`
	CoveredStrata   int     `json:"covered_strata"`
	UnresolvedCount int     `json:"unresolved_conflict_count"`
	Completeness    float64 `json:"completeness"`
	Agreement       float64 `json:"agreement"`
	StratumCoverage float64 `json:"stratum_coverage"`
}

type QualityIssue struct {
	Key     string `json:"key"`
	Code    string `json:"code"`
	Message string `json:"message"`
	ClipID  string `json:"clip_id,omitempty"`
	Stratum string `json:"stratum,omitempty"`
}

type QualityMetricChanges struct {
	SampleCount     int     `json:"sample_count"`
	CompleteCount   int     `json:"complete_count"`
	AgreementCount  int     `json:"agreement_count"`
	TotalStrata     int     `json:"total_strata"`
	CoveredStrata   int     `json:"covered_strata"`
	UnresolvedCount int     `json:"unresolved_conflict_count"`
	Completeness    float64 `json:"completeness"`
	Agreement       float64 `json:"agreement"`
	StratumCoverage float64 `json:"stratum_coverage"`
}

type QualityComparison struct {
	Added         []QualityIssue       `json:"added"`
	Ongoing       []QualityIssue       `json:"ongoing"`
	Resolved      []QualityIssue       `json:"resolved"`
	MetricChanges QualityMetricChanges `json:"metric_changes"`
}

type QualityCheckRecord struct {
	Sequence        int               `json:"sequence"`
	ActorID         string            `json:"actor_id"`
	RequestID       string            `json:"request_id"`
	CheckedRevision int64             `json:"checked_revision"`
	Thresholds      QualityThresholds `json:"thresholds"`
	Metrics         QualityMetrics    `json:"metrics"`
	Issues          []QualityIssue    `json:"issues"`
	Passed          bool              `json:"passed"`
	CheckedAt       time.Time         `json:"checked_at"`
	Comparison      QualityComparison `json:"comparison"`
}

type QualityResult struct {
	Passed          bool              `json:"passed"`
	Metrics         QualityMetrics    `json:"metrics"`
	Issues          []QualityIssue    `json:"issues"`
	CheckedAt       time.Time         `json:"checked_at"`
	CheckedRevision int64             `json:"checked_revision"`
	Thresholds      QualityThresholds `json:"thresholds"`
}

type ManifestClip struct {
	ClipID          string `json:"clip_id"`
	SourceURI       string `json:"source_uri"`
	ContentDigest   string `json:"content_digest"`
	RegionCode      string `json:"region_code"`
	RecordedAt      string `json:"recorded_at"`
	DurationMS      int64  `json:"duration_ms"`
	CandidateTaxon  string `json:"candidate_taxon"`
	FinalTaxonLabel string `json:"final_taxon_label"`
	Resolution      string `json:"resolution"`
}

type ReleaseManifest struct {
	BatchID        string         `json:"batch_id"`
	ReleaseVersion string         `json:"release_version"`
	ClipEntries    []ManifestClip `json:"clip_entries"`
	QualityMetrics QualityMetrics `json:"quality_metrics"`
	GeneratedAt    time.Time      `json:"generated_at"`
	SHA256Digest   string         `json:"sha256_digest"`
}

type CorpusBatch struct {
	ID                string                  `json:"id"`
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	Status            BatchStatus             `json:"status"`
	Revision          int64                   `json:"revision"`
	SamplingSeed      string                  `json:"sampling_seed"`
	SamplingQuota     map[string]int          `json:"sampling_quota,omitempty"`
	QualityThresholds QualityThresholds       `json:"quality_thresholds"`
	Clips             []RecordingClip         `json:"clips"`
	Annotations       []IndependentAnnotation `json:"annotations"`
	Adjudications     []AdjudicationDecision  `json:"adjudications"`
	QualityChecks     []QualityCheckRecord    `json:"quality_checks"`
	LastQualityResult *QualityResult          `json:"last_quality_result,omitempty"`
	Manifest          *ReleaseManifest        `json:"manifest,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	PublishedAt       *time.Time              `json:"published_at,omitempty"`
}

func (b *CorpusBatch) SampledClips() []RecordingClip {
	out := make([]RecordingClip, 0)
	for _, clip := range b.Clips {
		if clip.Sampled {
			out = append(out, clip)
		}
	}
	return out
}

func (b *CorpusBatch) ClipByID(id string) (*RecordingClip, bool) {
	for i := range b.Clips {
		if b.Clips[i].ID == id {
			return &b.Clips[i], true
		}
	}
	return nil, false
}
