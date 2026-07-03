package order

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrOrderNotFound    = errors.New("order: not found")
	ErrDuplicateOrder   = errors.New("order: duplicate customer_order_id")
	ErrCASConflict      = errors.New("order: CAS conflict, concurrent modification detected")
	ErrInvalidTransition = errors.New("order: invalid status transition")
)

// OrderRepository 订单数据访问层
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建OrderRepository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create 创建订单
func (r *OrderRepository) Create(ctx context.Context, order *Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// CreateWithTx 在事务中创建订单
func (r *OrderRepository) CreateWithTx(ctx context.Context, tx *gorm.DB, order *Order) error {
	return tx.WithContext(ctx).Create(order).Error
}

// FindByID 通过ID查找订单
func (r *OrderRepository) FindByID(ctx context.Context, id uint) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).First(&order, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

// FindByOrderSN 通过平台订单号查找
func (r *OrderRepository) FindByOrderSN(ctx context.Context, sn string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).Where("order_sn = ?", sn).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

// FindByCustomerOrderID 通过客户订单号+AppID查找（幂等检查）
func (r *OrderRepository) FindByCustomerOrderID(ctx context.Context, appID uint, customerOrderID string) (*Order, error) {
	var order Order
	err := r.db.WithContext(ctx).
		Where("app_id = ? AND customer_order_id = ?", appID, customerOrderID).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 未找到不算错误，用于幂等检查
		}
		return nil, err
	}
	return &order, nil
}

// UpdateStatusCAS CAS乐观锁状态更新
// UPDATE orders SET status=?, version=version+1 WHERE id=? AND status=? AND version=?
// 返回 bool 表示是否成功（RowsAffected > 0）
func (r *OrderRepository) UpdateStatusCAS(ctx context.Context, id uint, oldStatus, newStatus int8, version int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&Order{}).
		Where("id = ? AND status = ? AND version = ?", id, oldStatus, version).
		Updates(map[string]interface{}{
			"status":  newStatus,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// UpdateStatusCASWithTx 在事务中执行CAS更新
func (r *OrderRepository) UpdateStatusCASWithTx(ctx context.Context, tx *gorm.DB, id uint, oldStatus, newStatus int8, version int64) (bool, error) {
	result := tx.WithContext(ctx).
		Model(&Order{}).
		Where("id = ? AND status = ? AND version = ?", id, oldStatus, version).
		Updates(map[string]interface{}{
			"status":  newStatus,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CreateStatusChange 记录状态变更审计
func (r *OrderRepository) CreateStatusChange(ctx context.Context, change *OrderStatusChange) error {
	return r.db.WithContext(ctx).Create(change).Error
}

// CreateBuyParams 批量创建购买参数快照
func (r *OrderRepository) CreateBuyParams(ctx context.Context, tx *gorm.DB, params []OrderBuyParam) error {
	if len(params) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&params).Error
}

// ListByCustomer 分页查询客户订单
func (r *OrderRepository) ListByCustomer(ctx context.Context, customerID uint, status *int8, page, size int) ([]*Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&Order{}).Where("customer_id = ?", customerID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []*Order
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ListBySupplier 分页查询供货商订单
func (r *OrderRepository) ListBySupplier(ctx context.Context, supplierID uint, status *int8, page, size int) ([]*Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&Order{}).Where("supplier_id = ?", supplierID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []*Order
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// BatchFindByOrderSNs 批量查询订单
func (r *OrderRepository) BatchFindByOrderSNs(ctx context.Context, sns []string) ([]*Order, error) {
	var orders []*Order
	err := r.db.WithContext(ctx).Where("order_sn IN ?", sns).Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}
