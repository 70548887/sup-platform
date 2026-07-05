package settlement

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupSettlementTestDB 创建隔离的SQLite内存数据库并迁移settlement表
func setupSettlementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "打开测试数据库失败")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = Migrate(db)
	require.NoError(t, err, "settlement表迁移失败")
	return db
}

// newSettlementTestService 创建测试用SettlementService，注入mock aggregateFn
func newSettlementTestService(t *testing.T, db *gorm.DB, aggFn func(uint, string) (*AggregateResult, error)) *SettlementService {
	t.Helper()
	svc := NewSettlementService(db)
	svc.aggregateFn = aggFn
	return svc
}

// createTestSettlement 直接在DB中创建结算单（用于状态流转测试）
func createTestSettlement(t *testing.T, db *gorm.DB, supplierID uint, period string, amount decimal.Decimal, status string) *Settlement {
	t.Helper()
	commissionAmt := amount.Mul(DefaultCommissionRate).Round(6)
	netAmt := amount.Sub(commissionAmt)
	st := &Settlement{
		TenantID:         1,
		SupplierID:       supplierID,
		Period:           period,
		TotalOrders:      10,
		TotalAmount:      amount,
		CommissionRate:   DefaultCommissionRate,
		CommissionAmount: commissionAmt,
		NetAmount:        netAmt,
		Status:           status,
	}
	err := db.Create(st).Error
	require.NoError(t, err, "创建测试结算单失败")
	return st
}

// ---------- TestGenerateSettlement_Success ----------

func TestGenerateSettlement_Success(t *testing.T) {
	db := setupSettlementTestDB(t)
	ctx := context.Background()

	supplierID := uint(101)
	period := "2026-07"
	// 模拟聚合：10笔订单，总金额 1234.567890
	mockAgg := &AggregateResult{
		TotalOrders: 10,
		TotalAmount: decimal.RequireFromString("1234.567890"),
	}
	svc := newSettlementTestService(t, db, func(sid uint, p string) (*AggregateResult, error) {
		assert.Equal(t, supplierID, sid)
		assert.Equal(t, period, p)
		return mockAgg, nil
	})

	st, err := svc.GenerateSettlement(ctx, supplierID, period)
	require.NoError(t, err)
	require.NotNil(t, st)

	// 验证基本字段
	assert.Equal(t, supplierID, st.SupplierID)
	assert.Equal(t, period, st.Period)
	assert.Equal(t, 10, st.TotalOrders)
	assert.Equal(t, "pending", st.Status)
	assert.NotZero(t, st.ID)

	// 验证金额精度（佣金率 5%）
	// TotalAmount = 1234.567890
	// CommissionAmount = 1234.567890 * 0.05 = 61.7283945 → Round(6) = 61.728395
	// NetAmount = 1234.567890 - 61.728395 = 1172.839495
	expectedTotal := decimal.RequireFromString("1234.567890")
	expectedCommission := decimal.RequireFromString("61.728395")
	expectedNet := decimal.RequireFromString("1172.839495")

	assert.True(t, st.TotalAmount.Equal(expectedTotal),
		"TotalAmount应为%s，实际%s", expectedTotal, st.TotalAmount)
	assert.True(t, st.CommissionAmount.Equal(expectedCommission),
		"CommissionAmount应为%s，实际%s", expectedCommission, st.CommissionAmount)
	assert.True(t, st.NetAmount.Equal(expectedNet),
		"NetAmount应为%s，实际%s", expectedNet, st.NetAmount)

	// 验证佣金率
	assert.True(t, st.CommissionRate.Equal(DefaultCommissionRate),
		"佣金率应为0.05，实际%s", st.CommissionRate)

	// 验证已持久化
	var count int64
	db.Model(&Settlement{}).Count(&count)
	assert.Equal(t, int64(1), count, "应有1条结算单记录")
}

// ---------- TestGenerateSettlement_DuplicatePeriod ----------

