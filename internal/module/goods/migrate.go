package goods

import "gorm.io/gorm"

// AutoMigrate 执行goods模块的数据库自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&GoodsCategory{},
		&Goods{},
		&GoodsBuyParam{},
		&GoodsSupplierBinding{},
	)
}
