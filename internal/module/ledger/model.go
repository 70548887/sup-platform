package ledger

import "github.com/shopspring/decimal"

// Wallet 余额账户
type Wallet struct {
	ID       uint            `gorm:"primarykey"`
	TenantID uint            `gorm:"not null;default:1;index"`
	UserID    uint            `gorm:"not null;uniqueIndex"`
	Balance   decimal.Decimal `gorm:"type:decimal(16,6);not null;default:0"`
	Frozen    decimal.Decimal `gorm:"type:decimal(16,6);not null;default:0"` // 冻结金额
	Version   int64           `gorm:"not null;default:0"`                    // CAS
	UpdatedAt int64           `gorm:"autoUpdateTime"`
}

// LedgerEntry 不可变账本流水（只INSERT不UPDATE）
type LedgerEntry struct {
	ID           uint            `gorm:"primarykey"`
	TenantID     uint            `gorm:"not null;default:1;index"`
	WalletID     uint            `gorm:"not null;index"`
	UserID       uint            `gorm:"not null;index"`
	Type         string          `gorm:"size:30;not null;index"` // recharge/order_pay/order_refund/withdraw/freeze/unfreeze/bonus
	RelatedID    uint            `gorm:"default:0"`              // 关联业务ID
	Amount       decimal.Decimal `gorm:"type:decimal(16,6);not null"`  // 正数入账，负数出账
	BalanceAfter decimal.Decimal `gorm:"type:decimal(16,6);not null"`  // 变动后余额快照
	Note         string          `gorm:"size:500"`
	CreatedAt    int64           `gorm:"autoCreateTime;index"`
}

// Recharge 充值记录
type Recharge struct {
	ID            uint            `gorm:"primarykey"`
	UserID        uint            `gorm:"not null;index"`
	Amount        decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	PaymentMethod string          `gorm:"size:30"`              // wechat/alipay/manual
	TradeNo       string          `gorm:"size:64;uniqueIndex"` // 外部交易号
	Status        int8            `gorm:"not null;default:0"`  // 0待支付 1成功 2失败
	PaidAt        *int64
	CreatedAt     int64 `gorm:"autoCreateTime"`
}
