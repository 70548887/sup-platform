package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PeriodStats 某段时间内的汇总统计
type PeriodStats struct {
	Date            string          `json:"date,omitempty"`
	TotalOrders     int             `json:"total_orders"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	TotalRefunds    int             `json:"total_refunds"`
	RefundAmount    decimal.Decimal `json:"refund_amount"`
	NewCustomers    int             `json:"new_customers"`
	ActiveCustomers int             `json:"active_customers"`
}

// DashboardStats 综合大盘返回结构
type DashboardStats struct {
	TodayStats     PeriodStats `json:"today"`
	YesterdayStats PeriodStats `json:"yesterday"`
	WeekStats      PeriodStats `json:"week"`
	MonthStats     PeriodStats `json:"month"`
}

// RevenueTrendPoint 收入趋势数据点
type RevenueTrendPoint struct {
	Date         string          `json:"date"`
	TotalOrders  int             `json:"total_orders"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	TotalRefunds int             `json:"total_refunds"`
	RefundAmount decimal.Decimal `json:"refund_amount"`
}

// OrderStatusStats 按状态统计订单
type OrderStatusStats struct {
	Status int8            `json:"status"`
	Count  int             `json:"count"`
	Amount decimal.Decimal `json:"amount"`
}

// CustomerStats 客户统计
type CustomerStats struct {
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	NewCustomers    int    `json:"new_customers"`
	ActiveCustomers int    `json:"active_customers"`
	ChurnCustomers  int    `json:"churn_customers"`
}

// AnalyticsService 统计服务层（Facade）
type AnalyticsService struct {
	db        *gorm.DB
	repo      *AnalyticsRepository
	cache     cache.CacheProvider
	dashboard *DashboardService
	trend     *TrendService
	hotGoods  *HotGoodsService
}

// NewAnalyticsService 创建统计服务
func NewAnalyticsService(db *gorm.DB, redisClient *redis.Client, prefix string) *AnalyticsService {
	repo := NewAnalyticsRepository(db)
	c := cache.NewRedisCache(redisClient, prefix)
	return &AnalyticsService{
		db:        db,
		repo:      repo,
		cache:     c,
		dashboard: NewDashboardService(repo, c),
		trend:     NewTrendService(repo, c),
		hotGoods:  NewHotGoodsService(repo, c),
	}
}

// GetDashboard 综合大盘数据
func (s *AnalyticsService) GetDashboard(ctx context.Context) (*DashboardStats, error) {
	return s.dashboard.GetDashboard(ctx)
}

// GetRevenueTrend 收入趋势
func (s *AnalyticsService) GetRevenueTrend(ctx context.Context, startDate, endDate, granularity string) ([]RevenueTrendPoint, error) {
	return s.trend.GetRevenueTrend(ctx, startDate, endDate, granularity)
}

// GetHotGoods 热卖商品排行
func (s *AnalyticsService) GetHotGoods(ctx context.Context, topN int, date string) ([]HotGoods, error) {
	return s.hotGoods.GetHotGoods(ctx, topN, date)
}

// AggregateDaily 预聚合某一天的数据并落库
func (s *AnalyticsService) AggregateDaily(ctx context.Context, date string) error {
	if date == "" {
		date = time.Now().In(cst).Format("2006-01-02")
	}
	if _, _, err := dayBounds(date); err != nil {
		return fmt.Errorf("analytics: invalid date %s: %w", date, err)
	}

	stats, err := s.repo.AggregateOrderStats(date)
	if err != nil {
		return err
	}
	if err := s.repo.SaveDailyStats(stats); err != nil {
		return err
	}

	hotGoods, err := s.repo.AggregateHotGoods(date, 50)
	if err != nil {
		return err
	}
	if err := s.repo.SaveHotGoods(hotGoods); err != nil {
		return err
	}

	_ = s.cache.Del(ctx, "analytics:dashboard", fmt.Sprintf("analytics:hot:%s", date))

	return nil
}

// GetOrderStats 按状态统计订单
func (s *AnalyticsService) GetOrderStats(ctx context.Context, startDate, endDate string) ([]OrderStatusStats, error) {
	start, end, err := rangeBounds(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("analytics: invalid date range: %w", err)
	}
	return s.repo.GetOrderStatusStats(start, end)
}

// GetCustomerStats 客户统计
func (s *AnalyticsService) GetCustomerStats(ctx context.Context, startDate, endDate string) (*CustomerStats, error) {
	start, end, err := rangeBounds(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("analytics: invalid date range: %w", err)
	}
	stats, err := s.repo.GetCustomerStats(start, end)
	if err != nil {
		return nil, err
	}
	stats.StartDate = startDate
	stats.EndDate = endDate
	return stats, nil
}