func TestGenerateSettlement_DuplicatePeriod(t *testing.T) {
	db := setupSettlementTestDB(t)
	ctx := context.Background()

	supplierID := uint(202)
	period := "2026-06"
	mockAgg := &AggregateResult{
		TotalOrders: 5,
		TotalAmount: decimal.RequireFromString("500.000000"),
	}
	svc := newSettlementTestService(t, db, func(sid uint, p string) (*AggregateResult, error) {
		return mockAgg, nil
	})

	// 第一次生成：成功
	st1, err := svc.GenerateSettlement(ctx, supplierID, period)
	require.NoError(t, err)
	require.NotNil(t, st1)

	// 第二次同供应商同周期生成：应失败（唯一索引冲突）
	st2, err := svc.GenerateSettlement(ctx, supplierID, period)
	assert.Error(t, err, "同一供货商同一周期重复生成应返回错误")
	assert.Nil(t, st2)

	// 验证DB中仍只有1条记录
	var count int64
	db.Model(&Settlement{}).Where("supplier_id = ? AND period = ?", supplierID, period).Count(&count)
	assert.Equal(t, int64(1), count, "重复生成后应仍只有1条记录")

	// 不同周期应能正常生成
	st3, err := svc.GenerateSettlement(ctx, supplierID, "2026-08")
	require.NoError(t, err)
	require.NotNil(t, st3)

	db.Model(&Settlement{}).Where("supplier_id = ?", supplierID).Count(&count)
	assert.Equal(t, int64(2), count, "不同周期应生成第2条记录")
}

// ---------- TestConfirmSettlement_Success ----------

func TestConfirmSettlement_Success(t *testing.T) {
	db := setupSettlementTestDB(t)
	ctx := context.Background()
	svc := NewSettlementService(db)

	// 创建 pending 状态的结算单
	st := createTestSettlement(t, db, 301, "2026-07", decimal.RequireFromString("2000.000000"), "pending")

	// 确认结算单
	err := svc.ConfirmSettlement(ctx, st.ID)
	require.NoError(t, err)

	// 重新查询验证状态变更
	var updated Settlement
	err = db.First(&updated, st.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "confirmed", updated.Status, "状态应变为confirmed")
	assert.NotNil(t, updated.ConfirmedAt, "ConfirmedAt应有值")
	assert.Greater(t, *updated.ConfirmedAt, int64(0), "ConfirmedAt应为有效时间戳")

	// 再次确认应失败：非pending状态
	err = svc.ConfirmSettlement(ctx, st.ID)
	assert.Error(t, err, "非pending状态的结算单不应允许再次确认")
	assert.Contains(t, err.Error(), "pending")
}

// ---------- TestMarkPaid_Success ----------

func TestMarkPaid_Success(t *testing.T) {
	db := setupSettlementTestDB(t)
	ctx := context.Background()
	svc := NewSettlementService(db)

	// 测试完整状态流转：pending → confirmed → paid
	st := createTestSettlement(t, db, 401, "2026-07", decimal.RequireFromString("3000.000000"), "pending")

	// 直接MarkPaid应失败（需要先confirm）
	err := svc.MarkPaid(ctx, st.ID)
	assert.Error(t, err, "pending状态不应允许直接标记付款")
	assert.Contains(t, err.Error(), "confirmed")

	// 先确认
	err = svc.ConfirmSettlement(ctx, st.ID)
	require.NoError(t, err)

	// 再标记付款
	err = svc.MarkPaid(ctx, st.ID)
	require.NoError(t, err)

	// 验证最终状态
	var updated Settlement
	err = db.First(&updated, st.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "paid", updated.Status, "最终状态应为paid")
	assert.NotNil(t, updated.PaidAt, "PaidAt应有值")
	assert.Greater(t, *updated.PaidAt, int64(0), "PaidAt应为有效时间戳")

	// 再次MarkPaid应失败
	err = svc.MarkPaid(ctx, st.ID)
	assert.Error(t, err, "paid状态不应允许再次标记付款")
}

