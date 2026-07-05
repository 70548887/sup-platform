package recharge

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/ledger"
)

// setupRechargeTestDB 创建隔离SQLite内存数据库，迁移所有依赖表
func setupRechargeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	// SQLite内存模式必须限制为单连接，否则每个连接各自独立数据库
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移ledger模块表（Wallet, LedgerEntry, Recharge）
	err = ledger.AutoMigrate(db)
	require.NoError(t, err, "failed to migrate ledger tables")

	// 迁移recharge模块表（RechargeOrder）
	err = Migrate(db)
	require.NoError(t, err, "failed to migrate recharge tables")

	return db
}

// newRechargeTestService 创建RechargeService及依赖链
func newRechargeTestService(t *testing.T) (*RechargeService, *ledger.LedgerService, *gorm.DB) {
	t.Helper()
	db := setupRechargeTestDB(t)
	ledgerSvc := ledger.NewLedgerService(db)
	rechargeSvc := NewRechargeService(db, ledgerSvc)
	return rechargeSvc, ledgerSvc, db
}

// ---------- TestApply_Success ----------

func TestApply_Success(t *testing.T) {
	rechargeSvc, _, _ := newRechargeTestService(t)
	ctx := context.Background()

	// 正常创建充值申请
	order, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(100), "idem-key-001")
	require.NoError(t, err)
	require.NotNil(t, order)

	// 验证字段
	assert.Equal(t, uint(1), order.UserID)
	assert.True(t, order.Amount.Equal(decimal.NewFromInt(100)),
		"金额不匹配: got %s", order.Amount.String())
	assert.Equal(t, StatusPending, order.Status)
	assert.NotEmpty(t, order.RechargeSN)
	assert.Contains(t, order.RechargeSN, "RCH")
	assert.Equal(t, "idem-key-001", order.IdempotencyKey)
	assert.Equal(t, int64(0), order.Version)
}

// ---------- TestApprove_WalletCredit ----------

func TestApprove_WalletCredit(t *testing.T) {
	rechargeSvc, ledgerSvc, _ := newRechargeTestService(t)
	ctx := context.Background()

	// 准备：创建充值申请（200元）
	order, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(200), "")
	require.NoError(t, err)

	// 准备：确保用户有钱包（Credit要求钱包已存在）
	_, err = ledgerSvc.EnsureWallet(ctx, 1)
	require.NoError(t, err)

	// 记录审批前的余额
	balanceBefore, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)

	// 执行审批
	err = rechargeSvc.Approve(ctx, order.ID, 999)
	require.NoError(t, err)

	// 验证充值单状态 = StatusCompleted（pending → completed）
	updatedOrder, err := rechargeSvc.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updatedOrder.Status)

	// 验证版本号递增（CAS）
	assert.Equal(t, int64(1), updatedOrder.Version)

	// 验证审批人
	require.NotNil(t, updatedOrder.ApproverID)
	assert.Equal(t, uint(999), *updatedOrder.ApproverID)

	// 验证审批时间已设置
	require.NotNil(t, updatedOrder.ApprovedAt)

	// 验证钱包余额正确入账（+200）
	balanceAfter, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	expectedBalance := balanceBefore.Add(decimal.NewFromInt(200))
	assert.True(t, balanceAfter.Equal(expectedBalance),
		"钱包余额不匹配: got %s, want %s", balanceAfter.String(), expectedBalance.String())

	// 验证流水记录
	entries, total, err := ledgerSvc.GetHistory(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
	assert.Equal(t, "recharge", entries[0].Type)
	assert.True(t, entries[0].Amount.Equal(decimal.NewFromInt(200)),
		"流水金额应为正数200: got %s", entries[0].Amount.String())
	assert.True(t, entries[0].BalanceAfter.Equal(expectedBalance))
	assert.Equal(t, order.ID, entries[0].RelatedID)
}

// ---------- TestApprove_AlreadyProcessed ----------

func TestApprove_AlreadyProcessed(t *testing.T) {
	rechargeSvc, ledgerSvc, _ := newRechargeTestService(t)
	ctx := context.Background()

	// 准备：创建充值申请（100元）
	order, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(100), "")
	require.NoError(t, err)

	// 准备：确保用户有钱包
	_, err = ledgerSvc.EnsureWallet(ctx, 1)
	require.NoError(t, err)

	// 第一次审批：成功
	err = rechargeSvc.Approve(ctx, order.ID, 999)
	require.NoError(t, err)

	// 记录第一次审批后的余额
	balanceAfterFirst, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	assert.True(t, balanceAfterFirst.Equal(decimal.NewFromInt(100)),
		"第一次审批后余额应为100: got %s", balanceAfterFirst.String())

	// 第二次审批：应返回错误（已处理）
	err = rechargeSvc.Approve(ctx, order.ID, 999)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidStatus)

	// 验证余额未再次变化（幂等性：重复审批不会重复入账）
	balanceAfterSecond, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	assert.True(t, balanceAfterSecond.Equal(balanceAfterFirst),
		"重复审批后余额不应变化: got %s, want %s", balanceAfterSecond.String(), balanceAfterFirst.String())

	// 验证充值单状态仍为已完成
	updatedOrder, err := rechargeSvc.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updatedOrder.Status)

	// 验证只产生一条入账流水
	_, total, err := ledgerSvc.GetHistory(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "应只有一条入账流水")
}

