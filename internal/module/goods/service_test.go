package goods

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupGoodsTestDB 创建隔离SQLite内存数据库，迁移goods模块表
func setupGoodsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = AutoMigrate(db)
	require.NoError(t, err, "failed to migrate goods tables")

	return db
}

// createTestGoods 辅助函数：创建一个测试商品并返回
func createTestGoods(t *testing.T, svc *GoodsService, name string, supplierID uint, categoryID uint, price decimal.Decimal) *Goods {
	t.Helper()
	ctx := context.Background()
	goods, err := svc.CreateGoods(ctx, CreateGoodsParams{
		CategoryID: categoryID,
		SupplierID: supplierID,
		Name:       name,
		Price:      price,
	})
	require.NoError(t, err)
	require.NotNil(t, goods)
	return goods
}

// ---------- TestCreateGoods_Success ----------

func TestCreateGoods_Success(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	params := CreateGoodsParams{
		CategoryID:    1,
		SupplierID:    10,
		Name:          "测试商品A",
		Description:   "这是一个测试商品",
		Price:         decimal.NewFromFloat(99.50),
		CostPrice:     decimal.NewFromFloat(50.00),
		Unit:          "个",
		BuyMin:        1,
		BuyMax:        10,
		BuyBase:       1,
		IsCardProduct: false,
		Images:        `["https://example.com/img1.png"]`,
	}

	goods, err := svc.CreateGoods(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, goods)

	// 验证字段
	assert.Equal(t, params.Name, goods.Name)
	assert.Equal(t, params.Description, goods.Description)
	assert.True(t, goods.Price.Equal(params.Price), "价格应为%s，实际为%s", params.Price.String(), goods.Price.String())
	assert.True(t, goods.CostPrice.Equal(params.CostPrice))
	assert.Equal(t, params.CategoryID, goods.CategoryID)
	assert.Equal(t, params.SupplierID, goods.SupplierID)
	assert.Equal(t, params.Unit, goods.Unit)
	assert.Equal(t, params.BuyMin, goods.BuyMin)
	assert.Equal(t, params.BuyMax, goods.BuyMax)
	assert.Equal(t, params.BuyBase, goods.BuyBase)
	assert.Equal(t, int8(1), goods.Status) // 默认上架
	assert.NotEmpty(t, goods.SerialNumber)
	assert.Regexp(t, `^G\d{12}$`, goods.SerialNumber, "序列号格式应为G+年月日+4位序号")
	assert.Equal(t, params.Images, goods.Images)

	// 验证序列号唯一性：连续创建两个商品，序列号应不同
	goods2, err := svc.CreateGoods(ctx, CreateGoodsParams{
		CategoryID: 1,
		SupplierID: 10,
		Name:       "测试商品B",
		Price:      decimal.NewFromInt(10),
	})
	require.NoError(t, err)
	assert.NotEqual(t, goods.SerialNumber, goods2.SerialNumber, "不同商品的序列号应唯一")
}

// ---------- TestUpdateGoods_Success ----------

func TestUpdateGoods_Success(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	goods := createTestGoods(t, svc, "原始商品", 10, 1, decimal.NewFromInt(100))

	// 更新名称和价格
	newName := "更新后商品"
	newPrice := decimal.NewFromFloat(200.50)
	newDesc := "新的描述"
	newUnit := "箱"
	newStatus := int8(0)

	err := svc.UpdateGoods(ctx, goods.SerialNumber, UpdateGoodsParams{
		Name:        &newName,
		Price:       &newPrice,
		Description: &newDesc,
		Unit:        &newUnit,
		Status:      &newStatus,
	})
	require.NoError(t, err)

	// 重新查询验证
	updated, err := svc.GetGoods(ctx, goods.SerialNumber)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	assert.True(t, updated.Price.Equal(newPrice), "价格应为%s，实际为%s", newPrice.String(), updated.Price.String())
	assert.Equal(t, newDesc, updated.Description)
	assert.Equal(t, newUnit, updated.Unit)
	assert.Equal(t, newStatus, updated.Status)

	// 验证未更新的字段保持不变
	assert.Equal(t, goods.CategoryID, updated.CategoryID)
	assert.Equal(t, goods.SupplierID, updated.SupplierID)
}

