package settlement

import (
	"errors"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ErrDuplicatePeriod 同一供货商同一周期重复生成结算单
var ErrDuplicatePeriod = errors.New("该供货商该周期已存在结算单")

// SettlementRepository 结算数据访问层
type SettlementRepository struct {
	db *gorm.DB
}

// NewSettlementRepository 创建结算仓储
func NewSettlementRepository(db *gorm.DB) *SettlementRepository {
	return &SettlementRepository{db: db}
}

// CreateSettlement 创建结算单（含周期唯一性检查）
func (r *SettlementRepository) CreateSettlement(s *Settlement) error {
	var count int64
	if err := r.db.Model(&Settlement{}).
		Where("supplier_id = ? AND period = ?", s.SupplierID, s.Period).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrDuplicatePeriod
	}
	return r.db.Create(s).Error
}

// GetByID 根据ID查询结算单
func (r *SettlementRepository) GetByID(id uint) (*Settlement, error) {
	var s Settlement
	if err := r.db.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ListBySupplier 按供货商分页查询结算单
func (r *SettlementRepository) ListBySupplier(supplierID uint, page, size int) ([]Settlement, int64, error) {
	var list []Settlement
	var total int64

	query := r.db.Model(&Settlement{}).Where("supplier_id = ?", supplierID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ListAll 分页查询所有结算单（可选供货商筛选）
func (r *SettlementRepository) ListAll(supplierID uint, page, size int) ([]Settlement, int64, error) {
	var list []Settlement
	var total int64

	query := r.db.Model(&Settlement{})
	if supplierID > 0 {
		query = query.Where("supplier_id = ?", supplierID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateStatus 更新结算单状态
func (r *SettlementRepository) UpdateStatus(id uint, status string, timeField string, ts int64) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if timeField != "" {
		updates[timeField] = ts
	}
	return r.db.Model(&Settlement{}).Where("id = ?", id).Updates(updates).Error
}

// CreateProfitShare 创建分润记录
func (r *SettlementRepository) CreateProfitShare(ps *ProfitShare) error {
	return r.db.Create(ps).Error
}

// GetProfitShareByOrder 根据订单ID查询分润记录
func (r *SettlementRepository) GetProfitShareByOrder(orderID uint) (*ProfitShare, error) {
	var ps ProfitShare
	if err := r.db.Where("order_id = ?", orderID).First(&ps).Error; err != nil {
		return nil, err
	}
	return &ps, nil
}

// AggregateResult 聚合查询结果
type AggregateResult struct {
	TotalOrders int
	TotalAmount decimal.Decimal
}

// AggregateOrdersBySupplier 按供货商+月份聚合已完成订单金额
func (r *SettlementRepository) AggregateOrdersBySupplier(supplierID uint, period string) (*AggregateResult, error) {
	// period格式: "2026-07"，需要匹配对应月份的已完成订单
	// 订单status=5表示已完成（参考order模块）
	type result struct {
		TotalOrders int
		TotalAmount decimal.Decimal
	}
	var res result

	// 根据period计算时间范围（Unix时间戳）
	// period: "2026-07" → start: 2026-07-01 00:00:00, end: 2026-08-01 00:00:00
	err := r.db.Raw(`
		SELECT COUNT(*) as total_orders, COALESCE(SUM(amount), 0) as total_amount
		FROM orders
		WHERE supplier_id = ? AND status = 5
		AND FROM_UNIXTIME(completed_at, '%Y-%m') = ?
	`, supplierID, period).Scan(&res).Error

	if err != nil {
		return nil, err
	}

	return &AggregateResult{
		TotalOrders: res.TotalOrders,
		TotalAmount: res.TotalAmount,
	}, nil
}
