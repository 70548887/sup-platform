package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/order"
)

// setupNotifyTestDB 创建隔离SQLite内存数据库
func setupNotifyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移order模块表（OrderCallback在order包中）
	err = order.AutoMigrate(db)
	require.NoError(t, err, "failed to migrate order tables")

	return db
}

// newNotifyTestService 创建测试用的NotifyService
func newNotifyTestService(t *testing.T) (*NotifyService, *gorm.DB) {
	t.Helper()
	db := setupNotifyTestDB(t)
	svc := NewNotifyService(db)
	return svc, db
}

// ---------- TestSendOrderCallback_Success ----------

func TestSendOrderCallback_Success(t *testing.T) {
	svc, db := newNotifyTestService(t)
	ctx := context.Background()

	// 启动httptest server模拟回调目标
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"success": true}`)
	}))
	defer server.Close()

	// 创建测试订单数据
	ord := &order.Order{
		ID:        1,
		OrderSN:   "TEST-ORDER-001",
		Status:    3, // StatusProcessing
		NotifyURL: server.URL,
	}

	// 发送回调通知
	err := svc.SendOrderCallback(ctx, ord, "order.processing")
	require.NoError(t, err)

	// 等待goroutine异步执行完成
	time.Sleep(500 * time.Millisecond)

	// 验证callback记录已创建并投递成功
	var cb order.OrderCallback
	err = db.First(&cb, "order_id = ?", 1).Error
	require.NoError(t, err)
	assert.Equal(t, uint(1), cb.OrderID)
	assert.Equal(t, server.URL, cb.URL)
	assert.True(t, cb.Success)
	assert.Equal(t, 200, cb.StatusCode)
	assert.Contains(t, cb.Response, "success")
	assert.Contains(t, cb.Payload, "TEST-ORDER-001")
	assert.Contains(t, cb.Payload, "order.processing")
}

// ---------- TestSendOrderCallback_EmptyURL ----------

func TestSendOrderCallback_EmptyURL(t *testing.T) {
	svc, db := newNotifyTestService(t)
	ctx := context.Background()

	// 订单没有配置NotifyURL
	ord := &order.Order{
		ID:        2,
		OrderSN:   "TEST-ORDER-002",
		Status:    3,
		NotifyURL: "", // 空URL
	}

	// 应跳过，不报错
	err := svc.SendOrderCallback(ctx, ord, "order.processing")
	require.NoError(t, err)

	// 验证没有创建callback记录
	var count int64
	db.Model(&order.OrderCallback{}).Where("order_id = ?", 2).Count(&count)
	assert.Equal(t, int64(0), count)
}

// ---------- TestDeliverCallback_Success ----------

func TestDeliverCallback_Success(t *testing.T) {
	svc, db := newNotifyTestService(t)
	ctx := context.Background()

	// 启动httptest server模拟回调目标
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer server.Close()

	// 手动创建一条callback记录
	cb := &order.OrderCallback{
		OrderID: 3,
		URL:     server.URL,
		Payload: `{"order_sn":"TEST-003","status":3,"event":"order.processing","timestamp":1700000000}`,
		Success: false,
	}
	err := db.Create(cb).Error
	require.NoError(t, err)

	// 调用DeliverCallback
	err = svc.DeliverCallback(ctx, cb.ID)
	require.NoError(t, err)

	// 等待goroutine执行完
	time.Sleep(500 * time.Millisecond)

	// 验证投递成功
	var updatedCb order.OrderCallback
	err = db.First(&updatedCb, cb.ID).Error
	require.NoError(t, err)
	assert.True(t, updatedCb.Success)
	assert.Equal(t, 200, updatedCb.StatusCode)
}

// ---------- TestDeliverCallback_NotFound ----------

func TestDeliverCallback_NotFound(t *testing.T) {
	svc, _ := newNotifyTestService(t)
	ctx := context.Background()

	// 查找不存在的callback
	err := svc.DeliverCallback(ctx, 99999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get callback")
}

// ---------- TestRetryPendingCallbacks ----------

func TestRetryPendingCallbacks(t *testing.T) {
	svc, db := newNotifyTestService(t)
	ctx := context.Background()

	// 启动httptest server模拟回调目标
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer server.Close()

	// 创建一条待重试的callback记录（NextRetryAt已过期）
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	cb := &order.OrderCallback{
		OrderID:     4,
		URL:         server.URL,
		Payload:     `{"order_sn":"TEST-004","status":6,"event":"order.completed","timestamp":1700000000}`,
		Success:     false,
		RetryCount:  1,
		NextRetryAt: &pastTime,
	}
	err := db.Create(cb).Error
	require.NoError(t, err)

	// 重试待发送的回调
	err = svc.RetryPendingCallbacks(ctx)
	require.NoError(t, err)

	// 等待执行完成
	time.Sleep(500 * time.Millisecond)

	// 验证投递成功
	var updatedCb order.OrderCallback
	err = db.First(&updatedCb, cb.ID).Error
	require.NoError(t, err)
	assert.True(t, updatedCb.Success)
	assert.Equal(t, 200, updatedCb.StatusCode)
}
