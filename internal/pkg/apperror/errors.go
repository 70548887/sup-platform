package apperror

import "fmt"

// BusinessError 统一业务错误
type BusinessError struct {
	Code       int    // 业务错误码
	Message    string // 错误消息
	HTTPStatus int    // HTTP状态码
}

func (e *BusinessError) Error() string {
	return e.Message
}

// New 创建业务错误
func New(code int, message string, httpStatus int) *BusinessError {
	return &BusinessError{Code: code, Message: message, HTTPStatus: httpStatus}
}

// Is 支持 errors.Is 比较
func (e *BusinessError) Is(target error) bool {
	t, ok := target.(*BusinessError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Wrap 包装底层错误为业务错误
func Wrap(err error, bizErr *BusinessError) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", bizErr, err)
}
