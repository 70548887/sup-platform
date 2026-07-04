package notify

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/order"
)

// NotifyRepository 通知投递数据访问层
type NotifyRepository struct {
	db *gorm.DB
}

// NewNotifyRepository 创建NotifyRepository
func NewNotifyRepository(db *gorm.DB) *NotifyRepository {
	return &NotifyRepository{db: db}
}

// CreateCallback 创建通知投递记录
func (r *NotifyRepository) CreateCallback(ctx context.Context, cb *order.OrderCallback) error {
	return r.db.WithContext(ctx).Create(cb).Error
}

// UpdateCallback 更新通知投递记录
func (r *NotifyRepository) UpdateCallback(ctx context.Context, cb *order.OrderCallback) error {
	return r.db.WithContext(ctx).Save(cb).Error
}

// GetPendingCallbacks 查询所有待重试的通知记录
// 条件: success=false AND retry_count<5 AND (next_retry_at IS NULL OR next_retry_at <= now)
func (r *NotifyRepository) GetPendingCallbacks(ctx context.Context) ([]*order.OrderCallback, error) {
	var callbacks []*order.OrderCallback
	now := time.Now().Unix()
	err := r.db.WithContext(ctx).
		Where("success = ? AND retry_count < ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", false, 5, now).
		Find(&callbacks).Error
	if err != nil {
		return nil, err
	}
	return callbacks, nil
}

// GetCallbackByID 通过ID查询通知记录
func (r *NotifyRepository) GetCallbackByID(ctx context.Context, id uint) (*order.OrderCallback, error) {
	var cb order.OrderCallback
	err := r.db.WithContext(ctx).First(&cb, id).Error
	if err != nil {
		return nil, err
	}
	return &cb, nil
}
