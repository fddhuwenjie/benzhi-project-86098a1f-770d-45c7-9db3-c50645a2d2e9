package annotation_progress_cache_scope_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/httpapi"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

type snapshotRepository struct {
	mu    sync.RWMutex
	batch *domain.CorpusBatch
}

func (r *snapshotRepository) replace(batch *domain.CorpusBatch) {
	r.mu.Lock()
	r.batch = batch
	r.mu.Unlock()
}

func (r *snapshotRepository) GetBatch(context.Context, string) (*domain.CorpusBatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.batch, nil
}

func (r *snapshotRepository) CreateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unexpected create")
}

func (r *snapshotRepository) UpdateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unexpected update")
}

func (r *snapshotRepository) FindCommand(context.Context, string, string) (*store.IdempotencyRecord, error) {
	return nil, errors.New("unexpected idempotency read")
}

func (r *snapshotRepository) ListAudit(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unexpected audit read")
}

func (r *snapshotRepository) Close() error { return nil }

func TestAnnotationProgressCacheIsScopedByActorAndRevision(t *testing.T) {
	initial := batchSnapshot(7, []domain.IndependentAnnotation{
		{ClipID: "clip-one", AnnotatorID: "annotator-alpha", Confidence: 0.8},
	})
	repository := &snapshotRepository{batch: initial}
	handler := httpapi.New(service.New(repository, time.Now)).Handler()

	alphaBefore := requestProgress(t, handler, "annotator-alpha")
	if alphaBefore.SubmittedCount != 1 {
		t.Fatalf("预置快照无效: %#v", alphaBefore)
	}

	repository.replace(batchSnapshot(8, []domain.IndependentAnnotation{
		{ClipID: "clip-one", AnnotatorID: "annotator-alpha", Confidence: 0.8},
		{ClipID: "clip-two", AnnotatorID: "annotator-alpha", Confidence: 0.9},
	}))
	alphaAfter := requestProgress(t, handler, "annotator-alpha")
	betaAfter := requestProgress(t, handler, "annotator-beta")

	if alphaAfter.SubmittedCount != 2 || len(alphaAfter.PendingClipIDs) != 0 ||
		betaAfter.ActorID != "annotator-beta" || betaAfter.SubmittedCount != 0 || len(betaAfter.PendingClipIDs) != 2 {
		t.Fatalf("progress snapshots crossed actor or revision boundary: alpha=%#v beta=%#v", alphaAfter, betaAfter)
	}
}

func requestProgress(t *testing.T, handler http.Handler, actorID string) service.AnnotationProgress {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/batches/batch-cache/annotations?actor_id="+actorID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("查询 %s 进度返回 %d: %s", actorID, response.Code, response.Body.String())
	}
	var progress service.AnnotationProgress
	if err := json.Unmarshal(response.Body.Bytes(), &progress); err != nil {
		t.Fatalf("解析 %s 进度: %v", actorID, err)
	}
	return progress
}

func batchSnapshot(revision int64, annotations []domain.IndependentAnnotation) *domain.CorpusBatch {
	return &domain.CorpusBatch{
		ID: "batch-cache", Revision: revision, Status: domain.StatusAnnotating,
		Clips: []domain.RecordingClip{
			{ID: "clip-one", Sampled: true},
			{ID: "clip-two", Sampled: true},
		},
		Annotations: annotations,
	}
}
