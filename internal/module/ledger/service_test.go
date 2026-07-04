package ledger

import (
	"context"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB 创建隔离的SQLite内存数据库
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	// SQLite内存模式必须限制为单连接，否则每个连接各自独立数据库
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = AutoMigrate(db)
	require.NoError(t, err, "failed to auto migrate")

	return db
}

// newTestService 创建测试用的LedgerService
func newTestService(t *testing.T) (*LedgerService, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	svc := NewLedgerService(db)
	return svc, db
}

// ---------- TestEnsureWallet ----------

func TestEnsureWallet(t *testing.T) {
	tests := []struct {
		name   string
		userID uint
	}{
		{name: "创建新钱包", userID: 1},
		{name: "不同用户创建", userID: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t)
			ctx := context.Background()

			// 首次创建
			wallet, err := svc.EnsureWallet(ctx, tt.userID)
			require.NoError(t, err)
			assert.NotNil(t, wallet)
			assert.Equal(t, tt.userID, wallet.UserID)
			assert.True(t, wallet.Balance.Equal(decimal.Zero))
			assert.True(t, wallet.Frozen.Equal(decimal.Zero))

			// 重复调用应返回相同钱包
			wallet2, err := svc.EnsureWallet(ctx, tt.userID)
			require.NoError(t, err)
			assert.Equal(t, wallet.ID, wallet2.ID)
			assert.Equal(t, wallet.UserID, wallet2.UserID)
		})
	}
}

// ---------- TestDebit_Success ----------

func TestDebit_Success(t *testing.T) {
	tests := []struct {
		name        string
		initBalance decimal.Decimal
		debitAmount decimal.Decimal
		wantBalance decimal.Decimal
	}{
		{
			name:        "扣款100从余额500",
			initBalance: decimal.NewFromInt(500),
			debitAmount: decimal.NewFromInt(100),
			wantBalance: decimal.NewFromInt(400),
		},
		{
			name:        "扣款全部余额",
			initBalance: decimal.NewFromFloat(200.50),
			debitAmount: decimal.NewFromFloat(200.50),
			wantBalance: decimal.Zero,
		},
		{
			name:        "小金额扣款",
			initBalance: decimal.NewFromFloat(0.01),
			debitAmount: decimal.NewFromFloat(0.01),
			wantBalance: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newTestService(t)
			ctx := context.Background()

			// 准备：创建钱包并设初始余额
			wallet, err := svc.EnsureWallet(ctx, 1)
			require.NoError(t, err)

			// 直接更新余额用于测试
			db.Model(&Wallet{}).Where("id = ?", wallet.ID).Updates(map[string]interface{}{
				"balance": tt.initBalance,
			})

			// 执行扣款
			err = svc.Debit(ctx, db, 1, tt.debitAmount, "order_pay", 100, "测试扣款")
			require.NoError(t, err)

			// 验证余额
			balance, err := svc.GetBalance(ctx, 1)
			require.NoError(t, err)
			assert.True(t, balance.Equal(tt.wantBalance),
				"余额不匹配: got %s, want %s", balance.String(), tt.wantBalance.String())

			// 验证流水记录
			entries, total, err := svc.GetHistory(ctx, 1, 1, 10)
			require.NoError(t, err)
			assert.Equal(t, int64(1), total)
			assert.Len(t, entries, 1)
			assert.True(t, entries[0].Amount.Equal(tt.debitAmount.Neg()),
				"流水金额应为负数: got %s", entries[0].Amount.String())
			assert.True(t, entries[0].BalanceAfter.Equal(tt.wantBalance))
			assert.Equal(t, "order_pay", entries[0].Type)
			assert.Equal(t, uint(100), entries[0].RelatedID)
		})
	}
}

// ---------- TestDebit_InsufficientBalance ----------

func TestDebit_InsufficientBalance(t *testing.T) {
	tests := []struct {
		name        string
		initBalance decimal.Decimal
		debitAmount decimal.Decimal
	}{
		{
			name:        "余额不足-全额超出",
			initBalance: decimal.NewFromInt(50),
			debitAmount: decimal.NewFromInt(100),
		},
		{
			name:        "余额不足-差一分钱",
			initBalance: decimal.NewFromFloat(99.99),
			debitAmount: decimal.NewFromInt(100),
		},
		{
			name:        "零余额扣款",
			initBalance: decimal.Zero,
			debitAmount: decimal.NewFromInt(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newTestService(t)
			ctx := context.Background()

			// 准备钱包
			wallet, err := svc.EnsureWallet(ctx, 1)
			require.NoError(t, err)

			if tt.initBalance.GreaterThan(decimal.Zero) {
				db.Model(&Wallet{}).Where("id = ?", wallet.ID).Updates(map[string]interface{}{
					"balance": tt.initBalance,
				})
			}

			// 执行扣款应失败
			err = svc.Debit(ctx, db, 1, tt.debitAmount, "order_pay", 100, "测试扣款")
			assert.ErrorIs(t, err, ErrInsufficientBalance)

			// 验证余额未变化
			balance, err := svc.GetBalance(ctx, 1)
			require.NoError(t, err)
			assert.True(t, balance.Equal(tt.initBalance),
				"余额不应变化: got %s, want %s", balance.String(), tt.initBalance.String())

			// 验证无流水产生
			_, total, err := svc.GetHistory(ctx, 1, 1, 10)
			require.NoError(t, err)
			assert.Equal(t, int64(0), total)
		})
	}
}

