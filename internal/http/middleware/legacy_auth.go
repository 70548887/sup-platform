package middleware

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/pkg/signature"
)

const (
	// DefaultTimestampWindow 默认时间戳有效窗口（5分钟）
	DefaultTimestampWindow = 5 * time.Minute

	// ContextKeyUserID 注入Context的用户ID key
	ContextKeyUserID = "user_id"
	// ContextKeyAppID 注入Context的AppID key
	ContextKeyAppID = "app_id"
)

// LegacyAuthConfig Legacy认证中间件配置
type LegacyAuthConfig struct {
	TimestampWindow time.Duration    // 时间戳有效窗口
	RedisClient     *redis.Client    // Redis客户端（用于nonce防重放，可为nil表示降级）
	AuthService     *auth.AuthService // JWT认证服务（用于双认证模式，可为nil表示禁用JWT）
}

// LegacyAuth Legacy签名认证中间件（兼容JWT Bearer token）
// 验证流程：
// - 如果请求携带 Authorization: Bearer <token>，优先走JWT验证
// - 否则走传统Legacy签名验证：
//   1. 从Header提取 AppId, AppTimestamp, AppToken
//   2. 检查必填Header
//   3. 验证时间戳有效性
//   4. 查询数据库获取AppSecret
//   5. 验证应用状态
//   6. 验证IP白名单
//   7. 计算并对比签名
//   8. 注入UserID和AppID到Context
func LegacyAuth(db *gorm.DB, cfg *LegacyAuthConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = &LegacyAuthConfig{
			TimestampWindow: DefaultTimestampWindow,
		}
	}
	if cfg.TimestampWindow == 0 {
		cfg.TimestampWindow = DefaultTimestampWindow
	}

	authService := auth.NewAuthService(db, "", 0)

	return func(c *gin.Context) {
		// 优先检查JWT Bearer token（支持前端页面使用JWT登录）
		if cfg.AuthService != nil {
			if token := extractBearerToken(c); token != "" {
				claims, err := cfg.AuthService.VerifyJWT(token)
				if err == nil {
					c.Set(ContextKeyUserID, claims.UserID)
					c.Set(ContextKeyRole, claims.Role)
					c.Set("tenant_id", claims.TenantID)
					c.Next()
					return
				}
				// JWT验证失败，尝试Legacy认证（兼容旧客户端可能误带Authorization头）
			}
		}

		// 1. 提取Header（兼容大小写：Appid / AppId）
		appId := c.GetHeader("Appid")
		if appId == "" {
			appId = c.GetHeader("AppId")
		}
		if appId == "" {
			appId = c.GetHeader("appid")
		}

		appTimestamp := c.GetHeader("AppTimestamp")
		if appTimestamp == "" {
			appTimestamp = c.GetHeader("Apptimestamp")
		}

		appToken := c.GetHeader("AppToken")
		if appToken == "" {
			appToken = c.GetHeader("Apptoken")
		}

		// 读取可选的 AppNonce Header
		appNonce := c.GetHeader("AppNonce")
		if appNonce == "" {
			appNonce = c.GetHeader("Appnonce")
		}

		// 2. 检查必填Header
		if appId == "" || appTimestamp == "" || appToken == "" {
			response.AuthError(c, "缺少认证Header参数")
			c.Abort()
			return
		}

		// 3. 验证时间戳有效性
		ts, err := strconv.ParseInt(appTimestamp, 10, 64)
		if err != nil {
			response.AuthError(c, "时间戳格式错误")
			c.Abort()
			return
		}

		now := time.Now().Unix()
		diff := math.Abs(float64(now - ts))
		if diff > cfg.TimestampWindow.Seconds() {
			response.AuthError(c, "请求已过期")
			c.Abort()
			return
		}

		// 4. 查询数据库获取应用信息
		appInfo, err := authService.VerifyApiApp(appId)
		if err != nil {
			response.AuthError(c, "签名验证失败")
			c.Abort()
			return
		}

		// 5. 验证应用状态（必须为启用）
		if appInfo.Status != 1 {
			response.AuthError(c, "应用已禁用")
			c.Abort()
			return
		}

		// 6. 验证IP白名单
		if appInfo.IPWhitelist != "" {
			clientIP := c.ClientIP()
			if !authService.CheckIPWhitelist(clientIP, appInfo.IPWhitelist) {
				response.AuthError(c, "IP不在白名单内")
				c.Abort()
				return
			}
		}

		// 7. 计算签名并对比
		requestURI := c.Request.URL.Path

		if appNonce != "" {
			// 带 nonce 的签名验证
			if !signature.VerifyLegacyWithNonce(appId, appInfo.AppSecret, requestURI, appTimestamp, appNonce, strings.ToLower(appToken)) {
				response.AuthError(c, "签名验证失败")
				c.Abort()
				return
			}
			// 防重放检查
			if cfg.RedisClient != nil {
				nonceKey := fmt.Sprintf("legacy_nonce:%s:%s", appId, appNonce)
				ok, redisErr := cfg.RedisClient.SetNX(context.Background(), nonceKey, "1", 5*time.Minute).Result()
				if redisErr != nil {
					// Redis故障时降级：仅记录日志，不阻断请求
					log.Printf("[WARN] nonce check redis error: %v", redisErr)
				} else if !ok {
					response.AuthError(c, "请求重复(nonce已使用)")
					c.Abort()
					return
				}
			}
		} else {
			// 旧客户端不带 nonce，使用原有验证逻辑
			if !signature.VerifyLegacy(appId, appInfo.AppSecret, requestURI, appTimestamp, strings.ToLower(appToken)) {
				response.AuthError(c, "签名验证失败")
				c.Abort()
				return
			}
		}

		// 8. 认证成功，注入UserID和AppID到Context
		c.Set(ContextKeyUserID, appInfo.UserID)
		c.Set(ContextKeyAppID, appInfo.AppId)

		c.Next()
	}
}

// GetUserIDFromContext 从Context获取UserID
func GetUserIDFromContext(c *gin.Context) (uint, bool) {
	val, exists := c.Get(ContextKeyUserID)
	if !exists {
		return 0, false
	}
	userID, ok := val.(uint)
	return userID, ok
}

// GetAppIDFromContext 从Context获取AppID
func GetAppIDFromContext(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyAppID)
	if !exists {
		return "", false
	}
	appID, ok := val.(string)
	return appID, ok
}

// extractBearerToken 从Authorization Header提取Bearer token
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
