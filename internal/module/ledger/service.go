package ledger

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LedgerService 账本服务
type LedgerService struct {
	repo *LedgerRepository
	db   *gorm.DB
}

// NewLedgerService 创建LedgerService
func NewLedgerService(db *gorm.DB) *LedgerService {
	return &LedgerService{
		repo: NewLedgerRepository(db),
		db:   db,
	}
}

// EnsureWallet 确保用户有钱包（没有则创建）
func (s *LedgerService) EnsureWallet(ctx context.Context, userID uint) (*Wallet, error) {
	wallet, err := s.repo.GetWallet(ctx, userID)
	if err == nil {
		return wallet, nil
	}
	if err != ErrWalletNotFound {
		return nil, err
	}

	// 不存在则创建
	newWallet := &Wallet{
		UserID:  userID,
		Balance: decimal.Zero,
		Frozen:  decimal.Zero,
		Version: 0,
	}
	if err := s.repo.CreateWallet(ctx, newWallet); err != nil {
		// 可能并发创建，再尝试获取一次
		wallet, err2 := s.repo.GetWallet(ctx, userID)
		if err2 != nil {
			return nil, fmt.Errorf("ledger: create wallet failed: %w, retry get: %w", err, err2)
		}
		return wallet, nil
	}
	return newWallet, nil
}

// Debit 扣款（订单支付用）
// 在事务中：检查余额 → 创建流水（负数） → CAS更新Wallet
func (s *LedgerService) Debit(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("ledger: debit amount must be positive, got %s", amount.String())
	}

	// 获取钱包
	wallet, err := s.repo.GetWalletWithTx(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("ledger: get wallet failed: %w", err)
	}

	// 检查余额
	if wallet.Balance.LessThan(amount) {
		return ErrInsufficientBalance
	}

	// 计算新余额
	newBalance := wallet.Balance.Sub(amount)

	// CAS更新钱包余额
	ok, err := s.repo.UpdateWalletCAS(ctx, tx, wallet.ID, newBalance, wallet.Version)
	if err != nil {
		return fmt.Errorf("ledger: update wallet failed: %w", err)
	}
	if !ok {
		return ErrWalletCASConflict
	}

	// 创建流水记录（负数表示出账）
	entry := &LedgerEntry{
		WalletID:     wallet.ID,
		UserID:       userID,
		Type:         typ,
		RelatedID:    relatedID,
		Amount:       amount.Neg(), // 负数
		BalanceAfter: newBalance,
		Note:         note,
	}
	if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("ledger: create entry failed: %w", err)
	}

	return nil
}

// Credit 入账（充值/退款用）
func (s *LedgerService) Credit(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("ledger: credit amount must be positive, got %s", amount.String())
	}

	// 获取钱包
	wallet, err := s.repo.GetWalletWithTx(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("ledger: get wallet failed: %w", err)
	}

	// 计算新余额
	newBalance := wallet.Balance.Add(amount)

	// CAS更新钱包余额
	ok, err := s.repo.UpdateWalletCAS(ctx, tx, wallet.ID, newBalance, wallet.Version)
	if err != nil {
		return fmt.Errorf("ledger: update wallet failed: %w", err)
	}
	if !ok {
		return ErrWalletCASConflict
	}

	// 创建流水记录（正数表示入账）
	entry := &LedgerEntry{
		WalletID:     wallet.ID,
		UserID:       userID,
		Type:         typ,
		RelatedID:    relatedID,
		Amount:       amount, // 正数
		BalanceAfter: newBalance,
		Note:         note,
	}
	if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("ledger: create entry failed: %w", err)
	}

	return nil
}

// DebitWithRetry 带CAS重试的扣款方法（推荐调用入口）
// 最多重试maxRetries次，指数退避，全部失败后降级为行级锁
func (s *LedgerService) DebitWithRetry(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string, maxRetries int) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("ledger: debit amount must be positive, got %s", amount.String())
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// 读取最新wallet
		wallet, err := s.repo.GetWalletWithTx(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("ledger: get wallet failed: %w", err)
		}

		// 检查余额
		if wallet.Balance.LessThan(amount) {
			return ErrInsufficientBalance
		}

		// 计算新余额
		newBalance := wallet.Balance.Sub(amount)

		// CAS更新
		ok, err := s.repo.UpdateWalletCAS(ctx, tx, wallet.ID, newBalance, wallet.Version)
		if err != nil {
			return fmt.Errorf("ledger: update wallet failed: %w", err)
		}
		if ok {
			// CAS成功，创建流水记录
			entry := &LedgerEntry{
				WalletID:     wallet.ID,
				UserID:       userID,
				Type:         typ,
				RelatedID:    relatedID,
				Amount:       amount.Neg(),
				BalanceAfter: newBalance,
				Note:         note,
			}
			if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
				return fmt.Errorf("ledger: create entry failed: %w", err)
			}
			return nil
		}

		// CAS失败，指数退避重试
		lastErr = ErrWalletCASConflict
		if attempt < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 10 * time.Millisecond
			time.Sleep(backoff)
			log.Printf("[WARN] Wallet CAS conflict for user %d, retry %d/%d", userID, attempt+1, maxRetries)
		}
	}

	// 所有重试失败，降级为行级锁
	log.Printf("[WARN] CAS retries exhausted for user %d, falling back to row lock", userID)
	_ = lastErr
	return s.debitWithLock(ctx, tx, userID, amount, typ, relatedID, note)
}

