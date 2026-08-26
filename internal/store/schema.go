package store

import (
	"context"
	"fmt"
)

const schema = `
CREATE TABLE IF NOT EXISTS corpus_batches (
    id TEXT PRIMARY KEY,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    status TEXT NOT NULL,
    snapshot_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS idempotency_records (
    batch_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    response_json BLOB NOT NULL,
    revision INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (batch_id, request_id)
);
CREATE TABLE IF NOT EXISTS audit_events (
    event_id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    request_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload_digest TEXT NOT NULL,
    payload_json BLOB NOT NULL DEFAULT '{}',
    occurred_at TEXT NOT NULL,
    UNIQUE (batch_id, revision),
    UNIQUE (batch_id, request_id)
);
CREATE INDEX IF NOT EXISTS idx_audit_batch_order ON audit_events(batch_id, revision, occurred_at, event_id);
`

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("初始化 SQLite schema: %w", err)
	}
	hasPayload, err := s.hasColumn(ctx, "audit_events", "payload_json")
	if err != nil {
		return err
	}
	if !hasPayload {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE audit_events ADD COLUMN payload_json BLOB NOT NULL DEFAULT '{}'`); err != nil {
			return fmt.Errorf("升级审计事件 schema: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE audit_events
			SET payload_json = COALESCE((
				SELECT response_json FROM idempotency_records
				WHERE idempotency_records.batch_id = audit_events.batch_id
				AND idempotency_records.request_id = audit_events.request_id
			), '{}')`); err != nil {
			return fmt.Errorf("回填审计事件载荷: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) hasColumn(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, fmt.Errorf("读取 %s schema: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("扫描 %s schema: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("遍历 %s schema: %w", table, err)
	}
	return false, nil
}
