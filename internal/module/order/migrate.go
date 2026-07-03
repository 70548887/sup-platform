package order

import "gorm.io/gorm"

// AutoMigrate 执行order模块的数据库自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Order{},
		&OrderBuyParam{},
		&OrderStatusChange{},
		&OrderCard{},
		&OrderCallback{},
		&OrderProgress{},
	)
}
