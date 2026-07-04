package tenant

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateExistingData 将现有数据的tenant_id设置为默认值1
// 用于从单租户升级到多租户时的数据迁移
func MigrateExistingData(db *gorm.DB) error {
	tables := []string{
		"users", "api_apps", "orders", "goods", "wallets",
		"ledger_entries", "cards", "audit_logs",
		"pricing_rules", "customer_groups",
		"daily_stats", "hot_goods", "reconciliation_tasks",
	}

	for _, table := range tables {
		// 检查表是否存在
		if !db.Migrator().HasTable(table) {
			continue
		}
		// 检查是否有tenant_id列
		if !db.Migrator().HasColumn(table, "tenant_id") {
			continue
		}
		// 分批更新（1000条/批）
		result := db.Exec(fmt.Sprintf(
			"UPDATE `%s` SET tenant_id = 1 WHERE tenant_id = 0 OR tenant_id IS NULL",
			table,
		))
		if result.Error != nil {
			return fmt.Errorf("tenant: migrate data for table %s: %w", table, result.Error)
		}
	}
	return nil
}
