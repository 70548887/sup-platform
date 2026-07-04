package refund

import "gorm.io/gorm"

// Migrate 执行refund模块的数据库自动迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&RefundOrder{})
}
