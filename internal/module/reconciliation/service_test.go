package reconciliation

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
)

// setupReconciliationTestDB 创建隔离的 SQLite 内存数据库并迁移对账相关表
func setupReconciliationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "打开测试数据库失败")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, ledger.AutoMigrate(db), "ledger 表迁移失败")
	require.NoError(t, order.AutoMigrate(db), "order 表迁移失败")
	require.NoError(t, Migrate(db), "reconciliation 表迁移失败")
	return db
}

// createWalletWithLedger 创建钱包及对应流水，返回钱包
func createWalletWithLedger(t *testing.T, db *gorm.DB, userID uint, balance string) *ledger.Wallet {
	t.Helper()
	wallet := &ledger.Wallet{
		UserID:  userID,
		Balance: decimal.RequireFromString(balance),
	}
	require.NoError(t, db.Create(wallet).Error, "创建钱包失败")
	return wallet
}

// createLedgerEntry 创建一条账本流水
func createLedgerEntry(t *testing.T, db *gorm.DB, walletID, userID uint, entryType, amount string) {
	t.Helper()
	entry := &ledger.LedgerEntry{
		WalletID:     walletID,
		UserID:       userID,
		Type:         entryType,
		Amount:       decimal.RequireFromString(amount),
		BalanceAfter: decimal.RequireFromString(amount),
	}
	require.NoError(t, db.Create(entry).Error, "创建账本流水失败")
}

// createTestOrder 创建测试订单
func createTestOrder(t *testing.T, db *gorm.DB, customerID uint, amount string) {
	t.Helper()
	staticTime := time.Now().Unix()
	orderSN := "TEST-" + time.Now().Format("20060102150405") + "-" + string(rune('A'+customerID))
	o := &order.Order{
		OrderSN:     orderSN,
		AppID:       1,
		CustomerID:  customerID,
		SupplierID:  1,
		GoodsID:     1,
		GoodsSN:     "G001",
		GoodsName:   "测试商品",
		BuyNumber:   1,
		UnitPrice:   decimal.RequireFromString(amount),
		Amount:      decimal.RequireFromString(amount),
		Status:      2,
		PaidAt:      &staticTime,
		CompletedAt: &staticTime,
	}
	require.NoError(t, db.Create(o).Error, "创建测试订单失败")
}

// waitTaskDone 轮询等待对账任务完成
func waitTaskDone(t *testing.T, svc *ReconciliationService, taskID uint) *ReconciliationTask {
	t.Helper()
	ctx := context.Background()
	var task *ReconciliationTask
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err = svc.GetTask(ctx, taskID)
		require.NoError(t, err)
		if task.Status == StatusCompleted || task.Status == StatusFailed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return task
}

// ---------- TestRunBalanceCheck_Match ----------

func TestRunBalanceCheck_Match(t *testing.T) {
	db := setupReconciliationTestDB(t)
	ctx := context.Background()
	svc := NewReconciliationService(db)

	wallet := createWalletWithLedger(t, db, 1, "100.000000")
	createLedgerEntry(t, db, wallet.ID, wallet.UserID, "recharge", "100.000000")

	task, err := svc.RunBalanceCheck(ctx)
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, StatusRunning, task.Status)

	completed := waitTaskDone(t, svc, task.ID)
	require.Equal(t, StatusCompleted, completed.Status)
	assert.Equal(t, 1, completed.TotalChecked)
	assert.Equal(t, 0, completed.ErrorCount)

	errs, total, err := svc.ListErrors(ctx, task.ID, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, errs)
}

// ---------- TestRunBalanceCheck_Mismatch ----------

