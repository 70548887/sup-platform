package docking

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DockingRepository 订单对接任务数据访问层
type DockingRepository struct {
	db *gorm.DB
}

// NewDockingRepository 创建DockingRepository
func NewDockingRepository(db *gorm.DB) *DockingRepository {
	return &DockingRepository{db: db}
}

// Create 创建对接任务
func (r *DockingRepository) Create(ctx context.Context, task *OrderDockingTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID 根据ID获取任务
func (r *DockingRepository) GetByID(ctx context.Context, id uint) (*OrderDockingTask, error) {
	var task OrderDockingTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByOrderID 根据订单ID获取任务
func (r *DockingRepository) GetByOrderID(ctx context.Context, orderID uint) (*OrderDockingTask, error) {
	var task OrderDockingTask
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// LockTask CAS锁定任务: status=pending → locked，防止并发重复提交
// 返回是否锁定成功
func (r *DockingRepository) LockTask(ctx context.Context, taskID uint) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&OrderDockingTask{}).
		Where("id = ? AND status = ?", taskID, TaskPending).
		Update("status", TaskLocked)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// UpdateAfterSubmit 提交成功后更新任务
func (r *DockingRepository) UpdateAfterSubmit(ctx context.Context, task *OrderDockingTask) error {
	return r.db.WithContext(ctx).
		Model(task).
		Select("status", "external_order_id", "response_payload", "submitted_at").
		Updates(task).Error
}

// UpdateAfterFailure 提交失败后更新任务
// 如果retryCount >= maxRetry，标记为failed；否则回退为pending等待下次重试
func (r *DockingRepository) UpdateAfterFailure(ctx context.Context, task *OrderDockingTask) error {
	return r.db.WithContext(ctx).
		Model(task).
		Select("status", "retry_count", "error_message", "last_failure_at", "next_retry_at").
		Updates(task).Error
}

// GetPendingTasks 获取所有待执行的任务
// 条件: status=pending AND retry_count < max_retry AND (next_retry_at IS NULL OR next_retry_at <= now)
func (r *DockingRepository) GetPendingTasks(ctx context.Context) ([]*OrderDockingTask, error) {
	var tasks []*OrderDockingTask
	now := time.Now().Unix()
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < max_retry AND (next_retry_at IS NULL OR next_retry_at <= ?)", TaskPending, now).
		Find(&tasks).Error
	return tasks, err
}

// GetFailedTasks 分页获取失败任务
func (r *DockingRepository) GetFailedTasks(ctx context.Context, page, size int) ([]*OrderDockingTask, int64, error) {
	var tasks []*OrderDockingTask
	var total int64

	query := r.db.WithContext(ctx).Model(&OrderDockingTask{}).Where("status = ?", TaskFailed)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := query.Order("updated_at DESC").Offset(offset).Limit(size).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ResetForManualRetry CAS重置失败任务为待提交
// WHERE id=? AND status=failed → SET status=pending, retry_count=0, is_manual_retry=true
func (r *DockingRepository) ResetForManualRetry(ctx context.Context, taskID uint) error {
	result := r.db.WithContext(ctx).
		Model(&OrderDockingTask{}).
		Where("id = ? AND status = ?", taskID, TaskFailed).
		Updates(map[string]interface{}{
			"status":          TaskPending,
			"retry_count":     0,
			"is_manual_retry": true,
			"next_retry_at":   nil,
			"error_message":   "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountBySupplier 统计供货商的总任务数和失败数
func (r *DockingRepository) CountBySupplier(ctx context.Context, supplierID uint) (total int64, failed int64, err error) {
	if err = r.db.WithContext(ctx).Model(&OrderDockingTask{}).
		Where("supplier_id = ?", supplierID).
		Count(&total).Error; err != nil {
		return
	}
	if err = r.db.WithContext(ctx).Model(&OrderDockingTask{}).
		Where("supplier_id = ? AND status = ?", supplierID, TaskFailed).
		Count(&failed).Error; err != nil {
		return
	}
	return
}
