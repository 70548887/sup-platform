package reconciliation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 微差阈值：差异绝对值 <= 0.01 视为微差，自动冲销
var microDiffThreshold = decimal.NewFromFloat(0.01)

// 每批获取的钱包/客户数量
const batchSize = 1000

// ReconciliationService 对账服务
type ReconciliationService struct {
	repo *ReconciliationRepository
	db   *gorm.DB
}

// NewReconciliationService 创建ReconciliationService
func NewReconciliationService(db *gorm.DB) *ReconciliationService {
	return &ReconciliationService{
		repo: NewReconciliationRepository(db),
		db:   db,
	}
}

// RunBalanceCheck 余额校验：对比每个钱包的Balance与账本流水SUM(amount)
// 异步执行，立即返回task（状态为running）
func (s *ReconciliationService) RunBalanceCheck(ctx context.Context) (*ReconciliationTask, error) {
	now := time.Now().Unix()
	task := &ReconciliationTask{
		Type:      TypeBalanceCheck,
		Status:    StatusRunning,
		StartedAt: now,
	}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("reconciliation: create balance_check task failed: %w", err)
	}

	// 启动goroutine异步执行
	go s.executeBalanceCheck(task.ID)

	return task, nil
}

// executeBalanceCheck 执行余额校验（goroutine内调用）
func (s *ReconciliationService) executeBalanceCheck(taskID uint) {
	ctx := context.Background()
	var errorCount int
	var totalChecked int

	defer func() {
		now := time.Now().Unix()
		task, err := s.repo.GetTask(ctx, taskID)
		if err != nil {
			log.Printf("[ERROR] reconciliation: get task %d for finalization failed: %v", taskID, err)
			return
		}

		if r := recover(); r != nil {
			task.Status = StatusFailed
			task.TotalChecked = totalChecked
			task.ErrorCount = errorCount
			task.CompletedAt = &now
			if updateErr := s.repo.UpdateTask(ctx, task); updateErr != nil {
				log.Printf("[ERROR] reconciliation: update failed task %d: %v", taskID, updateErr)
			}
			log.Printf("[ERROR] reconciliation: balance_check task %d panicked: %v", taskID, r)
			return
		}

		task.Status = StatusCompleted
		task.TotalChecked = totalChecked
		task.ErrorCount = errorCount
		task.CompletedAt = &now
		if err := s.repo.UpdateTask(ctx, task); err != nil {
			log.Printf("[ERROR] reconciliation: update task %d after completion failed: %v", taskID, err)
		}
		log.Printf("[INFO] reconciliation: balance_check task %d completed, checked=%d, errors=%d",
			taskID, totalChecked, errorCount)
	}()

	offset := 0
	for {
		wallets, err := s.repo.GetAllWallets(ctx, offset, batchSize)
		if err != nil {
			log.Printf("[ERROR] reconciliation: get wallets batch (offset=%d) failed: %v", offset, err)
			return
		}
		if len(wallets) == 0 {
			break
		}

		for i := range wallets {
			wallet := &wallets[i]
			totalChecked++

			ledgerSum, err := s.repo.GetLedgerSum(ctx, wallet.UserID)
			if err != nil {
				log.Printf("[ERROR] reconciliation: get ledger sum for user %d failed: %v", wallet.UserID, err)
				continue
			}

			// 账本流水SUM(amount)应该等于钱包Balance
			// 因为：每次Credit/Debit都会同时更新Balance和创建对应金额的流水
			// Credit: amount正数 → Balance增加 → ledgerSum增加
			// Debit: amount负数 → Balance减少 → ledgerSum减少
			// 所以 ledgerSum == wallet.Balance (理论上)
			diff := ledgerSum.Sub(wallet.Balance)
			absDiff := diff.Abs()

			if absDiff.IsZero() {
				continue // 无差异，跳过
			}

			// 有差异
			recErr := &ReconciliationError{
				TaskID:     taskID,
				ErrorType:  ErrorBalanceMismatch,
				UserID:     wallet.UserID,
				Expected:   wallet.Balance,
				Actual:     ledgerSum,
				Difference: diff,
				Status:     ErrorStatusPending,
			}

			// 微差自动冲销
			if absDiff.LessThanOrEqual(microDiffThreshold) {
				recErr.Status = ErrorStatusAutoFixed
				recErr.Resolution = fmt.Sprintf("微差自动冲销: 差异 %s <= 阈值 %s", diff.StringFixed(6), microDiffThreshold.StringFixed(2))
				now := time.Now().Unix()
				recErr.ResolvedAt = &now
				recErr.ResolvedBy = "system:auto"
			} else {
				errorCount++
			}

			if createErr := s.repo.CreateError(ctx, recErr); createErr != nil {
				log.Printf("[ERROR] reconciliation: create error for user %d failed: %v", wallet.UserID, createErr)
			}
		}

		offset += len(wallets)
		if len(wallets) < batchSize {
			break
		}
	}
}

// RunCrossVerify 交叉验证：检查订单总金额 vs 对应流水总金额
// 异步执行，立即返回task（状态为running）
func (s *ReconciliationService) RunCrossVerify(ctx context.Context) (*ReconciliationTask, error) {
	now := time.Now().Unix()
	task := &ReconciliationTask{
		Type:      TypeCrossVerify,
		Status:    StatusRunning,
		StartedAt: now,
	}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("reconciliation: create cross_verify task failed: %w", err)
	}

	// 启动goroutine异步执行
	go s.executeCrossVerify(task.ID)

	return task, nil
}

