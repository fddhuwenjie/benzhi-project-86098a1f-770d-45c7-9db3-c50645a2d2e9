package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("资源不存在")
	ErrRevisionConflict = errors.New("修订号冲突")
	ErrIdempotency      = errors.New("request_id 已用于不同命令")
	ErrInvalidState     = errors.New("当前状态不允许该操作")
	ErrPublished        = errors.New("已发布批次不可修改")
	ErrIntegrity        = errors.New("数据完整性校验失败")
)

// ValidationError 表示可归因于请求字段的业务校验失败。
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

func IsValidation(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}
