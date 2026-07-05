package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
	"gorm.io/gorm"
)

// QuotaService API配额管理服务
type QuotaService struct {
	repo  *BillingRepository
	cache cache.CacheProvider
	db    *gorm.DB
}

// NewQuotaService 创建配额管理服务实例
func NewQuotaService(repo *BillingRepository, c cache.CacheProvider, db *gorm.DB) *QuotaService {
	return &QuotaService{repo: repo, cache: c, db: db}
}

// quotaCache 缓存结构
type quotaCache struct {
	Allowed   bool `json:"allowed"`
	Remaining int  `json:"remaining"`
}

// CheckQuota 检查租户API配额
// Redis key: {prefix}:tenant:{tenantID}:quota, TTL=5min
// 降级：Redis不可用时查DB
func (q *QuotaService) CheckQuota(ctx context.Context, tenantID uint) (allowed bool, remaining int, err error) {
	cacheKey := fmt.Sprintf("tenant:%d:quota", tenantID)

	// 尝试从缓存读取
	var qc quotaCache
	if cacheErr := q.cache.Get(ctx, cacheKey, &qc); cacheErr == nil {
		return qc.Allowed, qc.Remaining, nil
	}

	// 缓存miss或不可用，查DB
	_, plan, err := q.repo.GetSubscriptionWithPlan(tenantID)
	if err != nil {
		// 区分无订阅和系统错误
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 无订阅的租户：拒绝访问
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("billing: check quota get subscription: %w", err)
	}

	now := time.Now()
	usage, err := q.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		return false, 0, fmt.Errorf("billing: check quota get usage: %w", err)
	}

	remaining = plan.MaxAPICallsPerMonth - usage.APICallCount
	allowed = remaining > 0

	// 写入缓存
	_ = q.cache.Set(ctx, cacheKey, quotaCache{Allowed: allowed, Remaining: remaining}, 5*time.Minute)

	return allowed, remaining, nil
}

// RecordAPICall 记录一次API调用
// CAS重试3次，失败仅log不返回error
func (q *QuotaService) RecordAPICall(ctx context.Context, tenantID uint) error {
	now := time.Now()
	usage, err := q.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		log.Printf("billing: record api call get usage failed: tenant=%d err=%v", tenantID, err)
		return nil
	}

	// CAS重试3次
	for i := 0; i < 3; i++ {
		ok, casErr := q.repo.IncrementAPIUsageCAS(usage.ID, usage.Version)
		if casErr != nil {
			log.Printf("billing: record api call CAS error: tenant=%d err=%v", tenantID, casErr)
			return nil
		}
		if ok {
			// 成功后删除配额缓存
			q.invalidateQuotaCache(ctx, tenantID)
			return nil
		}
		// CAS冲突，重新读取version
		usage, err = q.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
		if err != nil {
			log.Printf("billing: record api call reload usage failed: tenant=%d err=%v", tenantID, err)
			return nil
		}
	}

	log.Printf("billing: record api call CAS exhausted retries: tenant=%d", tenantID)
	return nil
}

// GetUsage 获取租户当月使用量
func (q *QuotaService) GetUsage(ctx context.Context, tenantID uint) (*APIUsage, error) {
	now := time.Now()
	usage, err := q.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, fmt.Errorf("billing: get usage: %w", err)
	}
	return usage, nil
}

// invalidateQuotaCache 删除配额缓存
func (q *QuotaService) invalidateQuotaCache(ctx context.Context, tenantID uint) {
	cacheKey := fmt.Sprintf("tenant:%d:quota", tenantID)
	_ = q.cache.Del(ctx, cacheKey)
}
