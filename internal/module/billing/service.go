package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/70548887/sup-platform/internal/pkg/cache"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// overageRate 超额费率：每次超额API调用的单价
var overageRate = decimal.RequireFromString("0.01")

// BillingService 计费引擎服务
type BillingService struct {
	db          *gorm.DB
	repo        *BillingRepository
	redisClient *redis.Client
	prefix      string
	cache       cache.CacheProvider
	quota       *QuotaService
}

// NewBillingService 创建计费服务实例
func NewBillingService(db *gorm.DB, redisClient *redis.Client, prefix string) *BillingService {
	repo := NewBillingRepository(db)
	c := cache.NewRedisCache(redisClient, prefix)
	return &BillingService{
		db:          db,
		repo:        repo,
		redisClient: redisClient,
		prefix:      prefix,
		cache:       c,
		quota:       NewQuotaService(repo, c, db),
	}
}

// CheckQuota 检查租户API配额（委托给配额子服务）
func (s *BillingService) CheckQuota(ctx context.Context, tenantID uint) (allowed bool, remaining int, err error) {
	return s.quota.CheckQuota(ctx, tenantID)
}

// RecordAPICall 记录一次API调用（委托给配额子服务）
func (s *BillingService) RecordAPICall(ctx context.Context, tenantID uint) error {
	return s.quota.RecordAPICall(ctx, tenantID)
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
		overageCharge = decimal.NewFromInt(int64(overageCount)).Mul(overageRate)
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

// GetUsage 获取租户当月使用量（委托给配额子服务）
func (s *BillingService) GetUsage(ctx context.Context, tenantID uint) (*APIUsage, error) {
	return s.quota.GetUsage(ctx, tenantID)
}

// ListPlans 获取所有有效套餐
func (s *BillingService) ListPlans(ctx context.Context) ([]SubscriptionPlan, error) {
	plans, err := s.repo.ListPlans()
	if err != nil {
		return nil, fmt.Errorf("billing: list plans: %w", err)
	}
	return plans, nil
}

// invalidateQuotaCache 删除配额缓存（委托给配额子服务）
func (s *BillingService) invalidateQuotaCache(ctx context.Context, tenantID uint) {
	s.quota.invalidateQuotaCache(ctx, tenantID)
}
