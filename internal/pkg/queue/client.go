package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
)

// QueueClient Asynq任务客户端
type QueueClient struct {
	client  *asynq.Client
	enabled bool
}

// NewQueueClient 创建队列客户端
// redisAddr为空或连接失败时，enabled=false（降级模式）
func NewQueueClient(redisAddr, password string, db int) *QueueClient {
	if redisAddr == "" {
		log.Printf("[WARN] Queue client disabled: no redis address")
		return &QueueClient{enabled: false}
	}
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})
	return &QueueClient{client: client, enabled: true}
}

// Enqueue 入队任务
// enabled=false时返回error（调用方需降级处理）
func (q *QueueClient) Enqueue(ctx context.Context, taskType string, payload interface{}, opts ...asynq.Option) error {
	if !q.enabled || q.client == nil {
		return fmt.Errorf("queue: client not enabled")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: marshal payload: %w", err)
	}
	task := asynq.NewTask(taskType, data, opts...)
	_, err = q.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("queue: enqueue %s: %w", taskType, err)
	}
	return nil
}

// IsEnabled 是否启用
func (q *QueueClient) IsEnabled() bool {
	return q.enabled
}

// Close 关闭客户端
func (q *QueueClient) Close() error {
	if q.client != nil {
		return q.client.Close()
	}
	return nil
}
