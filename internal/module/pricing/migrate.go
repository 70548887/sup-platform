package pricing

import "gorm.io/gorm"

// Migrate 执行定价模块数据库迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&PricingRule{}, &CustomerGroup{}, &CustomerGroupMember{})
}
