package tenant

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrTenantNotFound = errors.New("tenant: not found")
	ErrAdminNotFound  = errors.New("tenant: admin not found")
	ErrAdminExists    = errors.New("tenant: admin already exists")
)

// TenantRepository 租户数据访问层
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository 创建TenantRepository
func NewTenantRepository(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// CreateTenant 创建租户
func (r *TenantRepository) CreateTenant(tenant *Tenant) error {
	return r.db.Create(tenant).Error
}

// GetByID 通过ID查找租户
func (r *TenantRepository) GetByID(id uint) (*Tenant, error) {
	var t Tenant
	err := r.db.First(&t, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &t, nil
}

// GetByDomain 通过域名查找租户
func (r *TenantRepository) GetByDomain(domain string) (*Tenant, error) {
	var t Tenant
	err := r.db.Where("domain = ?", domain).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &t, nil
}

// ListTenants 分页查询租户列表
func (r *TenantRepository) ListTenants(page, size int) ([]Tenant, int64, error) {
	var total int64
	if err := r.db.Model(&Tenant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tenants []Tenant
	offset := (page - 1) * size
	if err := r.db.Order("id DESC").Offset(offset).Limit(size).Find(&tenants).Error; err != nil {
		return nil, 0, err
	}
	return tenants, total, nil
}

// UpdateTenant 更新租户
func (r *TenantRepository) UpdateTenant(tenant *Tenant) error {
	return r.db.Save(tenant).Error
}

// CreateAdmin 创建租户管理员
func (r *TenantRepository) CreateAdmin(admin *TenantAdmin) error {
	return r.db.Create(admin).Error
}

// ListAdmins 查询租户的所有管理员
func (r *TenantRepository) ListAdmins(tenantID uint) ([]TenantAdmin, error) {
	var admins []TenantAdmin
	err := r.db.Where("tenant_id = ?", tenantID).Find(&admins).Error
	if err != nil {
		return nil, err
	}
	return admins, nil
}

// GetAdmin 获取指定租户的指定用户管理员记录
func (r *TenantRepository) GetAdmin(tenantID, userID uint) (*TenantAdmin, error) {
	var admin TenantAdmin
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}
	return &admin, nil
}

// RemoveAdmin 物理删除租户管理员
func (r *TenantRepository) RemoveAdmin(tenantID, userID uint) error {
	result := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&TenantAdmin{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAdminNotFound
	}
	return nil
}

// GetTenantByUserID 通过TenantAdmin表JOIN查询用户所属租户
func (r *TenantRepository) GetTenantByUserID(userID uint) (*Tenant, error) {
	var t Tenant
	err := r.db.Joins("JOIN tenant_admins ON tenant_admins.tenant_id = tenants.id").
		Where("tenant_admins.user_id = ?", userID).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &t, nil
}

// CountAdmins 统计租户管理员数量
func (r *TenantRepository) CountAdmins(tenantID uint) (int64, error) {
	var count int64
	err := r.db.Model(&TenantAdmin{}).Where("tenant_id = ?", tenantID).Count(&count).Error
	return count, err
}
