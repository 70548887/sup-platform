package recharge

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/pkg/crypto"
)

var (
	ErrAmountMustBePositive = errors.New("recharge: amount must be positive")
	ErrAlreadyProcessed     = errors.New("recharge: already processed")
	ErrInvalidStatus        = errors.New("recharge: invalid status for this operation")
)

// RechargeService 充值服务
type RechargeService struct {
	repo      *RechargeRepository
	db        *gorm.DB
	ledgerSvc *ledger.LedgerService
}

// NewRechargeService 创建RechargeService
func NewRechargeService(db *gorm.DB, ledgerSvc *ledger.LedgerService) *RechargeService {
	return &RechargeService{
		repo:      NewRechargeRepository(db),
		db:        db,
		ledgerSvc: ledgerSvc,
	}
}

// Apply 申请充值
// 1. 检查idempotencyKey是否已存在（如已存在返回现有记录，幂等）
// 2. 验证amount > 0
// 3. 生成RechargeSN: "RCH" + 时间戳 + 随机hex
// 4. 创建RechargeOrder(status=StatusPending)
func (s *RechargeService) Apply(ctx context.Context, userID uint, amount decimal.Decimal, idempotencyKey string) (*RechargeOrder, error) {
	// 1. 幂等检查
	if idempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("recharge: check idempotency key failed: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	// 2. 验证金额
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrAmountMustBePositive
	}

	// 3. 生成RechargeSN
	rechargeSN, err := generateRechargeSN()
	if err != nil {
		return nil, fmt.Errorf("recharge: generate recharge_sn failed: %w", err)
	}

	// 4. 创建充值单
	order := &RechargeOrder{
		RechargeSN:     rechargeSN,
		UserID:         userID,
		Amount:         amount,
		Status:         StatusPending,
		IdempotencyKey: idempotencyKey,
	}
	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("recharge: create order failed: %w", err)
	}

	return order, nil
}

// Approve 批准充值（直接到账）
// 1. GetByID获取充值单
// 2. 验证status==StatusPending
// 3. 在db.Transaction中执行:
//   - CAS更新: status=StatusPending → StatusCompleted, version+1, approverID, approvedAt=now
//   - ledgerSvc.Credit(ctx, tx, userID, amount, "recharge", rechargeID, "充值到账")
//
// 4. 如果CAS失败返回"already processed"幂等错误
func (s *RechargeService) Approve(ctx context.Context, rechargeID uint, approverID uint) error {
	// 1. 获取充值单
	rechargeOrder, err := s.repo.GetByID(ctx, rechargeID)
	if err != nil {
		return fmt.Errorf("recharge: get order failed: %w", err)
	}

	// 2. 验证状态
	if rechargeOrder.Status != StatusPending {
		return fmt.Errorf("%w: expected status %d, got %d", ErrInvalidStatus, StatusPending, rechargeOrder.Status)
	}

	// 3. 在事务中执行批准+入账
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 3a. CAS更新状态: StatusPending → StatusCompleted
		now := time.Now().Unix()
		updates := map[string]interface{}{
			"approver_id": approverID,
			"approved_at": now,
		}
		ok, err := s.repo.UpdateStatusCAS(ctx, tx, rechargeOrder.ID, StatusPending, StatusCompleted, rechargeOrder.Version, updates)
		if err != nil {
			return fmt.Errorf("recharge: CAS update failed: %w", err)
		}
		if !ok {
			return ErrAlreadyProcessed
		}

		// 3b. 调用ledger入账
		if err := s.ledgerSvc.Credit(ctx, tx, rechargeOrder.UserID, rechargeOrder.Amount, "recharge", rechargeOrder.ID, "充值到账"); err != nil {
			return fmt.Errorf("recharge: credit wallet failed: %w", err)
		}

		return nil
	})
}

// Reject 拒绝充值
// 1. GetByID获取充值单
// 2. 验证status==StatusPending
// 3. CAS更新: status=StatusPending → StatusRejected, version+1, approverID, approvalNote, approvedAt=now
func (s *RechargeService) Reject(ctx context.Context, rechargeID uint, approverID uint, note string) error {
	// 1. 获取充值单
	rechargeOrder, err := s.repo.GetByID(ctx, rechargeID)
	if err != nil {
		return fmt.Errorf("recharge: get order failed: %w", err)
	}

	// 2. 验证状态
	if rechargeOrder.Status != StatusPending {
		return fmt.Errorf("%w: expected status %d, got %d", ErrInvalidStatus, StatusPending, rechargeOrder.Status)
	}

	// 3. CAS更新为拒绝
	now := time.Now().Unix()
	updates := map[string]interface{}{
		"approver_id":  approverID,
		"approval_note": note,
		"approved_at":  now,
	}
	ok, err := s.repo.UpdateStatusCAS(ctx, nil, rechargeOrder.ID, StatusPending, StatusRejected, rechargeOrder.Version, updates)
	if err != nil {
		return fmt.Errorf("recharge: CAS update failed: %w", err)
	}
	if !ok {
		return ErrAlreadyProcessed
	}

	return nil
}

// GetByID 查询充值单
func (s *RechargeService) GetByID(ctx context.Context, id uint) (*RechargeOrder, error) {
	return s.repo.GetByID(ctx, id)
}

// List 分页查询充值单
func (s *RechargeService) List(ctx context.Context, userID uint, status *int8, page, size int) ([]*RechargeOrder, int64, error) {
	return s.repo.List(ctx, userID, status, page, size)
}

// generateRechargeSN 生成充值单号 (RCH + 时间戳 + 随机hex)
func generateRechargeSN() (string, error) {
	ts := time.Now().Format("20060102150405")
	b, err := crypto.GenerateRandomBytes(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("RCH%s%s", ts, hex.EncodeToString(b)), nil
}
