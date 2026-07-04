package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/billing"
)

// QuotaCheckMiddleware 租户API配额检查中间件
func QuotaCheckMiddleware(billingService *billing.BillingService, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// enabled=false或billingService==nil时跳过
		if !enabled || billingService == nil {
			c.Next()
			return
		}

		// 从context获取tenant_id
		tenantID := uint(0)
		if tid, exists := c.Get("tenant_id"); exists {
			if id, ok := tid.(uint); ok {
				tenantID = id
			}
		}
		if tenantID == 0 {
			c.Next()
			return
		}

		// 检查配额
		allowed, remaining, err := billingService.CheckQuota(c.Request.Context(), tenantID)
		if err != nil {
			// 降级放行
			c.Next()
			return
		}

		// 设置响应头
		c.Header("X-Quota-Remaining", strconv.Itoa(remaining))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, response.Response{
				Code:    http.StatusTooManyRequests,
				Message: "API配额已用尽，请升级套餐",
				Data:    nil,
			})
			c.Abort()
			return
		}

		// 异步记录调用（不阻塞请求）
		go billingService.RecordAPICall(context.Background(), tenantID)

		c.Next()
	}
}
