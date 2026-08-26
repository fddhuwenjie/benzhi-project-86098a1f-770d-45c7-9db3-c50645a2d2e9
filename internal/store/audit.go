package store

import (
	"context"
	"fmt"
	"time"

	"bioacoustic-corpus-release/internal/domain"
)

func (s *SQLiteStore) ListAudit(ctx context.Context, batchID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT event_id, batch_id, revision, request_id, actor_id, event_type, payload_digest, payload_json, occurred_at
        FROM audit_events WHERE batch_id = ? ORDER BY revision ASC, occurred_at ASC, event_id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("查询审计事件: %w", err)
	}
	defer rows.Close()
	events := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var event domain.AuditEvent
		var occurred string
		if err := rows.Scan(&event.EventID, &event.BatchID, &event.Revision, &event.RequestID,
			&event.ActorID, &event.EventType, &event.PayloadDigest, &event.Payload, &occurred); err != nil {
			return nil, fmt.Errorf("扫描审计事件: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, fmt.Errorf("解析审计时间: %w", err)
		}
		event.OccurredAt = parsed
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历审计事件: %w", err)
	}
	return events, nil
}
