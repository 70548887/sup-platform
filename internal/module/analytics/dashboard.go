package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
)

// DashboardService 大盘数据服务
type DashboardService struct {
	repo  *AnalyticsRepository
	cache cache.CacheProvider
}

// NewDashboardService 创建大盘数据服务
func NewDashboardService(repo *AnalyticsRepository, c cache.CacheProvider) *DashboardService {
	return &DashboardService{repo: repo, cache: c}
}

// GetDashboard 综合大盘数据
func (s *DashboardService) GetDashboard(ctx context.Context) (*DashboardStats, error) {
	var result DashboardStats
	err := s.cache.GetOrLoad(ctx, "analytics:dashboard", &result, time.Hour, func() (interface{}, error) {
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

		return &DashboardStats{
			TodayStats:     todayStats,
			YesterdayStats: yesterdayStats,
			WeekStats:      weekStats,
			MonthStats:     monthStats,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DashboardService) statsForDate(date string) (PeriodStats, error) {
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

func (s *DashboardService) sumRange(startDate, endDate string) (PeriodStats, error) {
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

// weekStart 计算本周一日期
func weekStart(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	days := wd - 1
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, cst).AddDate(0, 0, -days)
}
