package card

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/testutil"
)

// testGoods 测试用最小化商品模型，仅用于 ImportCards 的库存更新
type testGoods struct {
	ID    uint  `gorm:"primarykey"`
	Stock int64 `gorm:"not null;default:0"`
}

func (testGoods) TableName() string { return "goods" }

// setupCardTestDB 创建测试数据库并迁移卡密模型和最小化商品表
func setupCardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.SetupIsolatedTestDB()

	// SQLite 内存模式限制单连接，避免事务问题
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(&Card{}, &CardBatch{}, &testGoods{}))
	return db
}

// testEncryptKey 32字节 AES-256 测试密钥
var testEncryptKey = []byte("01234567890123456789012345678901")

// ---------- TestImportCards_Success ----------

func TestImportCards_Success(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, nil) // 不加密
	ctx := context.Background()

	contents := []string{"CARD-001", "CARD-002", "CARD-003"}

	t.Run("正常导入3张卡密", func(t *testing.T) {
		batch, err := svc.ImportCards(ctx, 1, "测试批次", contents)
		require.NoError(t, err)
		assert.NotNil(t, batch)
		assert.Equal(t, uint(1), batch.GoodsID)
		assert.Equal(t, "测试批次", batch.Name)
		assert.Equal(t, 3, batch.TotalCount)
		assert.Equal(t, int8(1), batch.Status)

		// 验证DB中卡密记录
		var cards []Card
		result := db.Where("batch_id = ?", batch.ID).Find(&cards)
		require.NoError(t, result.Error)
		assert.Len(t, cards, 3)
		for i, c := range cards {
			assert.Equal(t, contents[i], c.Content)
			assert.Equal(t, int8(1), c.Status) // 可用
			assert.Equal(t, uint(1), c.GoodsID)
		}
	})

	t.Run("空内容返回错误", func(t *testing.T) {
		_, err := svc.ImportCards(ctx, 1, "空批次", []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "卡密内容不能为空")
	})

	t.Run("空行被过滤", func(t *testing.T) {
		batch, err := svc.ImportCards(ctx, 1, "含空行批次", []string{"VALID-1", "", "  ", "VALID-2"})
		require.NoError(t, err)
		assert.Equal(t, 2, batch.TotalCount)
	})

	t.Run("商品ID为零返回错误", func(t *testing.T) {
		_, err := svc.ImportCards(ctx, 0, "无效批次", []string{"CARD"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "商品ID不能为空")
	})
}

// ---------- TestImportCards_Encrypt ----------

func TestImportCards_Encrypt(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, testEncryptKey) // 启用加密
	ctx := context.Background()

	contents := []string{"SECRET-KEY-001", "SECRET-KEY-002"}

	batch, err := svc.ImportCards(ctx, 1, "加密批次", contents)
	require.NoError(t, err)
	require.NotNil(t, batch)

	// 验证DB中存储的内容带 ENC: 前缀
	var cards []Card
	db.Where("batch_id = ?", batch.ID).Find(&cards)
	require.Len(t, cards, 2)

	for _, c := range cards {
		assert.True(t, strings.HasPrefix(c.Content, "ENC:"),
			"加密后DB中的内容应以 ENC: 开头, got: %s", c.Content)
		assert.NotEqual(t, "SECRET-KEY-001", c.Content)
		assert.NotEqual(t, "SECRET-KEY-002", c.Content)
	}

	// 验证通过 GetOrderCards 读取时能正确解密（先手动绑定到订单）
	orderID := uint(100)
	db.Model(&Card{}).Where("batch_id = ?", batch.ID).Updates(map[string]interface{}{
		"status":   int8(3),
		"order_id": orderID,
	})

	orderCards, err := svc.GetOrderCards(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, orderCards, 2)
	for i, c := range orderCards {
		assert.Equal(t, contents[i], c.Content, "解密后应还原为原始卡密")
	}
}

// ---------- TestIssueCards_Success ----------

