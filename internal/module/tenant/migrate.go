package tenant

import "gorm.io/gorm"

// Migrate 自动迁移租户相关表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Tenant{}, &TenantAdmin{})
}
