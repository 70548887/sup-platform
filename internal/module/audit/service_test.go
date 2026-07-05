package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/testutil"
)

// setupAuditTestDB 创建隔离的SQLite内存数据库并迁移audit表
func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.SetupIsolatedTestDB()

	// SQLite内存模式限制单连接，避免并发问题
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移audit模块表
	require.NoError(t, Migrate(db))
	return db
}

// seedLog 通过DB直接插入审计日志并强制设置指定时间戳
// 用于精确控制 created_at 以测试时间范围筛选
func seedLog(t *testing.T, db *gorm.DB, entry *AuditLog, ts int64) {
	t.Helper()
	require.NoError(t, db.Create(entry).Error)
	require.NoError(t, db.Model(&AuditLog{}).Where("id = ?", entry.ID).
		Update("created_at", ts).Error)
}

// ---------- TestRecordLog_Success ----------

func TestRecordLog_Success(t *testing.T) {
	db := setupAuditTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	// 1. 测试同步写入 LogSync
	entry := NewEntry(1, "admin", ActionLogin, "session", 1, `{"ip":"127.0.0.1"}`)
	entry.IP = "127.0.0.1"
	entry.UserAgent = "test-agent"

	err := svc.LogSync(ctx, entry)
	require.NoError(t, err)
	assert.NotZero(t, entry.ID, "LogSync应设置自增ID")

	// 通过GetByID验证记录内容
	got, err := svc.GetByID(ctx, entry.ID)
	require.NoError(t, err)
	assert.Equal(t, uint(1), got.UserID)
	assert.Equal(t, "admin", got.Username)
	assert.Equal(t, ActionLogin, got.Action)
	assert.Equal(t, "session", got.Resource)
	assert.Equal(t, uint(1), got.ResourceID)
	assert.Equal(t, "127.0.0.1", got.IP)
	assert.Equal(t, "test-agent", got.UserAgent)
	assert.NotZero(t, got.CreatedAt, "CreatedAt应自动填充")

	// 查询不存在的ID返回ErrAuditLogNotFound
	_, err = svc.GetByID(ctx, 9999)
	assert.ErrorIs(t, err, ErrAuditLogNotFound)

	// 2. 测试异步写入 Log
	asyncEntry := NewEntry(2, "async_admin", ActionLogout, "session", 2, "async logout detail")
	err = svc.Log(ctx, asyncEntry)
	require.NoError(t, err)

	// 轮询等待异步goroutine完成写入
	var found bool
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, _, _ := svc.Query(ctx, AuditFilter{UserID: 2}, 1, 10)
		if len(logs) > 0 {
			found = true
			assert.Equal(t, ActionLogout, logs[0].Action)
			assert.Equal(t, "async_admin", logs[0].Username)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, found, "异步Log应在2秒内完成写入")
}

// ---------- TestListLogs_WithFilters ----------

func TestListLogs_WithFilters(t *testing.T) {
	db := setupAuditTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	// 插入4条日志，不同操作类型/用户/资源/时间
	seedLog(t, db, NewEntry(1, "admin1", ActionLogin, "user", 1, "login1"), 1000)
	seedLog(t, db, NewEntry(2, "admin2", ActionOrderCreate, "order", 10, "order1"), 2000)
	seedLog(t, db, NewEntry(2, "admin2", ActionLogin, "user", 2, "login2"), 3000)
	seedLog(t, db, NewEntry(1, "admin1", ActionGoodsCreate, "goods", 20, "goods1"), 4000)

	t.Run("按操作类型筛选", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{Action: ActionLogin}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, logs, 2)
		for _, l := range logs {
			assert.Equal(t, ActionLogin, l.Action)
		}
	})

	t.Run("按用户ID筛选", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{UserID: 1}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, logs, 2)
		for _, l := range logs {
			assert.Equal(t, uint(1), l.UserID)
		}
	})

	t.Run("按资源类型筛选", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{Resource: "order"}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, logs, 1)
		assert.Equal(t, "order", logs[0].Resource)
	})

	t.Run("按时间范围筛选", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{StartTime: 2000, EndTime: 3000}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, logs, 2)
		for _, l := range logs {
			assert.GreaterOrEqual(t, l.CreatedAt, int64(2000))
			assert.LessOrEqual(t, l.CreatedAt, int64(3000))
		}
	})

	t.Run("组合条件筛选", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{UserID: 2, Action: ActionLogin}, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, logs, 1)
		assert.Equal(t, ActionLogin, logs[0].Action)
		assert.Equal(t, uint(2), logs[0].UserID)
	})
}

// ---------- TestListLogs_Pagination ----------

func TestListLogs_Pagination(t *testing.T) {
	db := setupAuditTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	// 插入5条日志，时间戳递增 (1000~5000)
	for i := 1; i <= 5; i++ {
		seedLog(t, db, NewEntry(uint(i), "user", ActionLogin, "session", uint(i), "pagination"), int64(i*1000))
	}

	t.Run("第一页返回2条且按时间倒序", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{}, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total, "总数应为5")
		assert.Len(t, logs, 2)
		// DESC排序：第一条时间戳最大(5000)
		assert.Equal(t, int64(5000), logs[0].CreatedAt)
		assert.Equal(t, int64(4000), logs[1].CreatedAt)
	})

	t.Run("第二页返回2条", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{}, 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, logs, 2)
		assert.Equal(t, int64(3000), logs[0].CreatedAt)
		assert.Equal(t, int64(2000), logs[1].CreatedAt)
	})

	t.Run("第三页返回1条", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{}, 3, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, logs, 1)
		assert.Equal(t, int64(1000), logs[0].CreatedAt)
	})

	t.Run("超出范围返回空列表", func(t *testing.T) {
		logs, total, err := svc.Query(ctx, AuditFilter{}, 10, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total, "总数始终为5")
		assert.Empty(t, logs)
	})
}

// ---------- TestGetStats ----------

func TestGetStats(t *testing.T) {
	db := setupAuditTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	// 插入4条日志:
	//   Action: login x2, order.create x1, goods.create x1
	//   Resource: user x1, order x2, goods x1
	seedLog(t, db, NewEntry(1, "admin1", ActionLogin, "user", 1, "login1"), 1000)
	seedLog(t, db, NewEntry(2, "admin2", ActionLogin, "order", 10, "login2"), 2000)
	seedLog(t, db, NewEntry(3, "admin3", ActionOrderCreate, "order", 20, "order1"), 3000)
	seedLog(t, db, NewEntry(4, "admin4", ActionGoodsCreate, "goods", 30, "goods1"), 4000)

	stats, err := svc.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// 验证总数
	assert.Equal(t, int64(4), stats.Total)

	// 验证按操作类型统计
	assert.Equal(t, int64(2), stats.ByAction[ActionLogin], "login操作应有2条")
	assert.Equal(t, int64(1), stats.ByAction[ActionOrderCreate], "order.create操作应有1条")
	assert.Equal(t, int64(1), stats.ByAction[ActionGoodsCreate], "goods.create操作应有1条")

	// 验证按资源类型统计
	assert.Equal(t, int64(1), stats.ByResource["user"], "user资源应有1条")
	assert.Equal(t, int64(2), stats.ByResource["order"], "order资源应有2条")
	assert.Equal(t, int64(1), stats.ByResource["goods"], "goods资源应有1条")

	// 空数据库场景
	emptyDB := setupAuditTestDB(t)
	emptySvc := NewAuditService(emptyDB)
	emptyStats, err := emptySvc.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), emptyStats.Total)
	assert.Empty(t, emptyStats.ByAction)
	assert.Empty(t, emptyStats.ByResource)
}
