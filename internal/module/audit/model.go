package audit

// AuditLog 审计日志
type AuditLog struct {
	ID         uint   `gorm:"primarykey"`
	UserID     uint   `gorm:"index"`                    // 操作者ID (0=system)
	Username   string `gorm:"size:50;index"`            // 操作者名称
	Action     string `gorm:"size:50;not null;index"`   // 操作类型
	Resource   string `gorm:"size:50;not null;index"`   // 资源类型(order/goods/refund等)
	ResourceID uint   `gorm:"index"`                    // 资源ID
	Detail     string `gorm:"type:text"`                // 操作详情JSON
	IP         string `gorm:"size:45"`                  // 客户端IP
	UserAgent  string `gorm:"size:500"`                 // UA
	CreatedAt  int64  `gorm:"autoCreateTime;index"`     // 操作时间
}

// 常用Action常量
const (
	ActionLogin         = "login"
	ActionLogout        = "logout"
	ActionOrderCreate   = "order.create"
	ActionOrderStatus   = "order.status_change"
	ActionRefundApply   = "refund.apply"
	ActionRefundApprove = "refund.approve"
	ActionRefundReject  = "refund.reject"
	ActionGoodsCreate   = "goods.create"
	ActionGoodsUpdate   = "goods.update"
	ActionGoodsDelete   = "goods.delete"
	ActionCardImport    = "card.import"
)

// AuditFilter 审计日志查询过滤条件
type AuditFilter struct {
	UserID     uint   // 按操作者ID过滤
	Username   string // 按操作者名称过滤
	Action     string // 按操作类型过滤
	Resource   string // 按资源类型过滤
	ResourceID uint   // 按资源ID过滤
	StartTime  int64  // 起始时间(Unix时间戳)
	EndTime    int64  // 结束时间(Unix时间戳)
}
