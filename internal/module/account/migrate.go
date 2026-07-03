package account

import "gorm.io/gorm"

// AutoMigrate 执行account模块的数据库自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&ApiApp{},
		&Role{},
		&Permission{},
		&LoginSession{},
	)
}
