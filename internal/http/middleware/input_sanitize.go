package middleware

import (
	"html"
	"net/url"

	"github.com/gin-gonic/gin"
)

// InputSanitizeMiddleware 输入净化中间件
// 对查询参数中的字符串做HTML转义，防止XSS
func InputSanitizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 重建查询参数（URL.Query()返回副本，需要重新编码回RawQuery）
		query := c.Request.URL.Query()
		sanitized := make(url.Values)
		for key, values := range query {
			for _, v := range values {
				sanitized.Add(key, html.EscapeString(v))
			}
		}
		c.Request.URL.RawQuery = sanitized.Encode()
		c.Next()
	}
}
