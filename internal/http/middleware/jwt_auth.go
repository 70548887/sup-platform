package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/auth"
)

const (
	// ContextKeyRole 注入Context的角色 key
	ContextKeyRole = "role"
)

// JWTAuth JWT认证中间件
// 用于管理后台和供货商后台
// 验证流程：
// 1. 从Header Authorization: Bearer <token> 提取token
// 2. 验证JWT签名和过期时间
// 3. 提取UserID和Role
// 4. 注入gin.Context
func JWTAuth(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从Header提取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.AuthError(c, "缺少Authorization Header")
			c.Abort()
			return
		}

		// 检查Bearer前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.AuthError(c, "Authorization格式错误，应为: Bearer <token>")
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			response.AuthError(c, "token不能为空")
			c.Abort()
			return
		}

		// 2. 验证JWT签名和过期时间
		claims, err := authService.VerifyJWT(tokenString)
		if err != nil {
			response.AuthError(c, "token无效或已过期")
			c.Abort()
			return
		}

		// 3-4. 提取UserID和Role，注入Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyRole, claims.Role)

		c.Next()
	}
}

// JWTAuthWithRole JWT认证中间件（带角色校验）
// 只允许指定角色访问
func JWTAuthWithRole(authService *auth.AuthService, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先执行JWT认证
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.AuthError(c, "缺少Authorization Header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.AuthError(c, "Authorization格式错误，应为: Bearer <token>")
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			response.AuthError(c, "token不能为空")
			c.Abort()
			return
		}

		claims, err := authService.VerifyJWT(tokenString)
		if err != nil {
			response.AuthError(c, "token无效或已过期")
			c.Abort()
			return
		}

		// 校验角色
		roleAllowed := false
		for _, role := range allowedRoles {
			if claims.Role == role {
				roleAllowed = true
				break
			}
		}
		if !roleAllowed {
			response.AuthError(c, "权限不足")
			c.Abort()
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyRole, claims.Role)

		c.Next()
	}
}

// GetRoleFromContext 从Context获取Role
func GetRoleFromContext(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyRole)
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}
