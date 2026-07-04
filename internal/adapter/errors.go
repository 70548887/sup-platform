package adapter

import "fmt"

// SubmitError 适配器提交错误（分类：可判断是否应重试）
type SubmitError struct {
	Type      string // "network" | "timeout" | "business" | "auth"
	Retryable bool   // 是否应重试
	Message   string // 错误详情
	Code      int    // 上游错误码（如有）
}

func (e *SubmitError) Error() string {
	return fmt.Sprintf("adapter error [%s]: %s (retryable=%v)", e.Type, e.Message, e.Retryable)
}

// 预定义错误类型常量
const (
	ErrTypeNetwork  = "network"
	ErrTypeTimeout  = "timeout"
	ErrTypeBusiness = "business"
	ErrTypeAuth     = "auth"
)

// NewNetworkError 创建网络错误
func NewNetworkError(msg string) *SubmitError {
	return &SubmitError{Type: ErrTypeNetwork, Retryable: true, Message: msg}
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(msg string) *SubmitError {
	return &SubmitError{Type: ErrTypeTimeout, Retryable: true, Message: msg}
}

// NewBusinessError 创建业务错误（不可重试）
func NewBusinessError(code int, msg string) *SubmitError {
	return &SubmitError{Type: ErrTypeBusiness, Retryable: false, Message: msg, Code: code}
}

// NewAuthError 创建认证错误
func NewAuthError(msg string) *SubmitError {
	return &SubmitError{Type: ErrTypeAuth, Retryable: false, Message: msg}
}