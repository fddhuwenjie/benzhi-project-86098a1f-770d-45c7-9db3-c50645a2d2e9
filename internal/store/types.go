package store

import (
	"context"
	"errors"

	"bioacoustic-corpus-release/internal/domain"
)

var ErrClosed = errors.New("仓储已关闭")

type IdempotencyRecord struct {
	BatchID     string
	RequestID   string
	Fingerprint string
	Response    []byte
	Revision    int64
}

type CommandWrite struct {
	Batch            *domain.CorpusBatch
	ExpectedRevision int64
	RequestID        string
	Fingerprint      string
	Response         []byte
	Event            domain.AuditEvent
}

type Repository interface {
	CreateCommand(context.Context, CommandWrite) error
	UpdateCommand(context.Context, CommandWrite) error
	FindCommand(context.Context, string, string) (*IdempotencyRecord, error)
	GetBatch(context.Context, string) (*domain.CorpusBatch, error)
	ListAudit(context.Context, string) ([]domain.AuditEvent, error)
	Close() error
}
