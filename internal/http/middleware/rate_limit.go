package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/pkg/ratelimit"
)

// RateLimitMiddleware API限流中间件
// 依赖LegacyAuth等前置认证中间件注入的 app_id
func RateLimitMiddleware(limiter *ratelimit.RateLimiter, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
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

		// 先查Redis缓存
		var rateLimitVal int
		cacheHit := false
		cacheKey := fmt.Sprintf("sup:ratelimit:config:%s", appIDStr)
		if redisClient != nil {
			val, err := redisClient.Get(c.Request.Context(), cacheKey).Int()
			if err == nil && val > 0 {
				rateLimitVal = val
				cacheHit = true
			}
			// 缓存未命中或Redis错误，继续查DB
		}

		// 缓存未命中时走DB查询
		if !cacheHit {
			err := db.Table("api_apps").Select("rate_limit").Where("app_id = ?", appIDStr).Scan(&rateLimitVal).Error
			if err != nil || rateLimitVal <= 0 {
				// 无配额配置或查询失败，默认不限流
				c.Next()
				return
			}
			// 查到后回写Redis缓存（TTL 5分钟）
			if redisClient != nil && rateLimitVal > 0 {
				redisClient.Set(c.Request.Context(), cacheKey, rateLimitVal, 5*time.Minute)
			}
		}

		allowed, remaining, resetAt, err := limiter.Allow(c.Request.Context(), appIDStr, rateLimitVal)
		if err != nil {
			// 限流服务异常，放行
			c.Next()
			return
		}

		// 设置响应头
		c.Header("X-RateLimit-Limit", strconv.Itoa(rateLimitVal))
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
