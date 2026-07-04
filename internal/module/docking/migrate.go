package docking

import "gorm.io/gorm"

// Migrate 自动迁移订单对接任务表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&OrderDockingTask{})
}
