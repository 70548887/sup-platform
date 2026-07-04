package ratelimit

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const luaScript = `
-- KEYS[1] = bucket key
-- ARGV[1] = capacity (max tokens)
-- ARGV[2] = refill rate (tokens per second)
-- ARGV[3] = now (unix timestamp in milliseconds)
-- ARGV[4] = requested tokens (1)
-- Returns: {allowed(0/1), remaining, reset_at_ms}

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill = now
end

-- Refill tokens
local elapsed = (now - last_refill) / 1000
local refill = math.floor(elapsed * refill_rate)
if refill > 0 then
    tokens = math.min(capacity, tokens + refill)
    last_refill = now
end

local allowed = 0
local remaining = tokens

if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
    remaining = tokens
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('EXPIRE', key, 3600)

-- reset_at: time when bucket will be full again
local deficit = capacity - remaining
local reset_at = now + math.ceil(deficit / refill_rate * 1000)

return {allowed, remaining, reset_at}
`

var script = redis.NewScript(luaScript)

// RateLimiter Redis令牌桶限流器
type RateLimiter struct {
	client    *redis.Client
	scriptSHA string
}

// NewRateLimiter 创建限流器实例
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// LoadScript 将Lua脚本加载到Redis并缓存SHA
// 当client为nil时直接返回nil，表示降级不加载
func (r *RateLimiter) LoadScript(ctx context.Context) error {
	if r.client == nil {
		return nil
	}

	sha, err := script.Load(ctx, r.client).Result()
	if err != nil {
		return fmt.Errorf("ratelimit: load script failed: %w", err)
	}

	r.scriptSHA = sha
	return nil
}

// Allow 判断指定应用是否允许当前请求通过
// limit 为每分钟请求配额上限，<=0 时直接放行
func (r *RateLimiter) Allow(ctx context.Context, appID string, limit int) (allowed bool, remaining int, resetAt int64, err error) {
	if r.client == nil || limit <= 0 {
		return true, -1, 0, nil
	}

	key := fmt.Sprintf("ratelimit:%s", appID)
	capacity := limit
	refillRate := float64(limit) / 60.0
	now := time.Now().UnixMilli()
	requested := 1

	run := func(sha string) (interface{}, error) {
		return r.client.EvalSha(ctx, sha, []string{key}, capacity, refillRate, now, requested).Result()
	}

	res, err := run(r.scriptSHA)
	if err != nil {
		// EVALSHA失败(NOSCRIPT)时重新加载脚本并重试一次
		if strings.Contains(err.Error(), "NOSCRIPT") {
			if loadErr := r.LoadScript(ctx); loadErr != nil {
				return true, -1, 0, nil
			}
			res, err = run(r.scriptSHA)
		}
	}
	if err != nil {
		// 其他Redis错误降级放行，避免影响业务可用性
		return true, -1, 0, nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		log.Printf("[WARN] ratelimit: unexpected script response type: %T", res)
		return true, -1, 0, nil
	}

	allowedVal, ok1 := arr[0].(int64)
	remainingVal, ok2 := arr[1].(int64)
	resetAtVal, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		log.Printf("[WARN] ratelimit: unexpected script response values: %v", arr)
		return true, -1, 0, nil
	}

	return allowedVal == 1, int(remainingVal), resetAtVal, nil
}
