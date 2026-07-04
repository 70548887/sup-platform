package reconciliation

import "github.com/shopspring/decimal"

// ReconciliationTask 对账任务
type ReconciliationTask struct {
	ID       uint   `gorm:"primarykey"`
	TenantID uint   `gorm:"not null;default:1;index"`
	Type         string `gorm:"size:30;not null"`                        // balance_check, cross_verify
	Status       string `gorm:"size:20;not null;default:running"`        // running, completed, failed
	TotalChecked int    `gorm:"default:0"`
	ErrorCount   int    `gorm:"default:0"`
	StartedAt    int64  `gorm:"not null"`
	CompletedAt  *int64
	CreatedAt    int64  `gorm:"autoCreateTime"`
}

// ReconciliationError 对账异常
type ReconciliationError struct {
	ID         uint            `gorm:"primarykey"`
	TaskID     uint            `gorm:"not null;index"`
	ErrorType  string          `gorm:"size:30;not null"`                        // balance_mismatch, cross_mismatch
	UserID     uint            `gorm:"not null;index"`
	Expected   decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Actual     decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Difference decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	Status     string          `gorm:"size:20;not null;default:pending"`        // pending, auto_fixed, manual_fixed, ignored
	Resolution string          `gorm:"size:500"`
	ResolvedBy string          `gorm:"size:50"`
	CreatedAt  int64           `gorm:"autoCreateTime"`
	ResolvedAt *int64
}

// 对账类型常量
const (
	TypeBalanceCheck = "balance_check"
	TypeCrossVerify  = "cross_verify"
)

// 任务状态常量
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// 异常类型常量
const (
	ErrorBalanceMismatch = "balance_mismatch"
	ErrorCrossMismatch   = "cross_mismatch"
)

// 异常处理状态常量
const (
	ErrorStatusPending    = "pending"
	ErrorStatusAutoFixed   = "auto_fixed"
	ErrorStatusManualFixed = "manual_fixed"
	ErrorStatusIgnored     = "ignored"
)
