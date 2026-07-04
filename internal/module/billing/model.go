package billing

import "github.com/shopspring/decimal"

// SubscriptionPlan SaaS套餐
type SubscriptionPlan struct {
	ID                  uint            `gorm:"primarykey"`
	Name                string          `gorm:"size:50;uniqueIndex;not null"` // basic, professional, enterprise
	DisplayName         string          `gorm:"size:100;not null"`
	MonthlyPrice        decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	MaxAPICallsPerMonth int             `gorm:"not null"`
	MaxOrders           int             `gorm:"not null;default:999999"`
	MaxAdmins           int             `gorm:"not null;default:5"`
	Features            string          `gorm:"type:text"` // JSON
	Status              int8            `gorm:"not null;default:1"`
	CreatedAt           int64           `gorm:"autoCreateTime"`
	UpdatedAt           int64           `gorm:"autoUpdateTime"`
}

// TenantSubscription 租户订阅
type TenantSubscription struct {
	ID        uint   `gorm:"primarykey"`
	TenantID  uint   `gorm:"uniqueIndex;not null"`
	PlanID    uint   `gorm:"not null"`
	StartAt   int64  `gorm:"not null"`
	EndAt     int64  `gorm:"not null"`
	AutoRenew bool   `gorm:"default:true"`
	Status    string `gorm:"size:20;not null;default:active"` // active, expired, cancelled
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// APIUsage API使用量（月度）
type APIUsage struct {
	ID           uint  `gorm:"primarykey"`
	TenantID     uint  `gorm:"not null;uniqueIndex:idx_tenant_month"`
	Year         int   `gorm:"not null;uniqueIndex:idx_tenant_month"`
	Month        int   `gorm:"not null;uniqueIndex:idx_tenant_month"`
	APICallCount int   `gorm:"not null;default:0"`
	OrderCount   int   `gorm:"not null;default:0"`
	Version      int64 `gorm:"not null;default:0"` // CAS乐观锁
	CreatedAt    int64 `gorm:"autoCreateTime"`
	UpdatedAt    int64 `gorm:"autoUpdateTime"`
}

// Invoice 账单
type Invoice struct {
	ID            uint            `gorm:"primarykey"`
	TenantID      uint            `gorm:"not null;index"`
	Month         string          `gorm:"size:7;not null"` // "2026-07"
	PlanFee       decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	OverageCharge decimal.Decimal `gorm:"type:decimal(16,6);default:0"`
	TotalAmount   decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Status        string          `gorm:"size:20;not null;default:pending"` // pending, paid, overdue
	IssuedAt      int64           `gorm:"not null"`
	PaidAt        *int64
	CreatedAt     int64 `gorm:"autoCreateTime"`
}
