package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

type rankedClip struct {
	index int
	rank  string
}

type SamplingPreviewStratum struct {
	Stratum        string   `json:"stratum"`
	CandidateCount int      `json:"candidate_count"`
	ClipIDs        []string `json:"clip_ids"`
	Quota          int      `json:"quota"`
	Reasons        []string `json:"reasons"`
}

func (b *CorpusBatch) PreviewSample(quota map[string]int) ([]SamplingPreviewStratum, error) {
	if b.Status != StatusPendingSampling {
		return nil, fmt.Errorf("%w: 仅待抽样批次可预览", ErrInvalidState)
	}
	strata := map[string][]rankedClip{}
	for i, clip := range b.Clips {
		key := StratumKey(clip)
		sum := sha256.Sum256([]byte(b.SamplingSeed + "\x00" + key + "\x00" + clip.ID))
		strata[key] = append(strata[key], rankedClip{i, hex.EncodeToString(sum[:])})
	}
	if quota == nil {
		quota = make(map[string]int, len(strata))
		for key := range strata {
			quota[key] = 1
		}
	}
	for key, n := range quota {
		if _, ok := strata[key]; !ok {
			return nil, Invalid("sampling_quota."+key, "包含不存在的分层")
		}
		if n <= 0 {
			return nil, Invalid("sampling_quota."+key, "配额必须为正")
		}
		if n > len(strata[key]) {
			return nil, Invalid("sampling_quota."+key, "配额超过候选量")
		}
	}
	if len(quota) != len(strata) {
		return nil, Invalid("sampling_quota", "必须为每个分层提供配额")
	}
	keys := make([]string, 0, len(strata))
	for k := range strata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SamplingPreviewStratum, 0, len(keys))
	for _, key := range keys {
		ranked := strata[key]
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].rank == ranked[j].rank {
				return b.Clips[ranked[i].index].ID < b.Clips[ranked[j].index].ID
			}
			return ranked[i].rank < ranked[j].rank
		})
		n := quota[key]
		ids := make([]string, len(ranked))
		reasons := make([]string, 0, n)
		for i, it := range ranked {
			ids[i] = b.Clips[it.index].ID
			if i < n {
				reasons = append(reasons, fmt.Sprintf("分层 %s 配额 %d，种子 %s，排序 %d", key, n, b.SamplingSeed, i+1))
			}
		}
		out = append(out, SamplingPreviewStratum{key, len(ranked), ids, n, reasons})
	}
	return out, nil
}

func (b *CorpusBatch) LockSample(quota map[string]int, now time.Time) error {
	if err := b.Status.EnsureMutable(); err != nil {
		return err
	}
	if b.Status != StatusPendingSampling {
		return fmt.Errorf("%w: 仅待抽样批次可锁定样本", ErrInvalidState)
	}
	strata := make(map[string][]rankedClip)
	for i, clip := range b.Clips {
		key := StratumKey(clip)
		sum := sha256.Sum256([]byte(b.SamplingSeed + "\x00" + key + "\x00" + clip.ID))
		strata[key] = append(strata[key], rankedClip{index: i, rank: hex.EncodeToString(sum[:])})
	}
	if len(strata) == 0 {
		return Invalid("clips", "没有可抽样片段")
	}
	for key := range quota {
		if _, ok := strata[key]; !ok {
			return Invalid("sampling_quota", "包含不存在的分层 "+key)
		}
		if quota[key] < 1 {
			return Invalid("sampling_quota", "每个分层配额至少为 1")
		}
	}
	if len(quota) != len(strata) {
		return Invalid("sampling_quota", "必须为每个分层提供配额")
	}
	b.SamplingQuota = make(map[string]int, len(strata))
	for key, ranked := range strata {
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].rank == ranked[j].rank {
				return b.Clips[ranked[i].index].ID < b.Clips[ranked[j].index].ID
			}
			return ranked[i].rank < ranked[j].rank
		})
		limit := quota[key]
		if limit == 0 {
			limit = 1
		}
		if limit > len(ranked) {
			return Invalid("sampling_quota."+key, "配额超过候选量")
		}
		b.SamplingQuota[key] = limit
		for position, item := range ranked {
			if position < limit {
				b.Clips[item.index].Sampled = true
				b.Clips[item.index].SampleReason = fmt.Sprintf("分层 %s 配额 %d，种子 %s，排序 %d", key, limit, b.SamplingSeed, position+1)
			}
		}
	}
	if err := b.transition(StatusAnnotating); err != nil {
		return err
	}
	b.touch(now)
	return nil
}
