package domain

import (
	"encoding/json"
	"time"
)

type AuditEvent struct {
	EventID       string          `json:"event_id"`
	BatchID       string          `json:"batch_id"`
	Revision      int64           `json:"revision"`
	RequestID     string          `json:"request_id"`
	ActorID       string          `json:"actor_id"`
	EventType     string          `json:"event_type"`
	PayloadDigest string          `json:"payload_digest"`
	Payload       json.RawMessage `json:"payload"`
	OccurredAt    time.Time       `json:"occurred_at"`
}
