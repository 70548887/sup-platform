package migrations

import (
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/docking"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/module/pricing"
	"github.com/70548887/sup-platform/internal/module/recharge"
	"github.com/70548887/sup-platform/internal/module/reconciliation"
	"github.com/70548887/sup-platform/internal/module/refund"
)

// RunAll 按依赖顺序执行所有模块的数据库迁移
// 顺序：account → goods → card → order → docking → ledger → recharge → audit → refund → pricing → analytics → reconciliation
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
	// docking 依赖 order
	if err := docking.Migrate(db); err != nil {
		return err
	}
	if err := ledger.AutoMigrate(db); err != nil {
		return err
	}
	// recharge 依赖 ledger
	if err := recharge.Migrate(db); err != nil {
		return err
	}
	if err := audit.Migrate(db); err != nil {
		return err
	}
	if err := refund.Migrate(db); err != nil {
		return err
	}
	// Phase 4A 模块迁移
	if err := pricing.Migrate(db); err != nil {
		return err
	}
	if err := analytics.Migrate(db); err != nil {
		return err
	}
	if err := reconciliation.Migrate(db); err != nil {
		return err
	}
	return nil
}
