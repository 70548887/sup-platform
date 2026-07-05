package migrations

import "gorm.io/gorm"

// createIndexes 创建复合索引（幂等，重复执行不报错）
func createIndexes(db *gorm.DB) error {
	indexes := []string{
		// 商品表
		"CREATE INDEX IF NOT EXISTS idx_goods_tenant_category_status ON goods(tenant_id, category_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_goods_supplier_status_updated ON goods(supplier_id, status, updated_at DESC)",

		// 商品购买参数表
		"CREATE INDEX IF NOT EXISTS idx_goods_buy_params_goods_sort ON goods_buy_params(goods_id, sort_order)",

		// 订单表
		"CREATE INDEX IF NOT EXISTS idx_orders_tenant_status ON orders(tenant_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_orders_app_custorder ON orders(app_id, customer_order_id)",
		"CREATE INDEX IF NOT EXISTS idx_orders_created_desc ON orders(created_at DESC)",

		// 卡密表
		"CREATE INDEX IF NOT EXISTS idx_cards_batch_status ON cards(batch_id, status)",

		// 定价规则表
		"CREATE INDEX IF NOT EXISTS idx_pricing_rules_goods_quantity ON pricing_rules(goods_id, min_quantity, max_quantity)",
	}

	for _, ddl := range indexes {
		if err := db.Exec(ddl).Error; err != nil {
			return err
		}
	}
	return nil
}
