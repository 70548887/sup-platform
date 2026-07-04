package recharge

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrRechargeNotFound = errors.New("recharge: not found")
)

// RechargeRepository 充值数据访问层
type RechargeRepository struct {
	db *gorm.DB
}

// NewRechargeRepository 创建RechargeRepository
func NewRechargeRepository(db *gorm.DB) *RechargeRepository {
	return &RechargeRepository{db: db}
}

// Create 创建充值单
func (r *RechargeRepository) Create(ctx context.Context, order *RechargeOrder) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetByID 通过ID查找充值单
func (r *RechargeRepository) GetByID(ctx context.Context, id uint) (*RechargeOrder, error) {
	var order RechargeOrder
	err := r.db.WithContext(ctx).First(&order, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRechargeNotFound
		}
		return nil, err
	}
	return &order, nil
}

// GetByIdempotencyKey 通过幂等键查找充值单
func (r *RechargeRepository) GetByIdempotencyKey(ctx context.Context, key string) (*RechargeOrder, error) {
	var order RechargeOrder
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

// UpdateStatusCAS 使用CAS更新充值单状态
// WHERE id=? AND status=? AND version=? → SET status=newStatus, version=version+1, ...
// 返回是否更新成功（RowsAffected > 0）
func (r *RechargeRepository) UpdateStatusCAS(ctx context.Context, tx *gorm.DB, id uint, oldStatus int8, newStatus int8, version int64, updates map[string]interface{}) (bool, error) {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = newStatus
	updates["version"] = gorm.Expr("version + 1")

	db := tx
	if db == nil {
		db = r.db
	}

	result := db.WithContext(ctx).Model(&RechargeOrder{}).
		Where("id = ? AND status = ? AND version = ?", id, oldStatus, version).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// List 分页查询充值单（支持按userID和status过滤）
func (r *RechargeRepository) List(ctx context.Context, userID uint, status *int8, page, size int) ([]*RechargeOrder, int64, error) {
	// size上限100
	if size > 100 {
		size = 100
	}
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}

	query := r.db.WithContext(ctx).Model(&RechargeOrder{}).Where("user_id = ?", userID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []*RechargeOrder
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ListPending 查询所有待审核的充值单
func (r *RechargeRepository) ListPending(ctx context.Context, page, size int) ([]*RechargeOrder, int64, error) {
	// size上限100
	if size > 100 {
		size = 100
	}
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}

	query := r.db.WithContext(ctx).Model(&RechargeOrder{}).Where("status = ?", StatusPending)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []*RechargeOrder
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}
