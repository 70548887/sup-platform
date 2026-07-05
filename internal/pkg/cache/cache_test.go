package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisCache_NilClient_GetReturnsCacheMiss(t *testing.T) {
	c := NewRedisCache(nil, "test")
	var dest string
	err := c.Get(context.Background(), "anykey", &dest)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_NilClient_SetReturnsNil(t *testing.T) {
	c := NewRedisCache(nil, "test")
	err := c.Set(context.Background(), "key", "value", time.Minute)
	assert.NoError(t, err)
}

func TestRedisCache_NilClient_DelReturnsNil(t *testing.T) {
	c := NewRedisCache(nil, "test")
	err := c.Del(context.Background(), "key1", "key2")
	assert.NoError(t, err)
}

func TestRedisCache_NilClient_GetOrLoad_CallsLoader(t *testing.T) {
	c := NewRedisCache(nil, "test")

	loaderCalled := false
	var dest map[string]string

	err := c.GetOrLoad(context.Background(), "key", &dest, time.Minute, func() (interface{}, error) {
		loaderCalled = true
		return map[string]string{"hello": "world"}, nil
	})

	require.NoError(t, err)
	assert.True(t, loaderCalled)
	assert.Equal(t, "world", dest["hello"])
}

func TestRedisCache_NilClient_GetOrLoad_LoaderError(t *testing.T) {
	c := NewRedisCache(nil, "test")

	var dest string
	err := c.GetOrLoad(context.Background(), "key", &dest, time.Minute, func() (interface{}, error) {
		return nil, assert.AnError
	})

	assert.Error(t, err)
}

func TestRedisCache_FullKey(t *testing.T) {
	c := NewRedisCache(nil, "myprefix")
	key := c.fullKey("somekey")
	assert.Equal(t, "myprefix:somekey", key)
}

func TestRedisCache_GetOrLoad_ResultSerialization(t *testing.T) {
	c := NewRedisCache(nil, "test")

	type Item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var dest Item
	err := c.GetOrLoad(context.Background(), "item", &dest, time.Minute, func() (interface{}, error) {
		return Item{ID: 42, Name: "test-item"}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 42, dest.ID)
	assert.Equal(t, "test-item", dest.Name)
}

func TestErrCacheMiss_Message(t *testing.T) {
	assert.Equal(t, "cache miss", ErrCacheMiss.Error())
}

func TestRedisCache_GetOrLoad_ComplexType(t *testing.T) {
	c := NewRedisCache(nil, "test")

	var dest []int
	err := c.GetOrLoad(context.Background(), "list", &dest, time.Minute, func() (interface{}, error) {
		return []int{1, 2, 3, 4, 5}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, dest)
}

func TestRedisCache_GetOrLoad_JsonCompatibility(t *testing.T) {
	// Verify the loader result can survive JSON marshal/unmarshal cycle
	c := NewRedisCache(nil, "test")

	type Nested struct {
		Tags []string `json:"tags"`
	}

	var dest Nested
	err := c.GetOrLoad(context.Background(), "nested", &dest, time.Minute, func() (interface{}, error) {
		return Nested{Tags: []string{"a", "b", "c"}}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, dest.Tags)

	// Verify serialization round-trip
	data, _ := json.Marshal(dest)
	var dest2 Nested
	_ = json.Unmarshal(data, &dest2)
	assert.Equal(t, dest, dest2)
}
