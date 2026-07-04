package pricing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PricingRepository 定价规则数据访问层
type PricingRepository struct {
	db *gorm.DB
}

// NewPricingRepository 创建定价规则仓储实例
func NewPricingRepository(db *gorm.DB) *PricingRepository {
	return &PricingRepository{db: db}
}

// GetApplicableRules 获取适用于指定商品、客户分组和数量的定价规则
func (r *PricingRepository) GetApplicableRules(ctx context.Context, goodsID uint, customerGroupIDs []uint, quantity int) ([]PricingRule, error) {
	now := time.Now().Unix()

	// 查询该商品所有启用的、数量范围匹配的规则
	query := r.db.WithContext(ctx).
		Where("goods_id = ? AND status = 1 AND min_quantity <= ? AND max_quantity >= ?", goodsID, quantity, quantity)

	var allRules []PricingRule
	err := query.Order("priority DESC").Find(&allRules).Error
	if err != nil {
		return nil, fmt.Errorf("pricing: 查询适用规则失败: %w", err)
	}

	// 在应用层过滤：按规则类型验证有效性
	var result []PricingRule
	for _, rule := range allRules {
		switch rule.RuleType {
		case "promotion":
			// 检查促销时间有效性
			if rule.StartAt != nil && *rule.StartAt > now {
				continue
			}
			if rule.EndAt != nil && *rule.EndAt < now {
				continue
			}
			result = append(result, rule)
		case "customer_group":
			// 检查客户分组匹配
			if rule.CustomerGroupID == nil {
				continue
			}
			matched := false
			for _, gid := range customerGroupIDs {
				if gid == *rule.CustomerGroupID {
					matched = true
					break
				}
			}
			if matched {
				result = append(result, rule)
			}
		case "tiered":
			result = append(result, rule)
		}
	}

	return result, nil
}

// CreateRule 创建定价规则
func (r *PricingRepository) CreateRule(rule *PricingRule) error {
	if err := r.db.Create(rule).Error; err != nil {
		return fmt.Errorf("pricing: 创建规则失败: %w", err)
	}
	return nil
}

// UpdateRuleCAS CAS乐观锁更新定价规则
func (r *PricingRepository) UpdateRuleCAS(rule *PricingRule, oldVersion int64) error {
	result := r.db.Model(&PricingRule{}).
		Where("id = ? AND version = ?", rule.ID, oldVersion).
		Updates(map[string]interface{}{
			"goods_id":          rule.GoodsID,
			"rule_type":         rule.RuleType,
			"customer_group_id": rule.CustomerGroupID,
			"min_quantity":      rule.MinQuantity,
			"max_quantity":      rule.MaxQuantity,
			"price":             rule.Price,
			"discount_percent":  rule.DiscountPercent,
			"promotion_name":    rule.PromotionName,
			"start_at":          rule.StartAt,
			"end_at":            rule.EndAt,
			"priority":          rule.Priority,
			"status":            rule.Status,
			"version":           oldVersion + 1,
		})
	if result.Error != nil {
		return fmt.Errorf("pricing: 更新规则失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("pricing: 规则版本冲突，请刷新后重试")
	}
	return nil
}

// DeleteRule 删除定价规则
func (r *PricingRepository) DeleteRule(id uint) error {
	if err := r.db.Delete(&PricingRule{}, id).Error; err != nil {
		return fmt.Errorf("pricing: 删除规则失败: %w", err)
	}
	return nil
}

// GetRule 根据ID获取定价规则
func (r *PricingRepository) GetRule(id uint) (*PricingRule, error) {
	var rule PricingRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pricing: 规则不存在")
		}
		return nil, fmt.Errorf("pricing: 查询规则失败: %w", err)
	}
	return &rule, nil
}

// ListRules 分页查询定价规则
func (r *PricingRepository) ListRules(goodsID *uint, ruleType string, page, size int) ([]PricingRule, int64, error) {
	query := r.db.Model(&PricingRule{})
	if goodsID != nil {
		query = query.Where("goods_id = ?", *goodsID)
	}
	if ruleType != "" {
		query = query.Where("rule_type = ?", ruleType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("pricing: 查询规则总数失败: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	var rules []PricingRule
	err := query.Order("priority DESC, id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&rules).Error
	if err != nil {
		return nil, 0, fmt.Errorf("pricing: 查询规则列表失败: %w", err)
	}
	return rules, total, nil
}

// CheckConflict 检查规则冲突（同商品+同类型+数量范围重叠，排除自身）
func (r *PricingRepository) CheckConflict(rule *PricingRule) (bool, error) {
	query := r.db.Model(&PricingRule{}).
		Where("goods_id = ? AND rule_type = ? AND status = 1", rule.GoodsID, rule.RuleType).
		Where("min_quantity <= ? AND max_quantity >= ?", rule.MaxQuantity, rule.MinQuantity)

	// 排除自身ID
	if rule.ID > 0 {
		query = query.Where("id != ?", rule.ID)
	}

	// customer_group类型需要额外匹配分组ID
	if rule.RuleType == "customer_group" && rule.CustomerGroupID != nil {
		query = query.Where("customer_group_id = ?", *rule.CustomerGroupID)
	}

	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("pricing: 检查规则冲突失败: %w", err)
	}
	return count > 0, nil
}

// CreateGroup 创建客户分组
func (r *PricingRepository) CreateGroup(g *CustomerGroup) error {
	if err := r.db.Create(g).Error; err != nil {
		return fmt.Errorf("pricing: 创建客户分组失败: %w", err)
	}
	return nil
}

// ListGroups 分页查询客户分组
func (r *PricingRepository) ListGroups(page, size int) ([]CustomerGroup, int64, error) {
	query := r.db.Model(&CustomerGroup{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("pricing: 查询分组总数失败: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	var groups []CustomerGroup
	err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&groups).Error
	if err != nil {
		return nil, 0, fmt.Errorf("pricing: 查询分组列表失败: %w", err)
	}
	return groups, total, nil
}

// GetGroup 根据ID获取客户分组
func (r *PricingRepository) GetGroup(id uint) (*CustomerGroup, error) {
	var group CustomerGroup
	err := r.db.Where("id = ?", id).First(&group).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pricing: 客户分组不存在")
		}
		return nil, fmt.Errorf("pricing: 查询客户分组失败: %w", err)
	}
	return &group, nil
}

// AddMember 添加分组成员
func (r *PricingRepository) AddMember(m *CustomerGroupMember) error {
	if err := r.db.Create(m).Error; err != nil {
		return fmt.Errorf("pricing: 添加分组成员失败: %w", err)
	}
	return nil
}

// RemoveMember 移除分组成员
func (r *PricingRepository) RemoveMember(groupID, customerID uint) error {
	result := r.db.Where("group_id = ? AND customer_id = ?", groupID, customerID).
		Delete(&CustomerGroupMember{})
	if result.Error != nil {
		return fmt.Errorf("pricing: 移除分组成员失败: %w", result.Error)
	}
	return nil
}

// GetGroupIDsByCustomer 获取客户所属的所有分组ID
func (r *PricingRepository) GetGroupIDsByCustomer(customerID uint) ([]uint, error) {
	var groupIDs []uint
	err := r.db.Model(&CustomerGroupMember{}).
		Select("group_id").
		Where("customer_id = ?", customerID).
		Find(&groupIDs).Error
	if err != nil {
		return nil, fmt.Errorf("pricing: 查询客户分组失败: %w", err)
	}
	return groupIDs, nil
}
