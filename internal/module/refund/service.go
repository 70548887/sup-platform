package refund

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/pkg/crypto"
)

var (
	ErrRefundAmountExceeded = errors.New("refund: amount exceeds refundable amount")
	ErrOrderNotRefundable   = errors.New("refund: order is not in a refundable status")
	ErrInvalidRefundStatus  = errors.New("refund: invalid refund status for this operation")
	ErrOrderNotBelong       = errors.New("refund: order does not belong to this customer")
	ErrCASConflict          = errors.New("refund: CAS conflict, concurrent modification detected")
)

// RefundService 退款服务
type RefundService struct {
	repo      *RefundRepository
	db        *gorm.DB
	orderSvc  *order.OrderService
	ledgerSvc *ledger.LedgerService
}

// NewRefundService 创建RefundService
func NewRefundService(db *gorm.DB, orderSvc *order.OrderService, ledgerSvc *ledger.LedgerService) *RefundService {
	return &RefundService{
		repo:      NewRefundRepository(db),
		db:        db,
		orderSvc:  orderSvc,
		ledgerSvc: ledgerSvc,
	}
}

// Apply 申请退款
// 1. 查找订单，验证归属
// 2. 验证金额不超过 order.Amount - order.RefundAmount
// 3. 生成RefundSN (RF + 时间戳 + 随机)
// 4. 创建退款单(status=RefundPending)
// 5. 将订单状态转为StatusRefunding
func (s *RefundService) Apply(ctx context.Context, customerID uint, orderSN string, amount decimal.Decimal, reason string) (*RefundOrder, error) {
	// 1. 查找订单
	ord, err := s.orderSvc.GetOrder(ctx, orderSN)
	if err != nil {
		return nil, fmt.Errorf("refund: find order failed: %w", err)
	}

	// 2. 验证订单归属
	if ord.CustomerID != customerID {
		return nil, ErrOrderNotBelong
	}

	// 3. 验证退款金额
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("refund: amount must be positive")
	}
	refundableAmount := ord.Amount.Sub(ord.RefundAmount)
	if amount.GreaterThan(refundableAmount) {
		return nil, fmt.Errorf("%w: requested %s, refundable %s", ErrRefundAmountExceeded, amount.String(), refundableAmount.String())
	}

	// 4. 生成RefundSN
	refundSN, err := generateRefundSN()
	if err != nil {
		return nil, fmt.Errorf("refund: generate refund_sn failed: %w", err)
	}

	// 5. 创建退款单(status=RefundPending)
	refundOrder := &RefundOrder{
		RefundSN:   refundSN,
		OrderID:    ord.ID,
		OrderSN:    ord.OrderSN,
		CustomerID: customerID,
		Amount:     amount,
		Reason:     reason,
		Status:     RefundPending,
	}
	if err := s.repo.Create(ctx, refundOrder); err != nil {
		return nil, fmt.Errorf("refund: create refund order failed: %w", err)
	}

	// 6. 将订单状态转为StatusRefunding
	// 使用TransitionStatus（CAS状态机），支持从StatusProcessing/StatusAbnormal转入
	if err := s.orderSvc.TransitionStatus(ctx, ord.ID, order.StatusRefunding, "system", fmt.Sprintf("退款申请: %s", refundSN)); err != nil {
		// 状态转移失败，退款单仍留在RefundPending状态，可后续手动处理
		return nil, fmt.Errorf("refund: transition order to refunding failed: %w", err)
	}

	return refundOrder, nil
}

