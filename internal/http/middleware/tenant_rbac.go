package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	tenantModule "github.com/70548887/sup-platform/internal/module/tenant"
)

// TenantRBACMiddleware 租户级RBAC权限验证中间件
// 验证当前用户是否为该租户的Admin，且角色允许访问该资源
func TenantRBACMiddleware(tenantSvc *tenantModule.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取tenant_id
		var tenantID uint
		if tid, exists := c.Get("tenant_id"); exists {
			if id, ok := tid.(uint); ok {
				tenantID = id
			}
		}
		if tenantID == 0 {
			response.AuthError(c, "未关联租户")
			c.Abort()
			return
		}

		// 获取user_id
		var userID uint
		if uid, exists := c.Get(ContextKeyUserID); exists {
			if id, ok := uid.(uint); ok {
				userID = id
			}
		}
		if userID == 0 {
			response.AuthError(c, "未认证")
			c.Abort()
			return
		}

		// 检查权限
		resource := c.Request.URL.Path
		action := c.Request.Method
		if !tenantSvc.CheckAdminPermission(c.Request.Context(), tenantID, userID, resource, action) {
			response.AuthError(c, "权限不足")
			c.Abort()
			return
		}

		c.Next()
	}
}
