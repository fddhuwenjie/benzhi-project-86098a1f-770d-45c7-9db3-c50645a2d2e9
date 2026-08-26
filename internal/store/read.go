package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bioacoustic-corpus-release/internal/domain"
)

func (s *SQLiteStore) GetBatch(ctx context.Context, id string) (*domain.CorpusBatch, error) {
	var snapshot []byte
	err := s.db.QueryRowContext(ctx, `SELECT snapshot_json FROM corpus_batches WHERE id = ?`, id).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取批次: %w", err)
	}
	return decodeBatch(snapshot)
}

func (s *SQLiteStore) FindCommand(ctx context.Context, batchID, requestID string) (*IdempotencyRecord, error) {
	record := &IdempotencyRecord{BatchID: batchID, RequestID: requestID}
	err := s.db.QueryRowContext(ctx, `
        SELECT fingerprint, response_json, revision
        FROM idempotency_records WHERE batch_id = ? AND request_id = ?`, batchID, requestID).
		Scan(&record.Fingerprint, &record.Response, &record.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取幂等记录: %w", err)
	}
	record.Response = append([]byte(nil), record.Response...)
	return record, nil
}
