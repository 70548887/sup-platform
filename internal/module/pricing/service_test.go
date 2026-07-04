package pricing

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupPricingTestDB 创建隔离SQLite内存数据库，迁移pricing表+goods表
func setupPricingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移pricing模块表
	err = Migrate(db)
	require.NoError(t, err, "failed to migrate pricing tables")

	// 创建goods表（pricing CostPrice保护需要）
	err = db.Exec(`CREATE TABLE IF NOT EXISTS goods (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cost_price DECIMAL(16,6) DEFAULT 0
	)`).Error
	require.NoError(t, err, "failed to create goods table")

	return db
}

// newPricingTestService 创建测试用的PricingService（Redis传nil测试降级）
func newPricingTestService(t *testing.T) (*PricingService, *gorm.DB) {
	t.Helper()
	db := setupPricingTestDB(t)
	svc := NewPricingService(db, nil, "test")
	return svc, db
}

// insertGoods 插入测试商品（含成本价）
func insertGoods(t *testing.T, db *gorm.DB, id uint, costPrice decimal.Decimal) {
	t.Helper()
	err := db.Exec("INSERT INTO goods (id, cost_price) VALUES (?, ?)", id, costPrice).Error
	require.NoError(t, err)
}

// ---------- TestCalculatePrice_BasePrice ----------

func TestCalculatePrice_BasePrice(t *testing.T) {
	svc, db := newPricingTestService(t)
	ctx := context.Background()

	goodsID := uint(1)
	basePrice := decimal.NewFromFloat(100.00)

	// 插入商品，成本价为0（不触发CostPrice保护）
	insertGoods(t, db, goodsID, decimal.Zero)

	// 无任何定价规则时，应返回basePrice
	price, err := svc.CalculatePrice(ctx, goodsID, 1, 1, basePrice)
	require.NoError(t, err)
	assert.True(t, price.Equal(basePrice),
		"无规则时应返回basePrice: got %s, want %s", price.String(), basePrice.String())
}

// ---------- TestCalculatePrice_PromotionPriority ----------

func TestCalculatePrice_PromotionPriority(t *testing.T) {
	svc, db := newPricingTestService(t)
	ctx := context.Background()

	goodsID := uint(2)
	basePrice := decimal.NewFromFloat(200.00)

	// 插入商品，成本价设为较低值
	insertGoods(t, db, goodsID, decimal.NewFromInt(10))

	now := time.Now().Unix()
	startAt := now - 3600
	endAt := now + 3600

	// 创建promotion规则（最高优先级）
	promoRule := PricingRule{
		GoodsID:       goodsID,
		RuleType:      "promotion",
		MinQuantity:   1,
		MaxQuantity:   999999,
		Price:         decimal.NewFromFloat(150.00), // 固定促销价150
		PromotionName: "测试促销",
		StartAt:       &startAt,
		EndAt:         &endAt,
		Priority:      100,
		Status:        1,
	}
	err := db.Create(&promoRule).Error
	require.NoError(t, err)

	// 创建tiered规则（低优先级）
	tieredRule := PricingRule{
		GoodsID:         goodsID,
		RuleType:        "tiered",
		MinQuantity:     1,
		MaxQuantity:     999999,
		DiscountPercent: decimal.NewFromInt(10), // 9折=180
		Priority:        50,
		Status:          1,
	}
	err = db.Create(&tieredRule).Error
	require.NoError(t, err)

	// 计算价格：promotion优先级最高，应返回150
	price, err := svc.CalculatePrice(ctx, goodsID, 0, 1, basePrice)
	require.NoError(t, err)
	expectedPrice := decimal.NewFromFloat(150.00)
	assert.True(t, price.Equal(expectedPrice),
		"促销规则优先: got %s, want %s", price.String(), expectedPrice.String())
}

// ---------- TestCalculatePrice_CustomerGroup ----------

func TestCalculatePrice_CustomerGroup(t *testing.T) {
	svc, db := newPricingTestService(t)
	ctx := context.Background()

	goodsID := uint(3)
	customerID := uint(100)
	basePrice := decimal.NewFromFloat(100.00)

	// 插入商品，成本价较低
	insertGoods(t, db, goodsID, decimal.NewFromInt(5))

	// 创建客户分组
	group := CustomerGroup{
		Name:   "VIP客户",
		Status: 1,
	}
	err := db.Create(&group).Error
	require.NoError(t, err)

	// 将客户加入分组
	member := CustomerGroupMember{
		GroupID:    group.ID,
		CustomerID: customerID,
	}
	err = db.Create(&member).Error
	require.NoError(t, err)

	// 创建customer_group定价规则
	groupRule := PricingRule{
		GoodsID:         goodsID,
		RuleType:        "customer_group",
		CustomerGroupID: &group.ID,
		MinQuantity:     1,
		MaxQuantity:     999999,
		Price:           decimal.NewFromFloat(80.00), // VIP固定价80
		Priority:        60,
		Status:          1,
	}
	err = db.Create(&groupRule).Error
	require.NoError(t, err)

	// 计算价格：客户在VIP组，应使用分组价80
	price, err := svc.CalculatePrice(ctx, goodsID, customerID, 1, basePrice)
	require.NoError(t, err)
	expectedPrice := decimal.NewFromFloat(80.00)
	assert.True(t, price.Equal(expectedPrice),
		"客户分组价: got %s, want %s", price.String(), expectedPrice.String())

	// 非VIP客户应返回basePrice
	priceNonVIP, err := svc.CalculatePrice(ctx, goodsID, 999, 1, basePrice)
	require.NoError(t, err)
	assert.True(t, priceNonVIP.Equal(basePrice),
		"非分组客户应返回basePrice: got %s, want %s", priceNonVIP.String(), basePrice.String())
}

