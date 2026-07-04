package analytics

import "github.com/shopspring/decimal"

// DailyStats 每日统计（预聚合）
type DailyStats struct {
	ID              uint            `gorm:"primarykey"`
	Date            string          `gorm:"size:10;not null;uniqueIndex"` // 2006-01-02
	TotalOrders     int             `gorm:"default:0"`
	TotalAmount     decimal.Decimal `gorm:"type:decimal(16,6);default:0"`
	TotalRefunds    int             `gorm:"default:0"`
	RefundAmount    decimal.Decimal `gorm:"type:decimal(16,6);default:0"`
	NewCustomers    int             `gorm:"default:0"`
	ActiveCustomers int             `gorm:"default:0"`
	CreatedAt       int64           `gorm:"autoCreateTime"`
}

// HotGoods 热卖商品排行
type HotGoods struct {
	ID          uint            `gorm:"primarykey"`
	Date        string          `gorm:"size:10;not null;index:idx_date_goods"`
	GoodsID     uint            `gorm:"not null;index:idx_date_goods"`
	GoodsName   string          `gorm:"size:200"`
	OrderCount  int             `gorm:"default:0"`
	TotalAmount decimal.Decimal `gorm:"type:decimal(16,6);default:0"`
	CreatedAt   int64           `gorm:"autoCreateTime"`
}
