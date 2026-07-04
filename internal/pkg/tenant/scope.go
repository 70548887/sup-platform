package tenant

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RegisterTenantScope 注册GORM全局Callback实现多租户自动过滤
func RegisterTenantScope(db *gorm.DB) {
	// 注册查询前回调
	db.Callback().Query().Before("gorm:query").Register("tenant:before_query", tenantQueryCallback)

	// 注册创建前回调（自动设置tenant_id）
	db.Callback().Create().Before("gorm:create").Register("tenant:before_create", tenantCreateCallback)

	// 注册更新前回调
	db.Callback().Update().Before("gorm:update").Register("tenant:before_update", tenantQueryCallback)

	// 注册删除前回调
	db.Callback().Delete().Before("gorm:delete").Register("tenant:before_delete", tenantQueryCallback)
}

func tenantQueryCallback(db *gorm.DB) {
	if db.Statement.Context == nil {
		return
	}

	// 检查是否应跳过
	if ShouldSkipScope(db.Statement.Context) {
		return
	}

	// 从Context获取tenantID
	tenantID, ok := GetTenantID(db.Statement.Context)
	if !ok || tenantID == 0 {
		return
	}

	// 检查当前模型是否有tenant_id字段
	if db.Statement.Schema != nil {
		if _, exists := db.Statement.Schema.FieldsByDBName["tenant_id"]; exists {
			db.Statement.AddClause(clause.Where{
				Exprs: []clause.Expression{
					clause.Eq{Column: clause.Column{Table: db.Statement.Table, Name: "tenant_id"}, Value: tenantID},
				},
			})
		}
	}
}

func tenantCreateCallback(db *gorm.DB) {
	if db.Statement.Context == nil {
		return
	}

	if ShouldSkipScope(db.Statement.Context) {
		return
	}

	tenantID, ok := GetTenantID(db.Statement.Context)
	if !ok || tenantID == 0 {
		return
	}

	// 检查Schema是否有tenant_id字段，如果有则设置值
	if db.Statement.Schema != nil {
		if field, ok := db.Statement.Schema.FieldsByDBName["tenant_id"]; ok {
			if db.Statement.ReflectValue.IsValid() {
				_ = field.Set(db.Statement.Context, db.Statement.ReflectValue, tenantID)
			}
		}
	}
}
