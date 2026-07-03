package migrations

import (
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
)

// RunAll 按依赖顺序执行所有模块的数据库迁移
// 顺序：account → goods → card → order → ledger
func RunAll(db *gorm.DB) error {
	if err := account.AutoMigrate(db); err != nil {
		return err
	}
	if err := goods.AutoMigrate(db); err != nil {
		return err
	}
	if err := card.AutoMigrate(db); err != nil {
		return err
	}
	if err := order.AutoMigrate(db); err != nil {
		return err
	}
	if err := ledger.AutoMigrate(db); err != nil {
		return err
	}
	return nil
}
