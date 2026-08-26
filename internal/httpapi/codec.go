package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxRequestBytes int64 = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &requestError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type 必须为 application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var syntax *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntax):
			return badRequest(fmt.Sprintf("JSON 在字节 %d 附近格式错误", syntax.Offset))
		case errors.As(err, &typeError):
			return badRequest(fmt.Sprintf("字段 %s 类型错误", typeError.Field))
		case errors.Is(err, io.EOF):
			return badRequest("请求体不能为空")
		case strings.Contains(err.Error(), "unknown field"):
			return badRequest("请求包含未知字段")
		case strings.Contains(err.Error(), "request body too large"):
			return &requestError{Status: http.StatusRequestEntityTooLarge, Code: "body_too_large", Message: "请求体超过 1 MiB"}
		default:
			return badRequest("无法解析 JSON 请求体")
		}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return badRequest("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
