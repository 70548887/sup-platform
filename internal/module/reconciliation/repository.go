package reconciliation

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
)

// ReconciliationRepository 对账数据访问层
type ReconciliationRepository struct {
	db *gorm.DB
}

// NewReconciliationRepository 创建ReconciliationRepository
func NewReconciliationRepository(db *gorm.DB) *ReconciliationRepository {
	return &ReconciliationRepository{db: db}
}

// CreateTask 创建对账任务
func (r *ReconciliationRepository) CreateTask(ctx context.Context, task *ReconciliationTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// UpdateTask 更新对账任务
func (r *ReconciliationRepository) UpdateTask(ctx context.Context, task *ReconciliationTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// GetTask 根据ID获取对账任务
func (r *ReconciliationRepository) GetTask(ctx context.Context, id uint) (*ReconciliationTask, error) {
	var task ReconciliationTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		return nil, fmt.Errorf("reconciliation: get task %d failed: %w", id, err)
	}
	return &task, nil
}

// ListTasks 分页查询对账任务
func (r *ReconciliationRepository) ListTasks(ctx context.Context, page, size int) ([]ReconciliationTask, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&ReconciliationTask{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("reconciliation: count tasks failed: %w", err)
	}

	var tasks []ReconciliationTask
	offset := (page - 1) * size
	if err := r.db.WithContext(ctx).
		Model(&ReconciliationTask{}).
		Order("id DESC").
		Offset(offset).
		Limit(size).
		Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("reconciliation: list tasks failed: %w", err)
	}
	return tasks, total, nil
}

// CreateError 创建对账异常记录
func (r *ReconciliationRepository) CreateError(ctx context.Context, err *ReconciliationError) error {
	return r.db.WithContext(ctx).Create(err).Error
}

// ListErrorsByTask 分页查询某任务下的对账异常
func (r *ReconciliationRepository) ListErrorsByTask(ctx context.Context, taskID uint, page, size int) ([]ReconciliationError, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&ReconciliationError{}).
		Where("task_id = ?", taskID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("reconciliation: count errors by task %d failed: %w", taskID, err)
	}

	var errs []ReconciliationError
	offset := (page - 1) * size
	if err := r.db.WithContext(ctx).
		Model(&ReconciliationError{}).
		Where("task_id = ?", taskID).
		Order("id DESC").
		Offset(offset).
		Limit(size).
		Find(&errs).Error; err != nil {
		return nil, 0, fmt.Errorf("reconciliation: list errors by task %d failed: %w", taskID, err)
	}
	return errs, total, nil
}

// GetError 根据ID获取对账异常
func (r *ReconciliationRepository) GetError(ctx context.Context, id uint) (*ReconciliationError, error) {
	var recErr ReconciliationError
	if err := r.db.WithContext(ctx).First(&recErr, id).Error; err != nil {
		return nil, fmt.Errorf("reconciliation: get error %d failed: %w", id, err)
	}
	return &recErr, nil
}

// UpdateError 更新对账异常
func (r *ReconciliationRepository) UpdateError(ctx context.Context, err *ReconciliationError) error {
	return r.db.WithContext(ctx).Save(err).Error
}

// GetAllWallets 分批获取所有钱包
func (r *ReconciliationRepository) GetAllWallets(ctx context.Context, offset, limit int) ([]ledger.Wallet, error) {
	var wallets []ledger.Wallet
	if err := r.db.WithContext(ctx).
		Model(&ledger.Wallet{}).
		Order("id ASC").
		Offset(offset).
		Limit(limit).
		Find(&wallets).Error; err != nil {
		return nil, fmt.Errorf("reconciliation: get wallets batch (offset=%d, limit=%d) failed: %w", offset, limit, err)
	}
	return wallets, nil
}

// GetLedgerSum 获取某用户所有流水金额总和
// 返回 SUM(amount)，如果没有流水返回 decimal.Zero
func (r *ReconciliationRepository) GetLedgerSum(ctx context.Context, userID uint) (decimal.Decimal, error) {
	var result decimal.Decimal
	err := r.db.WithContext(ctx).
		Model(&ledger.LedgerEntry{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("reconciliation: sum ledger entries for user %d failed: %w", userID, err)
	}
	return result, nil
}

// GetOrderPaySum 获取某用户所有订单支付流水的金额总和（负数）
func (r *ReconciliationRepository) GetOrderPaySum(ctx context.Context, userID uint) (decimal.Decimal, error) {
	var result decimal.Decimal
	err := r.db.WithContext(ctx).
		Model(&ledger.LedgerEntry{}).
		Where("user_id = ? AND type = ?", userID, "order_pay").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("reconciliation: sum order_pay ledger entries for user %d failed: %w", userID, err)
	}
	return result, nil
}

// GetOrderTotalByCustomer 获取某用户的订单总金额
func (r *ReconciliationRepository) GetOrderTotalByCustomer(ctx context.Context, customerID uint) (decimal.Decimal, error) {
	var result decimal.Decimal
	err := r.db.WithContext(ctx).
		Model(&order.Order{}).
		Where("customer_id = ?", customerID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("reconciliation: sum order amounts for customer %d failed: %w", customerID, err)
	}
	return result, nil
}

// CountWallets 统计钱包总数
func (r *ReconciliationRepository) CountWallets(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&ledger.Wallet{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("reconciliation: count wallets failed: %w", err)
	}
	return count, nil
}

// GetDistinctOrderCustomerIDs 获取有订单的客户ID列表（分批）
func (r *ReconciliationRepository) GetDistinctOrderCustomerIDs(ctx context.Context, offset, limit int) ([]uint, error) {
	var ids []uint
	if err := r.db.WithContext(ctx).
		Model(&order.Order{}).
		Distinct("customer_id").
		Order("customer_id ASC").
		Offset(offset).
		Limit(limit).
		Pluck("customer_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("reconciliation: get distinct customer ids (offset=%d, limit=%d) failed: %w", offset, limit, err)
	}
	return ids, nil
}
