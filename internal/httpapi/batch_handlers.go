package httpapi

import (
	"encoding/json"
	"net/http"

	"bioacoustic-corpus-release/internal/service"
)

func (a *API) PreviewSample(w http.ResponseWriter, r *http.Request) {
	var quota map[string]int
	if r.Method == http.MethodPost {
		var in service.SampleInput
		if err := decodeJSON(w, r, &in); err != nil {
			writeError(w, err)
			return
		}
		quota = in.Quota
	} else if raw := r.URL.Query().Get("quota"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &quota); err != nil {
			writeError(w, badRequest("quota 参数格式错误"))
			return
		}
	}
	preview, err := a.service.PreviewSample(r.Context(), r.PathValue("batchID"), quota)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *API) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var input service.CreateBatchInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.CreateBatch(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (a *API) GetBatch(w http.ResponseWriter, r *http.Request) {
	response, err := a.service.GetBatch(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) AddClips(w http.ResponseWriter, r *http.Request) {
	var input service.AddClipsInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.AddClips(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) CorrectClips(w http.ResponseWriter, r *http.Request) {
	var input service.CorrectClipsInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.CorrectClips(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) WithdrawClips(w http.ResponseWriter, r *http.Request) {
	var input service.WithdrawClipsInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.WithdrawClips(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) SubmitDraft(w http.ResponseWriter, r *http.Request) {
	var input service.MetaInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.SubmitDraft(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) LockSample(w http.ResponseWriter, r *http.Request) {
	var input service.SampleInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	response, err := a.service.LockSample(r.Context(), r.PathValue("batchID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
