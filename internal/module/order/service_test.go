package order

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/ledger"
)

// setupOrderTestDB 创建隔离SQLite内存数据库，迁移order+ledger表
func setupOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移ledger模块表（order依赖ledger扣款）
	err = ledger.AutoMigrate(db)
	require.NoError(t, err, "failed to migrate ledger tables")

	// 迁移order模块表
	err = AutoMigrate(db)
	require.NoError(t, err, "failed to migrate order tables")

	return db
}

// newOrderTestService 创建测试用的OrderService及其依赖
func newOrderTestService(t *testing.T) (*OrderService, *ledger.LedgerService, *gorm.DB) {
	t.Helper()
	db := setupOrderTestDB(t)
	ledgerSvc := ledger.NewLedgerService(db)
	orderSvc := NewOrderService(db, ledgerSvc)
	return orderSvc, ledgerSvc, db
}

// prepareCustomerWallet 创建客户钱包并充值到指定金额
func prepareCustomerWallet(t *testing.T, ledgerSvc *ledger.LedgerService, db *gorm.DB, customerID uint, balance decimal.Decimal) {
	t.Helper()
	ctx := context.Background()
	_, err := ledgerSvc.EnsureWallet(ctx, customerID)
	require.NoError(t, err)
	if balance.GreaterThan(decimal.Zero) {
		err = ledgerSvc.Credit(ctx, db, customerID, balance, "test_topup", 0, "测试充值")
		require.NoError(t, err)
	}
}

// ---------- TestCreateOrder_Success ----------

func TestCreateOrder_Success(t *testing.T) {
	orderSvc, ledgerSvc, db := newOrderTestService(t)
	ctx := context.Background()

	// 准备：给客户充值1000
	prepareCustomerWallet(t, ledgerSvc, db, 1, decimal.NewFromInt(1000))

	params := CreateOrderParams{
		AppID:           1,
		CustomerID:      1,
		SupplierID:      10,
		CustomerOrderID: "CUST-ORDER-001",
		GoodsID:         100,
		GoodsSN:         "G100",
		GoodsName:       "测试商品",
		BuyNumber:       2,
		UnitPrice:       decimal.NewFromFloat(50.5),
		BuyParams:       map[string]string{"phone": "13800138000"},
		NotifyURL:       "https://example.com/callback",
	}

	order, err := orderSvc.CreateOrder(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, order)

	// 验证订单字段
	assert.Equal(t, params.CustomerID, order.CustomerID)
	assert.Equal(t, params.SupplierID, order.SupplierID)
	assert.Equal(t, params.GoodsID, order.GoodsID)
	assert.Equal(t, params.GoodsSN, order.GoodsSN)
	assert.Equal(t, params.GoodsName, order.GoodsName)
	assert.Equal(t, params.BuyNumber, order.BuyNumber)
	assert.True(t, order.UnitPrice.Equal(params.UnitPrice))
	// Amount = UnitPrice * BuyNumber = 50.5 * 2 = 101
	expectedAmount := decimal.NewFromFloat(101)
	assert.True(t, order.Amount.Equal(expectedAmount),
		"Amount应为%s，实际为%s", expectedAmount.String(), order.Amount.String())
	assert.Equal(t, StatusPaid, order.Status)
	assert.Equal(t, int64(0), order.Version)
	assert.NotEmpty(t, order.OrderSN)
	assert.Equal(t, params.CustomerOrderID, order.CustomerOrderID)

	// 验证余额已扣减
	balance, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	expectedBalance := decimal.NewFromInt(1000).Sub(expectedAmount)
	assert.True(t, balance.Equal(expectedBalance),
		"余额应为%s，实际为%s", expectedBalance.String(), balance.String())
}

// ---------- TestCreateOrder_Idempotent ----------

