package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
)

// TrendService 趋势数据服务
type TrendService struct {
	repo  *AnalyticsRepository
	cache cache.CacheProvider
}

// NewTrendService 创建趋势数据服务
func NewTrendService(repo *AnalyticsRepository, c cache.CacheProvider) *TrendService {
	return &TrendService{repo: repo, cache: c}
}

// GetRevenueTrend 收入趋势
func (s *TrendService) GetRevenueTrend(ctx context.Context, startDate, endDate, granularity string) ([]RevenueTrendPoint, error) {
	if _, _, err := rangeBounds(startDate, endDate); err != nil {
		return nil, fmt.Errorf("analytics: invalid date range: %w", err)
	}
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return nil, fmt.Errorf("analytics: invalid granularity %s", granularity)
	}

	cacheKey := fmt.Sprintf("analytics:trend:%s:%s:%s", startDate, endDate, granularity)
	var result []RevenueTrendPoint
	err := s.cache.GetOrLoad(ctx, cacheKey, &result, time.Hour, func() (interface{}, error) {
		list, err := s.repo.GetStatsRange(startDate, endDate)
		if err != nil {
			return nil, err
		}
		return aggregateTrend(list, granularity), nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
