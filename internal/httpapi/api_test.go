package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

func testAPI(t *testing.T) *API {
	t.Helper()
	repository, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repository.Close() })
	return New(service.New(repository, time.Now))
}

func TestStrictJSONAndContentType(t *testing.T) {
	api := testAPI(t)
	unknown := `{"meta":{"request_id":"request-a","actor_id":"actor-a","expected_revision":0},"id":"batch-api","title":"API 批次","sampling_seed":"seed","quality_thresholds":{"min_completeness":1,"min_agreement":1,"min_stratum_coverage":1},"unexpected":true}`
	request := httptest.NewRequest(http.MethodPost, "/v1/batches", strings.NewReader(unknown))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_request") {
		t.Fatalf("未知字段响应不正确: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/batches", strings.NewReader(`{}`))
	response = httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("缺少 Content-Type 应返回 415，得到 %d", response.Code)
	}
}
