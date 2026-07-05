package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = fmt.Errorf("cache miss")

// CacheProvider 统一缓存接口
type CacheProvider interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	GetOrLoad(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func() (interface{}, error)) error
}

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{client: client, prefix: prefix}
}

func (c *RedisCache) fullKey(key string) string {
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c.client == nil {
		return ErrCacheMiss
	}
	val, err := c.client.Get(ctx, c.fullKey(key)).Result()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return ErrCacheMiss // 降级：Redis不可用视为miss
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// TTL添加随机偏移防雪崩（±10%）
	jitter := time.Duration(rand.Int63n(int64(ttl)/5)) - ttl/10
	return c.client.Set(ctx, c.fullKey(key), data, ttl+jitter).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	if c.client == nil {
		return nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.fullKey(k)
	}
	return c.client.Del(ctx, fullKeys...).Err()
}

func (c *RedisCache) GetOrLoad(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func() (interface{}, error)) error {
	// 先查缓存
	if err := c.Get(ctx, key, dest); err == nil {
		return nil
	}
	// 缓存miss，调用loader
	result, err := loader()
	if err != nil {
		return err
	}
	// 写入缓存（忽略错误）
	_ = c.Set(ctx, key, result, ttl)
	// 将result赋值到dest
	data, _ := json.Marshal(result)
	return json.Unmarshal(data, dest)
}
