package goods

import "github.com/shopspring/decimal"

// GoodsCategory 商品分类
type GoodsCategory struct {
	ID        uint   `gorm:"primarykey"`
	ParentID  uint   `gorm:"default:0;index"` // 0为顶级分类
	Name      string `gorm:"size:100;not null"`
	Icon      string `gorm:"size:255"`
	SortOrder int    `gorm:"default:0"`
	Status    int8   `gorm:"not null;default:1"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// Goods 商品
type Goods struct {
	ID                uint            `gorm:"primarykey"`
	TenantID          uint            `gorm:"not null;default:1;index:idx_tenant_category"`
	SerialNumber      string          `gorm:"size:32;uniqueIndex;not null"` // 商品编号
	CategoryID        uint            `gorm:"not null;index"`
	SupplierID        uint            `gorm:"not null;index"` // 供货商UserID
	Name              string          `gorm:"size:200;not null"`
	Description       string          `gorm:"type:text"`
	Price             decimal.Decimal `gorm:"type:decimal(16,6);not null"`
	CostPrice         decimal.Decimal `gorm:"type:decimal(16,6)"` // 成本价
	Stock             int             `gorm:"not null;default:0"`
	Unit              string          `gorm:"size:20;default:'件'"`
	BuyMin            int             `gorm:"default:1"`   // 最小购买量
	BuyMax            int             `gorm:"default:100"` // 最大购买量
	BuyBase           int             `gorm:"default:1"`   // 购买基数
	IsCardProduct     bool            `gorm:"default:false"`  // 是否卡密商品
	IsClose           bool            `gorm:"default:false"`  // 是否关闭下单
	IsRepeat          bool            `gorm:"default:true"`   // 是否允许重复下单
	AllowRefundStatus string          `gorm:"type:text"`      // 允许退单的状态JSON
	BuyParams         string          `gorm:"type:text"`      // 购买参数配置JSON
	Images            string          `gorm:"type:text"`      // 图片URL数组JSON
	Status            int8            `gorm:"not null;default:1;index"` // 1上架 0下架
	CreatedAt         int64           `gorm:"autoCreateTime"`
	UpdatedAt         int64           `gorm:"autoUpdateTime"`
}

// GoodsBuyParam 购买参数定义（独立表，便于管理复杂参数）
type GoodsBuyParam struct {
	ID          uint   `gorm:"primarykey"`
	GoodsID     uint   `gorm:"not null;index"`
	Name        string `gorm:"size:50;not null"`  // 参数名
	Type        string `gorm:"size:20;not null"`  // text/email/number/phone/qq
	Required    bool   `gorm:"default:false"`
	MinLength   int    `gorm:"default:0"`
	MaxLength   int    `gorm:"default:255"`
	Default     string `gorm:"size:255"`
	Placeholder string `gorm:"size:100"`
	SortOrder   int    `gorm:"default:0"`
}

// GoodsSupplierBinding 商品与供货商绑定（支持多供货商同一商品）
type GoodsSupplierBinding struct {
	ID         uint            `gorm:"primarykey"`
	GoodsID    uint            `gorm:"not null;uniqueIndex:idx_goods_supplier"`
	SupplierID uint            `gorm:"not null;uniqueIndex:idx_goods_supplier"`
	CostPrice  decimal.Decimal `gorm:"type:decimal(16,6)"`
	Priority   int             `gorm:"default:0"` // 优先级，越高越优先
	Status     int8            `gorm:"not null;default:1"`
	CreatedAt  int64           `gorm:"autoCreateTime"`
}