// ---------- TestCalculatePrice_Tiered ----------

func TestCalculatePrice_Tiered(t *testing.T) {
	svc, db := newPricingTestService(t)
	ctx := context.Background()

	goodsID := uint(4)
	basePrice := decimal.NewFromFloat(100.00)

	// 插入商品，成本价较低
	insertGoods(t, db, goodsID, decimal.NewFromInt(5))

	// 创建阶梯定价规则
	// 1-9件：9折
	tier1 := PricingRule{
		GoodsID:         goodsID,
		RuleType:        "tiered",
		MinQuantity:     1,
		MaxQuantity:     9,
		DiscountPercent: decimal.NewFromInt(10), // 10%折扣=90
		Priority:        10,
		Status:          1,
	}
	err := db.Create(&tier1).Error
	require.NoError(t, err)

	// 10-99件：8折
	tier2 := PricingRule{
		GoodsID:         goodsID,
		RuleType:        "tiered",
		MinQuantity:     10,
		MaxQuantity:     99,
		DiscountPercent: decimal.NewFromInt(20), // 20%折扣=80
		Priority:        10,
		Status:          1,
	}
	err = db.Create(&tier2).Error
	require.NoError(t, err)

	// 100+件：固定价60
	tier3 := PricingRule{
		GoodsID:     goodsID,
		RuleType:    "tiered",
		MinQuantity: 100,
		MaxQuantity: 999999,
		Price:       decimal.NewFromFloat(60.00),
		Priority:    10,
		Status:      1,
	}
	err = db.Create(&tier3).Error
	require.NoError(t, err)

	// 购买5件应走tier1：100 * (1-10/100) = 90
	price1, err := svc.CalculatePrice(ctx, goodsID, 0, 5, basePrice)
	require.NoError(t, err)
	expected1 := decimal.NewFromFloat(90.00)
	assert.True(t, price1.Equal(expected1),
		"阶梯1(5件): got %s, want %s", price1.String(), expected1.String())

	// 购买50件应走tier2：100 * (1-20/100) = 80
	price2, err := svc.CalculatePrice(ctx, goodsID, 0, 50, basePrice)
	require.NoError(t, err)
	expected2 := decimal.NewFromFloat(80.00)
	assert.True(t, price2.Equal(expected2),
		"阶梯2(50件): got %s, want %s", price2.String(), expected2.String())

	// 购买200件应走tier3：固定价60
	price3, err := svc.CalculatePrice(ctx, goodsID, 0, 200, basePrice)
	require.NoError(t, err)
	expected3 := decimal.NewFromFloat(60.00)
	assert.True(t, price3.Equal(expected3),
		"阶梯3(200件): got %s, want %s", price3.String(), expected3.String())
}

// ---------- TestCalculatePrice_CostProtection ----------

func TestCalculatePrice_CostProtection(t *testing.T) {
	svc, db := newPricingTestService(t)
	ctx := context.Background()

	goodsID := uint(5)
	basePrice := decimal.NewFromFloat(100.00)
	costPrice := decimal.NewFromFloat(70.00)

	// 插入商品，成本价70
	insertGoods(t, db, goodsID, costPrice)

	// 创建一个低于成本价的促销规则：固定价50（低于成本价70）
	now := time.Now().Unix()
	startAt := now - 3600
	endAt := now + 3600
	promoRule := PricingRule{
		GoodsID:       goodsID,
		RuleType:      "promotion",
		MinQuantity:   1,
		MaxQuantity:   999999,
		Price:         decimal.NewFromFloat(50.00), // 低于成本价
		PromotionName: "超低促销",
		StartAt:       &startAt,
		EndAt:         &endAt,
		Priority:      100,
		Status:        1,
	}
	err := db.Create(&promoRule).Error
	require.NoError(t, err)

	// 计算价格：促销价50但低于成本价70，应返回成本价70
	price, err := svc.CalculatePrice(ctx, goodsID, 0, 1, basePrice)
	require.NoError(t, err)
	assert.True(t, price.Equal(costPrice),
		"CostProtection: 最终价格不应低于成本价: got %s, want %s", price.String(), costPrice.String())
}

// ---------- TestCalculatePrice_NilRedis ----------

func TestCalculatePrice_NilRedis(t *testing.T) {
	// PricingService的redisClient为nil时应正常工作（降级查DB）
	db := setupPricingTestDB(t)
	ctx := context.Background()

	// 明确传nil作为redisClient
	svc := NewPricingService(db, nil, "test")

	goodsID := uint(6)
	basePrice := decimal.NewFromFloat(100.00)
	insertGoods(t, db, goodsID, decimal.Zero)

	// 创建一条tiered规则
	rule := PricingRule{
		GoodsID:         goodsID,
		RuleType:        "tiered",
		MinQuantity:     1,
		MaxQuantity:     999999,
		DiscountPercent: decimal.NewFromInt(15), // 15%折扣=85
		Priority:        10,
		Status:          1,
	}
	err := db.Create(&rule).Error
	require.NoError(t, err)

	// Redis为nil时应直接查DB，正常返回折扣价
	price, err := svc.CalculatePrice(ctx, goodsID, 0, 1, basePrice)
	require.NoError(t, err)
	// 100 * (1 - 15/100) = 85
	expectedPrice := decimal.NewFromFloat(85.00)
	assert.True(t, price.Equal(expectedPrice),
		"Redis为nil降级查DB: got %s, want %s", price.String(), expectedPrice.String())
}
