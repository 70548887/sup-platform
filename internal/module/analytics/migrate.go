package analytics

import "gorm.io/gorm"

// Migrate 执行统计模块的数据库迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&DailyStats{}, &HotGoods{})
}
