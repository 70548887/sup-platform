package docking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/adapter"
)

// 指数退避重试间隔: 30s, 2min, 8min, 32min, 2h
var retryDelays = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	8 * time.Minute,
	32 * time.Minute,
	2 * time.Hour,
}

// DockingService 订单对接服务
type DockingService struct {
	repo    *DockingRepository
	db      *gorm.DB
	factory *adapter.Factory
}

// NewDockingService 创建DockingService
func NewDockingService(db *gorm.DB, factory *adapter.Factory) *DockingService {
	return &DockingService{
		repo:    NewDockingRepository(db),
		db:      db,
		factory: factory,
	}
}

// SubmitOrder 创建对接任务并异步提交订单到上游供货商
// 幂等: 如果同一OrderID已存在任务则直接返回nil
func (s *DockingService) SubmitOrder(ctx context.Context, orderID uint, supplierID uint, orderSN string, params adapter.SubmitParams) error {
	// 序列化请求参数
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("docking: marshal request payload failed: %w", err)
	}

	// 创建任务记录
	task := &OrderDockingTask{
		OrderID:        orderID,
		SupplierID:     supplierID,
		Status:         TaskPending,
		MaxRetry:       5,
		RequestPayload: string(payloadBytes),
	}

	if err := s.repo.Create(ctx, task); err != nil {
		// 唯一索引冲突说明已存在，幂等返回nil
		if isDuplicateError(err) {
			return nil
		}
		return fmt.Errorf("docking: create task failed: %w", err)
	}

	// 启动goroutine异步执行提交
	go s.executeSubmit(task.ID, orderSN)

	return nil
}

// executeSubmit 执行单次提交逻辑
func (s *DockingService) executeSubmit(taskID uint, orderSN string) {
	ctx := context.Background()

	// 获取任务
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[ERROR] docking: get task %d failed: %v", taskID, err)
		return
	}

	// CAS锁定: pending → locked
	locked, err := s.repo.LockTask(ctx, taskID)
	if err != nil {
		log.Printf("[ERROR] docking: lock task %d failed: %v", taskID, err)
		return
	}
	if !locked {
		// 已被其他协程锁定或状态不对，跳过
		log.Printf("[WARN] docking: task %d lock failed (already locked or not pending)", taskID)
		return
	}

	// 获取适配器
	adp, err := s.factory.Get(task.SupplierID)
	if err != nil {
		s.handleFailure(ctx, task, fmt.Sprintf("get adapter failed: %v", err))
		return
	}

	// 反序列化请求参数
	var params adapter.SubmitParams
	if err := json.Unmarshal([]byte(task.RequestPayload), &params); err != nil {
		s.handleFailure(ctx, task, fmt.Sprintf("unmarshal request payload failed: %v", err))
		return
	}

	// 调用上游提交
	result, submitErr := adp.SubmitOrder(ctx, orderSN, params)

	if submitErr != nil {
		// 提交失败
		errMsg := submitErr.Error()
		s.handleFailure(ctx, task, errMsg)
		return
	}

	// 提交成功
	s.handleSuccess(ctx, task, result)
}

// handleSuccess 处理提交成功
func (s *DockingService) handleSuccess(ctx context.Context, task *OrderDockingTask, result *adapter.SubmitResult) {
	now := time.Now().Unix()
	task.Status = TaskSubmitted
	task.ExternalOrderID = result.ExternalOrderID
	task.SubmittedAt = &now

	// 序列化响应
	respBytes, _ := json.Marshal(result)
	task.ResponsePayload = string(respBytes)

	if err := s.repo.UpdateAfterSubmit(ctx, task); err != nil {
		log.Printf("[ERROR] docking: update task %d after submit failed: %v", task.ID, err)
		return
	}

	log.Printf("[INFO] docking: task %d submitted successfully (externalOrderID=%s)", task.ID, result.ExternalOrderID)
}

