package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/config"
	"github.com/70548887/sup-platform/internal/pkg/tenant"
)

// TenantContextMiddleware 从认证信息中提取TenantID并注入到Context
// 用于所有路由组，确保后续DB操作自动带租户过滤
func TenantContextMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tenantID uint

		// 尝试从JWT claims中获取TenantID（如果JWT中间件已解析并注入）
		if tid, exists := c.Get("tenant_id"); exists {
			if id, ok := tid.(uint); ok {
				tenantID = id
			}
		}

		// 如果未获取到TenantID，使用默认值
		if tenantID == 0 {
			tenantID = cfg.MultiTenant.DefaultTenantID
			if tenantID == 0 {
				tenantID = 1 // 最终兜底
			}
		}

		// 注入到Gin Context
		c.Set("tenant_id", tenantID)

		// 注入到Request Context（用于GORM Scope）
		ctx := tenant.SetTenantID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
