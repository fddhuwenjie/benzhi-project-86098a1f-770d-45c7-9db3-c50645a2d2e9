package annotation_error_chain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/httpapi"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

func TestAnnotationNotFoundPreservesDomainError(t *testing.T) {
	repo, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "annotation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := service.New(repo, nil)
	_, err = svc.CreateBatch(context.Background(), service.CreateBatchInput{
		Meta: service.CommandMeta{RequestID: "create", ActorID: "admin", ExpectedRevision: 0},
		ID:   "batch-errors", Title: "错误链测试", SamplingSeed: "seed",
		QualityThresholds: domain.QualityThresholds{MinCompleteness: 1, MinAgreement: 1, MinStratumCoverage: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	api := httpapi.New(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/batches/batch-errors/clips/clip-missing/annotations?actor_id=annotator", nil)
	resp := httptest.NewRecorder()
	api.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("未入样片段应返回 404 not_found，得到 %d，响应 %s", resp.Code, resp.Body.String())
	}
}
