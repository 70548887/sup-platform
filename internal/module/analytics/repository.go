package analytics

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/70548887/sup-platform/internal/module/refund"
)

var cst = time.FixedZone("CST", 8*3600)

// AnalyticsRepository 统计仓储层
type AnalyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository 创建统计仓储
func NewAnalyticsRepository(db *gorm.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// GetDailyStats 获取单日预聚合统计
func (r *AnalyticsRepository) GetDailyStats(date string) (*DailyStats, error) {
	var stats DailyStats
	err := r.db.Where("date = ?", date).First(&stats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("analytics: get daily stats failed: %w", err)
	}
	return &stats, nil
}

// GetStatsRange 获取日期范围内的预聚合统计
func (r *AnalyticsRepository) GetStatsRange(startDate, endDate string) ([]DailyStats, error) {
	var list []DailyStats
	err := r.db.Where("date >= ? AND date <= ?", startDate, endDate).Order("date ASC").Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("analytics: get stats range failed: %w", err)
	}
	return list, nil
}

// GetHotGoods 获取某日热卖排行
func (r *AnalyticsRepository) GetHotGoods(date string, topN int) ([]HotGoods, error) {
	var list []HotGoods
	err := r.db.Where("date = ?", date).Order("order_count DESC, total_amount DESC").Limit(topN).Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("analytics: get hot goods failed: %w", err)
	}
	return list, nil
}

// SaveDailyStats 保存或更新单日统计（按 date 唯一键 upsert）
func (r *AnalyticsRepository) SaveDailyStats(stats *DailyStats) error {
	err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_orders",
			"total_amount",
			"total_refunds",
			"refund_amount",
			"new_customers",
			"active_customers",
		}),
	}).Create(stats).Error
	if err != nil {
		return fmt.Errorf("analytics: save daily stats failed: %w", err)
	}
	return nil
}

// SaveHotGoods 保存某日热卖排行（先删除旧数据再批量插入）
func (r *AnalyticsRepository) SaveHotGoods(goods []HotGoods) error {
	if len(goods) == 0 {
		return nil
	}
	date := goods[0].Date
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("date = ?", date).Delete(&HotGoods{}).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(goods, 100).Error; err != nil {
			return err
		}
		return nil
	})
}

type aggregateOrderResult struct {
	TotalOrders int             `gorm:"column:total_orders"`
	TotalAmount decimal.Decimal `gorm:"column:total_amount"`
}

type aggregateRefundResult struct {
	TotalRefunds int             `gorm:"column:total_refunds"`
	RefundAmount decimal.Decimal `gorm:"column:refund_amount"`
}

// AggregateOrderStats 按日期实时聚合订单、退款、客户数据
func (r *AnalyticsRepository) AggregateOrderStats(date string) (*DailyStats, error) {
	start, end, err := dayBounds(date)
	if err != nil {
		return nil, fmt.Errorf("analytics: invalid date %s: %w", date, err)
	}

	stats := &DailyStats{Date: date}

	var orderRow aggregateOrderResult
	if err := r.db.Raw(
		"SELECT COUNT(*) AS total_orders, COALESCE(SUM(amount),0) AS total_amount FROM orders WHERE created_at >= ? AND created_at < ?",
		start, end,
	).Scan(&orderRow).Error; err != nil {
		return nil, fmt.Errorf("analytics: aggregate orders failed: %w", err)
	}
	stats.TotalOrders = orderRow.TotalOrders
	stats.TotalAmount = orderRow.TotalAmount

	var refundRow aggregateRefundResult
	if err := r.db.Raw(
		"SELECT COUNT(*) AS total_refunds, COALESCE(SUM(amount),0) AS refund_amount FROM refund_orders WHERE status IN (?,?) AND created_at >= ? AND created_at < ?",
		refund.RefundApproved, refund.RefundCompleted, start, end,
	).Scan(&refundRow).Error; err != nil {
		return nil, fmt.Errorf("analytics: aggregate refunds failed: %w", err)
	}
	stats.TotalRefunds = refundRow.TotalRefunds
	stats.RefundAmount = refundRow.RefundAmount

	var newCustomers int
	if err := r.db.Raw(
		"SELECT COUNT(DISTINCT customer_id) AS new_customers FROM orders WHERE created_at >= ? AND created_at < ? AND customer_id NOT IN (SELECT DISTINCT customer_id FROM orders WHERE created_at < ?)",
		start, end, start,
	).Scan(&newCustomers).Error; err != nil {
		return nil, fmt.Errorf("analytics: aggregate new customers failed: %w", err)
	}
	stats.NewCustomers = newCustomers

	var activeCustomers int
	if err := r.db.Raw(
		"SELECT COUNT(DISTINCT customer_id) AS active_customers FROM orders WHERE created_at >= ? AND created_at < ?",
		start, end,
	).Scan(&activeCustomers).Error; err != nil {
		return nil, fmt.Errorf("analytics: aggregate active customers failed: %w", err)
	}
	stats.ActiveCustomers = activeCustomers

	return stats, nil
}

