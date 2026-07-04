package reconciliation

import "gorm.io/gorm"

// Migrate 自动迁移对账相关表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ReconciliationTask{}, &ReconciliationError{})
}
