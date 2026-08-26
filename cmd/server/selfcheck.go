package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bioacoustic-corpus-release/internal/domain"
	"bioacoustic-corpus-release/internal/service"
)

type selfCheckClient struct {
	baseURL string
	client  *http.Client
}

func runSelfCheck(ctx context.Context, address string) error {
	tempDir, err := os.MkdirTemp("", "bioacoustic-self-check-")
	if err != nil {
		return fmt.Errorf("创建自检目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	rt, err := newRuntime(ctx, filepath.Join(tempDir, "self-check.db"), address)
	if err != nil {
		return err
	}
	defer rt.close()
	listener, err := rt.listen()
	if err != nil {
		return err
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- rt.serve(listener) }()
	client := &selfCheckClient{
		baseURL: "http://" + listener.Addr().String(),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
	if err := client.completeWorkflow(ctx); err != nil {
		rt.server.Close()
		<-serveResult
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rt.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("关闭自检服务: %w", err)
	}
	if err := <-serveResult; err != nil {
		return err
	}
	return nil
}

func (c *selfCheckClient) completeWorkflow(ctx context.Context) error {
	batchID := "batch-self-check"
	create := map[string]any{
		"meta": meta("req-create", "admin-user", 0), "id": batchID,
		"title": "回环自检批次", "description": "通过真实 HTTP 端点执行完整发布闭环",
		"sampling_seed":      "stable-self-check-seed",
		"quality_thresholds": map[string]any{"min_completeness": 1.0, "min_agreement": 0.5, "min_stratum_coverage": 1.0},
	}
	response, err := c.command(ctx, http.MethodPost, "/v1/batches", create, http.StatusCreated)
	if err != nil || response.Batch.Revision != 1 {
		return checkResponse("创建批次", response, err, 1)
	}
	digestA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	clips := map[string]any{
		"meta": meta("req-clips", "admin-user", 1),
		"clips": []map[string]any{
			{"id": "clip-alpha", "source_uri": "s3://corpus/alpha.wav", "content_digest": digestA, "region_code": "CN-YN", "recorded_at": "2026-01-10T08:00:00Z", "duration_ms": 1200, "candidate_taxon": "Strix aluco"},
			{"id": "clip-beta", "source_uri": "s3://corpus/beta.wav", "content_digest": digestB, "region_code": "CN-YN", "recorded_at": "2026-01-11T08:00:00Z", "duration_ms": 1400, "candidate_taxon": "Strix aluco"},
		},
	}
	if response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/clips", clips, http.StatusOK); err != nil || response.Batch.Revision != 2 {
		return checkResponse("登记片段", response, err, 2)
	}
	if response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/submit", map[string]any{"meta": meta("req-submit", "admin-user", 2)}, http.StatusOK); err != nil || response.Batch.Revision != 3 {
		return checkResponse("提交草稿", response, err, 3)
	}
	quota := map[string]int{"CN-YN|2026-01|strix aluco": 2}
	if response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/sample", map[string]any{"meta": meta("req-sample", "admin-user", 3), "quota": quota}, http.StatusOK); err != nil || response.Batch.Revision != 4 || response.Batch.SampleCount != 2 {
		return checkResponse("锁定样本", response, err, 4)
	}
	annotations := []struct {
		request, actor, clip, label string
	}{
		{"req-ann-a1", "annotator-a", "clip-alpha", "Strix aluco"},
		{"req-ann-b1", "annotator-b", "clip-alpha", "Asio otus"},
		{"req-ann-a2", "annotator-a", "clip-beta", "Strix aluco"},
		{"req-ann-b2", "annotator-b", "clip-beta", "Strix aluco"},
	}
	for i, item := range annotations {
		payload := map[string]any{
			"meta": meta(item.request, item.actor, int64(4+i)), "clip_id": item.clip,
			"taxon_label": item.label, "confidence": 0.9, "evidence_note": "声谱图谐波和节律证据",
		}
		response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/annotations", payload, http.StatusOK)
		if err != nil || response.Batch.Revision != int64(5+i) {
			return checkResponse("提交双人标注", response, err, int64(5+i))
		}
		if i == 0 {
			if err := c.assertIsolation(ctx, batchID); err != nil {
				return err
			}
		}
	}
	if response.Batch.Status != domain.StatusPendingAdjudication || response.Batch.PendingConflictCount != 1 {
		return fmt.Errorf("双标完成后未生成一个待仲裁冲突")
	}
	decision := map[string]any{
		"meta": meta("req-adjudicate", "taxonomist", 8), "clip_id": "clip-alpha",
		"final_label": "Strix aluco", "reason": "参考叫声基频和地域分布裁定",
		"evidence_refs": []string{"spectrogram://clip-alpha#12-31", "taxonomy://strix-aluco"},
	}
	if response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/adjudications", decision, http.StatusOK); err != nil || response.Batch.Revision != 9 || response.Batch.Status != domain.StatusPendingQualityReview {
		return checkResponse("冲突仲裁", response, err, 9)
	}
	quality := map[string]any{"meta": meta("req-quality", "quality-owner", 9)}
	if response, err = c.command(ctx, http.MethodPost, "/v1/batches/"+batchID+"/quality-check", quality, http.StatusOK); err != nil || response.Batch.Revision != 10 || response.Batch.Status != domain.StatusPublished || response.Quality == nil || !response.Quality.Passed {
		return checkResponse("质量门禁", response, err, 10)
	}
	if err := c.assertRelease(ctx, batchID); err != nil {
		return err
	}
	return nil
}

