package recharge

import "github.com/shopspring/decimal"

// RechargeOrder 充值申请单
type RechargeOrder struct {
	ID             uint            `gorm:"primarykey"`
	RechargeSN     string          `gorm:"size:32;uniqueIndex;not null"` // 充值单号
	UserID         uint            `gorm:"not null;index"`
	Amount         decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Status         int8            `gorm:"not null;default:1"`           // 1=待审核 2=已批准 3=已拒绝 4=已到账
	IdempotencyKey string          `gorm:"size:64;uniqueIndex"`          // 幂等键
	ApproverID     *uint                                                 // 审核人
	ApprovalNote   string          `gorm:"size:500"`
	ApprovedAt     *int64
	Version        int64 `gorm:"not null;default:0"` // CAS防重复审核
	CreatedAt      int64 `gorm:"autoCreateTime"`
	UpdatedAt      int64 `gorm:"autoUpdateTime"`
}

// 状态常量
const (
	StatusPending   int8 = 1 // 待审核
	StatusApproved  int8 = 2 // 已批准
	StatusRejected  int8 = 3 // 已拒绝
	StatusCompleted int8 = 4 // 已到账
)
