package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ClipPatch struct {
	ID             string
	SourceURI      *string
	ContentDigest  *string
	RegionCode     *string
	RecordedAt     *time.Time
	DurationMS     *int64
	CandidateTaxon *string
}

type ClipPayloadChange struct {
	ClipID       string `json:"clip_id"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

type ClipMutationResult struct {
	AffectedClipIDs []string            `json:"affected_clip_ids"`
	Changes         []ClipPayloadChange `json:"changes"`
}

func clipPayloadDigest(clip *RecordingClip) string {
	encoded, _ := json.Marshal(clip)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (b *CorpusBatch) CorrectClips(patches []ClipPatch, now time.Time) (*ClipMutationResult, error) {
	if err := b.ensureDraftClipMutation(); err != nil {
		return nil, err
	}
	if len(patches) == 0 || len(patches) > 1000 {
		return nil, Invalid("clips", "每次须纠错 1 至 1000 个片段")
	}
	working := append([]RecordingClip(nil), b.Clips...)
	positions := make(map[string]int, len(working))
	for i := range working {
		positions[working[i].ID] = i
	}
	seen := make(map[string]bool, len(patches))
	changes := make([]ClipPayloadChange, 0, len(patches))
	for _, patch := range patches {
		if err := ValidateID("clips["+patch.ID+"].id", patch.ID); err != nil {
			return nil, err
		}
		if seen[patch.ID] {
			return nil, Invalid("clips["+patch.ID+"].id", "纠错请求内片段标识重复")
		}
		seen[patch.ID] = true
		position, ok := positions[patch.ID]
		if !ok {
			return nil, Invalid("clips["+patch.ID+"].id", "片段不存在")
		}
		if patch.SourceURI == nil && patch.ContentDigest == nil && patch.RegionCode == nil && patch.RecordedAt == nil && patch.DurationMS == nil && patch.CandidateTaxon == nil {
			return nil, Invalid("clips["+patch.ID+"]", "至少提供一个待替换字段")
		}
		before := working[position]
		after := before
		if patch.SourceURI != nil {
			after.SourceURI = *patch.SourceURI
		}
		if patch.ContentDigest != nil {
			after.ContentDigest = NormalizeDigest(*patch.ContentDigest)
		}
		if patch.RegionCode != nil {
			after.RegionCode = *patch.RegionCode
		}
		if patch.RecordedAt != nil {
			after.RecordedAt = *patch.RecordedAt
		}
		if patch.DurationMS != nil {
			after.DurationMS = *patch.DurationMS
		}
		if patch.CandidateTaxon != nil {
			after.CandidateTaxon = *patch.CandidateTaxon
		}
		working[position] = after
		beforeCopy, afterCopy := before, after
		changes = append(changes, ClipPayloadChange{ClipID: patch.ID, BeforeSHA256: clipPayloadDigest(&beforeCopy), AfterSHA256: clipPayloadDigest(&afterCopy)})
	}
	if err := validateCompleteClipSet(working, now, seen); err != nil {
		return nil, err
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ClipID < changes[j].ClipID })
	b.Clips = working
	b.touch(now)
	return &ClipMutationResult{AffectedClipIDs: changeClipIDs(changes), Changes: changes}, nil
}

func (b *CorpusBatch) WithdrawClips(clipIDs []string, now time.Time) (*ClipMutationResult, error) {
	if err := b.ensureDraftClipMutation(); err != nil {
		return nil, err
	}
	if len(clipIDs) == 0 || len(clipIDs) > 1000 {
		return nil, Invalid("clip_ids", "每次须撤销 1 至 1000 个片段")
	}
	targets := make(map[string]bool, len(clipIDs))
	for _, id := range clipIDs {
		if err := ValidateID("clip_ids["+id+"]", id); err != nil {
			return nil, err
		}
		if targets[id] {
			return nil, Invalid("clip_ids["+id+"]", "撤销请求内片段标识重复")
		}
		targets[id] = true
	}
	remaining := make([]RecordingClip, 0, len(b.Clips)-len(targets))
	changes := make([]ClipPayloadChange, 0, len(targets))
	for i := range b.Clips {
		clip := b.Clips[i]
		if targets[clip.ID] {
			clipCopy := clip
			changes = append(changes, ClipPayloadChange{ClipID: clip.ID, BeforeSHA256: clipPayloadDigest(&clipCopy), AfterSHA256: clipPayloadDigest(nil)})
			delete(targets, clip.ID)
			continue
		}
		remaining = append(remaining, clip)
	}
	if len(targets) != 0 {
		missing := make([]string, 0, len(targets))
		for id := range targets {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, Invalid("clip_ids["+missing[0]+"]", "片段不存在")
	}
	if err := validateCompleteClipSet(remaining, now, nil); err != nil {
		return nil, err
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ClipID < changes[j].ClipID })
	b.Clips = remaining
	b.touch(now)
	return &ClipMutationResult{AffectedClipIDs: changeClipIDs(changes), Changes: changes}, nil
}

func (b *CorpusBatch) ensureDraftClipMutation() error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusDraft {
		return fmt.Errorf("%w: 仅草稿状态可纠错或撤销片段", ErrInvalidState)
	}
	return nil
}

func validateCompleteClipSet(clips []RecordingClip, now time.Time, preferred map[string]bool) error {
	ids := make(map[string]string, len(clips))
	digests := make(map[string]string, len(clips))
	sources := make(map[string]string, len(clips))
	for _, clip := range clips {
		if err := ValidateClip(clip, now); err != nil {
			if validation, ok := err.(*ValidationError); ok {
				return Invalid("clips["+clip.ID+"]."+strings.TrimPrefix(validation.Field, "clip."), validation.Message)
			}
			return err
		}
		if other, exists := ids[clip.ID]; exists {
			return Invalid("clips["+clip.ID+"].id", "与片段 "+other+" 的标识重复")
		}
		ids[clip.ID] = clip.ID
		digest := NormalizeDigest(clip.ContentDigest)
		if other, exists := digests[digest]; exists {
			if !preferred[clip.ID] && preferred[other] {
				return Invalid("clips["+other+"].content_digest", "与片段 "+clip.ID+" 的内容摘要重复")
			}
			return Invalid("clips["+clip.ID+"].content_digest", "与片段 "+other+" 的内容摘要重复")
		}
		digests[digest] = clip.ID
		source := strings.TrimSpace(clip.SourceURI)
		if other, exists := sources[source]; exists {
			if !preferred[clip.ID] && preferred[other] {
				return Invalid("clips["+other+"].source_uri", "与片段 "+clip.ID+" 的来源 URI 重复")
			}
			return Invalid("clips["+clip.ID+"].source_uri", "与片段 "+other+" 的来源 URI 重复")
		}
		sources[source] = clip.ID
	}
	return nil
}

func changeClipIDs(changes []ClipPayloadChange) []string {
	ids := make([]string, len(changes))
	for i := range changes {
		ids[i] = changes[i].ClipID
	}
	return ids
}
