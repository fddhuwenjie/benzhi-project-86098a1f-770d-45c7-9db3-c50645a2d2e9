package httpapi

import (
	"net/http"
	"strings"

	"bioacoustic-corpus-release/internal/service"
)

func (a *API) SubmitAnnotation(w http.ResponseWriter, r *http.Request) {
	var input service.AnnotationHTTPInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	var response *service.CommandResponse
	var err error
	if input.Annotations != nil {
		if input.ClipID != "" || input.TaxonLabel != "" || input.EvidenceNote != "" {
			writeError(w, badRequest("单条标注字段与 annotations 不可同时提供"))
			return
		}
		response, err = a.service.SubmitAnnotations(r.Context(), r.PathValue("batchID"), service.AnnotationDeliveryInput{Meta: input.Meta, Annotations: input.Annotations})
	} else {
		response, err = a.service.SubmitAnnotation(r.Context(), r.PathValue("batchID"), service.AnnotationInput{
			Meta: input.Meta, ClipID: input.ClipID, TaxonLabel: input.TaxonLabel,
			Confidence: input.Confidence, EvidenceNote: input.EvidenceNote,
		})
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) GetAnnotations(w http.ResponseWriter, r *http.Request) {
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	if actorID == "" {
		writeError(w, badRequest("actor_id 查询参数不能为空"))
		return
	}
	if r.URL.Query().Get("progress") == "true" {
		progress, err := a.service.AnnotationProgress(r.Context(), r.PathValue("batchID"), actorID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, progress)
		return
	}
	annotations, err := a.service.GetAnnotations(r.Context(), r.PathValue("batchID"), r.PathValue("clipID"), actorID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"annotations": annotations})
}

func (a *API) GetAnnotationProgress(w http.ResponseWriter, r *http.Request) {
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	if actorID == "" {
		writeError(w, badRequest("actor_id 查询参数不能为空"))
		return
	}
	p, err := a.service.AnnotationProgress(r.Context(), r.PathValue("batchID"), actorID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) GetConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts, err := a.service.PendingConflicts(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	if stratum := r.URL.Query().Get("stratum"); stratum != "" {
		filtered := conflicts[:0]
		for _, c := range conflicts {
			if c.Stratum == stratum {
				filtered = append(filtered, c)
			}
		}
		conflicts = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

func (a *API) Adjudicate(w http.ResponseWriter, r *http.Request) {
	var input service.AdjudicationInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.Adjudicate(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) RunQualityGate(w http.ResponseWriter, r *http.Request) {
	var input service.MetaInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.RunQualityGate(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
