package settlement

import "gorm.io/gorm"

// Migrate 结算模块数据库迁移
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Settlement{}, &ProfitShare{})
}
