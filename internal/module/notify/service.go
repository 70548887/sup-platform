package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/pkg/queue"
)

// 指数退避重试间隔: 30s, 2min, 8min, 32min, 2h
var retryDelays = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	8 * time.Minute,
	32 * time.Minute,
	2 * time.Hour,
}

const (
	maxRetryCount = 5
	httpTimeout   = 10 * time.Second
)

// NotifyService 通知投递服务
type NotifyService struct {
	repo        *NotifyRepository
	db          *gorm.DB
	queueClient *queue.QueueClient
}

// NewNotifyService 创建NotifyService
func NewNotifyService(db *gorm.DB) *NotifyService {
	return &NotifyService{
		repo: NewNotifyRepository(db),
		db:   db,
	}
}

// SetQueueClient 设置队列客户端
func (s *NotifyService) SetQueueClient(qc *queue.QueueClient) {
	s.queueClient = qc
}

// callbackPayload 通知负载
type callbackPayload struct {
	OrderSN   string `json:"order_sn"`
	Status    int8   `json:"status"`
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
}

// SendOrderCallback 构造payload并异步投递通知
// 1. 如果order.NotifyURL为空则跳过
// 2. 构造payload JSON（订单号、状态、事件类型、时间戳）
// 3. 创建OrderCallback记录（初始Success=false）
// 4. 启动goroutine执行HTTP POST投递
func (s *NotifyService) SendOrderCallback(ctx context.Context, ord *order.Order, eventType string) error {
	// 如果没有配置通知地址，跳过
	if ord.NotifyURL == "" {
		return nil
	}

	// 构造payload
	payload := callbackPayload{
		OrderSN:   ord.OrderSN,
		Status:    ord.Status,
		Event:     eventType,
		Timestamp: time.Now().Unix(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal payload failed: %w", err)
	}

	// 创建OrderCallback记录（初始Success=false）
	cb := &order.OrderCallback{
		OrderID: ord.ID,
		URL:     ord.NotifyURL,
		Payload: string(payloadBytes),
		Success: false,
	}
	if err := s.repo.CreateCallback(ctx, cb); err != nil {
		return fmt.Errorf("notify: create callback record failed: %w", err)
	}

	// 优先使用队列
	if s.queueClient != nil && s.queueClient.IsEnabled() {
		qPayload := queue.WebhookPayload{CallbackID: cb.ID, URL: ord.NotifyURL, Body: string(payloadBytes)}
		if enqErr := s.queueClient.Enqueue(ctx, queue.TypeWebhookDeliver, qPayload); enqErr == nil {
			return nil // 入队成功，不用goroutine
		} else {
			// 入队失败，降级到goroutine
			log.Printf("[WARN] notify: enqueue failed, fallback to goroutine: %v", enqErr)
		}
	}

	// 启动goroutine异步投递
	go s.deliverCallback(cb.ID, ord.NotifyURL, string(payloadBytes))

	return nil
}

// deliverCallback 执行HTTP POST投递
// 1. HTTP POST到url，超时10秒
// 2. 记录StatusCode和Response
// 3. 如果失败且RetryCount < 5，计算下次重试时间（指数退避）
// 4. 更新OrderCallback记录
func (s *NotifyService) deliverCallback(callbackID uint, url string, payload string) {
	ctx := context.Background()

	// 查询当前记录
	cb, err := s.repo.GetCallbackByID(ctx, callbackID)
	if err != nil {
		log.Printf("[ERROR] notify: get callback %d failed: %v", callbackID, err)
		return
	}

	// 执行HTTP POST
	statusCode, respBody, err := s.doHTTPPost(url, payload)

	// 记录结果
	cb.StatusCode = statusCode
	cb.Response = respBody

	if err == nil && statusCode >= 200 && statusCode < 300 {
		// 投递成功
		cb.Success = true
		log.Printf("[INFO] notify: callback %d delivered successfully (status=%d)", callbackID, statusCode)
	} else {
		// 投递失败
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = fmt.Sprintf("unexpected status code: %d", statusCode)
		}
		log.Printf("[WARN] notify: callback %d delivery failed: %s", callbackID, errMsg)

		cb.RetryCount++
		if cb.RetryCount < maxRetryCount {
			// 计算下次重试时间（指数退避）
			delayIdx := cb.RetryCount - 1
			if delayIdx >= len(retryDelays) {
				delayIdx = len(retryDelays) - 1
			}
			nextRetry := time.Now().Add(retryDelays[delayIdx]).Unix()
			cb.NextRetryAt = &nextRetry
		}
	}

	// 更新记录
	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		log.Printf("[ERROR] notify: update callback %d failed: %v", callbackID, err)
	}
}

// doHTTPPOST 执行HTTP POST请求
func (s *NotifyService) doHTTPPost(url string, payload string) (int, string, error) {
	httpClient := &http.Client{
		Timeout: httpTimeout,
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(payload))
	if err != nil {
		return 0, "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("read response failed: %w", err)
	}

	// 限制响应体存储大小（最多4KB）
	respStr := string(body)
	if len(respStr) > 4096 {
		respStr = respStr[:4096]
	}

	return resp.StatusCode, respStr, nil
}

// RetryPendingCallbacks 重试所有待重试的通知
// 查询所有 Success=false AND RetryCount<5 AND NextRetryAt<=now 的记录，逐个重试投递
func (s *NotifyService) RetryPendingCallbacks(ctx context.Context) error {
	callbacks, err := s.repo.GetPendingCallbacks(ctx)
	if err != nil {
		return fmt.Errorf("notify: get pending callbacks failed: %w", err)
	}

	for _, cb := range callbacks {
		// 逐个重试投递
		s.redeliverCallback(cb)
	}

	return nil
}

// redeliverCallback 重试投递单个通知
func (s *NotifyService) redeliverCallback(cb *order.OrderCallback) {
	ctx := context.Background()

	// 执行HTTP POST
	statusCode, respBody, err := s.doHTTPPost(cb.URL, cb.Payload)

	// 记录结果
	cb.StatusCode = statusCode
	cb.Response = respBody

	if err == nil && statusCode >= 200 && statusCode < 300 {
		cb.Success = true
		log.Printf("[INFO] notify: callback %d retry delivered successfully (status=%d, retries=%d)",
			cb.ID, statusCode, cb.RetryCount)
	} else {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = fmt.Sprintf("unexpected status code: %d", statusCode)
		}
		log.Printf("[WARN] notify: callback %d retry failed (retries=%d): %s",
			cb.ID, cb.RetryCount, errMsg)

		cb.RetryCount++
		if cb.RetryCount < maxRetryCount {
			delayIdx := cb.RetryCount - 1
			if delayIdx >= len(retryDelays) {
				delayIdx = len(retryDelays) - 1
			}
			nextRetry := time.Now().Add(retryDelays[delayIdx]).Unix()
			cb.NextRetryAt = &nextRetry
		} else {
			// 达到最大重试次数，清空NextRetryAt避免无限查询
			cb.NextRetryAt = nil
			log.Printf("[ERROR] notify: callback %d reached max retries (%d), giving up",
				cb.ID, cb.RetryCount)
		}
	}

	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		log.Printf("[ERROR] notify: update callback %d after retry failed: %v", cb.ID, err)
	}
}
