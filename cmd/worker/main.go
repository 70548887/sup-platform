package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hibiken/asynq"

	"github.com/70548887/sup-platform/internal/pkg/queue"
)

func main() {
	redisAddr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	concurrency := 10

	srv := queue.NewQueueServer(redisAddr, redisPassword, 0, concurrency)

	// 注册任务处理函数
	srv.HandleFunc(queue.TypeWebhookDeliver, handleWebhookDeliver)
	srv.HandleFunc(queue.TypeDockingSubmit, handleDockingSubmit)
	srv.HandleFunc(queue.TypeReconciliationRun, handleReconciliation)
	srv.HandleFunc(queue.TypeAnalyticsAggregate, handleAnalyticsAggregate)

	log.Printf("[INFO] Worker starting with concurrency=%d, redis=%s", concurrency, redisAddr)
	if err := srv.Run(); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

func handleWebhookDeliver(ctx context.Context, t *asynq.Task) error {
	var p queue.WebhookPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("webhook deliver: unmarshal: %w", err)
	}
	log.Printf("[TASK] webhook deliver: callback_id=%d url=%s", p.CallbackID, p.URL)
	// TODO: 实际投递逻辑（后续集成NotifyService）
	return nil
}

func handleDockingSubmit(ctx context.Context, t *asynq.Task) error {
	var p queue.DockingPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("docking submit: unmarshal: %w", err)
	}
	log.Printf("[TASK] docking submit: task_id=%d order_sn=%s", p.TaskID, p.OrderSN)
	// TODO: 实际提交逻辑
	return nil
}

func handleReconciliation(ctx context.Context, t *asynq.Task) error {
	var p queue.ReconciliationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("reconciliation: unmarshal: %w", err)
	}
	log.Printf("[TASK] reconciliation: type=%s", p.Type)
	// TODO: 实际对账逻辑
	return nil
}

func handleAnalyticsAggregate(ctx context.Context, t *asynq.Task) error {
	var p queue.AnalyticsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("analytics aggregate: unmarshal: %w", err)
	}
	log.Printf("[TASK] analytics aggregate: date=%s", p.Date)
	// TODO: 实际聚合逻辑
	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
