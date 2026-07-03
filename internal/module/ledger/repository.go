package ledger

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	ErrWalletNotFound      = errors.New("ledger: wallet not found")
	ErrInsufficientBalance = errors.New("ledger: insufficient balance")
	ErrWalletCASConflict   = errors.New("ledger: wallet CAS conflict, concurrent modification detected")
)

// LedgerRepository 账本数据访问层
type LedgerRepository struct {
	db *gorm.DB
}

// NewLedgerRepository 创建LedgerRepository
func NewLedgerRepository(db *gorm.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// CreateEntry 创建流水记录（只INSERT不UPDATE）
func (r *LedgerRepository) CreateEntry(ctx context.Context, tx *gorm.DB, entry *LedgerEntry) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.WithContext(ctx).Create(entry).Error
}

// GetWallet 获取用户钱包
func (r *LedgerRepository) GetWallet(ctx context.Context, userID uint) (*Wallet, error) {
	var wallet Wallet
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

// GetWalletWithTx 在事务中获取用户钱包（加行锁）
func (r *LedgerRepository) GetWalletWithTx(ctx context.Context, tx *gorm.DB, userID uint) (*Wallet, error) {
	var wallet Wallet
	err := tx.WithContext(ctx).Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

// CreateWallet 创建钱包
func (r *LedgerRepository) CreateWallet(ctx context.Context, wallet *Wallet) error {
	return r.db.WithContext(ctx).Create(wallet).Error
}

// UpdateWalletCAS CAS乐观锁更新钱包余额
// UPDATE wallets SET balance=?, version=version+1 WHERE id=? AND version=?
// 返回 bool 表示是否成功（RowsAffected > 0）
func (r *LedgerRepository) UpdateWalletCAS(ctx context.Context, tx *gorm.DB, walletID uint, newBalance decimal.Decimal, version int64) (bool, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	result := db.WithContext(ctx).
		Model(&Wallet{}).
		Where("id = ? AND version = ?", walletID, version).
		Updates(map[string]interface{}{
			"balance": newBalance,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// GetHistory 分页查询流水记录
func (r *LedgerRepository) GetHistory(ctx context.Context, userID uint, page, size int) ([]*LedgerEntry, int64, error) {
	query := r.db.WithContext(ctx).Model(&LedgerEntry{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var entries []*LedgerEntry
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}
