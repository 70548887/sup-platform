package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetAndGetTenantID(t *testing.T) {
	ctx := context.Background()

	// 初始没有tenant_id
	id, ok := GetTenantID(ctx)
	assert.False(t, ok)
	assert.Equal(t, uint(0), id)

	// 设置后可以获取
	ctx = SetTenantID(ctx, 42)
	id, ok = GetTenantID(ctx)
	assert.True(t, ok)
	assert.Equal(t, uint(42), id)
}

func TestGetTenantID_ZeroValue(t *testing.T) {
	ctx := SetTenantID(context.Background(), 0)
	id, ok := GetTenantID(ctx)
	assert.True(t, ok)
	assert.Equal(t, uint(0), id)
}

func TestWithSkipTenantScope_And_ShouldSkipScope(t *testing.T) {
	ctx := context.Background()

	// 默认不跳过
	assert.False(t, ShouldSkipScope(ctx))

	// 设置跳过后
	ctx = WithSkipTenantScope(ctx)
	assert.True(t, ShouldSkipScope(ctx))
}

func TestShouldSkipScope_NotSet(t *testing.T) {
	ctx := context.Background()
	assert.False(t, ShouldSkipScope(ctx))
}

func TestTenantID_ContextIsolation(t *testing.T) {
	ctx1 := SetTenantID(context.Background(), 1)
	ctx2 := SetTenantID(context.Background(), 2)

	id1, _ := GetTenantID(ctx1)
	id2, _ := GetTenantID(ctx2)

	assert.Equal(t, uint(1), id1)
	assert.Equal(t, uint(2), id2)
}

func TestSkipScope_ContextIsolation(t *testing.T) {
	ctx1 := WithSkipTenantScope(context.Background())
	ctx2 := context.Background()

	assert.True(t, ShouldSkipScope(ctx1))
	assert.False(t, ShouldSkipScope(ctx2))
}
