package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CSRFProtection CSRF保护中间件
// 仅应用于浏览器端路由（admin/tenant-admin），不应用于Open API路由
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// GET/HEAD/OPTIONS 请求跳过验证
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			// 为GET请求生成CSRF token并设置到Cookie
			token := generateCSRFToken()
			c.SetCookie("csrf_token", token, 3600, "/", "", false, false) // httpOnly=false，前端需要读取
			c.Next()
			return
		}

		// POST/PUT/DELETE/PATCH 请求验证CSRF token
		cookieToken, err := c.Cookie("csrf_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1002, "message": "missing csrf token"})
			return
		}

		headerToken := c.GetHeader("X-CSRF-Token")
		if headerToken == "" {
			headerToken = c.PostForm("_csrf")
		}

		if headerToken == "" || headerToken != cookieToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1002, "message": "invalid csrf token"})
			return
		}

		c.Next()
	}
}

// generateCSRFToken 生成随机CSRF token
func generateCSRFToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