// Approve 批准退款
// 1. 验证状态=RefundPending
// 2. 在事务中执行退款:
//    a. CAS更新退款单状态 RefundPending -> RefundCompleted (含审批信息)
//    b. ledgerSvc.Credit(退款金额到客户钱包)
//    c. 更新订单RefundAmount累加（带金额上限约束）
//    d. 订单状态 StatusRefunding -> StatusReturned -> StatusRefunded
func (s *RefundService) Approve(ctx context.Context, refundID uint, reviewerID uint, note string) error {
	// 1. 获取退款单
	refund, err := s.repo.GetByID(ctx, refundID)
	if err != nil {
		return fmt.Errorf("refund: get refund order failed: %w", err)
	}

	// 2. 验证状态
	if refund.Status != RefundPending {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidRefundStatus, RefundPending, refund.Status)
	}

	// 3. 在事务中执行退款（退款单状态变更+钱包入账+订单更新原子完成）
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 3a. CAS更新退款单: RefundPending -> RefundCompleted，同时写入审批信息
		now := time.Now().Unix()
		resultRefund := tx.WithContext(ctx).Model(&RefundOrder{}).
			Where("id = ? AND status = ?", refundID, RefundPending).
			Updates(map[string]interface{}{
				"status":      RefundCompleted,
				"reviewer_id": reviewerID,
				"review_note": note,
				"reviewed_at": now,
				"refunded_at": now,
			})
		if resultRefund.Error != nil {
			return fmt.Errorf("refund: update refund order status failed: %w", resultRefund.Error)
		}
		if resultRefund.RowsAffected == 0 {
			return fmt.Errorf("%w: CAS update refund status", ErrCASConflict)
		}

		// 3b. Credit退款金额到客户钱包
		creditNote := fmt.Sprintf("退款入账: %s", refund.RefundSN)
		if err := s.ledgerSvc.Credit(ctx, tx, refund.CustomerID, refund.Amount, "refund", refund.ID, creditNote); err != nil {
			return fmt.Errorf("refund: credit wallet failed: %w", err)
		}

		// 3c. 更新订单RefundAmount累加（带金额上限约束，防止并发超额退款）
		result := tx.WithContext(ctx).Model(&order.Order{}).
			Where("id = ? AND refund_amount + ? <= amount", refund.OrderID, refund.Amount).
			UpdateColumn("refund_amount", gorm.Expr("refund_amount + ?", refund.Amount))
		if result.Error != nil {
			return fmt.Errorf("refund: update order refund_amount failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: refund amount exceeds order amount", ErrRefundAmountExceeded)
		}

		// 3d. 订单状态转移: StatusRefunding -> StatusReturned -> StatusRefunded
		// 注意: 不使用TransitionStatus，因为它不支持事务参数，
		// 且转入StatusRefunded会触发processRefund造成重复入账。
		// 在事务内直接执行CAS状态转移 + 写入审计记录。

		// 重新读取订单获取当前状态和版本
		var ord order.Order
		if err := tx.WithContext(ctx).Where("id = ?", refund.OrderID).First(&ord).Error; err != nil {
			return fmt.Errorf("refund: reload order failed: %w", err)
		}

		// StatusRefunding -> StatusReturned
		if ord.Status == order.StatusRefunding {
			result := tx.WithContext(ctx).Model(&order.Order{}).
				Where("id = ? AND status = ? AND version = ?", ord.ID, order.StatusRefunding, ord.Version).
				Updates(map[string]interface{}{
					"status":  order.StatusReturned,
					"version": gorm.Expr("version + 1"),
				})
			if result.Error != nil {
				return fmt.Errorf("refund: transition to returned failed: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("%w: transition to returned", ErrCASConflict)
			}

			// 审计记录
			audit := &order.OrderStatusChange{
				OrderID:   ord.ID,
				OldStatus: order.StatusRefunding,
				NewStatus: order.StatusReturned,
				Operator:  fmt.Sprintf("admin:%d", reviewerID),
				Remark:    fmt.Sprintf("退款批准: %s", refund.RefundSN),
			}
			if err := tx.WithContext(ctx).Create(audit).Error; err != nil {
				return fmt.Errorf("refund: create audit record (returned) failed: %w", err)
			}

			ord.Version++ // 版本号已递增
		}

		// StatusReturned -> StatusRefunded
		result = tx.WithContext(ctx).Model(&order.Order{}).
			Where("id = ? AND status = ? AND version = ?", ord.ID, order.StatusReturned, ord.Version).
			Updates(map[string]interface{}{
				"status":  order.StatusRefunded,
				"version": gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return fmt.Errorf("refund: transition to refunded failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: transition to refunded", ErrCASConflict)
		}

		// 审计记录
		auditRefunded := &order.OrderStatusChange{
			OrderID:   ord.ID,
			OldStatus: order.StatusReturned,
			NewStatus: order.StatusRefunded,
			Operator:  fmt.Sprintf("admin:%d", reviewerID),
			Remark:    fmt.Sprintf("退款完成: %s", refund.RefundSN),
		}
		if err := tx.WithContext(ctx).Create(auditRefunded).Error; err != nil {
			return fmt.Errorf("refund: create audit record (refunded) failed: %w", err)
		}

		return nil
	})
}

