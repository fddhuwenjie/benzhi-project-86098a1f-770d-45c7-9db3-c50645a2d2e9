package service

import (
	"sync"
	"time"

	"bioacoustic-corpus-release/internal/audit"
	"bioacoustic-corpus-release/internal/store"
)

type Service struct {
	repository  store.Repository
	auditor     *audit.Builder
	auditReader *audit.Reader
	locks       *batchLocks
	progressMu  sync.RWMutex
	progress    map[string]*AnnotationProgress
	now         func() time.Time
}

func New(repository store.Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		repository: repository, auditor: audit.NewBuilder(now),
		auditReader: audit.NewReader(repository), locks: newBatchLocks(),
		progress: make(map[string]*AnnotationProgress), now: now,
	}
}
