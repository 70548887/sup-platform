package analytics

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
)

// setupAnalyticsTestDB 创建隔离的 SQLite 内存数据库并迁移 analytics 表
func setupAnalyticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "打开测试数据库失败")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, Migrate(db), "analytics 表迁移失败")
	return db
}

// createDailyStats 创建一条 DailyStats 测试数据
func createDailyStats(t *testing.T, db *gorm.DB, date string, orders int, amount string) {
	t.Helper()
	err := db.Create(&DailyStats{
		Date:        date,
		TotalOrders: orders,
		TotalAmount: decimal.RequireFromString(amount),
	}).Error
	require.NoError(t, err, "创建 DailyStats 失败")
}

// ---------- TestGetPeriodStats ----------

func TestGetPeriodStats(t *testing.T) {
	db := setupAnalyticsTestDB(t)
	ctx := context.Background()
	svc := NewAnalyticsService(db, nil, "test")

	now := time.Now().In(cst)
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
	weekStart := weekStart(now).Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Format("2006-01-02")

	// 今日：5 单，500
	createDailyStats(t, db, today, 5, "500.000000")
	// 昨日：3 单，300
	createDailyStats(t, db, yesterday, 3, "300.000000")

	stats, err := svc.GetDashboard(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// 今日
	assert.Equal(t, today, stats.TodayStats.Date)
	assert.Equal(t, 5, stats.TodayStats.TotalOrders)
	assert.True(t, stats.TodayStats.TotalAmount.Equal(decimal.RequireFromString("500.000000")),
		"今日金额应为 500，实际 %s", stats.TodayStats.TotalAmount)

	// 昨日
	assert.Equal(t, yesterday, stats.YesterdayStats.Date)
	assert.Equal(t, 3, stats.YesterdayStats.TotalOrders)
	assert.True(t, stats.YesterdayStats.TotalAmount.Equal(decimal.RequireFromString("300.000000")),
		"昨日金额应为 300，实际 %s", stats.YesterdayStats.TotalAmount)

	// 本周：昨日一定 >= 本周起始，因此本周累计 = 今日 + 昨日
	assert.Equal(t, weekStart+"~"+today, stats.WeekStats.Date)
	assert.Equal(t, 8, stats.WeekStats.TotalOrders)
	assert.True(t, stats.WeekStats.TotalAmount.Equal(decimal.RequireFromString("800.000000")),
		"本周金额应为 800，实际 %s", stats.WeekStats.TotalAmount)

	// 本月：如果昨日仍在当月则累计今日 + 昨日，否则仅今日
	assert.Equal(t, monthStart+"~"+today, stats.MonthStats.Date)
	var expectedMonthOrders int
	var expectedMonthAmount decimal.Decimal
	if yesterday >= monthStart {
		expectedMonthOrders = 8
		expectedMonthAmount = decimal.RequireFromString("800.000000")
	} else {
		expectedMonthOrders = 5
		expectedMonthAmount = decimal.RequireFromString("500.000000")
	}
	assert.Equal(t, expectedMonthOrders, stats.MonthStats.TotalOrders)
	assert.True(t, stats.MonthStats.TotalAmount.Equal(expectedMonthAmount),
		"本月金额应为 %s，实际 %s", expectedMonthAmount, stats.MonthStats.TotalAmount)
}

// ---------- TestGetRevenueTrend ----------

func TestGetRevenueTrend(t *testing.T) {
	db := setupAnalyticsTestDB(t)
	ctx := context.Background()
	svc := NewAnalyticsService(db, nil, "test")

	createDailyStats(t, db, "2026-06-01", 2, "100.000000")
	createDailyStats(t, db, "2026-06-02", 3, "200.000000")
	createDailyStats(t, db, "2026-06-03", 1, "50.000000")
	createDailyStats(t, db, "2026-06-04", 4, "400.000000")

	trend, err := svc.GetRevenueTrend(ctx, "2026-06-01", "2026-06-04", "day")
	require.NoError(t, err)
	require.Len(t, trend, 4, "应按天分 4 组")

	expected := []struct {
		date    string
		orders  int
		amount  string
	}{
		{"2026-06-01", 2, "100.000000"},
		{"2026-06-02", 3, "200.000000"},
		{"2026-06-03", 1, "50.000000"},
		{"2026-06-04", 4, "400.000000"},
	}

	for i, exp := range expected {
		assert.Equal(t, exp.date, trend[i].Date)
		assert.Equal(t, exp.orders, trend[i].TotalOrders)
		assert.True(t, trend[i].TotalAmount.Equal(decimal.RequireFromString(exp.amount)),
			"第 %d 个数据点金额应为 %s，实际 %s", i, exp.amount, trend[i].TotalAmount)
	}
}

// ---------- TestGetHotGoods ----------

func TestGetHotGoods(t *testing.T) {
	db := setupAnalyticsTestDB(t)
	ctx := context.Background()
	svc := NewAnalyticsService(db, nil, "test")

	date := "2026-06-15"
	err := db.CreateInBatches([]HotGoods{
		{Date: date, GoodsID: 1, GoodsName: "商品A", OrderCount: 10, TotalAmount: decimal.RequireFromString("1000.000000")},
		{Date: date, GoodsID: 2, GoodsName: "商品B", OrderCount: 8, TotalAmount: decimal.RequireFromString("800.000000")},
		{Date: date, GoodsID: 3, GoodsName: "商品C", OrderCount: 5, TotalAmount: decimal.RequireFromString("600.000000")},
	}, 100).Error
	require.NoError(t, err, "创建热卖商品数据失败")

	list, err := svc.GetHotGoods(ctx, 2, date)
	require.NoError(t, err)
	require.Len(t, list, 2, "应返回 Top2")

	assert.Equal(t, uint(1), list[0].GoodsID)
	assert.Equal(t, "商品A", list[0].GoodsName)
	assert.Equal(t, 10, list[0].OrderCount)
	assert.True(t, list[0].TotalAmount.Equal(decimal.RequireFromString("1000.000000")))

	assert.Equal(t, uint(2), list[1].GoodsID)
	assert.Equal(t, "商品B", list[1].GoodsName)
}
