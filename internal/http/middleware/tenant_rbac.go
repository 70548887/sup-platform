package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

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
			c.JSON(http.StatusForbidden, gin.H{
				"code":    100,
				"message": "未关联租户",
				"data":    nil,
			})
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
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    100,
				"message": "未认证",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 检查权限
		resource := c.Request.URL.Path
		action := c.Request.Method
		if !tenantSvc.CheckAdminPermission(c.Request.Context(), tenantID, userID, resource, action) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    100,
				"message": "权限不足",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