// debitWithLock 降级方法：使用行级锁保证一致性
func (s *LedgerService) debitWithLock(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string) error {
	var wallet Wallet
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return fmt.Errorf("ledger: get wallet with lock failed: %w", err)
	}

	if wallet.Balance.LessThan(amount) {
		return ErrInsufficientBalance
	}

	newBalance := wallet.Balance.Sub(amount)
	if err := tx.WithContext(ctx).Model(&wallet).Updates(map[string]interface{}{
		"balance": newBalance,
		"version": wallet.Version + 1,
	}).Error; err != nil {
		return fmt.Errorf("ledger: update wallet failed: %w", err)
	}

	entry := &LedgerEntry{
		WalletID:     wallet.ID,
		UserID:       userID,
		Type:         typ,
		RelatedID:    relatedID,
		Amount:       amount.Neg(),
		BalanceAfter: newBalance,
		Note:         note,
	}
	if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("ledger: create entry failed: %w", err)
	}
	return nil
}

// CreditWithRetry 带CAS重试的入账方法（推荐调用入口）
// 最多重试maxRetries次，指数退避，全部失败后降级为行级锁
func (s *LedgerService) CreditWithRetry(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string, maxRetries int) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("ledger: credit amount must be positive, got %s", amount.String())
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// 读取最新wallet
		wallet, err := s.repo.GetWalletWithTx(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("ledger: get wallet failed: %w", err)
		}

		// 计算新余额
		newBalance := wallet.Balance.Add(amount)

		// CAS更新
		ok, err := s.repo.UpdateWalletCAS(ctx, tx, wallet.ID, newBalance, wallet.Version)
		if err != nil {
			return fmt.Errorf("ledger: update wallet failed: %w", err)
		}
		if ok {
			// CAS成功，创建流水记录
			entry := &LedgerEntry{
				WalletID:     wallet.ID,
				UserID:       userID,
				Type:         typ,
				RelatedID:    relatedID,
				Amount:       amount,
				BalanceAfter: newBalance,
				Note:         note,
			}
			if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
				return fmt.Errorf("ledger: create entry failed: %w", err)
			}
			return nil
		}

		// CAS失败，指数退避重试
		lastErr = ErrWalletCASConflict
		if attempt < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 10 * time.Millisecond
			time.Sleep(backoff)
			log.Printf("[WARN] Wallet CAS conflict for user %d (credit), retry %d/%d", userID, attempt+1, maxRetries)
		}
	}

	// 所有重试失败，降级为行级锁
	log.Printf("[WARN] CAS retries exhausted for user %d (credit), falling back to row lock", userID)
	_ = lastErr
	return s.creditWithLock(ctx, tx, userID, amount, typ, relatedID, note)
}

// creditWithLock 入账降级方法：使用行级锁保证一致性
func (s *LedgerService) creditWithLock(ctx context.Context, tx *gorm.DB, userID uint, amount decimal.Decimal, typ string, relatedID uint, note string) error {
	var wallet Wallet
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return fmt.Errorf("ledger: get wallet with lock failed: %w", err)
	}

	newBalance := wallet.Balance.Add(amount)
	if err := tx.WithContext(ctx).Model(&wallet).Updates(map[string]interface{}{
		"balance": newBalance,
		"version": wallet.Version + 1,
	}).Error; err != nil {
		return fmt.Errorf("ledger: update wallet failed: %w", err)
	}

	entry := &LedgerEntry{
		WalletID:     wallet.ID,
		UserID:       userID,
		Type:         typ,
		RelatedID:    relatedID,
		Amount:       amount,
		BalanceAfter: newBalance,
		Note:         note,
	}
	if err := s.repo.CreateEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("ledger: create entry failed: %w", err)
	}
	return nil
}

// GetBalance 查余额
func (s *LedgerService) GetBalance(ctx context.Context, userID uint) (decimal.Decimal, error) {
	wallet, err := s.repo.GetWallet(ctx, userID)
	if err != nil {
		if err == ErrWalletNotFound {
			return decimal.Zero, nil
		}
		return decimal.Zero, err
	}
	return wallet.Balance, nil
}

// GetHistory 查流水
func (s *LedgerService) GetHistory(ctx context.Context, userID uint, page, size int) ([]*LedgerEntry, int64, error) {
	return s.repo.GetHistory(ctx, userID, page, size)
}