// executeCrossVerify 执行交叉验证（goroutine内调用）
// 对每个有订单的客户：比较 SUM(orders.amount) 与 |SUM(ledger_entries.amount WHERE type='order_pay')|
func (s *ReconciliationService) executeCrossVerify(taskID uint) {
	ctx := context.Background()
	var errorCount int
	var totalChecked int

	defer func() {
		now := time.Now().Unix()
		task, err := s.repo.GetTask(ctx, taskID)
		if err != nil {
			log.Printf("[ERROR] reconciliation: get task %d for finalization failed: %v", taskID, err)
			return
		}

		if r := recover(); r != nil {
			task.Status = StatusFailed
			task.TotalChecked = totalChecked
			task.ErrorCount = errorCount
			task.CompletedAt = &now
			if updateErr := s.repo.UpdateTask(ctx, task); updateErr != nil {
				log.Printf("[ERROR] reconciliation: update failed task %d: %v", taskID, updateErr)
			}
			log.Printf("[ERROR] reconciliation: cross_verify task %d panicked: %v", taskID, r)
			return
		}

		task.Status = StatusCompleted
		task.TotalChecked = totalChecked
		task.ErrorCount = errorCount
		task.CompletedAt = &now
		if err := s.repo.UpdateTask(ctx, task); err != nil {
			log.Printf("[ERROR] reconciliation: update task %d after completion failed: %v", taskID, err)
		}
		log.Printf("[INFO] reconciliation: cross_verify task %d completed, checked=%d, errors=%d",
			taskID, totalChecked, errorCount)
	}()

	offset := 0
	for {
		customerIDs, err := s.repo.GetDistinctOrderCustomerIDs(ctx, offset, batchSize)
		if err != nil {
			log.Printf("[ERROR] reconciliation: get distinct customer ids (offset=%d) failed: %v", offset, err)
			return
		}
		if len(customerIDs) == 0 {
			break
		}

		for _, customerID := range customerIDs {
			totalChecked++

			// 期望值：订单总金额（正数）
			orderTotal, err := s.repo.GetOrderTotalByCustomer(ctx, customerID)
			if err != nil {
				log.Printf("[ERROR] reconciliation: get order total for customer %d failed: %v", customerID, err)
				continue
			}

			// 实际值：订单支付流水总和（负数，取绝对值）
			ledgerPaySum, err := s.repo.GetOrderPaySum(ctx, customerID)
			if err != nil {
				log.Printf("[ERROR] reconciliation: get order_pay sum for customer %d failed: %v", customerID, err)
				continue
			}
			actualPay := ledgerPaySum.Abs() // 流水是负数，取绝对值

			diff := orderTotal.Sub(actualPay)
			absDiff := diff.Abs()

			if absDiff.IsZero() {
				continue // 无差异，跳过
			}

			// 有差异
			recErr := &ReconciliationError{
				TaskID:     taskID,
				ErrorType:  ErrorCrossMismatch,
				UserID:     customerID,
				Expected:   orderTotal,
				Actual:     actualPay,
				Difference: diff,
				Status:     ErrorStatusPending,
			}

			// 微差自动冲销
			if absDiff.LessThanOrEqual(microDiffThreshold) {
				recErr.Status = ErrorStatusAutoFixed
				recErr.Resolution = fmt.Sprintf("微差自动冲销: 差异 %s <= 阈值 %s", diff.StringFixed(6), microDiffThreshold.StringFixed(2))
				now := time.Now().Unix()
				recErr.ResolvedAt = &now
				recErr.ResolvedBy = "system:auto"
			} else {
				errorCount++
			}

			if createErr := s.repo.CreateError(ctx, recErr); createErr != nil {
				log.Printf("[ERROR] reconciliation: create error for customer %d failed: %v", customerID, createErr)
			}
		}

		offset += len(customerIDs)
		if len(customerIDs) < batchSize {
			break
		}
	}
}

// ResolveError 手动处理对账异常
// action: manual_fixed, ignored
func (s *ReconciliationService) ResolveError(ctx context.Context, errorID uint, action string, note string, operator string) error {
	recErr, err := s.repo.GetError(ctx, errorID)
	if err != nil {
		return fmt.Errorf("reconciliation: get error %d failed: %w", errorID, err)
	}

	// 验证action合法性
	switch action {
	case ErrorStatusManualFixed, ErrorStatusIgnored:
		// 合法
	default:
		return fmt.Errorf("reconciliation: invalid action %q, must be %s or %s", action, ErrorStatusManualFixed, ErrorStatusIgnored)
	}

	now := time.Now().Unix()
	recErr.Status = action
	recErr.Resolution = note
	recErr.ResolvedBy = operator
	recErr.ResolvedAt = &now

	if err := s.repo.UpdateError(ctx, recErr); err != nil {
		return fmt.Errorf("reconciliation: update error %d failed: %w", errorID, err)
	}

	return nil
}

// GetTask 获取对账任务详情
func (s *ReconciliationService) GetTask(ctx context.Context, id uint) (*ReconciliationTask, error) {
	return s.repo.GetTask(ctx, id)
}

// ListTasks 分页查询对账任务
func (s *ReconciliationService) ListTasks(ctx context.Context, page, size int) ([]ReconciliationTask, int64, error) {
	return s.repo.ListTasks(ctx, page, size)
}

// ListErrors 分页查询某任务下的对账异常
func (s *ReconciliationService) ListErrors(ctx context.Context, taskID uint, page, size int) ([]ReconciliationError, int64, error) {
	return s.repo.ListErrorsByTask(ctx, taskID, page, size)
}

// GetError 获取对账异常详情
func (s *ReconciliationService) GetError(ctx context.Context, id uint) (*ReconciliationError, error) {
	return s.repo.GetError(ctx, id)
}