func TestIssueCards_Success(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, testEncryptKey)
	ctx := context.Background()

	// 导入3张卡密
	contents := []string{"ISSUE-001", "ISSUE-002", "ISSUE-003"}
	batch, err := svc.ImportCards(ctx, 1, "发卡测试批次", contents)
	require.NoError(t, err)

	// 验证初始可用数量
	count, err := svc.GetAvailableCount(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 发放2张给订单100
	tx := db.Begin()
	require.NoError(t, tx.Error)

	issued, err := svc.IssueCards(ctx, tx, 1, 100, 2)
	require.NoError(t, err)
	assert.Len(t, issued, 2)

	require.NoError(t, tx.Commit().Error)

	// 验证发放的卡密内容已解密
	assert.Equal(t, "ISSUE-001", issued[0].Content)
	assert.Equal(t, "ISSUE-002", issued[1].Content)

	// 验证DB中已发放卡密状态为已用（status=3）
	for _, c := range issued {
		var dbCard Card
		db.First(&dbCard, c.ID)
		assert.Equal(t, int8(3), dbCard.Status, "已发放卡密状态应为3(已用)")
		assert.NotNil(t, dbCard.OrderID)
		assert.Equal(t, uint(100), *dbCard.OrderID)
		assert.NotNil(t, dbCard.UsedAt)
	}

	// 验证剩余可用数量
	count, err = svc.GetAvailableCount(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 验证剩余的那张卡密仍可用
	var remaining Card
	db.Where("batch_id = ? AND status = 1", batch.ID).First(&remaining)
	assert.NotZero(t, remaining.ID, "应剩余1张可用卡密")
}

// ---------- TestIssueCards_InsufficientStock ----------

func TestIssueCards_InsufficientStock(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, nil)
	ctx := context.Background()

	// 导入2张卡密
	_, err := svc.ImportCards(ctx, 1, "少量批次", []string{"FEW-001", "FEW-002"})
	require.NoError(t, err)

	// 尝试发放5张，超出库存
	tx := db.Begin()
	require.NoError(t, tx.Error)

	_, err = svc.IssueCards(ctx, tx, 1, 200, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不足")

	_ = tx.Rollback()

	// 验证卡密未被消耗
	count, err := svc.GetAvailableCount(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "库存不足时不应消耗任何卡密")
}

// ---------- TestGetOrderCards_Decrypt ----------

func TestGetOrderCards_Decrypt(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, testEncryptKey)
	ctx := context.Background()

	// 导入加密卡密
	contents := []string{"DECRYPT-001", "DECRYPT-002"}
	batch, err := svc.ImportCards(ctx, 1, "解密测试批次", contents)
	require.NoError(t, err)

	// 手动绑定到订单200（模拟 IssueCards 的效果）
	orderID := uint(200)
	db.Model(&Card{}).Where("batch_id = ?", batch.ID).Updates(map[string]interface{}{
		"status":   int8(3),
		"order_id": orderID,
	})

	// 获取订单卡密，应自动解密
	cards, err := svc.GetOrderCards(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, cards, 2)

	for i, c := range cards {
		assert.Equal(t, contents[i], c.Content, "应解密为原始卡密内容")
		assert.Equal(t, int8(3), c.Status)
		assert.NotNil(t, c.OrderID)
		assert.Equal(t, orderID, *c.OrderID)
	}
}

// ---------- TestGetOrderCards_LegacyPlaintext ----------

func TestGetOrderCards_LegacyPlaintext(t *testing.T) {
	db := setupCardTestDB(t)
	svc := NewCardService(db, testEncryptKey) // 即使配置了加密密钥
	ctx := context.Background()

	// 手动插入旧明文卡密（无 ENC: 前缀），模拟历史数据
	orderID := uint(300)
	legacyCards := []*Card{
		{BatchID: 1, GoodsID: 1, Content: "LEGACY-PLAIN-001", Status: 3, OrderID: &orderID},
		{BatchID: 1, GoodsID: 1, Content: "LEGACY-PLAIN-002", Status: 3, OrderID: &orderID},
	}
	for _, c := range legacyCards {
		require.NoError(t, db.Create(c).Error)
	}

	// 获取订单卡密，旧的明文内容应原样返回
	cards, err := svc.GetOrderCards(ctx, orderID)
	require.NoError(t, err)
	require.Len(t, cards, 2)

	assert.Equal(t, "LEGACY-PLAIN-001", cards[0].Content)
	assert.Equal(t, "LEGACY-PLAIN-002", cards[1].Content)
}
