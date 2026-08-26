package httpapi

import (
	"errors"
	"net/http"

	"bioacoustic-corpus-release/internal/domain"
)

type requestError struct {
	Status  int
	Code    string
	Message string
	Field   string
}

func (e *requestError) Error() string { return e.Message }

func badRequest(message string) error {
	return &requestError{Status: http.StatusBadRequest, Code: "invalid_request", Message: message}
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "服务处理请求时发生内部错误"
	field := ""
	var req *requestError
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &req):
		status, code, message, field = req.Status, req.Code, req.Message, req.Field
	case errors.As(err, &validation):
		status, code, message, field = http.StatusUnprocessableEntity, "validation_failed", validation.Message, validation.Field
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "请求的资源不存在"
	case errors.Is(err, domain.ErrRevisionConflict):
		status, code, message = http.StatusConflict, "revision_conflict", "expected_revision 与当前修订不一致"
	case errors.Is(err, domain.ErrIdempotency):
		status, code, message = http.StatusConflict, "idempotency_conflict", "request_id 已用于不同命令"
	case errors.Is(err, domain.ErrPublished):
		status, code, message = http.StatusConflict, "batch_published", "批次已发布，业务数据不可修改"
	case errors.Is(err, domain.ErrInvalidState):
		status, code, message = http.StatusConflict, "invalid_state", err.Error()
	case errors.Is(err, domain.ErrIntegrity):
		status, code, message = http.StatusInternalServerError, "integrity_error", "持久化内容完整性校验失败"
	}
	body := errorBody{}
	body.Error.Code = code
	body.Error.Message = message
	body.Error.Field = field
	writeJSON(w, status, body)
}
