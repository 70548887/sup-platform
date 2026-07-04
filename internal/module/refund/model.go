package refund

import "github.com/shopspring/decimal"

// RefundOrder 退款单
type RefundOrder struct {
	ID         uint            `gorm:"primarykey"`
	RefundSN   string          `gorm:"size:32;uniqueIndex;not null"` // 退款单号
	OrderID    uint            `gorm:"not null;index"`
	OrderSN    string          `gorm:"size:32;not null"`
	CustomerID uint            `gorm:"not null;index"`
	Amount     decimal.Decimal `gorm:"type:decimal(16,6);not null"` // 退款金额
	Reason     string          `gorm:"size:500"`
	Status     int8            `gorm:"not null;default:1"` // 1=待审核 2=已批准 3=已拒绝 4=已退款
	ReviewerID *uint           // 审核人
	ReviewNote string          `gorm:"size:500"`
	ReviewedAt *int64
	RefundedAt *int64
	CreatedAt  int64 `gorm:"autoCreateTime"`
	UpdatedAt  int64 `gorm:"autoUpdateTime"`
}

const (
	RefundPending   int8 = 1 // 待审核
	RefundApproved  int8 = 2 // 已批准
	RefundRejected  int8 = 3 // 已拒绝
	RefundCompleted int8 = 4 // 已退款
)
