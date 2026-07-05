package ratelimit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter_NilClient(t *testing.T) {
	rl := NewRateLimiter(nil)
	assert.NotNil(t, rl)
}

func TestRateLimiter_LoadScript_NilClient(t *testing.T) {
	rl := NewRateLimiter(nil)
	err := rl.LoadScript(context.Background())
	// nil client 应返回nil（降级模式）
	assert.NoError(t, err)
}

func TestRateLimiter_Allow_NilClient(t *testing.T) {
	rl := NewRateLimiter(nil)

	allowed, remaining, resetAt, err := rl.Allow(context.Background(), "app123", 100)
	// nil client 应直接放行
	assert.True(t, allowed)
	assert.Equal(t, -1, remaining)
	assert.Equal(t, int64(0), resetAt)
	assert.NoError(t, err)
}

func TestRateLimiter_Allow_ZeroLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	allowed, remaining, resetAt, err := rl.Allow(context.Background(), "app123", 0)
	// limit <= 0 直接放行
	assert.True(t, allowed)
	assert.Equal(t, -1, remaining)
	assert.Equal(t, int64(0), resetAt)
	assert.NoError(t, err)
}

func TestRateLimiter_Allow_NegativeLimit(t *testing.T) {
	rl := NewRateLimiter(nil)

	allowed, remaining, resetAt, err := rl.Allow(context.Background(), "app123", -10)
	assert.True(t, allowed)
	assert.Equal(t, -1, remaining)
	assert.Equal(t, int64(0), resetAt)
	assert.NoError(t, err)
}
