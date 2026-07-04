package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
)

func (h *Handler) analyticsService() *analytics.AnalyticsService {
	if h.AnalyticsSvc != nil {
		return h.AnalyticsSvc
	}
	return analytics.NewAnalyticsService(h.DB, nil, "sup")
}

// __END__

// GetDashboard GET /admin/analytics/dashboard — 综合大盘
func (h *Handler) GetDashboard(c *gin.Context) {
	svc := h.analyticsService()
	dash, err := svc.GetDashboard(c.Request.Context())
	if err != nil {
		response.Error(c, fmt.Sprintf("查询大盘失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"today":     formatPeriodStats(dash.TodayStats),
		"yesterday": formatPeriodStats(dash.YesterdayStats),
		"week":      formatPeriodStats(dash.WeekStats),
		"month":     formatPeriodStats(dash.MonthStats),
	})
}

// GetRevenueTrend GET /admin/analytics/revenue-trend — 收入趋势
func (h *Handler) GetRevenueTrend(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	granularity := c.DefaultQuery("granularity", "day")

	if startDate == "" || endDate == "" {
		response.ParamError(c, "date", "start_date 和 end_date 不能为空")
		return
	}

	svc := h.analyticsService()
	list, err := svc.GetRevenueTrend(c.Request.Context(), startDate, endDate, granularity)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询收入趋势失败: %v", err))
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, p := range list {
		items = append(items, gin.H{
			"date":          p.Date,
			"total_orders":  p.TotalOrders,
			"total_amount":  p.TotalAmount.StringFixed(2),
			"total_refunds": p.TotalRefunds,
			"refund_amount": p.RefundAmount.StringFixed(2),
		})
	}

	response.Success(c, gin.H{
		"list":        items,
		"granularity": granularity,
		"start_date":  startDate,
		"end_date":    endDate,
	})
}

// GetHotGoods GET /admin/analytics/hot-goods — 热卖排行
func (h *Handler) GetHotGoods(c *gin.Context) {
	topN, _ := strconv.Atoi(c.DefaultQuery("top_n", "10"))
	if topN < 1 {
		topN = 10
	}
	if topN > 100 {
		topN = 100
	}

	date := c.DefaultQuery("date", "")

	svc := h.analyticsService()
	list, err := svc.GetHotGoods(c.Request.Context(), topN, date)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询热卖商品失败: %v", err))
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, g := range list {
		items = append(items, gin.H{
			"goods_id":     g.GoodsID,
			"goods_name":   g.GoodsName,
			"order_count":  g.OrderCount,
			"total_amount": g.TotalAmount.StringFixed(2),
		})
	}

	response.Success(c, gin.H{
		"list": items,
		"date": date,
	})
}

// GetOrderStats GET /admin/analytics/order-stats — 订单统计（按状态分组）
func (h *Handler) GetOrderStats(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		response.ParamError(c, "date", "start_date 和 end_date 不能为空")
		return
	}

	svc := h.analyticsService()
	list, err := svc.GetOrderStats(c.Request.Context(), startDate, endDate)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询订单统计失败: %v", err))
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, s := range list {
		items = append(items, gin.H{
			"status": s.Status,
			"count":  s.Count,
			"amount": s.Amount.StringFixed(2),
		})
	}

	response.Success(c, gin.H{
		"list":       items,
		"start_date": startDate,
		"end_date":   endDate,
	})
}

// GetCustomerStats GET /admin/analytics/customer-stats — 客户统计
func (h *Handler) GetCustomerStats(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		response.ParamError(c, "date", "start_date 和 end_date 不能为空")
		return
	}

	svc := h.analyticsService()
	stats, err := svc.GetCustomerStats(c.Request.Context(), startDate, endDate)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询客户统计失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"start_date":       stats.StartDate,
		"end_date":         stats.EndDate,
		"new_customers":    stats.NewCustomers,
		"active_customers": stats.ActiveCustomers,
		"churn_customers":  stats.ChurnCustomers,
	})
}

// TriggerAggregate POST /admin/analytics/aggregate — 手动触发预聚合
func (h *Handler) TriggerAggregate(c *gin.Context) {
	var req struct {
		Date string `json:"date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "date", "请求参数格式错误")
		return
	}

	date := req.Date
	if date == "" {
		cst := time.FixedZone("CST", 8*3600)
		date = time.Now().In(cst).Format("2006-01-02")
	}

	svc := h.analyticsService()
	if err := svc.AggregateDaily(c.Request.Context(), date); err != nil {
		response.Error(c, fmt.Sprintf("触发统计聚合失败: %v", err))
		return
	}

	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(c.Request.Context(), audit.NewEntry(
		adminID, adminName, "analytics.aggregate", "analytics", 0,
		fmt.Sprintf("手动触发数据统计聚合, date=%s", date),
	))

	response.Success(c, gin.H{
		"date":   date,
		"status": "ok",
	})
}

func formatPeriodStats(ps analytics.PeriodStats) gin.H {
	return gin.H{
		"date":             ps.Date,
		"total_orders":     ps.TotalOrders,
		"total_amount":     ps.TotalAmount.StringFixed(2),
		"total_refunds":    ps.TotalRefunds,
		"refund_amount":    ps.RefundAmount.StringFixed(2),
		"new_customers":    ps.NewCustomers,
		"active_customers": ps.ActiveCustomers,
	}
}
