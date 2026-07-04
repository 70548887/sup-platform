package recharge

import "gorm.io/gorm"

// Migrate auto-migrate recharge_orders table
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&RechargeOrder{})
}
