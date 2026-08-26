package audit

import (
	"strings"
	"testing"

	"bioacoustic-corpus-release/internal/domain"
)

func TestValidateTimelineRejectsRevisionGap(t *testing.T) {
	batch := &domain.CorpusBatch{ID: "batch-audit", Revision: 3}
	events := []domain.AuditEvent{
		{BatchID: batch.ID, Revision: 1, RequestID: "request-one", ActorID: "actor-one", EventType: "created", PayloadDigest: strings.Repeat("a", 64)},
		{BatchID: batch.ID, Revision: 3, RequestID: "request-three", ActorID: "actor-one", EventType: "updated", PayloadDigest: strings.Repeat("b", 64)},
	}
	if err := ValidateTimeline(batch, events); err == nil {
		t.Fatal("修订缺口应使审计验证失败")
	}
}
