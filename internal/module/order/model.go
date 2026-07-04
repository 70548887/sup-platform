package order

import "github.com/shopspring/decimal"

// Order 订单主表
type Order struct {
	ID                 uint            `gorm:"primarykey"`
	TenantID           uint            `gorm:"not null;default:1;index:idx_tenant_status"`
	OrderSN            string          `gorm:"size:32;uniqueIndex;not null"`          // 平台订单号
	CustomerOrderID    string          `gorm:"size:64;index:idx_app_custorder"`       // 客户方订单号（幂等）
	AppID              uint            `gorm:"not null;index:idx_app_custorder"`      // API应用ID
	CustomerID         uint            `gorm:"not null;index"`
	SupplierID         uint            `gorm:"not null;index"`
	GoodsID            uint            `gorm:"not null"`
	GoodsSN            string          `gorm:"size:32;not null"`
	GoodsName          string          `gorm:"size:200"`
	BuyNumber          int             `gorm:"not null"`
	UnitPrice          decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Amount             decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	RefundAmount       decimal.Decimal `gorm:"type:decimal(16,6);default:0"`
	Status             int8            `gorm:"not null;default:1;index"` // 1-9状态
	Version            int64           `gorm:"not null;default:0"`       // CAS乐观锁
	CallbackStartNum   int             `gorm:"default:0"`                // 总进度
	CallbackCurrentNum int             `gorm:"default:0"`                // 当前进度
	NotifyURL          string          `gorm:"size:500"`                 // 供货商通知地址
	Remark             string          `gorm:"size:500"`
	PaidAt             *int64
	CompletedAt        *int64
	CreatedAt          int64 `gorm:"autoCreateTime;index"`
	UpdatedAt          int64 `gorm:"autoUpdateTime"`
}

// OrderBuyParam 订单购买参数快照
type OrderBuyParam struct {
	ID      uint   `gorm:"primarykey"`
	OrderID uint   `gorm:"not null;index"`
	Name    string `gorm:"size:50;not null"`
	Value   string `gorm:"size:500;not null"`
}

// OrderStatusChange 状态变更审计
type OrderStatusChange struct {
	ID        uint   `gorm:"primarykey"`
	OrderID   uint   `gorm:"not null;index"`
	OldStatus int8   `gorm:"not null"`
	NewStatus int8   `gorm:"not null"`
	Operator  string `gorm:"size:50"`  // 操作者（system/supplier/admin）
	Remark    string `gorm:"size:500"`
	CreatedAt int64  `gorm:"autoCreateTime"`
}

// OrderCard 订单卡密关联
type OrderCard struct {
	ID      uint `gorm:"primarykey"`
	OrderID uint `gorm:"not null;index"`
	CardID  uint `gorm:"not null;uniqueIndex"` // 一张卡密只能关联一个订单
}

// OrderCallback 通知投递记录
type OrderCallback struct {
	ID          uint   `gorm:"primarykey"`
	OrderID     uint   `gorm:"not null;index"`
	URL         string `gorm:"size:500;not null"`
	Payload     string `gorm:"type:text"`
	StatusCode  int    // HTTP响应码
	Response    string `gorm:"type:text"`
	RetryCount  int    `gorm:"default:0"`
	Success     bool   `gorm:"default:false"`
	CreatedAt   int64  `gorm:"autoCreateTime"`
	NextRetryAt *int64 // 下次重试时间
}

// OrderProgress 进度追踪
type OrderProgress struct {
	ID         uint  `gorm:"primarykey"`
	OrderID    uint  `gorm:"not null;uniqueIndex"`
	StartNum   int   `gorm:"not null"`            // 总数
	CurrentNum int   `gorm:"not null;default:0"`  // 当前完成数
	UpdatedAt  int64 `gorm:"autoUpdateTime"`
}
