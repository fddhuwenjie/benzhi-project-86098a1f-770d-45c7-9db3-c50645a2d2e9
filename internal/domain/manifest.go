package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type manifestDigestPayload struct {
	BatchID        string         `json:"batch_id"`
	ReleaseVersion string         `json:"release_version"`
	ClipEntries    []ManifestClip `json:"clip_entries"`
	QualityMetrics QualityMetrics `json:"quality_metrics"`
}

func ManifestDigest(m *ReleaseManifest) (string, error) {
	if m == nil {
		return "", ErrNotFound
	}
	entries := append([]ManifestClip(nil), m.ClipEntries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ClipID < entries[j].ClipID })
	payload, err := json.Marshal(manifestDigestPayload{BatchID: m.BatchID, ReleaseVersion: m.ReleaseVersion, ClipEntries: entries, QualityMetrics: m.QualityMetrics})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (b *CorpusBatch) buildManifest(metrics QualityMetrics, now time.Time) (*ReleaseManifest, error) {
	entries := make([]ManifestClip, 0, len(b.SampledClips()))
	for _, clip := range b.SampledClips() {
		label, resolution, ok := b.FinalLabel(clip.ID)
		if !ok {
			return nil, fmt.Errorf("%w: 片段 %s 没有最终标签", ErrInvalidState, clip.ID)
		}
		entries = append(entries, ManifestClip{
			ClipID: clip.ID, SourceURI: clip.SourceURI, ContentDigest: clip.ContentDigest,
			RegionCode: clip.RegionCode, RecordedAt: clip.RecordedAt.UTC().Format(time.RFC3339Nano),
			DurationMS: clip.DurationMS, CandidateTaxon: clip.CandidateTaxon,
			FinalTaxonLabel: label, Resolution: resolution,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ClipID < entries[j].ClipID })
	version := fmt.Sprintf("r%d", b.Revision+1)
	payload := manifestDigestPayload{BatchID: b.ID, ReleaseVersion: version, ClipEntries: entries, QualityMetrics: metrics}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("编码发布清单: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return &ReleaseManifest{
		BatchID: b.ID, ReleaseVersion: version, ClipEntries: entries,
		QualityMetrics: metrics, GeneratedAt: now.UTC(), SHA256Digest: hex.EncodeToString(digest[:]),
	}, nil
}
