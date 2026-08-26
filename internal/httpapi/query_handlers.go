package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"bioacoustic-corpus-release/internal/service"
)

func (a *API) GetQualityHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := service.QualityHistoryFilter{Offset: 0, Limit: 100}
	var err error
	if q.Get("offset") != "" {
		filter.Offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil {
			writeError(w, badRequest("offset 参数错误"))
			return
		}
	}
	if q.Get("limit") != "" {
		filter.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			writeError(w, badRequest("limit 参数错误"))
			return
		}
	}
	if raw := q.Get("passed"); raw != "" {
		if raw != "true" && raw != "false" {
			writeError(w, badRequest("passed 参数须为 true 或 false"))
			return
		}
		value := raw == "true"
		filter.Passed = &value
	}
	if q.Get("min_revision") != "" {
		filter.MinRevision, err = strconv.ParseInt(q.Get("min_revision"), 10, 64)
		if err != nil {
			writeError(w, badRequest("min_revision 参数错误"))
			return
		}
	}
	if q.Get("max_revision") != "" {
		filter.MaxRevision, err = strconv.ParseInt(q.Get("max_revision"), 10, 64)
		if err != nil {
			writeError(w, badRequest("max_revision 参数错误"))
			return
		}
	}
	filter.IssueCode = q.Get("issue_code")
	filter.ClipID = q.Get("clip_id")
	filter.Stratum = q.Get("stratum")
	page, err := a.service.QualityHistory(r.Context(), r.PathValue("batchID"), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) GetQualityCheck(w http.ResponseWriter, r *http.Request) {
	sequence, err := strconv.Atoi(r.PathValue("sequence"))
	if err != nil {
		writeError(w, badRequest("sequence 路径参数错误"))
		return
	}
	record, err := a.service.QualityCheck(r.Context(), r.PathValue("batchID"), sequence)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (a *API) GetManifest(w http.ResponseWriter, r *http.Request) {
	offset, limit := 0, 1000
	var err error
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil {
			writeError(w, badRequest("offset 参数错误"))
			return
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			writeError(w, badRequest("limit 参数错误"))
			return
		}
	}
	page, e := a.service.GetManifestPage(r.Context(), r.PathValue("batchID"), offset, limit)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) GetAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	parse := func(k string) (int, error) {
		if q.Get(k) == "" {
			return 0, nil
		}
		return strconv.Atoi(q.Get(k))
	}
	offset, err := parse("offset")
	if err != nil {
		writeError(w, badRequest("offset 参数错误"))
		return
	}
	limit := 1000
	if q.Get("limit") != "" {
		limit, err = parse("limit")
		if err != nil {
			writeError(w, badRequest("limit 参数错误"))
			return
		}
	}
	minRev, err := parse("min_revision")
	if err != nil {
		writeError(w, badRequest("min_revision 参数错误"))
		return
	}
	maxRev, err := parse("max_revision")
	if err != nil {
		writeError(w, badRequest("max_revision 参数错误"))
		return
	}
	if q.Get("cursor") != "" {
		offset, err = strconv.Atoi(q.Get("cursor"))
		if err != nil {
			writeError(w, badRequest("cursor 参数错误"))
			return
		}
	}
	p, err := a.service.AuditPage(r.Context(), r.PathValue("batchID"), strings.TrimSpace(q.Get("actor_id")), strings.TrimSpace(q.Get("event_type")), minRev, maxRev, offset, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