// ---------- TestListGoods_WithFilters ----------

func TestListGoods_WithFilters(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	// 创建多个商品，分属不同分类、供货商、状态
	createTestGoods(t, svc, "商品A-分类1-供货商10", 10, 1, decimal.NewFromInt(10))
	createTestGoods(t, svc, "商品B-分类1-供货商20", 20, 1, decimal.NewFromInt(20))
	createTestGoods(t, svc, "商品C-分类2-供货商10", 10, 2, decimal.NewFromInt(30))
	g4 := createTestGoods(t, svc, "商品D-分类2-供货商20", 20, 2, decimal.NewFromInt(40))

	// 下架商品D
	offlineStatus := int8(0)
	err := svc.UpdateGoods(ctx, g4.SerialNumber, UpdateGoodsParams{Status: &offlineStatus})
	require.NoError(t, err)

	t.Run("按分类筛选", func(t *testing.T) {
		catID := uint(1)
		list, total, err := svc.ListGoods(ctx, GoodsFilter{CategoryID: &catID})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		for _, g := range list {
			assert.Equal(t, catID, g.CategoryID)
		}
	})

	t.Run("按供货商筛选", func(t *testing.T) {
		supID := uint(10)
		list, total, err := svc.ListGoods(ctx, GoodsFilter{SupplierID: &supID})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		for _, g := range list {
			assert.Equal(t, supID, g.SupplierID)
		}
	})

	t.Run("按状态筛选(上架)", func(t *testing.T) {
		status := int8(1)
		list, total, err := svc.ListGoods(ctx, GoodsFilter{Status: &status})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total, "应有3个上架商品")
		for _, g := range list {
			assert.Equal(t, status, g.Status)
		}
	})

	t.Run("按名称模糊搜索", func(t *testing.T) {
		list, total, err := svc.ListGoods(ctx, GoodsFilter{Name: "分类2"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		for _, g := range list {
			assert.Contains(t, g.Name, "分类2")
		}
	})

	t.Run("组合筛选(分类+供货商)", func(t *testing.T) {
		catID := uint(1)
		supID := uint(10)
		list, total, err := svc.ListGoods(ctx, GoodsFilter{
			CategoryID: &catID,
			SupplierID: &supID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "商品A-分类1-供货商10", list[0].Name)
	})

	t.Run("按序列号精确匹配", func(t *testing.T) {
		list, total, err := svc.ListGoods(ctx, GoodsFilter{SerialNumber: g4.SerialNumber})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, g4.ID, list[0].ID)
	})
}

// ---------- TestGetGoodsDetail ----------

func TestGetGoodsDetail(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	goods := createTestGoods(t, svc, "详情测试商品", 10, 1, decimal.NewFromFloat(123.456))

	t.Run("正常获取", func(t *testing.T) {
		detail, err := svc.GetGoods(ctx, goods.SerialNumber)
		require.NoError(t, err)
		require.NotNil(t, detail)
		assert.Equal(t, goods.ID, detail.ID)
		assert.Equal(t, goods.SerialNumber, detail.SerialNumber)
		assert.Equal(t, "详情测试商品", detail.Name)
		assert.True(t, detail.Price.Equal(decimal.NewFromFloat(123.456)))
	})

	t.Run("不存在的序列号", func(t *testing.T) {
		detail, err := svc.GetGoods(ctx, "NONEXISTENT_SN")
		assert.Error(t, err)
		assert.Nil(t, detail)
		assert.Contains(t, err.Error(), "商品不存在")
	})
}

// ---------- TestBindSupplier ----------

func TestBindSupplier(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	goods := createTestGoods(t, svc, "供货商绑定测试", 10, 1, decimal.NewFromInt(50))

	t.Run("创建供货商绑定", func(t *testing.T) {
		binding := &GoodsSupplierBinding{
			GoodsID:    goods.ID,
			SupplierID: 20,
			CostPrice:  decimal.NewFromFloat(30.00),
			Priority:   5,
			Status:     1,
		}
		err := db.WithContext(ctx).Create(binding).Error
		require.NoError(t, err)
		assert.NotZero(t, binding.ID)
	})

	t.Run("同一商品绑定多个供货商", func(t *testing.T) {
		binding2 := &GoodsSupplierBinding{
			GoodsID:    goods.ID,
			SupplierID: 30,
			CostPrice:  decimal.NewFromFloat(28.00),
			Priority:   10,
			Status:     1,
		}
		err := db.WithContext(ctx).Create(binding2).Error
		require.NoError(t, err)

		// 查询该商品的所有供货商绑定
		var bindings []GoodsSupplierBinding
		err = db.WithContext(ctx).Where("goods_id = ?", goods.ID).Find(&bindings).Error
		require.NoError(t, err)
		assert.Len(t, bindings, 2, "应有2个供货商绑定")
	})

	t.Run("同一商品同一供货商不能重复绑定(唯一索引)", func(t *testing.T) {
		duplicate := &GoodsSupplierBinding{
			GoodsID:    goods.ID,
			SupplierID: 20, // 已绑定过
			CostPrice:  decimal.NewFromFloat(25.00),
			Priority:   1,
			Status:     1,
		}
		err := db.WithContext(ctx).Create(duplicate).Error
		assert.Error(t, err, "重复绑定应因唯一索引冲突而失败")
	})
}

// ---------- TestUpdateStock_Success ----------

func TestUpdateStock_Success(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	goods := createTestGoods(t, svc, "库存测试商品", 10, 1, decimal.NewFromInt(10))

	// 先增加库存到100
	err := svc.repo.AddStock(ctx, goods.ID, 100)
	require.NoError(t, err)

	// 验证库存已增加
	refreshed, err := svc.GetGoods(ctx, goods.SerialNumber)
	require.NoError(t, err)
	assert.Equal(t, 100, refreshed.Stock, "库存应为100")

	t.Run("正常扣减库存", func(t *testing.T) {
		err := svc.DeductStock(ctx, db, goods.ID, 30)
		require.NoError(t, err)

		refreshed, err := svc.GetGoods(ctx, goods.SerialNumber)
		require.NoError(t, err)
		assert.Equal(t, 70, refreshed.Stock, "扣减30后库存应为70")
	})

	t.Run("库存不足扣减应失败", func(t *testing.T) {
		err := svc.DeductStock(ctx, db, goods.ID, 200)
		assert.Error(t, err, "库存不足时应返回错误")
		assert.Contains(t, err.Error(), "库存不足")

		// 验证库存未变化
		refreshed, err := svc.GetGoods(ctx, goods.SerialNumber)
		require.NoError(t, err)
		assert.Equal(t, 70, refreshed.Stock, "扣减失败后库存应保持不变")
	})

	t.Run("扣减数量为0应失败", func(t *testing.T) {
		err := svc.DeductStock(ctx, db, goods.ID, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "扣减数量必须大于0")
	})

	t.Run("增加库存", func(t *testing.T) {
		err := svc.repo.AddStock(ctx, goods.ID, 50)
		require.NoError(t, err)

		refreshed, err := svc.GetGoods(ctx, goods.SerialNumber)
		require.NoError(t, err)
		assert.Equal(t, 120, refreshed.Stock, "增加50后库存应为120")
	})
}

// ---------- TestCategoryTree ----------

func TestCategoryTree(t *testing.T) {
	db := setupGoodsTestDB(t)
	svc := NewGoodsService(db)
	ctx := context.Background()

	// 构建多级分类树：
	// 电子产品 (parent_id=0)
	//   ├── 手机 (parent_id=电子产品.ID)
	//   │   ├── 智能手机 (parent_id=手机.ID)
	//   │   └── 功能手机 (parent_id=手机.ID)
	//   └── 电脑 (parent_id=电子产品.ID)
	// 服装 (parent_id=0)

	electronics, err := svc.CreateCategory(ctx, "电子产品", 0)
	require.NoError(t, err)

	phone, err := svc.CreateCategory(ctx, "手机", electronics.ID)
	require.NoError(t, err)

	smartphone, err := svc.CreateCategory(ctx, "智能手机", phone.ID)
	require.NoError(t, err)

	featurePhone, err := svc.CreateCategory(ctx, "功能手机", phone.ID)
	require.NoError(t, err)

	computer, err := svc.CreateCategory(ctx, "电脑", electronics.ID)
	require.NoError(t, err)

	clothing, err := svc.CreateCategory(ctx, "服装", 0)
	require.NoError(t, err)

	t.Run("查询顶级分类", func(t *testing.T) {
		topCats, err := svc.GetCategories(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, topCats, 2, "应有2个顶级分类")

		names := []string{topCats[0].Name, topCats[1].Name}
		assert.Contains(t, names, "电子产品")
		assert.Contains(t, names, "服装")

		for _, cat := range topCats {
			assert.Equal(t, uint(0), cat.ParentID, "顶级分类parent_id应为0")
		}
	})

	t.Run("查询电子产品子分类", func(t *testing.T) {
		children, err := svc.GetCategories(ctx, &electronics.ID)
		require.NoError(t, err)
		assert.Len(t, children, 2, "电子产品应有2个子分类")

		names := []string{children[0].Name, children[1].Name}
		assert.Contains(t, names, "手机")
		assert.Contains(t, names, "电脑")

		for _, child := range children {
			assert.Equal(t, electronics.ID, child.ParentID, "子分类parent_id应指向电子产品")
		}
	})

	t.Run("查询手机子分类(多级嵌套)", func(t *testing.T) {
		children, err := svc.GetCategories(ctx, &phone.ID)
		require.NoError(t, err)
		assert.Len(t, children, 2, "手机应有2个子分类")

		names := []string{children[0].Name, children[1].Name}
		assert.Contains(t, names, "智能手机")
		assert.Contains(t, names, "功能手机")

		for _, child := range children {
			assert.Equal(t, phone.ID, child.ParentID, "子分类parent_id应指向手机")
		}
	})

	t.Run("叶子节点无子分类", func(t *testing.T) {
		children, err := svc.GetCategories(ctx, &smartphone.ID)
		require.NoError(t, err)
		assert.Empty(t, children, "智能手机作为叶子节点应无子分类")

		children2, err := svc.GetCategories(ctx, &featurePhone.ID)
		require.NoError(t, err)
		assert.Empty(t, children2, "功能手机作为叶子节点应无子分类")

		children3, err := svc.GetCategories(ctx, &computer.ID)
		require.NoError(t, err)
		assert.Empty(t, children3, "电脑作为叶子节点应无子分类")
	})

	t.Run("验证分类parent_id嵌套正确性", func(t *testing.T) {
		// 从叶子节点向上追溯到顶级分类
		var leaf GoodsCategory
		err := db.WithContext(ctx).Where("id = ?", smartphone.ID).First(&leaf).Error
		require.NoError(t, err)
		assert.Equal(t, phone.ID, leaf.ParentID, "智能手机→手机")

		var parent GoodsCategory
		err = db.WithContext(ctx).Where("id = ?", leaf.ParentID).First(&parent).Error
		require.NoError(t, err)
		assert.Equal(t, "手机", parent.Name)
		assert.Equal(t, electronics.ID, parent.ParentID, "手机→电子产品")

		var grandparent GoodsCategory
		err = db.WithContext(ctx).Where("id = ?", parent.ParentID).First(&grandparent).Error
		require.NoError(t, err)
		assert.Equal(t, "电子产品", grandparent.Name)
		assert.Equal(t, uint(0), grandparent.ParentID, "电子产品为顶级分类")
	})

	t.Run("创建空名称分类应失败", func(t *testing.T) {
		_, err := svc.CreateCategory(ctx, "", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "分类名称不能为空")
	})

	_ = clothing // suppress unused warning
}
