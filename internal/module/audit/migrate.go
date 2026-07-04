package audit

import "gorm.io/gorm"

// Migrate 执行audit模块的数据库自动迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&AuditLog{})
}
