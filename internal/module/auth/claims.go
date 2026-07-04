package auth

import "github.com/golang-jwt/jwt/v5"

// Claims JWT自定义Claims
// TenantID: 租户ID，旧Token解析时默认为0（视为平台管理员）
type Claims struct {
	UserID   uint   `json:"user_id"`
	Role     string `json:"role"`      // admin / supplier / customer
	TenantID uint   `json:"tenant_id"` // 租户ID，0=平台超管
	jwt.RegisteredClaims
}