// Reject 拒绝退款
// 1. 验证状态=RefundPending
// 2. 在事务中执行:
//    a. CAS更新退款单状态 RefundPending -> RefundRejected
//    b. 订单状态回退: StatusRefunding -> StatusProcessing
func (s *RefundService) Reject(ctx context.Context, refundID uint, reviewerID uint, note string) error {
	// 1. 获取退款单
	refund, err := s.repo.GetByID(ctx, refundID)
	if err != nil {
		return fmt.Errorf("refund: get refund order failed: %w", err)
	}

	// 2. 验证状态
	if refund.Status != RefundPending {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidRefundStatus, RefundPending, refund.Status)
	}

	// 3. 在事务中执行退款拒绝（退款单状态变更+订单状态回退原子完成）
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 3a. CAS更新退款单: RefundPending -> RefundRejected，同时写入审批信息
		now := time.Now().Unix()
		resultRefund := tx.WithContext(ctx).Model(&RefundOrder{}).
			Where("id = ? AND status = ?", refundID, RefundPending).
			Updates(map[string]interface{}{
				"status":      RefundRejected,
				"reviewer_id": reviewerID,
				"review_note": note,
				"reviewed_at": now,
			})
		if resultRefund.Error != nil {
			return fmt.Errorf("refund: update refund order to rejected failed: %w", resultRefund.Error)
		}
		if resultRefund.RowsAffected == 0 {
			return fmt.Errorf("%w: CAS update refund status to rejected", ErrCASConflict)
		}

		// 3b. 订单状态回退: StatusRefunding -> StatusProcessing
		var ord order.Order
		if err := tx.WithContext(ctx).Where("id = ?", refund.OrderID).First(&ord).Error; err != nil {
			return fmt.Errorf("refund: reload order failed: %w", err)
		}

		// 只有订单还在StatusRefunding时才回退
		if ord.Status != order.StatusRefunding {
			return nil
		}

		// CAS更新: StatusRefunding -> StatusProcessing
		result := tx.WithContext(ctx).Model(&order.Order{}).
			Where("id = ? AND status = ? AND version = ?", ord.ID, order.StatusRefunding, ord.Version).
			Updates(map[string]interface{}{
				"status":  order.StatusProcessing,
				"version": gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return fmt.Errorf("refund: revert order status failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: revert order status", ErrCASConflict)
		}

		// 审计记录
		audit := &order.OrderStatusChange{
			OrderID:   ord.ID,
			OldStatus: order.StatusRefunding,
			NewStatus: order.StatusProcessing,
			Operator:  fmt.Sprintf("admin:%d", reviewerID),
			Remark:    fmt.Sprintf("退款拒绝: %s, %s", refund.RefundSN, note),
		}
		if err := tx.WithContext(ctx).Create(audit).Error; err != nil {
			return fmt.Errorf("refund: create audit record failed: %w", err)
		}

		return nil
	})
}

// GetByID 查询退款单
func (s *RefundService) GetByID(ctx context.Context, id uint) (*RefundOrder, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByRefundSN 通过退款单号查询
func (s *RefundService) GetByRefundSN(ctx context.Context, sn string) (*RefundOrder, error) {
	return s.repo.GetByRefundSN(ctx, sn)
}

// List 分页查询客户退款单
func (s *RefundService) List(ctx context.Context, customerID uint, page, size int) ([]*RefundOrder, int64, error) {
	return s.repo.List(ctx, customerID, page, size)
}

// generateRefundSN 生成退款单号 (RF + 时间戳 + 随机hex)
func generateRefundSN() (string, error) {
	ts := time.Now().Format("20060102150405")
	b, err := crypto.GenerateRandomBytes(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("RF%s%s", ts, hex.EncodeToString(b)), nil
}
