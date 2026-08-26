package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bioacoustic-corpus-release/internal/domain"
)

func (s *SQLiteStore) CreateCommand(ctx context.Context, write CommandWrite) error {
	encoded, err := validateWrite(write)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始创建事务: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
        INSERT INTO corpus_batches(id, revision, status, snapshot_json, created_at, updated_at)
        VALUES(?, ?, ?, ?, ?, ?)`, write.Batch.ID, write.Batch.Revision, write.Batch.Status,
		encoded, write.Batch.CreatedAt.Format(time.RFC3339Nano), write.Batch.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		if isConstraint(err) {
			return domain.ErrRevisionConflict
		}
		return fmt.Errorf("创建批次: %w", err)
	}
	if err := insertEvidence(ctx, tx, write); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交创建事务: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateCommand(ctx context.Context, write CommandWrite) error {
	encoded, err := validateWrite(write)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始更新事务: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
        UPDATE corpus_batches SET revision = ?, status = ?, snapshot_json = ?, updated_at = ?
        WHERE id = ? AND revision = ?`, write.Batch.Revision, write.Batch.Status, encoded,
		write.Batch.UpdatedAt.Format(time.RFC3339Nano), write.Batch.ID, write.ExpectedRevision)
	if err != nil {
		return fmt.Errorf("更新批次: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("确认批次更新: %w", err)
	}
	if changed != 1 {
		return domain.ErrRevisionConflict
	}
	if err := insertEvidence(ctx, tx, write); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交更新事务: %w", err)
	}
	return nil
}

func validateWrite(write CommandWrite) ([]byte, error) {
	if write.Batch == nil || write.Batch.ID == "" {
		return nil, fmt.Errorf("批次不能为空")
	}
	if write.RequestID == "" || write.Fingerprint == "" || len(write.Response) == 0 {
		return nil, fmt.Errorf("命令证据不完整")
	}
	if write.Event.BatchID != write.Batch.ID || write.Event.Revision != write.Batch.Revision || write.Event.RequestID != write.RequestID {
		return nil, fmt.Errorf("审计事件与命令不一致")
	}
	return encodeBatch(write.Batch)
}

func insertEvidence(ctx context.Context, tx *sql.Tx, write CommandWrite) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO idempotency_records(batch_id, request_id, fingerprint, response_json, revision, created_at)
        VALUES(?, ?, ?, ?, ?, ?)`, write.Batch.ID, write.RequestID, write.Fingerprint,
		write.Response, write.Batch.Revision, write.Event.OccurredAt.Format(time.RFC3339Nano))
	if err != nil {
		if isConstraint(err) {
			return domain.ErrIdempotency
		}
		return fmt.Errorf("保存幂等记录: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_events(event_id, batch_id, revision, request_id, actor_id, event_type, payload_digest, payload_json, occurred_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, write.Event.EventID, write.Event.BatchID, write.Event.Revision,
		write.Event.RequestID, write.Event.ActorID, write.Event.EventType, write.Event.PayloadDigest,
		write.Event.Payload, write.Event.OccurredAt.Format(time.RFC3339Nano))
	if err != nil {
		if isConstraint(err) {
			return domain.ErrIdempotency
		}
		return fmt.Errorf("保存审计事件: %w", err)
	}
	return nil
}

func isConstraint(err error) bool {
	if err == nil {
		return false
	}
	var sqliteError interface{ Code() int }
	if errors.As(err, &sqliteError) && sqliteError.Code() == 19 {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint") || strings.Contains(message, "unique")
}
