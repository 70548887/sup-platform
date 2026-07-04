package audit

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// contextKeyUserID 与 http/middleware 中 ContextKeyUserID 保持一致
const contextKeyUserID = "user_id"

// isWriteMethod 判断是否为写操作
func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

// AuditMiddleware 审计中间件
// 只记录写操作(POST/PUT/DELETE/PATCH)的请求
// 从Gin Context获取userID（如果有JWT认证信息）
// 响应完成后异步记录
func AuditMiddleware(svc *AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只记录写操作
		if !isWriteMethod(c.Request.Method) {
			c.Next()
			return
		}

		// 先执行后续handler
		c.Next()

		// 响应完成后异步记录审计日志
		// 从Context获取userID（如果有JWT认证信息）
		var userID uint
		var username string
		if val, exists := c.Get(contextKeyUserID); exists {
			if uid, ok := val.(uint); ok {
				userID = uid
			}
		}
		// 尝试从Context获取username（如果存在）
		if val, exists := c.Get("username"); exists {
			if name, ok := val.(string); ok {
				username = name
			}
		}

		// 构造操作详情JSON
		detail := fmt.Sprintf(`{"method":"%s","path":"%s","status":%d}`,
			c.Request.Method, c.Request.URL.Path, c.Writer.Status())

		entry := &AuditLog{
			UserID:    userID,
			Username:  username,
			Action:    c.Request.Method,
			Resource:  c.Request.URL.Path,
			Detail:    detail,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
		}

		// 异步写入，不阻塞响应
		_ = svc.Log(context.Background(), entry)
	}
}
