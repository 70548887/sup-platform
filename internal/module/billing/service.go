package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// BillingService 计费引擎服务
type BillingService struct {
	db          *gorm.DB
	repo        *BillingRepository
	redisClient *redis.Client
	prefix      string
}

// NewBillingService 创建计费服务实例
func NewBillingService(db *gorm.DB, redisClient *redis.Client, prefix string) *BillingService {
	return &BillingService{
		db:          db,
		repo:        NewBillingRepository(db),
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// quotaCache Redis缓存结构
type quotaCache struct {
	Allowed   bool `json:"allowed"`
	Remaining int  `json:"remaining"`
}

// CheckQuota 检查租户API配额
// Redis key: {prefix}:tenant:{tenantID}:quota, TTL=5min
// 降级：Redis不可用时查DB
func (s *BillingService) CheckQuota(ctx context.Context, tenantID uint) (allowed bool, remaining int, err error) {
	cacheKey := fmt.Sprintf("%s:tenant:%d:quota", s.prefix, tenantID)

	// 尝试从Redis读取缓存
	if s.redisClient != nil {
		cached, redisErr := s.redisClient.Get(ctx, cacheKey).Result()
		if redisErr == nil && cached != "" {
			var qc quotaCache
			if jsonErr := json.Unmarshal([]byte(cached), &qc); jsonErr == nil {
				return qc.Allowed, qc.Remaining, nil
			}
		}
		// Redis错误时静默降级，继续查DB
	}

	// Redis miss或不可用，查DB
	_, plan, err := s.repo.GetSubscriptionWithPlan(tenantID)
	if err != nil {
		return false, 0, fmt.Errorf("billing: check quota get subscription: %w", err)
	}

	now := time.Now()
	usage, err := s.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		return false, 0, fmt.Errorf("billing: check quota get usage: %w", err)
	}

	remaining = plan.MaxAPICallsPerMonth - usage.APICallCount
	allowed = remaining > 0

	// 写入Redis缓存
	if s.redisClient != nil {
		qc := quotaCache{Allowed: allowed, Remaining: remaining}
		if data, jsonErr := json.Marshal(qc); jsonErr == nil {
			_ = s.redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err()
		}
	}

	return allowed, remaining, nil
}

// RecordAPICall 记录一次API调用
// CAS重试3次，失败仅log不返回error
func (s *BillingService) RecordAPICall(ctx context.Context, tenantID uint) error {
	now := time.Now()
	usage, err := s.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		log.Printf("billing: record api call get usage failed: tenant=%d err=%v", tenantID, err)
		return nil
	}

	// CAS重试3次
	for i := 0; i < 3; i++ {
		ok, casErr := s.repo.IncrementAPIUsageCAS(usage.ID, usage.Version)
		if casErr != nil {
			log.Printf("billing: record api call CAS error: tenant=%d err=%v", tenantID, casErr)
			return nil
		}
		if ok {
			// 成功后删除Redis配额缓存
			s.invalidateQuotaCache(ctx, tenantID)
			return nil
		}
		// CAS冲突，重新读取version
		usage, err = s.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
		if err != nil {
			log.Printf("billing: record api call reload usage failed: tenant=%d err=%v", tenantID, err)
			return nil
		}
	}

	log.Printf("billing: record api call CAS exhausted retries: tenant=%d", tenantID)
	return nil
}

// GenerateMonthlyInvoice 生成月度账单
func (s *BillingService) GenerateMonthlyInvoice(ctx context.Context, tenantID uint, month string) (*Invoice, error) {
	_, plan, err := s.repo.GetSubscriptionWithPlan(tenantID)
	if err != nil {
		return nil, fmt.Errorf("billing: generate invoice get subscription: %w", err)
	}

	// 解析month字段获取年月
	t, parseErr := time.Parse("2006-01", month)
	if parseErr != nil {
		return nil, fmt.Errorf("billing: generate invoice parse month %q: %w", month, parseErr)
	}

	usage, err := s.repo.GetOrCreateUsage(tenantID, t.Year(), int(t.Month()))
	if err != nil {
		return nil, fmt.Errorf("billing: generate invoice get usage: %w", err)
	}

	// 计算超额费用: max(0, usage.APICallCount - plan.MaxAPICallsPerMonth) * 0.01
	overageCount := usage.APICallCount - plan.MaxAPICallsPerMonth
	var overageCharge decimal.Decimal
	if overageCount > 0 {
		overageCharge = decimal.NewFromInt(int64(overageCount)).Mul(decimal.NewFromFloat(0.01))
	} else {
		overageCharge = decimal.Zero
	}

	planFee := plan.MonthlyPrice
	totalAmount := planFee.Add(overageCharge)

	invoice := &Invoice{
		TenantID:      tenantID,
		Month:         month,
		PlanFee:       planFee,
		OverageCharge: overageCharge,
		TotalAmount:   totalAmount,
		Status:        "pending",
		IssuedAt:      time.Now().Unix(),
	}

	if err := s.repo.CreateInvoice(invoice); err != nil {
		return nil, fmt.Errorf("billing: generate invoice create: %w", err)
	}

	return invoice, nil
}

