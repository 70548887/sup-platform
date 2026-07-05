package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// validRoles 合法的管理员角色
var validRoles = map[string]bool{
	AdminRoleBoss:    true,
	AdminRoleFinance: true,
	AdminRoleOps:     true,
	AdminRoleSupport: true,
}

// TenantService 租户核心服务
type TenantService struct {
	db   *gorm.DB
	repo *TenantRepository
}

// NewTenantService 创建TenantService
func NewTenantService(db *gorm.DB) *TenantService {
	return &TenantService{
		db:   db,
		repo: NewTenantRepository(db),
	}
}

// CreateTenant 创建租户
// 1. 创建Tenant记录
// 2. 自动将owner添加为boss角色的TenantAdmin
func (s *TenantService) CreateTenant(ctx context.Context, name, domain string, ownerUserID uint, tenantType string) (*Tenant, error) {
	tenant := &Tenant{
		Name:        name,
		Domain:      domain,
		Type:        tenantType,
		OwnerUserID: ownerUserID,
		Status:      1,
		MaxAdmins:   5,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(tenant).Error; err != nil {
			return fmt.Errorf("tenant: create tenant failed: %w", err)
		}

		admin := &TenantAdmin{
			TenantID:  tenant.ID,
			UserID:    ownerUserID,
			AdminRole: AdminRoleBoss,
			Status:    1,
		}
		if err := tx.Create(admin).Error; err != nil {
			return fmt.Errorf("tenant: create owner admin failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

// GetTenant 通过ID查询租户
func (s *TenantService) GetTenant(ctx context.Context, id uint) (*Tenant, error) {
	return s.repo.GetByID(id)
}

// ListTenants 分页查询租户列表
func (s *TenantService) ListTenants(ctx context.Context, page, size int) ([]Tenant, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.repo.ListTenants(page, size)
}

// AddAdmin 添加租户管理员
// 检查MaxAdmins限制和role合法性
func (s *TenantService) AddAdmin(ctx context.Context, tenantID, userID uint, role, permissions string) error {
	// 验证角色合法性
	if !validRoles[role] {
		return fmt.Errorf("tenant: invalid admin role: %s", role)
	}

	// 获取租户信息
	t, err := s.repo.GetByID(tenantID)
	if err != nil {
		return fmt.Errorf("tenant: get tenant failed: %w", err)
	}

	// 检查管理员数量限制
	count, err := s.repo.CountAdmins(tenantID)
	if err != nil {
		return fmt.Errorf("tenant: count admins failed: %w", err)
	}
	if int(count) >= t.MaxAdmins {
		return fmt.Errorf("tenant: max admins limit reached (%d)", t.MaxAdmins)
	}

	admin := &TenantAdmin{
		TenantID:    tenantID,
		UserID:      userID,
		AdminRole:   role,
		Permissions: permissions,
		Status:      1,
	}
	if err := s.repo.CreateAdmin(admin); err != nil {
		return fmt.Errorf("tenant: add admin failed: %w", err)
	}
	return nil
}

// RemoveAdmin 移除租户管理员
func (s *TenantService) RemoveAdmin(ctx context.Context, tenantID, userID uint) error {
	if err := s.repo.RemoveAdmin(tenantID, userID); err != nil {
		return fmt.Errorf("tenant: remove admin failed: %w", err)
	}
	return nil
}

// ListAdmins 列出租户的所有管理员
func (s *TenantService) ListAdmins(ctx context.Context, tenantID uint) ([]TenantAdmin, error) {
	return s.repo.ListAdmins(tenantID)
}

// CheckAdminPermission 检查管理员权限
// boss: 全部权限
// finance: resource包含"billing"/"analytics"/"ledger"/"recharge" → true
// ops: resource包含"order"/"goods"/"docking"/"pricing" → true
// support: action=="GET" → true（只读）
func (s *TenantService) CheckAdminPermission(ctx context.Context, tenantID, userID uint, resource, action string) bool {
	admin, err := s.repo.GetAdmin(tenantID, userID)
	if err != nil {
		return false
	}

	switch admin.AdminRole {
	case AdminRoleBoss:
		return true
	case AdminRoleFinance:
		res := strings.ToLower(resource)
		return strings.Contains(res, "billing") ||
			strings.Contains(res, "analytics") ||
			strings.Contains(res, "ledger") ||
			strings.Contains(res, "recharge")
	case AdminRoleOps:
		res := strings.ToLower(resource)
		return strings.Contains(res, "order") ||
			strings.Contains(res, "goods") ||
			strings.Contains(res, "docking") ||
			strings.Contains(res, "pricing")
	case AdminRoleSupport:
		return strings.ToUpper(action) == "GET"
	default:
		return false
	}
}

// ToggleFeature 切换租户功能开关
// 根据 feature 名称在 Tenant.Features JSON 中切换布尔值
// 首次切换时默认从 false 开启为 true
func (s *TenantService) ToggleFeature(ctx context.Context, tenantID uint, feature string) (*Tenant, error) {
	tenant, err := s.repo.GetByID(tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant: get tenant failed: %w", err)
	}

	features := make(map[string]interface{})
	if tenant.Features != "" {
		if err := json.Unmarshal([]byte(tenant.Features), &features); err != nil {
			return nil, fmt.Errorf("tenant: invalid features json: %w", err)
		}
	}

	current := false
	if v, ok := features[feature]; ok {
		if b, ok := v.(bool); ok {
			current = b
		}
	}
	features[feature] = !current

	b, err := json.Marshal(features)
	if err != nil {
		return nil, fmt.Errorf("tenant: marshal features failed: %w", err)
	}
	tenant.Features = string(b)

	if err := s.repo.UpdateTenant(tenant); err != nil {
		return nil, fmt.Errorf("tenant: update tenant failed: %w", err)
	}

	return tenant, nil
}

// InitDefaultTenant 初始化默认租户
// 检查ID=1的Tenant是否存在，不存在则创建
func (s *TenantService) InitDefaultTenant(ctx context.Context) error {
	_, err := s.repo.GetByID(1)
	if err == nil {
		return nil // 已存在
	}
	if err != ErrTenantNotFound {
		return fmt.Errorf("tenant: check default tenant failed: %w", err)
	}

	tenant := &Tenant{
		Name:      "平台默认租户",
		Domain:    "default",
		Type:      "private",
		Status:    1,
		MaxAdmins: 5,
	}
	if err := s.repo.CreateTenant(tenant); err != nil {
		return fmt.Errorf("tenant: create default tenant failed: %w", err)
	}
	return nil
}
