package pricing

import "github.com/shopspring/decimal"

// PricingRule 定价规则
type PricingRule struct {
	ID              uint            `gorm:"primarykey"`
	GoodsID         uint            `gorm:"not null;index:idx_goods_type"`
	RuleType        string          `gorm:"size:30;not null;index:idx_goods_type"` // customer_group, tiered, promotion
	CustomerGroupID *uint           `gorm:"index"`
	MinQuantity     int             `gorm:"default:1"`
	MaxQuantity     int             `gorm:"default:999999"`
	Price           decimal.Decimal `gorm:"type:decimal(16,6)"`
	DiscountPercent decimal.Decimal `gorm:"type:decimal(8,4);default:0"`
	PromotionName   string          `gorm:"size:100"`
	StartAt         *int64
	EndAt           *int64
	Priority        int    `gorm:"default:0;not null"`
	Status          int8   `gorm:"not null;default:1"` // 1=active, 0=disabled
	Version         int64  `gorm:"not null;default:0"`
	CreatedAt       int64  `gorm:"autoCreateTime"`
	UpdatedAt       int64  `gorm:"autoUpdateTime"`
}

// CustomerGroup 客户分组
type CustomerGroup struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"size:50;not null"`
	Description string `gorm:"size:200"`
	Status      int8   `gorm:"not null;default:1"`
	CreatedAt   int64  `gorm:"autoCreateTime"`
	UpdatedAt   int64  `gorm:"autoUpdateTime"`
}

// CustomerGroupMember 分组成员
type CustomerGroupMember struct {
	ID         uint  `gorm:"primarykey"`
	GroupID    uint  `gorm:"not null;uniqueIndex:idx_group_customer"`
	CustomerID uint  `gorm:"not null;uniqueIndex:idx_group_customer"`
	CreatedAt  int64 `gorm:"autoCreateTime"`
}
