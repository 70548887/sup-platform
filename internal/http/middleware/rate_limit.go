package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/pkg/ratelimit"
)

// RateLimitMiddleware API限流中间件
// 依赖LegacyAuth等前置认证中间件注入的 app_id
func RateLimitMiddleware(limiter *ratelimit.RateLimiter, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 降级：limiter或db为nil时跳过
		if limiter == nil || db == nil {
			c.Next()
			return
		}

		// 从context获取app_id（LegacyAuth认证后注入）
		appID, exists := c.Get("app_id")
		if !exists {
			c.Next()
			return
		}

		appIDStr := fmt.Sprintf("%v", appID)

		// 从DB查询应用限流配额
		var rateLimit int
		err := db.Table("api_apps").Select("rate_limit").Where("app_id = ?", appIDStr).Scan(&rateLimit).Error
		if err != nil || rateLimit <= 0 {
			// 无配额配置或查询失败，默认不限流
			c.Next()
			return
		}

		allowed, remaining, resetAt, err := limiter.Allow(c.Request.Context(), appIDStr, rateLimit)
		if err != nil {
			// 限流服务异常，放行
			c.Next()
			return
		}

		// 设置响应头
		c.Header("X-RateLimit-Limit", strconv.Itoa(rateLimit))
		if remaining >= 0 {
			c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		}
		if resetAt > 0 {
			c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt/1000, 10))
		}

		if !allowed {
			c.JSON(http.StatusOK, response.Response{
				Code:    429,
				Message: "rate limit exceeded",
				Data:    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
