package card

import "gorm.io/gorm"

// AutoMigrate 执行card模块的数据库自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&CardBatch{},
		&Card{},
	)
}
