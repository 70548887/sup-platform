package billing

import "gorm.io/gorm"

// Migrate 自动迁移计费相关表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&SubscriptionPlan{}, &TenantSubscription{}, &APIUsage{}, &Invoice{})
}
