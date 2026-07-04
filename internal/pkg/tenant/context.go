package tenant

import (
	"context"

	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	tenantIDKey  contextKey = "tenant_id"
	skipScopeKey contextKey = "skip_tenant_scope"
)

// SetTenantID 注入租户ID到Context
func SetTenantID(ctx context.Context, tenantID uint) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// GetTenantID 从Context提取租户ID
func GetTenantID(ctx context.Context) (uint, bool) {
	val := ctx.Value(tenantIDKey)
	if val == nil {
		return 0, false
	}
	id, ok := val.(uint)
	return id, ok
}

// TenantIDFromGin 从Gin Context获取tenant_id
func TenantIDFromGin(c *gin.Context) uint {
	val, exists := c.Get("tenant_id")
	if !exists {
		return 0
	}
	if id, ok := val.(uint); ok {
		return id
	}
	return 0
}

// WithSkipTenantScope 返回跳过Scope的Context（仅平台超级管理员使用）
func WithSkipTenantScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipScopeKey, true)
}

// ShouldSkipScope 检查是否应跳过租户Scope
func ShouldSkipScope(ctx context.Context) bool {
	val := ctx.Value(skipScopeKey)
	if val == nil {
		return false
	}
	skip, ok := val.(bool)
	return ok && skip
}