// handleFailure 处理提交失败
func (s *DockingService) handleFailure(ctx context.Context, task *OrderDockingTask, errMsg string) {
	now := time.Now().Unix()
	task.RetryCount++
	task.ErrorMessage = errMsg
	task.LastFailureAt = &now

	if task.RetryCount >= task.MaxRetry {
		// 超过最大重试次数，标记为失败
		task.Status = TaskFailed
		task.NextRetryAt = nil
		log.Printf("[ERROR] docking: task %d reached max retries (%d), marked as failed: %s",
			task.ID, task.MaxRetry, errMsg)
	} else {
		// 回退为pending，计算下次重试时间（指数退避）
		task.Status = TaskPending
		delayIdx := task.RetryCount - 1
		if delayIdx >= len(retryDelays) {
			delayIdx = len(retryDelays) - 1
		}
		nextRetry := time.Now().Add(retryDelays[delayIdx]).Unix()
		task.NextRetryAt = &nextRetry
		log.Printf("[WARN] docking: task %d submit failed (retry=%d/%d), next retry at %v: %s",
			task.ID, task.RetryCount, task.MaxRetry, time.Unix(nextRetry, 0).Format("15:04:05"), errMsg)
	}

	if err := s.repo.UpdateAfterFailure(ctx, task); err != nil {
		log.Printf("[ERROR] docking: update task %d after failure failed: %v", task.ID, err)
	}
}

// RetryPendingTasks 重试所有到期的待提交任务
func (s *DockingService) RetryPendingTasks(ctx context.Context) error {
	tasks, err := s.repo.GetPendingTasks(ctx)
	if err != nil {
		return fmt.Errorf("docking: get pending tasks failed: %w", err)
	}

	for _, task := range tasks {
		// 反序列化获取orderSN（从params中取CustomerOrderID作为orderSN）
		var params adapter.SubmitParams
		if err := json.Unmarshal([]byte(task.RequestPayload), &params); err != nil {
			log.Printf("[ERROR] docking: unmarshal task %d payload failed: %v", task.ID, err)
			continue
		}
		s.executeSubmit(task.ID, params.CustomerOrderID)
	}

	return nil
}

// ManualRetry 人工重试失败任务
func (s *DockingService) ManualRetry(ctx context.Context, taskID uint) error {
	// CAS: failed → pending, retry_count=0
	if err := s.repo.ResetForManualRetry(ctx, taskID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("docking: task %d not found or not in failed status", taskID)
		}
		return fmt.Errorf("docking: reset task %d for manual retry failed: %w", taskID, err)
	}

	// 获取任务信息以取得orderSN
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("docking: get task %d failed: %w", taskID, err)
	}

	var params adapter.SubmitParams
	if err := json.Unmarshal([]byte(task.RequestPayload), &params); err != nil {
		return fmt.Errorf("docking: unmarshal task %d payload failed: %w", taskID, err)
	}

	// 启动goroutine触发一次提交
	go s.executeSubmit(taskID, params.CustomerOrderID)

	return nil
}

// GetFailedTasks 分页获取失败任务列表
func (s *DockingService) GetFailedTasks(ctx context.Context, page, size int) ([]*OrderDockingTask, int64, error) {
	return s.repo.GetFailedTasks(ctx, page, size)
}

// GetByOrderID 根据订单ID获取对接任务
func (s *DockingService) GetByOrderID(ctx context.Context, orderID uint) (*OrderDockingTask, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

// GetFailureStats 统计某供货商的总任务数和失败数
func (s *DockingService) GetFailureStats(ctx context.Context, supplierID uint) (total int64, failed int64, err error) {
	return s.repo.CountBySupplier(ctx, supplierID)
}

// isDuplicateError 判断是否为唯一索引冲突错误
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL 1062: Duplicate entry
	errStr := err.Error()
	return strings.Contains(errStr, "1062") || strings.Contains(errStr, "Duplicate entry") || strings.Contains(errStr, "UNIQUE constraint failed")
}