// ---------- TestCredit_Success ----------

func TestCredit_Success(t *testing.T) {
	tests := []struct {
		name        string
		initBalance decimal.Decimal
		creditAmt   decimal.Decimal
		wantBalance decimal.Decimal
	}{
		{
			name:        "充值100到空钱包",
			initBalance: decimal.Zero,
			creditAmt:   decimal.NewFromInt(100),
			wantBalance: decimal.NewFromInt(100),
		},
		{
			name:        "充值到已有余额钱包",
			initBalance: decimal.NewFromInt(200),
			creditAmt:   decimal.NewFromFloat(50.55),
			wantBalance: decimal.NewFromFloat(250.55),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newTestService(t)
			ctx := context.Background()

			// 准备钱包
			wallet, err := svc.EnsureWallet(ctx, 1)
			require.NoError(t, err)

			if tt.initBalance.GreaterThan(decimal.Zero) {
				db.Model(&Wallet{}).Where("id = ?", wallet.ID).Updates(map[string]interface{}{
					"balance": tt.initBalance,
				})
			}

			// 执行入账
			err = svc.Credit(ctx, db, 1, tt.creditAmt, "recharge", 200, "测试充值")
			require.NoError(t, err)

			// 验证余额
			balance, err := svc.GetBalance(ctx, 1)
			require.NoError(t, err)
			assert.True(t, balance.Equal(tt.wantBalance),
				"余额不匹配: got %s, want %s", balance.String(), tt.wantBalance.String())

			// 验证流水记录
			entries, total, err := svc.GetHistory(ctx, 1, 1, 10)
			require.NoError(t, err)
			assert.Equal(t, int64(1), total)
			assert.Len(t, entries, 1)
			assert.True(t, entries[0].Amount.Equal(tt.creditAmt),
				"流水金额应为正数: got %s", entries[0].Amount.String())
			assert.True(t, entries[0].BalanceAfter.Equal(tt.wantBalance))
			assert.Equal(t, "recharge", entries[0].Type)
		})
	}
}

// ---------- TestConcurrentDebit ----------

func TestConcurrentDebit(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 准备钱包，初始余额1000
	_, err := svc.EnsureWallet(ctx, 1)
	require.NoError(t, err)

	initBalance := decimal.NewFromInt(1000)
	db.Model(&Wallet{}).Where("user_id = ?", 1).Updates(map[string]interface{}{
		"balance": initBalance,
	})

	// 10个goroutine并发扣款，每次扣10
	concurrency := 10
	debitAmount := decimal.NewFromInt(10)
	var wg sync.WaitGroup
	successCount := int32(0)
	var mu sync.Mutex

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			// 每次扣款独立使用db（SQLite不支持真正的并发事务，逐个重试）
			err := svc.Debit(ctx, db, 1, debitAmount, "order_pay", 100, "并发扣款")
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
			// CAS冲突是预期行为，忽略 ErrWalletCASConflict
		}()
	}
	wg.Wait()

	// 验证最终余额一致性：余额 = 初始余额 - (成功次数 * 扣款金额)
	balance, err := svc.GetBalance(ctx, 1)
	require.NoError(t, err)

	expectedBalance := initBalance.Sub(debitAmount.Mul(decimal.NewFromInt(int64(successCount))))
	assert.True(t, balance.Equal(expectedBalance),
		"并发扣款后余额不一致: got %s, expected %s (成功%d次)",
		balance.String(), expectedBalance.String(), successCount)

	// 验证流水数量等于成功次数
	_, total, err := svc.GetHistory(ctx, 1, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(successCount), total,
		"流水数量应等于成功扣款次数")

	// 余额不应为负数
	assert.True(t, balance.GreaterThanOrEqual(decimal.Zero), "余额不应为负数")

	t.Logf("并发扣款: %d次尝试, %d次成功, 最终余额: %s", concurrency, successCount, balance.String())
}

// ---------- 边界测试 ----------

func TestDebit_InvalidAmount(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name   string
		amount decimal.Decimal
	}{
		{name: "零金额", amount: decimal.Zero},
		{name: "负金额", amount: decimal.NewFromInt(-10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc2, db2 := newTestService(t)
			_ = svc
			_, _ = svc2.EnsureWallet(ctx, 1)
			err := svc2.Debit(ctx, db2, 1, tt.amount, "order_pay", 1, "test")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "must be positive")
		})
	}
}

func TestCredit_InvalidAmount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		amount decimal.Decimal
	}{
		{name: "零金额", amount: decimal.Zero},
		{name: "负金额", amount: decimal.NewFromInt(-10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := newTestService(t)
			_, _ = svc.EnsureWallet(ctx, 1)
			err := svc.Credit(ctx, db, 1, tt.amount, "recharge", 1, "test")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "must be positive")
		})
	}
}
