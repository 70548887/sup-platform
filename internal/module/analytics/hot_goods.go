package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
)

// HotGoodsService 热卖商品服务
type HotGoodsService struct {
	repo  *AnalyticsRepository
	cache cache.CacheProvider
}

// NewHotGoodsService 创建热卖商品服务
func NewHotGoodsService(repo *AnalyticsRepository, c cache.CacheProvider) *HotGoodsService {
	return &HotGoodsService{repo: repo, cache: c}
}

// GetHotGoods 热卖商品排行
func (s *HotGoodsService) GetHotGoods(ctx context.Context, topN int, date string) ([]HotGoods, error) {
	if date == "" {
		date = time.Now().In(cst).Format("2006-01-02")
	}
	if topN <= 0 {
		topN = 10
	}
	if topN > 100 {
		topN = 100
	}

	cacheKey := fmt.Sprintf("analytics:hot:%s", date)
	var result []HotGoods
	err := s.cache.GetOrLoad(ctx, cacheKey, &result, time.Hour, func() (interface{}, error) {
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
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
