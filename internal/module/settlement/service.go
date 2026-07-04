package settlement

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// DefaultCommissionRate 默认平台佣金比例 5%
var DefaultCommissionRate = decimal.NewFromFloat(0.05)

// SettlementService 结算分润服务
type SettlementService struct {
	db   *gorm.DB
	repo *SettlementRepository
}

// NewSettlementService 创建结算服务
func NewSettlementService(db *gorm.DB) *SettlementService {
	return &SettlementService{
		db:   db,
		repo: NewSettlementRepository(db),
	}
}

// GenerateSettlement 生成结算单
// 聚合该供货商该月已完成订单金额，计算佣金，创建Settlement记录
func (s *SettlementService) GenerateSettlement(ctx context.Context, supplierID uint, period string) (*Settlement, error) {
	if supplierID == 0 {
		return nil, errors.New("supplier_id不能为空")
	}
	if len(period) != 7 {
		return nil, errors.New("period格式无效，应为YYYY-MM")
	}

	// 聚合该供货商该月已完成订单
	agg, err := s.repo.AggregateOrdersBySupplier(supplierID, period)
	if err != nil {
		return nil, err
	}

	if agg.TotalOrders == 0 {
		return nil, errors.New("该供货商该月无已完成订单")
	}

	// 计算佣金
	commissionAmount := agg.TotalAmount.Mul(DefaultCommissionRate).Round(6)
	netAmount := agg.TotalAmount.Sub(commissionAmount)

	settlement := &Settlement{
		TenantID:         1, // 默认租户
		SupplierID:       supplierID,
		Period:           period,
		TotalOrders:      agg.TotalOrders,
		TotalAmount:      agg.TotalAmount,
		CommissionRate:   DefaultCommissionRate,
		CommissionAmount: commissionAmount,
		NetAmount:        netAmount,
		Status:           "pending",
	}

	if err := s.repo.CreateSettlement(settlement); err != nil {
		return nil, err
	}

	return settlement, nil
}

// ConfirmSettlement 确认结算单（状态→confirmed）
func (s *SettlementService) ConfirmSettlement(ctx context.Context, id uint) error {
	st, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if st.Status != "pending" {
		return errors.New("只有pending状态的结算单可以确认")
	}

	now := time.Now().Unix()
	return s.repo.UpdateStatus(id, "confirmed", "confirmed_at", now)
}

// MarkPaid 标记已付款（状态→paid）
func (s *SettlementService) MarkPaid(ctx context.Context, id uint) error {
	st, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if st.Status != "confirmed" {
		return errors.New("只有confirmed状态的结算单可以标记付款")
	}

	now := time.Now().Unix()
	return s.repo.UpdateStatus(id, "paid", "paid_at", now)
}

// CalculateProfitShare 计算订单分润
// 查询订单信息，计算平台利润和供货商利润，创建ProfitShare记录
func (s *SettlementService) CalculateProfitShare(ctx context.Context, orderID uint, supplierID uint, orderAmount decimal.Decimal) (*ProfitShare, error) {
	// 检查是否已存在分润记录
	existing, err := s.repo.GetProfitShareByOrder(orderID)
	if err == nil && existing != nil {
		return existing, nil // 幂等
	}

	platformProfit := orderAmount.Mul(DefaultCommissionRate).Round(6)
	supplierProfit := orderAmount.Sub(platformProfit)

	ps := &ProfitShare{
		TenantID:       1, // 默认租户
		OrderID:        orderID,
		SupplierID:     supplierID,
		OrderAmount:    orderAmount,
		PlatformRate:   DefaultCommissionRate,
		PlatformProfit: platformProfit,
		SupplierProfit: supplierProfit,
	}

	if err := s.repo.CreateProfitShare(ps); err != nil {
		return nil, err
	}

	return ps, nil
}

// ListSettlements 查询结算单列表
func (s *SettlementService) ListSettlements(ctx context.Context, supplierID uint, page, size int) ([]Settlement, int64, error) {
	return s.repo.ListAll(supplierID, page, size)
}

// GetSettlement 获取结算单详情
func (s *SettlementService) GetSettlement(ctx context.Context, id uint) (*Settlement, error) {
	return s.repo.GetByID(id)
}