func TestCreateOrder_Idempotent(t *testing.T) {
	orderSvc, ledgerSvc, db := newOrderTestService(t)
	ctx := context.Background()

	// 准备：给客户充值2000
	prepareCustomerWallet(t, ledgerSvc, db, 1, decimal.NewFromInt(2000))

	params := CreateOrderParams{
		AppID:           1,
		CustomerID:      1,
		SupplierID:      10,
		CustomerOrderID: "IDEMPOTENT-001",
		GoodsID:         100,
		GoodsSN:         "G100",
		GoodsName:       "测试商品",
		BuyNumber:       1,
		UnitPrice:       decimal.NewFromInt(100),
	}

	// 第一次创建
	order1, err := orderSvc.CreateOrder(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, order1)

	// 第二次同CustomerOrderID创建应返回相同订单
	order2, err := orderSvc.CreateOrder(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, order2)

	// 应返回同一个订单
	assert.Equal(t, order1.ID, order2.ID)
	assert.Equal(t, order1.OrderSN, order2.OrderSN)

	// 验证余额只扣了一次（100）
	balance, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	expectedBalance := decimal.NewFromInt(1900)
	assert.True(t, balance.Equal(expectedBalance),
		"幂等调用不应重复扣款: 余额应为%s，实际为%s", expectedBalance.String(), balance.String())
}

// ---------- TestTransitionStatus_ValidTransitions ----------

func TestTransitionStatus_ValidTransitions(t *testing.T) {
	tests := []struct {
		name      string
		from      int8
		to        int8
	}{
		{"Paid→Processing", StatusPaid, StatusProcessing},
		{"Paid→Pending", StatusPaid, StatusPending},
		{"Processing→Completed", StatusProcessing, StatusCompleted},
		{"Processing→Replenishing", StatusProcessing, StatusReplenishing},
		{"Processing→Refunding", StatusProcessing, StatusRefunding},
		{"Replenishing→Processing", StatusReplenishing, StatusProcessing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderSvc, ledgerSvc, db := newOrderTestService(t)
			ctx := context.Background()

			// 准备钱包
			prepareCustomerWallet(t, ledgerSvc, db, 1, decimal.NewFromInt(1000))

			// 创建订单（初始状态StatusPaid）
			params := CreateOrderParams{
				AppID:           1,
				CustomerID:      1,
				SupplierID:      10,
				CustomerOrderID: "",
				GoodsID:         100,
				GoodsSN:         "G100",
				GoodsName:       "测试商品",
				BuyNumber:       1,
				UnitPrice:       decimal.NewFromInt(10),
			}
			order, err := orderSvc.CreateOrder(ctx, params)
			require.NoError(t, err)

			// 如果测试起始状态不是StatusPaid，需要先转移到目标起始状态
			if tt.from != StatusPaid {
				// 从Paid先到中间状态
				intermediates := getPathTo(StatusPaid, tt.from)
				for _, intermediate := range intermediates {
					err = orderSvc.TransitionStatus(ctx, order.ID, intermediate, "test", "测试前置转移")
					require.NoError(t, err, "前置转移失败: →%d", intermediate)
				}
			}

			// 执行目标状态转移
			err = orderSvc.TransitionStatus(ctx, order.ID, tt.to, "test", "测试转移")
			assert.NoError(t, err, "合法转移应成功: %d→%d", tt.from, tt.to)

			// 验证状态已更新
			updated, err := orderSvc.GetOrderByID(ctx, order.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.to, updated.Status)
		})
	}
}

// getPathTo 获取从当前状态到目标状态的路径（简单映射）
func getPathTo(from, to int8) []int8 {
	// 简单状态路径映射
	paths := map[[2]int8][]int8{
		{StatusPaid, StatusPending}:      {StatusPending},
		{StatusPaid, StatusProcessing}:   {StatusProcessing},
		{StatusPaid, StatusReplenishing}: {StatusProcessing, StatusReplenishing},
		{StatusPaid, StatusRefunding}:    {StatusProcessing, StatusRefunding},
		{StatusPaid, StatusAbnormal}:     {StatusProcessing, StatusAbnormal},
		{StatusPaid, StatusCompleted}:    {StatusProcessing, StatusCompleted},
	}
	key := [2]int8{from, to}
	if path, ok := paths[key]; ok {
		return path
	}
	return nil
}

// ---------- TestTransitionStatus_InvalidTransition ----------