// InitDefaultPlans 确保三个默认套餐存在
func (s *BillingService) InitDefaultPlans(ctx context.Context) error {
	defaults := []SubscriptionPlan{
		{
			Name:                "basic",
			DisplayName:         "基础版",
			MonthlyPrice:        decimal.NewFromInt(99),
			MaxAPICallsPerMonth: 10000,
			MaxOrders:           1000,
			MaxAdmins:           3,
			Features:            `{"support":"email"}`,
			Status:              1,
		},
		{
			Name:                "professional",
			DisplayName:         "专业版",
			MonthlyPrice:        decimal.NewFromInt(299),
			MaxAPICallsPerMonth: 100000,
			MaxOrders:           10000,
			MaxAdmins:           10,
			Features:            `{"support":"email,phone","priority":true}`,
			Status:              1,
		},
		{
			Name:                "enterprise",
			DisplayName:         "企业版",
			MonthlyPrice:        decimal.NewFromInt(999),
			MaxAPICallsPerMonth: 1000000,
			MaxOrders:           100000,
			MaxAdmins:           50,
			Features:            `{"support":"email,phone,dedicated","priority":true,"custom_domain":true}`,
			Status:              1,
		},
	}

	for i := range defaults {
		existing, err := s.repo.GetPlanByName(defaults[i].Name)
		if err != nil {
			return fmt.Errorf("billing: init default plans check %s: %w", defaults[i].Name, err)
		}
		if existing == nil {
			if err := s.repo.CreatePlan(&defaults[i]); err != nil {
				return fmt.Errorf("billing: init default plans create %s: %w", defaults[i].Name, err)
			}
		}
	}

	return nil
}

// CreateSubscription 创建租户订阅
func (s *BillingService) CreateSubscription(ctx context.Context, tenantID, planID uint) (*TenantSubscription, error) {
	plan, err := s.repo.GetPlan(planID)
	if err != nil {
		return nil, fmt.Errorf("billing: create subscription get plan: %w", err)
	}
	_ = plan

	now := time.Now()
	sub := &TenantSubscription{
		TenantID:  tenantID,
		PlanID:    planID,
		StartAt:   now.Unix(),
		EndAt:     now.AddDate(0, 1, 0).Unix(),
		AutoRenew: true,
		Status:    "active",
	}

	if err := s.repo.CreateSubscription(sub); err != nil {
		return nil, fmt.Errorf("billing: create subscription: %w", err)
	}

	return sub, nil
}

// UpgradeSubscription 升级租户套餐
func (s *BillingService) UpgradeSubscription(ctx context.Context, tenantID, newPlanID uint) (*TenantSubscription, error) {
	sub, err := s.repo.GetCurrentSubscription(tenantID)
	if err != nil {
		return nil, fmt.Errorf("billing: upgrade subscription get current: %w", err)
	}

	// 验证新套餐存在
	_, err = s.repo.GetPlan(newPlanID)
	if err != nil {
		return nil, fmt.Errorf("billing: upgrade subscription get plan: %w", err)
	}

	sub.PlanID = newPlanID
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return nil, fmt.Errorf("billing: upgrade subscription update: %w", err)
	}

	// 升级后清除配额缓存
	s.invalidateQuotaCache(ctx, tenantID)

	return sub, nil
}

// CancelSubscription 取消租户订阅
func (s *BillingService) CancelSubscription(ctx context.Context, tenantID uint) error {
	sub, err := s.repo.GetCurrentSubscription(tenantID)
	if err != nil {
		return fmt.Errorf("billing: cancel subscription get current: %w", err)
	}

	sub.Status = "cancelled"
	sub.AutoRenew = false
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return fmt.Errorf("billing: cancel subscription update: %w", err)
	}

	// 取消后清除配额缓存
	s.invalidateQuotaCache(ctx, tenantID)

	return nil
}

// GetSubscription 获取租户当前订阅
func (s *BillingService) GetSubscription(ctx context.Context, tenantID uint) (*TenantSubscription, *SubscriptionPlan, error) {
	sub, plan, err := s.repo.GetSubscriptionWithPlan(tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("billing: get subscription: %w", err)
	}
	return sub, plan, nil
}

// GetUsage 获取租户当月使用量
func (s *BillingService) GetUsage(ctx context.Context, tenantID uint) (*APIUsage, error) {
	now := time.Now()
	usage, err := s.repo.GetOrCreateUsage(tenantID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, fmt.Errorf("billing: get usage: %w", err)
	}
	return usage, nil
}

// ListPlans 获取所有有效套餐
func (s *BillingService) ListPlans(ctx context.Context) ([]SubscriptionPlan, error) {
	plans, err := s.repo.ListPlans()
	if err != nil {
		return nil, fmt.Errorf("billing: list plans: %w", err)
	}
	return plans, nil
}

// invalidateQuotaCache 删除Redis配额缓存
func (s *BillingService) invalidateQuotaCache(ctx context.Context, tenantID uint) {
	if s.redisClient == nil {
		return
	}
	cacheKey := fmt.Sprintf("%s:tenant:%d:quota", s.prefix, tenantID)
	_ = s.redisClient.Del(ctx, cacheKey).Err()
}