func TestRunBalanceCheck_Mismatch(t *testing.T) {
	db := setupReconciliationTestDB(t)
	ctx := context.Background()
	svc := NewReconciliationService(db)

	// 钱包余额 100，但流水总和只有 80，应产生差异
	wallet := createWalletWithLedger(t, db, 2, "100.000000")
	createLedgerEntry(t, db, wallet.ID, wallet.UserID, "recharge", "80.000000")

	task, err := svc.RunBalanceCheck(ctx)
	require.NoError(t, err)
	require.NotNil(t, task)

	completed := waitTaskDone(t, svc, task.ID)
	require.Equal(t, StatusCompleted, completed.Status)
	assert.Equal(t, 1, completed.TotalChecked)
	assert.Equal(t, 1, completed.ErrorCount)

	errs, total, err := svc.ListErrors(ctx, task.ID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, errs, 1)

	recErr := errs[0]
	assert.Equal(t, ErrorBalanceMismatch, recErr.ErrorType)
	assert.Equal(t, uint(2), recErr.UserID)
	assert.True(t, recErr.Expected.Equal(decimal.RequireFromString("100.000000")),
		"Expected 应为 100，实际 %s", recErr.Expected)
	assert.True(t, recErr.Actual.Equal(decimal.RequireFromString("80.000000")),
		"Actual 应为 80，实际 %s", recErr.Actual)
	assert.True(t, recErr.Difference.Equal(decimal.RequireFromString("-20.000000")),
		"Difference 应为 -20，实际 %s", recErr.Difference)
	assert.Equal(t, ErrorStatusPending, recErr.Status)
}

// ---------- TestRunCrossVerify ----------

func TestRunCrossVerify(t *testing.T) {
	t.Run("订单金额与流水金额一致", func(t *testing.T) {
		db := setupReconciliationTestDB(t)
		ctx := context.Background()
		svc := NewReconciliationService(db)

		customerID := uint(10)
		wallet := createWalletWithLedger(t, db, customerID, "0.000000")
		createTestOrder(t, db, customerID, "100.000000")
		createLedgerEntry(t, db, wallet.ID, customerID, "order_pay", "-100.000000")

		task, err := svc.RunCrossVerify(ctx)
		require.NoError(t, err)
		require.NotNil(t, task)

		completed := waitTaskDone(t, svc, task.ID)
		require.Equal(t, StatusCompleted, completed.Status)
		assert.Equal(t, 1, completed.TotalChecked)
		assert.Equal(t, 0, completed.ErrorCount)

		errs, total, err := svc.ListErrors(ctx, task.ID, 1, 100)
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, errs)
	})

	t.Run("订单金额与流水金额不一致", func(t *testing.T) {
		db := setupReconciliationTestDB(t)
		ctx := context.Background()
		svc := NewReconciliationService(db)

		customerID := uint(11)
		wallet := createWalletWithLedger(t, db, customerID, "0.000000")
		createTestOrder(t, db, customerID, "100.000000")
		createLedgerEntry(t, db, wallet.ID, customerID, "order_pay", "-90.000000")

		task, err := svc.RunCrossVerify(ctx)
		require.NoError(t, err)
		require.NotNil(t, task)

		completed := waitTaskDone(t, svc, task.ID)
		require.Equal(t, StatusCompleted, completed.Status)
		assert.Equal(t, 1, completed.TotalChecked)
		assert.Equal(t, 1, completed.ErrorCount)

		errs, total, err := svc.ListErrors(ctx, task.ID, 1, 100)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, errs, 1)

		recErr := errs[0]
		assert.Equal(t, ErrorCrossMismatch, recErr.ErrorType)
		assert.Equal(t, customerID, recErr.UserID)
		assert.True(t, recErr.Expected.Equal(decimal.RequireFromString("100.000000")),
			"Expected 应为 100，实际 %s", recErr.Expected)
		assert.True(t, recErr.Actual.Equal(decimal.RequireFromString("90.000000")),
			"Actual 应为 90，实际 %s", recErr.Actual)
		assert.True(t, recErr.Difference.Equal(decimal.RequireFromString("10.000000")),
			"Difference 应为 10，实际 %s", recErr.Difference)
		assert.Equal(t, ErrorStatusPending, recErr.Status)
	})
}
