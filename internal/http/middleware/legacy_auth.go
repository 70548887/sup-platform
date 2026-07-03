package middleware

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	TimestampWindow time.Duration // 时间戳有效窗口
}

// LegacyAuth Legacy签名认证中间件
// 验证流程：
// 1. 从Header提取 AppId, AppTimestamp, AppToken
// 2. 检查必填Header
// 3. 验证时间戳有效性
// 4. 查询数据库获取AppSecret
// 5. 验证应用状态
// 6. 验证IP白名单
// 7. 计算并对比签名
// 8. 注入UserID和AppID到Context
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

		// 7. 计算签名并对比（使用ConstantTimeCompare防时序攻击）
		requestURI := c.Request.URL.Path
		// 统一token为小写进行对比
		if !signature.VerifyLegacy(appId, appInfo.AppSecret, requestURI, appTimestamp, strings.ToLower(appToken)) {
			response.AuthError(c, "签名验证失败")
			c.Abort()
			return
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
