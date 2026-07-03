package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/ledger"
)

// CreateOrderParams 创建订单参数
type CreateOrderParams struct {
	AppID           uint
	CustomerID      uint
	SupplierID      uint
	CustomerOrderID string // 幂等键
	GoodsID         uint
	GoodsSN         string
	GoodsName       string
	BuyNumber       int
	UnitPrice       decimal.Decimal
	BuyParams       map[string]string
	NotifyURL       string
}

// OrderService 订单核心服务
type OrderService struct {
	repo      *OrderRepository
	ledgerSvc *ledger.LedgerService
	db        *gorm.DB
}

// NewOrderService 创建OrderService
func NewOrderService(db *gorm.DB, ledgerSvc *ledger.LedgerService) *OrderService {
	return &OrderService{
		repo:      NewOrderRepository(db),
		ledgerSvc: ledgerSvc,
		db:        db,
	}
}

// CreateOrder 创建订单
// 1. 检查幂等（customer_order_id唯一）
// 2. 计算总金额
// 3. 在事务中：创建订单 + 扣减余额（通过ledgerSvc） + 保存购买参数快照
func (s *OrderService) CreateOrder(ctx context.Context, params CreateOrderParams) (*Order, error) {
	// 1. 幂等检查
	if params.CustomerOrderID != "" {
		existing, err := s.repo.FindByCustomerOrderID(ctx, params.AppID, params.CustomerOrderID)
		if err != nil {
			return nil, fmt.Errorf("order: idempotency check failed: %w", err)
		}
		if existing != nil {
			return existing, nil // 幂等返回已有订单
		}
	}

	// 2. 计算总金额
	amount := params.UnitPrice.Mul(decimal.NewFromInt(int64(params.BuyNumber)))

	// 3. 生成订单号
	orderSN, err := generateOrderSN()
	if err != nil {
		return nil, fmt.Errorf("order: generate order_sn failed: %w", err)
	}

	// 4. 构建订单
	now := time.Now().Unix()
	order := &Order{
		OrderSN:         orderSN,
		CustomerOrderID: params.CustomerOrderID,
		AppID:           params.AppID,
		CustomerID:      params.CustomerID,
		SupplierID:      params.SupplierID,
		GoodsID:         params.GoodsID,
		GoodsSN:         params.GoodsSN,
		GoodsName:       params.GoodsName,
		BuyNumber:       params.BuyNumber,
		UnitPrice:       params.UnitPrice,
		Amount:          amount,
		RefundAmount:    decimal.Zero,
		Status:          StatusPaid,
		Version:         0,
		NotifyURL:       params.NotifyURL,
		PaidAt:          &now,
	}

	// 5. 事务：创建订单 + 扣减余额 + 保存快照
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建订单
		if err := s.repo.CreateWithTx(ctx, tx, order); err != nil {
			return fmt.Errorf("order: create order failed: %w", err)
		}

		// 扣减余额
		note := fmt.Sprintf("订单支付: %s", orderSN)
		if err := s.ledgerSvc.Debit(ctx, tx, params.CustomerID, amount, "order_pay", order.ID, note); err != nil {
			return fmt.Errorf("order: debit failed: %w", err)
		}

		// 保存购买参数快照
		if len(params.BuyParams) > 0 {
			var buyParams []OrderBuyParam
			for name, value := range params.BuyParams {
				buyParams = append(buyParams, OrderBuyParam{
					OrderID: order.ID,
					Name:    name,
					Value:   value,
				})
			}
			if err := s.repo.CreateBuyParams(ctx, tx, buyParams); err != nil {
				return fmt.Errorf("order: save buy params failed: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return order, nil
}

// TransitionStatus CAS状态转移
// 1. 验证转移合法性（查ValidTransitions）
// 2. CAS更新（带version）
// 3. 写入order_status_changes审计记录
// 4. 如果是退款终态，触发退款流程
func (s *OrderService) TransitionStatus(ctx context.Context, orderID uint, newStatus int8, operator string, remark string) error {
	// 获取当前订单
	order, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return err
	}

	// 验证状态转移合法性
	if !IsValidTransition(order.Status, newStatus) {
		return fmt.Errorf("%w: cannot transition from %d to %d", ErrInvalidTransition, order.Status, newStatus)
	}

	// CAS更新状态
	ok, err := s.repo.UpdateStatusCAS(ctx, orderID, order.Status, newStatus, order.Version)
	if err != nil {
		return fmt.Errorf("order: update status failed: %w", err)
	}
	if !ok {
		return ErrCASConflict
	}

	// 写入审计记录
	change := &OrderStatusChange{
		OrderID:   orderID,
		OldStatus: order.Status,
		NewStatus: newStatus,
		Operator:  operator,
		Remark:    remark,
	}
	if err := s.repo.CreateStatusChange(ctx, change); err != nil {
		// 审计记录写入失败不回滚状态变更，仅记录错误
		return fmt.Errorf("order: save status change audit failed: %w", err)
	}

	// 如果是退款终态，触发退款流程
	if newStatus == StatusRefunded {
		if err := s.processRefund(ctx, order); err != nil {
			return fmt.Errorf("order: refund process failed: %w", err)
		}
	}

	return nil
}

// processRefund 处理退款
func (s *OrderService) processRefund(ctx context.Context, order *Order) error {
	refundAmount := order.Amount
	if order.RefundAmount.GreaterThan(decimal.Zero) {
		refundAmount = order.RefundAmount
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		note := fmt.Sprintf("订单退款: %s", order.OrderSN)
		return s.ledgerSvc.Credit(ctx, tx, order.CustomerID, refundAmount, "order_refund", order.ID, note)
	})
}

// GetOrder 通过订单号查询
func (s *OrderService) GetOrder(ctx context.Context, orderSN string) (*Order, error) {
	return s.repo.FindByOrderSN(ctx, orderSN)
}

// GetOrderByID 通过ID查询
func (s *OrderService) GetOrderByID(ctx context.Context, id uint) (*Order, error) {
	return s.repo.FindByID(ctx, id)
}

// BatchGetOrders 批量查询
func (s *OrderService) BatchGetOrders(ctx context.Context, orderSNs []string) ([]*Order, error) {
	if len(orderSNs) == 0 {
		return nil, nil
	}
	return s.repo.BatchFindByOrderSNs(ctx, orderSNs)
}

// ListByCustomer 分页查询客户订单
func (s *OrderService) ListByCustomer(ctx context.Context, customerID uint, status *int8, page, size int) ([]*Order, int64, error) {
	return s.repo.ListByCustomer(ctx, customerID, status, page, size)
}

// ListBySupplier 分页查询供货商订单
func (s *OrderService) ListBySupplier(ctx context.Context, supplierID uint, status *int8, page, size int) ([]*Order, int64, error) {
	return s.repo.ListBySupplier(ctx, supplierID, status, page, size)
}

// generateOrderSN 生成唯一订单号（时间戳+随机hex）
func generateOrderSN() (string, error) {
	ts := time.Now().Format("20060102150405")
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("ORD%s%s", ts, hex.EncodeToString(b)), nil
}