// ---------- TestCalculateProfitShare ----------

func TestCalculateProfitShare(t *testing.T) {
	db := setupSettlementTestDB(t)
	ctx := context.Background()
	svc := NewSettlementService(db)

	t.Run("正常计算分润-整数金额", func(t *testing.T) {
		orderID := uint(1001)
		supplierID := uint(501)
		orderAmount := decimal.RequireFromString("1000.000000")

		ps, err := svc.CalculateProfitShare(ctx, orderID, supplierID, orderAmount)
		require.NoError(t, err)
		require.NotNil(t, ps)

		// 平台佣金 = 1000 * 0.05 = 50.000000
		expectedPlatform := decimal.RequireFromString("50.000000")
		// 供货商利润 = 1000 - 50 = 950.000000
		expectedSupplier := decimal.RequireFromString("950.000000")

		assert.True(t, ps.PlatformProfit.Equal(expectedPlatform),
			"平台利润应为%s，实际%s", expectedPlatform, ps.PlatformProfit)
		assert.True(t, ps.SupplierProfit.Equal(expectedSupplier),
			"供货商利润应为%s，实际%s", expectedSupplier, ps.SupplierProfit)
		assert.True(t, ps.PlatformRate.Equal(DefaultCommissionRate),
			"佣金率应为0.05")
		assert.Equal(t, orderID, ps.OrderID)
		assert.Equal(t, supplierID, ps.SupplierID)
		assert.NotZero(t, ps.ID, "分润记录应已持久化")
	})

	t.Run("小数金额精度验证", func(t *testing.T) {
		orderID := uint(1002)
		supplierID := uint(501)
		// 使用含多位小数的金额，验证decimal精度
		orderAmount := decimal.RequireFromString("199.990000")

		ps, err := svc.CalculateProfitShare(ctx, orderID, supplierID, orderAmount)
		require.NoError(t, err)

		// 平台佣金 = 199.990000 * 0.05 = 9.99950000 → Round(6) = 9.999500
		expectedPlatform := decimal.RequireFromString("9.999500")
		// 供货商利润 = 199.990000 - 9.999500 = 189.990500
		expectedSupplier := decimal.RequireFromString("189.990500")

		assert.True(t, ps.PlatformProfit.Equal(expectedPlatform),
			"平台利润应为%s，实际%s", expectedPlatform, ps.PlatformProfit)
		assert.True(t, ps.SupplierProfit.Equal(expectedSupplier),
			"供货商利润应为%s，实际%s", expectedSupplier, ps.SupplierProfit)

		// 验证：平台利润 + 供货商利润 = 订单金额
		sum := ps.PlatformProfit.Add(ps.SupplierProfit)
		assert.True(t, sum.Equal(orderAmount),
			"平台利润+供货商利润应等于订单金额：%s + %s = %s，期望%s",
			ps.PlatformProfit, ps.SupplierProfit, sum, orderAmount)
	})

	t.Run("幂等性-同一订单重复调用返回已有记录", func(t *testing.T) {
		orderID := uint(1003)
		supplierID := uint(501)
		orderAmount := decimal.RequireFromString("500.000000")

		ps1, err := svc.CalculateProfitShare(ctx, orderID, supplierID, orderAmount)
		require.NoError(t, err)

		// 使用不同金额再次调用同一orderID
		ps2, err := svc.CalculateProfitShare(ctx, orderID, supplierID, decimal.RequireFromString("999.000000"))
		require.NoError(t, err)

		// 应返回第一次创建的记录（幂等）
		assert.Equal(t, ps1.ID, ps2.ID, "同一orderID应返回相同记录（幂等）")
		assert.True(t, ps2.OrderAmount.Equal(orderAmount),
			"幂等返回的金额应为首次创建的金额")
	})
}
