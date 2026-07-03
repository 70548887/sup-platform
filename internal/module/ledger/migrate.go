package ledger

import "gorm.io/gorm"

// AutoMigrate 执行ledger模块的数据库自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Wallet{},
		&LedgerEntry{},
		&Recharge{},
	)
}