func TestTransitionStatus_InvalidTransition(t *testing.T) {
	tests := []struct {
		name string
		from int8
		to   int8
	}{
		{"Paid→Completed(非法)", StatusPaid, StatusCompleted},
		{"Paid→Refunded(非法)", StatusPaid, StatusRefunded},
		{"Completed→Processing(终态不可转移)", StatusCompleted, StatusProcessing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderSvc, ledgerSvc, db := newOrderTestService(t)
			ctx := context.Background()

			prepareCustomerWallet(t, ledgerSvc, db, 1, decimal.NewFromInt(1000))

			params := CreateOrderParams{
				AppID:      1,
				CustomerID: 1,
				SupplierID: 10,
				GoodsID:    100,
				GoodsSN:    "G100",
				GoodsName:  "测试商品",
				BuyNumber:  1,
				UnitPrice:  decimal.NewFromInt(10),
			}
			order, err := orderSvc.CreateOrder(ctx, params)
			require.NoError(t, err)

			// 如果需要先转到特定状态
			if tt.from != StatusPaid {
				intermediates := getPathTo(StatusPaid, tt.from)
				for _, intermediate := range intermediates {
					err = orderSvc.TransitionStatus(ctx, order.ID, intermediate, "test", "前置")
					require.NoError(t, err)
				}
			}

			// 执行非法转移
			err = orderSvc.TransitionStatus(ctx, order.ID, tt.to, "test", "非法转移")
			assert.Error(t, err, "非法转移应返回错误: %d→%d", tt.from, tt.to)
			assert.ErrorIs(t, err, ErrInvalidTransition)
		})
	}
}

// ---------- TestTransitionStatus_CASConflict ----------

func TestTransitionStatus_CASConflict(t *testing.T) {
	orderSvc, ledgerSvc, db := newOrderTestService(t)
	ctx := context.Background()

	prepareCustomerWallet(t, ledgerSvc, db, 1, decimal.NewFromInt(1000))

	params := CreateOrderParams{
		AppID:      1,
		CustomerID: 1,
		SupplierID: 10,
		GoodsID:    100,
		GoodsSN:    "G100",
		GoodsName:  "测试商品",
		BuyNumber:  1,
		UnitPrice:  decimal.NewFromInt(10),
	}
	order, err := orderSvc.CreateOrder(ctx, params)
	require.NoError(t, err)

	// 第一次转移：Paid→Processing（成功，version从0变为1）
	err = orderSvc.TransitionStatus(ctx, order.ID, StatusProcessing, "user1", "正常转移")
	require.NoError(t, err)

	// 模拟并发：直接用旧version更新（模拟另一个进程同时操作）
	// 手动将version回退来模拟并发冲突场景
	db.Model(&Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"version": 999, // 设置一个不同的version
	})

	// 再次转移：由于内部读取的version和DB中的version不匹配（TransitionStatus内部会重新读取）
	// 但实际上TransitionStatus每次都重新读取order，所以我们需要另一种方式来测试CAS冲突

	// 重新设置：创建新订单来测试真正的CAS冲突
	params2 := CreateOrderParams{
		AppID:      1,
		CustomerID: 1,
		SupplierID: 10,
		GoodsID:    101,
		GoodsSN:    "G101",
		GoodsName:  "测试CAS",
		BuyNumber:  1,
		UnitPrice:  decimal.NewFromInt(10),
	}
	order2, err := orderSvc.CreateOrder(ctx, params2)
	require.NoError(t, err)

	// 在TransitionStatus内部读取order后、CAS更新前，修改version来模拟并发
	// 直接在DB层先修改version，使CAS条件不匹配
	db.Model(&Order{}).Where("id = ?", order2.ID).Updates(map[string]interface{}{
		"version": 100, // 修改version使CAS检查失败
	})

	// TransitionStatus内部会读取order（此时version=100），然后CAS用version=100去更新
	// 由于我们修改了version为100但status仍然是StatusPaid，所以CAS应该成功
	// 要让CAS失败，需要在读取和更新之间改变version

	// 更直接的方法：手动调用repo的CAS方法来验证冲突
	repo := NewOrderRepository(db)
	// 先把order2恢复到正常状态
	db.Model(&Order{}).Where("id = ?", order2.ID).Updates(map[string]interface{}{
		"version": 0,
		"status":  StatusPaid,
	})

	// 用错误的version调用CAS
	ok, err := repo.UpdateStatusCAS(ctx, order2.ID, StatusPaid, StatusProcessing, 999)
	require.NoError(t, err)
	assert.False(t, ok, "version不匹配时CAS应失败")

	// 用正确的version调用CAS
	ok, err = repo.UpdateStatusCAS(ctx, order2.ID, StatusPaid, StatusProcessing, 0)
	require.NoError(t, err)
	assert.True(t, ok, "version匹配时CAS应成功")

	// 验证version已递增
	updated, err := orderSvc.GetOrderByID(ctx, order2.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated.Version, "CAS成功后version应递增")
}
