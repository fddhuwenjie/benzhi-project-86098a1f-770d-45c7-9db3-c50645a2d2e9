package manifest_cancel_cache_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/httpapi"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

type publishedRepository struct {
	batch *domain.CorpusBatch
}

func (r *publishedRepository) GetBatch(ctx context.Context, id string) (*domain.CorpusBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if id != r.batch.ID {
		return nil, domain.ErrNotFound
	}
	copy := *r.batch
	return &copy, nil
}

func (*publishedRepository) CreateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unexpected CreateCommand")
}

func (*publishedRepository) UpdateCommand(context.Context, store.CommandWrite) error {
	return errors.New("unexpected UpdateCommand")
}

func (*publishedRepository) FindCommand(context.Context, string, string) (*store.IdempotencyRecord, error) {
	return nil, errors.New("unexpected FindCommand")
}

func (*publishedRepository) ListAudit(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, errors.New("unexpected ListAudit")
}

func (*publishedRepository) Close() error { return nil }

func TestCanceledManifestRequestDoesNotPoisonLaterRequests(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	manifest := &domain.ReleaseManifest{
		BatchID:        "batch-cancel-cache",
		ReleaseVersion: "r9",
		ClipEntries:    []domain.ManifestClip{},
		GeneratedAt:    now,
	}
	digest, err := domain.ManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.SHA256Digest = digest
	repository := &publishedRepository{batch: &domain.CorpusBatch{
		ID:       manifest.BatchID,
		Status:   domain.StatusPublished,
		Revision: 9,
		Manifest: manifest,
	}}
	handler := httpapi.New(service.New(repository, func() time.Time { return now })).Handler()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	first := httptest.NewRequest(http.MethodGet, "/v1/batches/batch-cancel-cache/manifest", nil).WithContext(canceled)
	handler.ServeHTTP(httptest.NewRecorder(), first)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/batches/batch-cancel-cache/manifest", nil)
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"release_version":"r9"`) {
		t.Fatalf("取消请求污染了后续清单读取: status=%d body=%s", response.Code, response.Body.String())
	}
}
