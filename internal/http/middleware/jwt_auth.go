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
// 1. 从Header Authorization: Bearer <token> 或 Cookie auth_token 提取token
// 2. 验证JWT签名和过期时间
// 3. 提取UserID和Role
// 4. 注入gin.Context
func JWTAuth(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从Header或Cookie提取token
		tokenString := extractToken(c)
		if tokenString == "" {
			response.AuthError(c, "缺少认证凭证")
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
		c.Set("tenant_id", claims.TenantID)

		c.Next()
	}
}

// JWTAuthWithRole JWT认证中间件（带角色校验）
// 只允许指定角色访问
func JWTAuthWithRole(authService *auth.AuthService, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header或Cookie提取token
		tokenString := extractToken(c)
		if tokenString == "" {
			response.AuthError(c, "缺少认证凭证")
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
		c.Set("tenant_id", claims.TenantID)

		c.Next()
	}
}

// extractToken 从Header或Cookie中提取token
// 优先级：Authorization Header > auth_token Cookie
func extractToken(c *gin.Context) string {
	// 优先从Authorization Header提取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			if token := strings.TrimSpace(parts[1]); token != "" {
				return token
			}
		}
	}

	// 回退：从Cookie提取
	if token, err := c.Cookie("auth_token"); err == nil && token != "" {
		return token
	}

	return ""
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
