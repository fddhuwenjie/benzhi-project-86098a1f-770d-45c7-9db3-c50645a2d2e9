package httpapi

import (
	"net/http"

	"bioacoustic-corpus-release/internal/service"
)

type API struct {
	service *service.Service
	mux     *http.ServeMux
}

func New(svc *service.Service) *API {
	api := &API{service: svc, mux: http.NewServeMux()}
	api.registerRoutes()
	return api
}

func (a *API) Handler() http.Handler {
	return securityHeaders(a.mux)
}

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /healthz", a.Health)
	a.mux.HandleFunc("POST /v1/batches", a.CreateBatch)
	a.mux.HandleFunc("GET /v1/batches/{batchID}", a.GetBatch)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/clips", a.AddClips)
	a.mux.HandleFunc("PATCH /v1/batches/{batchID}/clips", a.CorrectClips)
	a.mux.HandleFunc("DELETE /v1/batches/{batchID}/clips", a.WithdrawClips)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/submit", a.SubmitDraft)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/sample", a.LockSample)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/sample/preview", a.PreviewSample)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/sample-preview", a.PreviewSample)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/sample/preview", a.PreviewSample)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/annotations", a.SubmitAnnotation)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/annotations", a.GetAnnotationProgress)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/clips/{clipID}/annotations", a.GetAnnotations)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/conflicts", a.GetConflicts)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/adjudications", a.Adjudicate)
	a.mux.HandleFunc("POST /v1/batches/{batchID}/quality-check", a.RunQualityGate)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/quality-checks", a.GetQualityHistory)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/quality-checks/{sequence}", a.GetQualityCheck)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/manifest", a.GetManifest)
	a.mux.HandleFunc("GET /v1/batches/{batchID}/audit", a.GetAudit)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
