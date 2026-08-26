package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"bioacoustic-corpus-release/internal/domain"
)

func ValidateTimeline(batch *domain.CorpusBatch, events []domain.AuditEvent) error {
	if batch == nil {
		return fmt.Errorf("批次不能为空")
	}
	if len(events) == 0 {
		return fmt.Errorf("批次 %s 没有审计事件", batch.ID)
	}
	expected := int64(1)
	seenRequests := make(map[string]bool, len(events))
	for i, event := range events {
		if event.BatchID != batch.ID {
			return fmt.Errorf("事件 %d 的批次标识不一致", i)
		}
		if event.Revision != expected {
			return fmt.Errorf("事件 %d 修订不连续: 得到 %d，期望 %d", i, event.Revision, expected)
		}
		if event.RequestID == "" || seenRequests[event.RequestID] {
			return fmt.Errorf("事件 %d 的 request_id 为空或重复", i)
		}
		if event.ActorID == "" || event.EventType == "" || len(event.PayloadDigest) != 64 {
			return fmt.Errorf("事件 %d 内容不完整", i)
		}
		digest := sha256.Sum256(event.Payload)
		if hex.EncodeToString(digest[:]) != event.PayloadDigest {
			return fmt.Errorf("事件 %d 载荷摘要不一致", i)
		}
		seenRequests[event.RequestID] = true
		expected++
	}
	if events[len(events)-1].Revision != batch.Revision {
		return fmt.Errorf("审计末修订 %d 与批次修订 %d 不一致", events[len(events)-1].Revision, batch.Revision)
	}
	return nil
}
