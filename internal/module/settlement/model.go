package settlement

import "github.com/shopspring/decimal"

// Settlement 结算单
type Settlement struct {
	ID               uint            `gorm:"primarykey"`
	TenantID         uint            `gorm:"not null;index"`
	SupplierID       uint            `gorm:"not null;index:idx_supplier_period"`
	Period           string          `gorm:"size:7;not null;index:idx_supplier_period"` // "2026-07"
	TotalOrders      int             `gorm:"default:0"`
	TotalAmount      decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	CommissionRate   decimal.Decimal `gorm:"type:decimal(8,4);not null"`  // 平台佣金比例
	CommissionAmount decimal.Decimal `gorm:"type:decimal(16,6);not null"` // 平台佣金金额
	NetAmount        decimal.Decimal `gorm:"type:decimal(16,6);not null"` // 供货商实收
	Status           string          `gorm:"size:20;not null;default:pending"` // pending/confirmed/paid
	ConfirmedAt      *int64
	PaidAt           *int64
	CreatedAt        int64 `gorm:"autoCreateTime"`
	UpdatedAt        int64 `gorm:"autoUpdateTime"`
}

// ProfitShare 分润记录
type ProfitShare struct {
	ID             uint            `gorm:"primarykey"`
	TenantID       uint            `gorm:"not null;index"`
	OrderID        uint            `gorm:"not null;index"`
	SupplierID     uint            `gorm:"not null;index"`
	OrderAmount    decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	PlatformRate   decimal.Decimal `gorm:"type:decimal(8,4);not null"`
	PlatformProfit decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	SupplierProfit decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	CreatedAt      int64           `gorm:"autoCreateTime"`
}
