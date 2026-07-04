package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"time"

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

// AnalyticsService 统计服务层
type AnalyticsService struct {
	db          *gorm.DB
	repo        *AnalyticsRepository
	redisClient *redis.Client
	prefix      string
}

// NewAnalyticsService 创建统计服务
func NewAnalyticsService(db *gorm.DB, redisClient *redis.Client, prefix string) *AnalyticsService {
	return &AnalyticsService{
		db:          db,
		repo:        NewAnalyticsRepository(db),
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// GetDashboard 综合大盘数据
func (s *AnalyticsService) GetDashboard(ctx context.Context) (*DashboardStats, error) {
	key := fmt.Sprintf("%s:analytics:dashboard", s.prefix)

	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, key).Result()
		if err == nil {
			var cached DashboardStats
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return &cached, nil
			}
		} else if err != redis.Nil {
			// Redis 降级：继续查库
		}
	}

	now := time.Now().In(cst)
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
	weekStart := weekStart(now).Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cst).Format("2006-01-02")

	todayStats, err := s.statsForDate(today)
	if err != nil {
		return nil, err
	}
	yesterdayStats, err := s.statsForDate(yesterday)
	if err != nil {
		return nil, err
	}
	weekStats, err := s.sumRange(weekStart, today)
	if err != nil {
		return nil, err
	}
	monthStats, err := s.sumRange(monthStart, today)
	if err != nil {
		return nil, err
	}

	res := &DashboardStats{
		TodayStats:     todayStats,
		YesterdayStats: yesterdayStats,
		WeekStats:      weekStats,
		MonthStats:     monthStats,
	}

	if s.redisClient != nil {
		b, _ := json.Marshal(res)
		ttl := time.Hour + time.Duration(rand.Intn(300))*time.Second
		_ = s.redisClient.Set(ctx, key, b, ttl).Err()
	}

	return res, nil
}

// GetRevenueTrend 收入趋势
func (s *AnalyticsService) GetRevenueTrend(ctx context.Context, startDate, endDate, granularity string) ([]RevenueTrendPoint, error) {
	if _, _, err := rangeBounds(startDate, endDate); err != nil {
		return nil, fmt.Errorf("analytics: invalid date range: %w", err)
	}
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return nil, fmt.Errorf("analytics: invalid granularity %s", granularity)
	}

	key := fmt.Sprintf("%s:analytics:trend:%s:%s:%s", s.prefix, startDate, endDate, granularity)

	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, key).Result()
		if err == nil {
			var cached []RevenueTrendPoint
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		} else if err != redis.Nil {
			// Redis 降级：继续查库
		}
	}

	list, err := s.repo.GetStatsRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := aggregateTrend(list, granularity)

	if s.redisClient != nil {
		b, _ := json.Marshal(result)
		ttl := time.Hour + time.Duration(rand.Intn(300))*time.Second
		_ = s.redisClient.Set(ctx, key, b, ttl).Err()
	}

	return result, nil
}

// GetHotGoods 热卖商品排行
func (s *AnalyticsService) GetHotGoods(ctx context.Context, topN int, date string) ([]HotGoods, error) {
	if date == "" {
		date = time.Now().In(cst).Format("2006-01-02")
	}
	if topN <= 0 {
		topN = 10
	}
	if topN > 100 {
		topN = 100
	}

	key := fmt.Sprintf("%s:analytics:hot:%s", s.prefix, date)

	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, key).Result()
		if err == nil {
			var cached []HotGoods
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		} else if err != redis.Nil {
			// Redis 降级：继续查库
		}
	}

	list, err := s.repo.GetHotGoods(date, topN)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		list, err = s.repo.AggregateHotGoods(date, topN)
		if err != nil {
			return nil, err
		}
	}

	if s.redisClient != nil {
		b, _ := json.Marshal(list)
		ttl := time.Hour + time.Duration(rand.Intn(300))*time.Second
		_ = s.redisClient.Set(ctx, key, b, ttl).Err()
	}

	return list, nil
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

	if s.redisClient != nil {
		dashboardKey := fmt.Sprintf("%s:analytics:dashboard", s.prefix)
		hotKey := fmt.Sprintf("%s:analytics:hot:%s", s.prefix, date)
		_ = s.redisClient.Del(ctx, dashboardKey, hotKey).Err()
	}

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

func (s *AnalyticsService) statsForDate(date string) (PeriodStats, error) {
	ds, err := s.repo.GetDailyStats(date)
	if err != nil {
		return PeriodStats{}, err
	}
	if ds == nil {
		return PeriodStats{Date: date}, nil
	}
	return PeriodStats{
		Date:            ds.Date,
		TotalOrders:     ds.TotalOrders,
		TotalAmount:     ds.TotalAmount,
		TotalRefunds:    ds.TotalRefunds,
		RefundAmount:    ds.RefundAmount,
		NewCustomers:    ds.NewCustomers,
		ActiveCustomers: ds.ActiveCustomers,
	}, nil
}

func (s *AnalyticsService) sumRange(startDate, endDate string) (PeriodStats, error) {
	list, err := s.repo.GetStatsRange(startDate, endDate)
	if err != nil {
		return PeriodStats{}, err
	}
	ps := PeriodStats{Date: fmt.Sprintf("%s~%s", startDate, endDate)}
	for _, ds := range list {
		ps.TotalOrders += ds.TotalOrders
		ps.TotalAmount = ps.TotalAmount.Add(ds.TotalAmount)
		ps.TotalRefunds += ds.TotalRefunds
		ps.RefundAmount = ps.RefundAmount.Add(ds.RefundAmount)
		ps.NewCustomers += ds.NewCustomers
		ps.ActiveCustomers += ds.ActiveCustomers
	}
	return ps, nil
}

func aggregateTrend(list []DailyStats, granularity string) []RevenueTrendPoint {
	groups := make(map[string]*RevenueTrendPoint)
	keys := make([]string, 0)

	for _, ds := range list {
		key := trendKey(ds.Date, granularity)
		point, ok := groups[key]
		if !ok {
			point = &RevenueTrendPoint{Date: key}
			groups[key] = point
			keys = append(keys, key)
		}
		point.TotalOrders += ds.TotalOrders
		point.TotalAmount = point.TotalAmount.Add(ds.TotalAmount)
		point.TotalRefunds += ds.TotalRefunds
		point.RefundAmount = point.RefundAmount.Add(ds.RefundAmount)
	}

	sort.Strings(keys)
	result := make([]RevenueTrendPoint, 0, len(keys))
	for _, k := range keys {
		result = append(result, *groups[k])
	}
	return result
}

func trendKey(date string, granularity string) string {
	t, err := time.ParseInLocation("2006-01-02", date, cst)
	if err != nil {
		return date
	}
	switch granularity {
	case "week":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "month":
		return t.Format("2006-01")
	default:
		return date
	}
}

func weekStart(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	days := wd - 1
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, cst).AddDate(0, 0, -days)
}
