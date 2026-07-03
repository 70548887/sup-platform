package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构（兼容亿乐legacy格式）
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Error 业务错误
func Error(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    1,
		Message: message,
		Data:    nil,
	})
}

// ParamError 参数验证错误
func ParamError(c *gin.Context, field, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    2,
		Message: "参数验证错误",
		Data: map[string]string{
			"field":   field,
			"message": message,
		},
	})
}

// AuthError 授权错误
func AuthError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    100,
		Message: message,
		Data:    nil,
	})
}
