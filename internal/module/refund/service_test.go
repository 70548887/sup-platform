package refund

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
	"github.com/70548887/sup-platform/internal/module/order"
)

// setupRefundTestDB 创建隔离SQLite内存数据库，迁移所有依赖表
func setupRefundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移ledger模块表
	err = ledger.AutoMigrate(db)
	require.NoError(t, err, "failed to migrate ledger tables")

	// 迁移order模块表
	err = order.AutoMigrate(db)
	require.NoError(t, err, "failed to migrate order tables")

	// 迁移refund模块表
	err = Migrate(db)
	require.NoError(t, err, "failed to migrate refund tables")

	return db
}

// newRefundTestService 创建RefundService及依赖链
func newRefundTestService(t *testing.T) (*RefundService, *order.OrderService, *ledger.LedgerService, *gorm.DB) {
	t.Helper()
	db := setupRefundTestDB(t)
	ledgerSvc := ledger.NewLedgerService(db)
	orderSvc := order.NewOrderService(db, ledgerSvc)
	refundSvc := NewRefundService(db, orderSvc, ledgerSvc)
	return refundSvc, orderSvc, ledgerSvc, db
}

// createTestOrder 创建一个已付款的测试订单并转入Processing状态
func createTestOrder(t *testing.T, orderSvc *order.OrderService, ledgerSvc *ledger.LedgerService, db *gorm.DB, customerID uint, amount decimal.Decimal) *order.Order {
	t.Helper()
	ctx := context.Background()

	// 确保钱包有足够余额
	_, err := ledgerSvc.EnsureWallet(ctx, customerID)
	require.NoError(t, err)
	err = ledgerSvc.Credit(ctx, db, customerID, amount.Mul(decimal.NewFromInt(2)), "test_topup", 0, "测试充值")
	require.NoError(t, err)

	// 创建订单
	ord, err := orderSvc.CreateOrder(ctx, order.CreateOrderParams{
		AppID:           1,
		CustomerID:      customerID,
		SupplierID:      10,
		CustomerOrderID: "",
		GoodsID:         100,
		GoodsSN:         "G100",
		GoodsName:       "测试商品",
		BuyNumber:       1,
		UnitPrice:       amount,
		NotifyURL:       "",
	})
	require.NoError(t, err)

	// 转移到Processing状态（StatusPaid -> StatusProcessing）
	err = orderSvc.TransitionStatus(ctx, ord.ID, order.StatusProcessing, "system", "test")
	require.NoError(t, err)

	// 重新查询获取最新状态
	updatedOrd, err := orderSvc.GetOrderByID(ctx, ord.ID)
	require.NoError(t, err)
	return updatedOrd
}

// ---------- TestApply_Success ----------

func TestApply_Success(t *testing.T) {
	refundSvc, orderSvc, ledgerSvc, db := newRefundTestService(t)
	ctx := context.Background()

	// 创建测试订单（金额100）
	ord := createTestOrder(t, orderSvc, ledgerSvc, db, 1, decimal.NewFromInt(100))

	// 申请退款50
	refundOrder, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(50), "商品质量问题")
	require.NoError(t, err)
	assert.NotNil(t, refundOrder)
	assert.Equal(t, uint(1), refundOrder.CustomerID)
	assert.Equal(t, ord.ID, refundOrder.OrderID)
	assert.True(t, refundOrder.Amount.Equal(decimal.NewFromInt(50)))
	assert.Equal(t, RefundPending, refundOrder.Status)
	assert.NotEmpty(t, refundOrder.RefundSN)
	assert.Equal(t, "商品质量问题", refundOrder.Reason)

	// 验证订单已进入Refunding状态
	updatedOrd, err := orderSvc.GetOrderByID(ctx, ord.ID)
	require.NoError(t, err)
	assert.Equal(t, order.StatusRefunding, updatedOrd.Status)
}

// ---------- TestApply_AmountExceedsRefundable ----------

func TestApply_AmountExceedsRefundable(t *testing.T) {
	refundSvc, orderSvc, ledgerSvc, db := newRefundTestService(t)
	ctx := context.Background()

	// 创建测试订单（金额100）
	ord := createTestOrder(t, orderSvc, ledgerSvc, db, 1, decimal.NewFromInt(100))

	// 申请退款150（超过订单金额），应失败
	_, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(150), "超额退款")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRefundAmountExceeded)
}

// ---------- TestApply_OrderNotFound ----------

func TestApply_OrderNotFound(t *testing.T) {
	refundSvc, _, _, _ := newRefundTestService(t)
	ctx := context.Background()

	// 查找不存在的订单
	_, err := refundSvc.Apply(ctx, 1, "NON_EXIST_ORDER_SN", decimal.NewFromInt(10), "不存在的订单")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find order failed")
}

