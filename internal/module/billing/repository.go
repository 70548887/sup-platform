package billing

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrPlanNotFound         = errors.New("billing: plan not found")
	ErrSubscriptionNotFound = errors.New("billing: subscription not found")
	ErrInvoiceNotFound      = errors.New("billing: invoice not found")
	ErrCASConflict          = errors.New("billing: CAS conflict, concurrent modification detected")
)

// BillingRepository 计费数据访问层
type BillingRepository struct {
	db *gorm.DB
}

// NewBillingRepository 创建BillingRepository
func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

// --- SubscriptionPlan ---

// CreatePlan 创建套餐
func (r *BillingRepository) CreatePlan(plan *SubscriptionPlan) error {
	return r.db.Create(plan).Error
}

// ListPlans 获取所有有效套餐
func (r *BillingRepository) ListPlans() ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	err := r.db.Where("status = 1").Order("monthly_price ASC").Find(&plans).Error
	return plans, err
}

// GetPlan 通过ID获取套餐
func (r *BillingRepository) GetPlan(id uint) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	err := r.db.First(&plan, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// GetPlanByName 通过名称获取套餐
func (r *BillingRepository) GetPlanByName(name string) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	err := r.db.Where("name = ?", name).First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// --- TenantSubscription ---

// GetCurrentSubscription 获取租户当前订阅
func (r *BillingRepository) GetCurrentSubscription(tenantID uint) (*TenantSubscription, error) {
	var sub TenantSubscription
	err := r.db.Where("tenant_id = ? AND status = ?", tenantID, "active").First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return &sub, nil
}

// GetSubscriptionWithPlan 获取租户订阅及对应套餐（JOIN查询）
func (r *BillingRepository) GetSubscriptionWithPlan(tenantID uint) (*TenantSubscription, *SubscriptionPlan, error) {
	var sub TenantSubscription
	err := r.db.Where("tenant_id = ? AND status = ?", tenantID, "active").First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrSubscriptionNotFound
		}
		return nil, nil, err
	}

	var plan SubscriptionPlan
	err = r.db.First(&plan, sub.PlanID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &sub, nil, ErrPlanNotFound
		}
		return nil, nil, err
	}

	return &sub, &plan, nil
}

// CreateSubscription 创建订阅
func (r *BillingRepository) CreateSubscription(sub *TenantSubscription) error {
	return r.db.Create(sub).Error
}

// UpdateSubscription 更新订阅
func (r *BillingRepository) UpdateSubscription(sub *TenantSubscription) error {
	return r.db.Save(sub).Error
}

// ListAllSubscriptions 分页查询所有订阅
func (r *BillingRepository) ListAllSubscriptions(page, size int) ([]TenantSubscription, int64, error) {
	var total int64
	query := r.db.Model(&TenantSubscription{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var subs []TenantSubscription
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&subs).Error; err != nil {
		return nil, 0, err
	}
	return subs, total, nil
}

// --- APIUsage ---

// GetOrCreateUsage 获取或创建月度使用量记录
func (r *BillingRepository) GetOrCreateUsage(tenantID uint, year, month int) (*APIUsage, error) {
	var usage APIUsage
	err := r.db.Where("tenant_id = ? AND year = ? AND month = ?", tenantID, year, month).First(&usage).Error
	if err == nil {
		return &usage, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 不存在则创建
	usage = APIUsage{
		TenantID: tenantID,
		Year:     year,
		Month:    month,
	}
	if err := r.db.Create(&usage).Error; err != nil {
		// 并发创建冲突时再查一次
		var existing APIUsage
		if err2 := r.db.Where("tenant_id = ? AND year = ? AND month = ?", tenantID, year, month).First(&existing).Error; err2 == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &usage, nil
}

// IncrementAPIUsageCAS CAS乐观锁递增API调用次数
// UPDATE api_usages SET api_call_count=api_call_count+1, version=version+1 WHERE id=? AND version=?
func (r *BillingRepository) IncrementAPIUsageCAS(id uint, currentVersion int64) (bool, error) {
	result := r.db.Model(&APIUsage{}).
		Where("id = ? AND version = ?", id, currentVersion).
		Updates(map[string]interface{}{
			"api_call_count": gorm.Expr("api_call_count + 1"),
			"version":        gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// --- Invoice ---

// CreateInvoice 创建账单
func (r *BillingRepository) CreateInvoice(invoice *Invoice) error {
	return r.db.Create(invoice).Error
}

// ListInvoices 分页查询租户账单
func (r *BillingRepository) ListInvoices(tenantID uint, page, size int) ([]Invoice, int64, error) {
	var total int64
	query := r.db.Model(&Invoice{}).Where("tenant_id = ?", tenantID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var invoices []Invoice
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&invoices).Error; err != nil {
		return nil, 0, err
	}
	return invoices, total, nil
}

// ListAllInvoices 分页查询所有账单（平台管理）
func (r *BillingRepository) ListAllInvoices(page, size int) ([]Invoice, int64, error) {
	var total int64
	query := r.db.Model(&Invoice{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var invoices []Invoice
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&invoices).Error; err != nil {
		return nil, 0, err
	}
	return invoices, total, nil
}

// UpdateInvoiceStatus 更新账单状态
func (r *BillingRepository) UpdateInvoiceStatus(id uint, status string, paidAt *int64) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if paidAt != nil {
		updates["paid_at"] = *paidAt
	}
	result := r.db.Model(&Invoice{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// GetInvoice 通过ID获取账单
func (r *BillingRepository) GetInvoice(id uint) (*Invoice, error) {
	var invoice Invoice
	err := r.db.First(&invoice, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	return &invoice, nil
}