type hotGoodsRow struct {
	GoodsID     uint
	GoodsName   string
	OrderCount  int             `gorm:"column:order_count"`
	TotalAmount decimal.Decimal `gorm:"column:total_amount"`
}

// AggregateHotGoods 按日期实时聚合热卖商品
func (r *AnalyticsRepository) AggregateHotGoods(date string, topN int) ([]HotGoods, error) {
	start, end, err := dayBounds(date)
	if err != nil {
		return nil, fmt.Errorf("analytics: invalid date %s: %w", date, err)
	}

	var rows []hotGoodsRow
	if err := r.db.Raw(
		"SELECT goods_id, goods_name, COUNT(*) AS order_count, COALESCE(SUM(amount),0) AS total_amount FROM orders WHERE created_at >= ? AND created_at < ? GROUP BY goods_id, goods_name ORDER BY order_count DESC LIMIT ?",
		start, end, topN,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("analytics: aggregate hot goods failed: %w", err)
	}

	list := make([]HotGoods, 0, len(rows))
	for _, row := range rows {
		list = append(list, HotGoods{
			Date:        date,
			GoodsID:     row.GoodsID,
			GoodsName:   row.GoodsName,
			OrderCount:  row.OrderCount,
			TotalAmount: row.TotalAmount,
		})
	}
	return list, nil
}

// GetOrderStatusStats 按状态统计订单
func (r *AnalyticsRepository) GetOrderStatusStats(start, end int64) ([]OrderStatusStats, error) {
	var rows []OrderStatusStats
	err := r.db.Raw(
		"SELECT status, COUNT(*) AS count, COALESCE(SUM(amount),0) AS amount FROM orders WHERE created_at >= ? AND created_at < ? GROUP BY status ORDER BY status",
		start, end,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("analytics: get order status stats failed: %w", err)
	}
	return rows, nil
}

// GetCustomerStats 统计客户数据
func (r *AnalyticsRepository) GetCustomerStats(start, end int64) (*CustomerStats, error) {
	var activeCustomers int
	if err := r.db.Raw(
		"SELECT COUNT(DISTINCT customer_id) AS active_customers FROM orders WHERE created_at >= ? AND created_at < ?",
		start, end,
	).Scan(&activeCustomers).Error; err != nil {
		return nil, fmt.Errorf("analytics: get active customers failed: %w", err)
	}

	var newCustomers int
	if err := r.db.Raw(
		"SELECT COUNT(DISTINCT customer_id) AS new_customers FROM orders WHERE created_at >= ? AND created_at < ? AND customer_id NOT IN (SELECT DISTINCT customer_id FROM orders WHERE created_at < ?)",
		start, end, start,
	).Scan(&newCustomers).Error; err != nil {
		return nil, fmt.Errorf("analytics: get new customers failed: %w", err)
	}

	var churnCustomers int
	if err := r.db.Raw(
		"SELECT COUNT(DISTINCT customer_id) AS churn_customers FROM orders WHERE created_at < ? AND customer_id NOT IN (SELECT DISTINCT customer_id FROM orders WHERE created_at >= ? AND created_at < ?)",
		start, start, end,
	).Scan(&churnCustomers).Error; err != nil {
		return nil, fmt.Errorf("analytics: get churn customers failed: %w", err)
	}

	return &CustomerStats{
		NewCustomers:    newCustomers,
		ActiveCustomers: activeCustomers,
		ChurnCustomers:  churnCustomers,
	}, nil
}

// dayBounds 返回指定日期（UTC+8）的 [开始, 结束) Unix 时间戳
func dayBounds(date string) (int64, int64, error) {
	t, err := time.ParseInLocation("2006-01-02", date, cst)
	if err != nil {
		return 0, 0, err
	}
	return t.Unix(), t.Add(24 * time.Hour).Unix(), nil
}

// rangeBounds 返回日期范围（UTC+8）的 [开始, 结束) Unix 时间戳
func rangeBounds(startDate, endDate string) (int64, int64, error) {
	start, _, err := dayBounds(startDate)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start_date: %w", err)
	}
	_, end, err := dayBounds(endDate)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end_date: %w", err)
	}
	if end < start {
		return 0, 0, fmt.Errorf("end_date must be after start_date")
	}
	return start, end, nil
}
