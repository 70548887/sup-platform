package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PricingService 定价规则服务
type PricingService struct {
	db          *gorm.DB
	repo        *PricingRepository
	redisClient *redis.Client
	prefix      string
}

// NewPricingService 创建定价规则服务实例
func NewPricingService(db *gorm.DB, redisClient *redis.Client, prefix string) *PricingService {
	return &PricingService{
		db:          db,
		repo:        NewPricingRepository(db),
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// CalculatePrice 计算最终价格
// 优先级：promotion > customer_group > tiered > basePrice
// CostPrice保护：最终价格不低于商品成本价
func (s *PricingService) CalculatePrice(ctx context.Context, goodsID uint, customerID uint, quantity int, basePrice decimal.Decimal) (decimal.Decimal, error) {
	// 1. 获取客户所属分组IDs
	var groupIDs []uint
	if customerID > 0 {
		ids, err := s.repo.GetGroupIDsByCustomer(customerID)
		if err != nil {
			// 降级：查询分组失败不阻塞，继续计算
			groupIDs = nil
		} else {
			groupIDs = ids
		}
	}

	// 2. 尝试从Redis缓存读取规则（key含quantity维度）
	cacheKey := fmt.Sprintf("%s:pricing:%d:%d", s.prefix, goodsID, quantity)
	var rules []PricingRule
	cacheHit := false

	if s.redisClient != nil {
		cached, err := s.redisClient.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			if jsonErr := json.Unmarshal([]byte(cached), &rules); jsonErr == nil {
				cacheHit = true
			}
		}
		// Redis错误时静默降级，继续查DB
	}

	// 3. 缓存未命中，查DB获取适用规则
	if !cacheHit {
		var err error
		rules, err = s.repo.GetApplicableRules(ctx, goodsID, groupIDs, quantity)
		if err != nil {
			// 查询失败返回basePrice，不阻塞调用方
			return basePrice, nil
		}

		// 4. 写入Redis缓存（基础TTL 1小时 + 随机偏移±5分钟防雪崩）
		if s.redisClient != nil {
			baseTTL := time.Hour
			randomOffset := time.Duration(rand.Intn(600)-300) * time.Second
			ttl := baseTTL + randomOffset
			if data, jsonErr := json.Marshal(rules); jsonErr == nil {
				_ = s.redisClient.Set(ctx, cacheKey, data, ttl).Err()
			}
		}
	}

	// 5. 按优先级选择最佳规则
	finalPrice := s.selectBestPrice(rules, groupIDs, basePrice)

	// 6. CostPrice保护：从goods表读取成本价
	var goods struct {
		CostPrice decimal.Decimal
	}
	if err := s.db.Table("goods").Select("cost_price").Where("id = ?", goodsID).Scan(&goods).Error; err == nil {
		if goods.CostPrice.GreaterThan(decimal.Zero) && finalPrice.LessThan(goods.CostPrice) {
			finalPrice = goods.CostPrice
		}
	}

	return finalPrice, nil
}

// selectBestPrice 按优先级选择最佳规则并计算价格
func (s *PricingService) selectBestPrice(rules []PricingRule, groupIDs []uint, basePrice decimal.Decimal) decimal.Decimal {
	// 按优先级排序已在repo层完成（priority DESC）
	// 优先级选择：promotion > customer_group > tiered

	// 先筛选promotion类型（Priority最高的）
	var bestPromo *PricingRule
	for i := range rules {
		if rules[i].RuleType == "promotion" {
			if bestPromo == nil || rules[i].Priority > bestPromo.Priority {
				bestPromo = &rules[i]
			}
		}
	}
	if bestPromo != nil {
		return s.applyRule(bestPromo, basePrice)
	}

	// 再筛选customer_group类型
	var bestGroup *PricingRule
	for i := range rules {
		if rules[i].RuleType == "customer_group" {
			if bestGroup == nil || rules[i].Priority > bestGroup.Priority {
				bestGroup = &rules[i]
			}
		}
	}
	if bestGroup != nil {
		return s.applyRule(bestGroup, basePrice)
	}

	// 再筛选tiered类型
	var bestTiered *PricingRule
	for i := range rules {
		if rules[i].RuleType == "tiered" {
			if bestTiered == nil || rules[i].Priority > bestTiered.Priority {
				bestTiered = &rules[i]
			}
		}
	}
	if bestTiered != nil {
		return s.applyRule(bestTiered, basePrice)
	}

	// 没有匹配规则，返回basePrice
	return basePrice
}

// applyRule 应用单条规则计算价格
func (s *PricingService) applyRule(rule *PricingRule, basePrice decimal.Decimal) decimal.Decimal {
	if rule.Price.GreaterThan(decimal.Zero) {
		// 固定价
		return rule.Price
	}
	if rule.DiscountPercent.GreaterThan(decimal.Zero) {
		// 折扣价 = basePrice * (1 - discount/100)
		discount := rule.DiscountPercent.Div(decimal.NewFromInt(100))
		return basePrice.Mul(decimal.NewFromInt(1).Sub(discount))
	}
	return basePrice
}

// CreateRule 创建定价规则（含冲突检测）
func (s *PricingService) CreateRule(ctx context.Context, rule *PricingRule) error {
	conflict, err := s.repo.CheckConflict(rule)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("pricing: 存在冲突的定价规则（同商品+同类型+数量范围重叠）")
	}
	if err := s.repo.CreateRule(rule); err != nil {
		return err
	}
	s.invalidateCache(ctx, rule.GoodsID)
	return nil
}

