package refund

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrRefundNotFound = errors.New("refund: not found")
)

// RefundRepository 退款数据访问层
type RefundRepository struct {
	db *gorm.DB
}

// NewRefundRepository 创建RefundRepository
func NewRefundRepository(db *gorm.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

// Create 创建退款单
func (r *RefundRepository) Create(ctx context.Context, refund *RefundOrder) error {
	return r.db.WithContext(ctx).Create(refund).Error
}

// GetByID 通过ID查找退款单
func (r *RefundRepository) GetByID(ctx context.Context, id uint) (*RefundOrder, error) {
	var refund RefundOrder
	err := r.db.WithContext(ctx).First(&refund, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &refund, nil
}

// GetByRefundSN 通过退款单号查找
func (r *RefundRepository) GetByRefundSN(ctx context.Context, sn string) (*RefundOrder, error) {
	var refund RefundOrder
	err := r.db.WithContext(ctx).Where("refund_sn = ?", sn).First(&refund).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &refund, nil
}

// UpdateStatus 更新退款单状态
func (r *RefundRepository) UpdateStatus(ctx context.Context, id uint, status int8, updates map[string]interface{}) error {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = status
	result := r.db.WithContext(ctx).Model(&RefundOrder{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRefundNotFound
	}
	return nil
}

// List 分页查询客户退款单
func (r *RefundRepository) List(ctx context.Context, customerID uint, page, size int) ([]*RefundOrder, int64, error) {
	query := r.db.WithContext(ctx).Model(&RefundOrder{}).Where("customer_id = ?", customerID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var refunds []*RefundOrder
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&refunds).Error; err != nil {
		return nil, 0, err
	}
	return refunds, total, nil
}
