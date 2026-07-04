package tenant

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/order"
)

// DashboardData 仪表盘综合数据
type DashboardData struct {
	Today PeriodStat `json:"today"`
	Week  PeriodStat `json:"week"`
	Month PeriodStat `json:"month"`
}

// PeriodStat 时段统计
type PeriodStat struct {
	OrderCount  int64  `json:"order_count"`
	OrderAmount string `json:"order_amount"`
}

// GetDashboard GET /tenant-admin/dashboard — 租户仪表盘
func (h *Handler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()
	db := h.DB.WithContext(ctx)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Unix()

	// 本周起始（周一）
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -(weekday - 1)).Unix()

	// 本月起始
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Unix()

	// 查询今日
	todayStat := queryPeriodStat(db, todayStart)
	// 查询本周
	weekStat := queryPeriodStat(db, weekStart)
	// 查询本月
	monthStat := queryPeriodStat(db, monthStart)

	response.Success(c, DashboardData{
		Today: todayStat,
		Week:  weekStat,
		Month: monthStat,
	})
}

// queryPeriodStat 查询某时间段内的订单数和金额
func queryPeriodStat(db *gorm.DB, startUnix int64) PeriodStat {
	var result struct {
		Count  int64
		Amount decimal.Decimal
	}
	db.Model(&order.Order{}).
		Where("created_at >= ?", startUnix).
		Select("COUNT(*) as count, COALESCE(SUM(amount), 0) as amount").
		Scan(&result)

	return PeriodStat{
		OrderCount:  result.Count,
		OrderAmount: result.Amount.StringFixed(2),
	}
}