// UpdateRule CAS更新定价规则
func (s *PricingService) UpdateRule(ctx context.Context, rule *PricingRule, version int64) error {
	if err := s.repo.UpdateRuleCAS(rule, version); err != nil {
		return err
	}
	s.invalidateCache(ctx, rule.GoodsID)
	return nil
}

// DeleteRule 删除定价规则
func (s *PricingService) DeleteRule(ctx context.Context, id uint) error {
	// 先获取规则信息用于清除缓存
	rule, err := s.repo.GetRule(id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteRule(id); err != nil {
		return err
	}
	s.invalidateCache(ctx, rule.GoodsID)
	return nil
}

// ListRules 分页查询定价规则
func (s *PricingService) ListRules(ctx context.Context, goodsID *uint, ruleType string, page, size int) ([]PricingRule, int64, error) {
	return s.repo.ListRules(goodsID, ruleType, page, size)
}

// GetRule 获取单条定价规则
func (s *PricingService) GetRule(ctx context.Context, id uint) (*PricingRule, error) {
	return s.repo.GetRule(id)
}

// CreateGroup 创建客户分组
func (s *PricingService) CreateGroup(ctx context.Context, g *CustomerGroup) error {
	return s.repo.CreateGroup(g)
}

// ListGroups 分页查询客户分组
func (s *PricingService) ListGroups(ctx context.Context, page, size int) ([]CustomerGroup, int64, error) {
	return s.repo.ListGroups(page, size)
}

// GetGroup 获取单条客户分组
func (s *PricingService) GetGroup(ctx context.Context, id uint) (*CustomerGroup, error) {
	return s.repo.GetGroup(id)
}

// AddMember 添加分组成员
func (s *PricingService) AddMember(ctx context.Context, m *CustomerGroupMember) error {
	return s.repo.AddMember(m)
}

// RemoveMember 移除分组成员
func (s *PricingService) RemoveMember(ctx context.Context, groupID, customerID uint) error {
	return s.repo.RemoveMember(groupID, customerID)
}

// InvalidateGoodsPricingCache 当定价规则变更时调用，批量清除该商品所有定价缓存
func (s *PricingService) InvalidateGoodsPricingCache(ctx context.Context, goodsID uint) error {
	if s.redisClient == nil {
		return nil
	}
	// 使用通配符删除该商品所有quantity维度的缓存
	pattern := fmt.Sprintf("%s:pricing:%d:*", s.prefix, goodsID)
	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return s.redisClient.Del(ctx, keys...).Err()
	}
	return nil
}

// invalidateCache 内部调用，删除Redis定价缓存
func (s *PricingService) invalidateCache(ctx context.Context, goodsID uint) {
	_ = s.InvalidateGoodsPricingCache(ctx, goodsID)
}
