package tenant

// Tenant 租户
type Tenant struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"size:100;not null"`
	Domain      string `gorm:"size:100;uniqueIndex"`
	Type        string `gorm:"size:20;not null;default:saas"` // saas, private
	OwnerUserID uint   `gorm:"index"`
	Status      int8   `gorm:"not null;default:1"` // 1=active, 0=disabled
	MaxAdmins   int    `gorm:"default:5"`
	CreatedAt   int64  `gorm:"autoCreateTime"`
	UpdatedAt   int64  `gorm:"autoUpdateTime"`
}

// TenantAdmin 租户管理员
type TenantAdmin struct {
	ID          uint   `gorm:"primarykey"`
	TenantID    uint   `gorm:"not null;uniqueIndex:idx_tenant_user"`
	UserID      uint   `gorm:"not null;uniqueIndex:idx_tenant_user"`
	AdminRole   string `gorm:"size:20;not null"` // boss, finance, ops, support
	Permissions string `gorm:"type:text"`        // JSON权限列表
	Status      int8   `gorm:"not null;default:1"`
	CreatedAt   int64  `gorm:"autoCreateTime"`
}

// Admin角色常量
const (
	AdminRoleBoss    = "boss"
	AdminRoleFinance = "finance"
	AdminRoleOps     = "ops"
	AdminRoleSupport = "support"
)