func meta(requestID, actorID string, revision int64) map[string]any {
	return map[string]any{"request_id": requestID, "actor_id": actorID, "expected_revision": revision}
}

func (c *selfCheckClient) command(ctx context.Context, method, path string, payload any, expectedStatus int) (*service.CommandResponse, error) {
	var response service.CommandResponse
	if err := c.do(ctx, method, path, payload, expectedStatus, &response); err != nil {
		return &response, err
	}
	return &response, nil
}

func (c *selfCheckClient) do(ctx context.Context, method, path string, payload any, expectedStatus int, target any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expectedStatus {
		return fmt.Errorf("%s %s 返回 %d，期望 %d: %s", method, path, response.StatusCode, expectedStatus, body)
	}
	if target != nil && len(body) > 0 {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("解析 %s 响应: %w", path, err)
		}
	}
	return nil
}

func (c *selfCheckClient) assertIsolation(ctx context.Context, batchID string) error {
	var result struct {
		Annotations []domain.IndependentAnnotation `json:"annotations"`
	}
	path := "/v1/batches/" + batchID + "/clips/clip-alpha/annotations?actor_id=annotator-b"
	if err := c.do(ctx, http.MethodGet, path, nil, http.StatusOK, &result); err != nil {
		return err
	}
	if len(result.Annotations) != 0 {
		return fmt.Errorf("第二名标注员在提交前看到了第一份结果")
	}
	return nil
}

func (c *selfCheckClient) assertRelease(ctx context.Context, batchID string) error {
	var manifest domain.ReleaseManifest
	if err := c.do(ctx, http.MethodGet, "/v1/batches/"+batchID+"/manifest", nil, http.StatusOK, &manifest); err != nil {
		return err
	}
	if len(manifest.ClipEntries) != 2 || len(manifest.SHA256Digest) != 64 {
		return fmt.Errorf("发布清单条目或摘要无效")
	}
	var timeline struct {
		Events []domain.AuditEvent `json:"events"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/batches/"+batchID+"/audit", nil, http.StatusOK, &timeline); err != nil {
		return err
	}
	if len(timeline.Events) != 10 || timeline.Events[9].Revision != 10 {
		return fmt.Errorf("审计时间线不完整")
	}
	return nil
}

func checkResponse(stage string, response *service.CommandResponse, err error, expected int64) error {
	if err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	return fmt.Errorf("%s 后修订为 %d，期望 %d", stage, response.Batch.Revision, expected)
}