// ---------- TestApprove_Success ----------

func TestApprove_Success(t *testing.T) {
	refundSvc, orderSvc, ledgerSvc, db := newRefundTestService(t)
	ctx := context.Background()

	// 创建测试订单（金额100）
	ord := createTestOrder(t, orderSvc, ledgerSvc, db, 1, decimal.NewFromInt(100))

	// 申请退款50
	refundOrder, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(50), "退款测试")
	require.NoError(t, err)

	// 记录审批前的钱包余额
	balanceBefore, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)

	// 批准退款
	err = refundSvc.Approve(ctx, refundOrder.ID, 999, "同意退款")
	require.NoError(t, err)

	// 验证退款单状态 = RefundCompleted
	updatedRefund, err := refundSvc.GetByID(ctx, refundOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, RefundCompleted, updatedRefund.Status)

	// 验证订单refund_amount累加正确
	updatedOrd, err := orderSvc.GetOrderByID(ctx, ord.ID)
	require.NoError(t, err)
	assert.True(t, updatedOrd.RefundAmount.Equal(decimal.NewFromInt(50)),
		"refund_amount should be 50, got %s", updatedOrd.RefundAmount.String())

	// 验证订单状态已流转到Refunded
	assert.Equal(t, order.StatusRefunded, updatedOrd.Status)

	// 验证钱包credit入账
	balanceAfter, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	expectedBalance := balanceBefore.Add(decimal.NewFromInt(50))
	assert.True(t, balanceAfter.Equal(expectedBalance),
		"wallet balance after refund: got %s, want %s", balanceAfter.String(), expectedBalance.String())
}

// ---------- TestApprove_ConcurrentExceedsAmount ----------

func TestApprove_ConcurrentExceedsAmount(t *testing.T) {
	refundSvc, orderSvc, ledgerSvc, db := newRefundTestService(t)
	ctx := context.Background()

	// 创建测试订单（金额100）
	ord := createTestOrder(t, orderSvc, ledgerSvc, db, 1, decimal.NewFromInt(100))

	// 先创建退款单1（60元）
	refund1, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(60), "退款1")
	require.NoError(t, err)

	// 将订单状态回退到Processing，以便再创建退款单
	// 直接更新数据库状态模拟
	db.Model(&order.Order{}).Where("id = ?", ord.ID).Updates(map[string]interface{}{
		"status":  order.StatusProcessing,
		"version": gorm.Expr("version + 1"),
	})

	// 创建退款单2（60元），合计120超过订单金额100
	refund2, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(60), "退款2")
	require.NoError(t, err)

	// 先批准退款1（60元）
	// 需要订单在Refunding状态
	db.Model(&order.Order{}).Where("id = ?", ord.ID).Updates(map[string]interface{}{
		"status":  order.StatusRefunding,
		"version": gorm.Expr("version + 1"),
	})
	err = refundSvc.Approve(ctx, refund1.ID, 999, "批准退款1")
	require.NoError(t, err)

	// 再批准退款2（60元），此时refund_amount已经是60，再加60=120>100，应失败
	// 需要重设订单状态为 Refunding 以通过状态检查
	db.Model(&order.Order{}).Where("id = ?", ord.ID).Updates(map[string]interface{}{
		"status":  order.StatusRefunding,
		"version": gorm.Expr("version + 1"),
	})
	err = refundSvc.Approve(ctx, refund2.ID, 999, "批准退款2")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRefundAmountExceeded)
}

// ---------- TestReject_Success ----------

func TestReject_Success(t *testing.T) {
	refundSvc, orderSvc, ledgerSvc, db := newRefundTestService(t)
	ctx := context.Background()

	// 创建测试订单（金额100）
	ord := createTestOrder(t, orderSvc, ledgerSvc, db, 1, decimal.NewFromInt(100))

	// 申请退款50
	refundOrder, err := refundSvc.Apply(ctx, 1, ord.OrderSN, decimal.NewFromInt(50), "退款测试")
	require.NoError(t, err)

	// 拒绝退款
	err = refundSvc.Reject(ctx, refundOrder.ID, 999, "不符合退款条件")
	require.NoError(t, err)

	// 验证退款单状态 = RefundRejected
	updatedRefund, err := refundSvc.GetByID(ctx, refundOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, RefundRejected, updatedRefund.Status)

	// 验证订单状态回退到Processing
	updatedOrd, err := orderSvc.GetOrderByID(ctx, ord.ID)
	require.NoError(t, err)
	assert.Equal(t, order.StatusProcessing, updatedOrd.Status)
}