// ---------- TestReject_Success ----------

func TestReject_Success(t *testing.T) {
	rechargeSvc, ledgerSvc, _ := newRechargeTestService(t)
	ctx := context.Background()

	// 准备：创建充值申请（500元）
	order, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(500), "")
	require.NoError(t, err)

	// 准备：确保用户有钱包
	_, err = ledgerSvc.EnsureWallet(ctx, 1)
	require.NoError(t, err)

	// 执行拒绝
	err = rechargeSvc.Reject(ctx, order.ID, 888, "金额异常，拒绝充值")
	require.NoError(t, err)

	// 验证充值单状态 = StatusRejected（pending → rejected）
	updatedOrder, err := rechargeSvc.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRejected, updatedOrder.Status)

	// 验证版本号递增（CAS）
	assert.Equal(t, int64(1), updatedOrder.Version)

	// 验证审批人
	require.NotNil(t, updatedOrder.ApproverID)
	assert.Equal(t, uint(888), *updatedOrder.ApproverID)

	// 验证审批备注
	assert.Equal(t, "金额异常，拒绝充值", updatedOrder.ApprovalNote)

	// 验证审批时间已设置
	require.NotNil(t, updatedOrder.ApprovedAt)

	// 验证钱包余额未变化（拒绝不入账）
	balance, err := ledgerSvc.GetBalance(ctx, 1)
	require.NoError(t, err)
	assert.True(t, balance.Equal(decimal.Zero),
		"拒绝充值后余额应为0: got %s", balance.String())

	// 验证无流水产生
	_, total, err := ledgerSvc.GetHistory(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total, "拒绝充值不应产生流水")
}

// ---------- TestList_Pagination ----------

func TestList_Pagination(t *testing.T) {
	rechargeSvc, _, _ := newRechargeTestService(t)
	ctx := context.Background()

	// 创建5条充值记录（同一用户，使用不同幂等键避免唯一约束冲突）
	for i := 0; i < 5; i++ {
		_, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(int64(i+1)*100), fmt.Sprintf("list-key-%d", i))
		require.NoError(t, err)
	}

	// 测试第1页，每页2条
	orders, total, err := rechargeSvc.List(ctx, 1, nil, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total, "总数应为5")
	assert.Len(t, orders, 2, "第1页应返回2条")

	// 测试第2页，每页2条
	orders, total, err = rechargeSvc.List(ctx, 1, nil, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 2, "第2页应返回2条")

	// 测试第3页，每页2条（只有1条）
	orders, total, err = rechargeSvc.List(ctx, 1, nil, 3, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 1, "第3页应返回1条")

	// 测试状态过滤：全部为pending
	pendingStatus := StatusPending
	orders, total, err = rechargeSvc.List(ctx, 1, &pendingStatus, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, orders, 5)
	for _, o := range orders {
		assert.Equal(t, StatusPending, o.Status)
	}

	// 测试不同用户的数据隔离
	_, err = rechargeSvc.Apply(ctx, 2, decimal.NewFromInt(999), "list-key-user2")
	require.NoError(t, err)
	orders, total, err = rechargeSvc.List(ctx, 2, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, orders, 1)
	assert.Equal(t, uint(2), orders[0].UserID)
}

// ---------- TestApply_Idempotency ----------

func TestApply_Idempotency(t *testing.T) {
	rechargeSvc, _, _ := newRechargeTestService(t)
	ctx := context.Background()

	idemKey := "idem-unique-key-123"

	// 第一次申请
	order1, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(300), idemKey)
	require.NoError(t, err)
	require.NotNil(t, order1)

	// 使用相同幂等键再次申请，应返回同一条记录
	order2, err := rechargeSvc.Apply(ctx, 1, decimal.NewFromInt(300), idemKey)
	require.NoError(t, err)
	require.NotNil(t, order2)

	// 验证返回的是同一条记录（幂等）
	assert.Equal(t, order1.ID, order2.ID)
	assert.Equal(t, order1.RechargeSN, order2.RechargeSN)
	assert.Equal(t, order1.Amount, order2.Amount)

	// 验证数据库中只有一条记录
	_, total, err := rechargeSvc.List(ctx, 1, nil, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "幂等键应保证只创建一条记录")
}
