package docking

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/adapter"
)

// mockAdapter 模拟供货商适配器
type mockAdapter struct {
	submitResult *adapter.SubmitResult
	submitErr    error
}

func (m *mockAdapter) Name() string { return "mock" }

func (m *mockAdapter) SubmitOrder(_ context.Context, _ string, _ adapter.SubmitParams) (*adapter.SubmitResult, error) {
	return m.submitResult, m.submitErr
}

func (m *mockAdapter) QueryOrder(_ context.Context, _ string) (*adapter.QueryResult, error) {
	return nil, nil
}

func (m *mockAdapter) ParseCallback(_ []byte) (*adapter.CallbackData, error) {
	return nil, nil
}

// setupDockingTestDB 创建隔离SQLite内存数据库
func setupDockingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = Migrate(db)
	require.NoError(t, err, "failed to migrate docking tables")

	return db
}

// newDockingTestService 创建测试用的DockingService（含mock adapter）
func newDockingTestService(t *testing.T) (*DockingService, *adapter.Factory, *gorm.DB) {
	t.Helper()
	db := setupDockingTestDB(t)
	factory := adapter.NewFactory()

	// 注册mock adapter
	factory.Register(10, &mockAdapter{
		submitResult: &adapter.SubmitResult{
			ExternalOrderID: "EXT-001",
			Status:          2,
			Message:         "success",
		},
	})

	svc := NewDockingService(db, factory)
	return svc, factory, db
}

// ---------- TestSubmitOrder_Success ----------

func TestSubmitOrder_Success(t *testing.T) {
	svc, _, db := newDockingTestService(t)
	ctx := context.Background()

	params := adapter.SubmitParams{
		GoodsSN:         "G100",
		GoodsName:       "测试商品",
		BuyNumber:       1,
		NotifyURL:       "http://example.com/callback",
		CustomerOrderID: "ORDER-001",
	}

	err := svc.SubmitOrder(ctx, 1, 10, "ORDER-001", params)
	require.NoError(t, err)

	// 验证任务已创建
	var task OrderDockingTask
	err = db.First(&task, "order_id = ?", 1).Error
	require.NoError(t, err)
	assert.Equal(t, uint(1), task.OrderID)
	assert.Equal(t, uint(10), task.SupplierID)
	assert.Equal(t, 5, task.MaxRetry)
	assert.NotEmpty(t, task.RequestPayload)

	// 验证payload内容
	var savedParams adapter.SubmitParams
	err = json.Unmarshal([]byte(task.RequestPayload), &savedParams)
	require.NoError(t, err)
	assert.Equal(t, "G100", savedParams.GoodsSN)
	assert.Equal(t, "ORDER-001", savedParams.CustomerOrderID)

	// 等待goroutine异步执行完成
	time.Sleep(200 * time.Millisecond)

	// 验证任务已提交成功（异步执行后状态为submitted）
	err = db.First(&task, "order_id = ?", 1).Error
	require.NoError(t, err)
	assert.Equal(t, TaskSubmitted, task.Status)
	assert.Equal(t, "EXT-001", task.ExternalOrderID)
}

// ---------- TestSubmitOrder_Idempotent ----------

func TestSubmitOrder_Idempotent(t *testing.T) {
	svc, _, db := newDockingTestService(t)
	ctx := context.Background()

	params := adapter.SubmitParams{
		GoodsSN:         "G100",
		CustomerOrderID: "ORDER-001",
	}

	// 第一次提交
	err := svc.SubmitOrder(ctx, 1, 10, "ORDER-001", params)
	require.NoError(t, err)

	// 第二次提交（幂等）
	err = svc.SubmitOrder(ctx, 1, 10, "ORDER-001", params)
	require.NoError(t, err) // 不应报错

	// 验证只有一条记录
	var count int64
	db.Model(&OrderDockingTask{}).Where("order_id = ?", 1).Count(&count)
	assert.Equal(t, int64(1), count)
}

// ---------- TestExecuteTask_Success ----------

func TestExecuteTask_Success(t *testing.T) {
	svc, _, db := newDockingTestService(t)
	ctx := context.Background()

	// 手动创建一个pending任务
	params := adapter.SubmitParams{
		GoodsSN:         "G100",
		CustomerOrderID: "ORDER-002",
	}
	payloadBytes, _ := json.Marshal(params)
	task := &OrderDockingTask{
		OrderID:        2,
		SupplierID:     10,
		Status:         TaskPending,
		MaxRetry:       5,
		RequestPayload: string(payloadBytes),
	}
	err := db.Create(task).Error
	require.NoError(t, err)

	// 执行任务
	err = svc.ExecuteTask(ctx, task.ID)
	require.NoError(t, err)

	// 等待goroutine执行完
	time.Sleep(200 * time.Millisecond)

	// 验证任务状态
	var updatedTask OrderDockingTask
	err = db.First(&updatedTask, task.ID).Error
	require.NoError(t, err)
	assert.Equal(t, TaskSubmitted, updatedTask.Status)
	assert.Equal(t, "EXT-001", updatedTask.ExternalOrderID)
}

// ---------- TestExecuteTask_NotFound ----------

func TestExecuteTask_NotFound(t *testing.T) {
	svc, _, _ := newDockingTestService(t)
	ctx := context.Background()

	// 执行不存在的任务
	err := svc.ExecuteTask(ctx, 99999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get task")
}

// ---------- TestRetryPendingTasks ----------

func TestRetryPendingTasks(t *testing.T) {
	svc, _, db := newDockingTestService(t)
	ctx := context.Background()

	// 创建一个到期的pending任务
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	params := adapter.SubmitParams{
		GoodsSN:         "G200",
		CustomerOrderID: "ORDER-003",
	}
	payloadBytes, _ := json.Marshal(params)
	task := &OrderDockingTask{
		OrderID:        3,
		SupplierID:     10,
		Status:         TaskPending,
		MaxRetry:       5,
		RetryCount:     1,
		NextRetryAt:    &pastTime,
		RequestPayload: string(payloadBytes),
	}
	err := db.Create(task).Error
	require.NoError(t, err)

	// 重试待提交任务
	err = svc.RetryPendingTasks(ctx)
	require.NoError(t, err)

	// 等待goroutine执行完
	time.Sleep(200 * time.Millisecond)

	// 验证任务已提交成功
	var updatedTask OrderDockingTask
	err = db.First(&updatedTask, task.ID).Error
	require.NoError(t, err)
	assert.Equal(t, TaskSubmitted, updatedTask.Status)
}
