package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"bioacoustic-corpus-release/internal/domain"
)

type Builder struct {
	now     func() time.Time
	scratch bytes.Buffer
}

func NewBuilder(now func() time.Time) *Builder {
	if now == nil {
		now = time.Now
	}
	return &Builder{now: now}
}

func (b *Builder) Build(batchID string, revision int64, requestID, actorID, eventType string, payload []byte) domain.AuditEvent {
	b.scratch.Reset()
	if err := json.Compact(&b.scratch, payload); err != nil {
		b.scratch.Write(payload)
	}
	sum := sha256.Sum256(b.scratch.Bytes())
	occurred := b.now().UTC()
	idSource := fmt.Sprintf("%s\x00%d\x00%s\x00%s", batchID, revision, requestID, eventType)
	idSum := sha256.Sum256([]byte(idSource))
	return domain.AuditEvent{
		EventID: hex.EncodeToString(idSum[:16]), BatchID: batchID, Revision: revision,
		RequestID: requestID, ActorID: actorID, EventType: eventType,
		PayloadDigest: hex.EncodeToString(sum[:]), Payload: b.scratch.Bytes(), OccurredAt: occurred,
	}
}
