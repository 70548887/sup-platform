package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// createIndexes 创建复合索引（幂等，重复执行不报错）
// 兼容 MySQL 8.0.12（不支持 CREATE INDEX IF NOT EXISTS）
func createIndexes(db *gorm.DB) error {
	type indexDef struct {
		table string
		name  string
		cols  string
	}

	indexes := []indexDef{
		// 商品表
		{"goods", "idx_goods_tenant_category_status", "tenant_id, category_id, status"},
		{"goods", "idx_goods_supplier_status_updated", "supplier_id, status, updated_at"},

		// 商品购买参数表
		{"goods_buy_params", "idx_goods_buy_params_goods_sort", "goods_id, sort_order"},

		// 订单表
		{"orders", "idx_orders_tenant_status", "tenant_id, status"},
		{"orders", "idx_orders_app_custorder", "app_id, customer_order_id"},
		{"orders", "idx_orders_created_desc", "created_at"},

		// 卡密表
		{"cards", "idx_cards_batch_status", "batch_id, status"},

		// 定价规则表
		{"pricing_rules", "idx_pricing_rules_goods_quantity", "goods_id, min_quantity, max_quantity"},

		// 用户表
		{"users", "idx_users_login_attempts", "username, login_attempts"},

		// 登录日志表
		{"login_logs", "idx_login_logs_user_time", "user_id, created_at DESC"},
		{"login_logs", "idx_login_logs_ip_time", "client_ip, created_at DESC"},
	}

	for _, idx := range indexes {
		if indexExists(db, idx.table, idx.name) {
			continue
		}
		ddl := fmt.Sprintf("CREATE INDEX %s ON %s(%s)", idx.name, idx.table, idx.cols)
		if err := db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("create indexes: %w", err)
		}
	}
	return nil
}

// indexExists 检查索引是否存在
func indexExists(db *gorm.DB, table, indexName string) bool {
	var count int64
	db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?", table, indexName).Scan(&count)
	return count > 0
}
