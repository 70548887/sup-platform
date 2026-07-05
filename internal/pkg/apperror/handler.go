package apperror

import (
	"errors"

	"github.com/gin-gonic/gin"
)

// ErrorHandler Gin中间件，将panic和error统一转换为JSON响应
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 如果已经有response写入，不再处理
		if c.Writer.Written() {
			return
		}

		// 检查是否有错误设置
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			var bizErr *BusinessError
			if errors.As(err, &bizErr) {
				c.JSON(bizErr.HTTPStatus, gin.H{
					"code":    bizErr.Code,
					"message": bizErr.Message,
				})
			} else {
				c.JSON(500, gin.H{
					"code":    1005,
					"message": "internal server error",
				})
			}
		}
	}
}
