package card

// CardBatch 卡密批次
type CardBatch struct {
	ID         uint   `gorm:"primarykey"`
	GoodsID    uint   `gorm:"not null;index"`
	Name       string `gorm:"size:100"`
	TotalCount int    `gorm:"not null;default:0"` // 批次总数
	UsedCount  int    `gorm:"not null;default:0"` // 已用数
	Status     int8   `gorm:"not null;default:1"`
	CreatedAt  int64  `gorm:"autoCreateTime"`
}

// Card 单张卡密
type Card struct {
	ID       uint   `gorm:"primarykey"`
	TenantID uint   `gorm:"not null;default:1;index"`
	BatchID   uint   `gorm:"not null;index"`
	GoodsID   uint   `gorm:"not null;index:idx_goods_status"`
	Content   string `gorm:"type:text;not null"` // 卡密内容（TODO: AES-GCM加密）
	Password  string `gorm:"size:128"`           // 可选密码
	Status    int8   `gorm:"not null;default:1;index:idx_goods_status"` // 1可用 2锁定 3已用
	OrderID   *uint  `gorm:"index"`  // 关联订单
	UsedAt    *int64
	CreatedAt int64  `gorm:"autoCreateTime"`
}
