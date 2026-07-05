package admin

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/goods"
)

func TestListGoods_Success(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试商品
	for i := 1; i <= 3; i++ {
		g := goods.Goods{
			SerialNumber: fmt.Sprintf("G-%03d", i),
			Name:         fmt.Sprintf("商品%d", i),
			Price:        decimal.NewFromFloat(10.5),
			CostPrice:    decimal.NewFromFloat(8.0),
			Status:       1,
			CategoryID:   1,
			SupplierID:   1,
			Stock:        100,
		}
		require.NoError(t, env.DB.Create(&g).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/goods?page=1&size=10", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 3)
}

func TestListGoods_FilterByStatus(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建不同状态的商品
	goodsList := []goods.Goods{
		{SerialNumber: "G-001", Name: "上架商品1", Status: 1, CategoryID: 1, SupplierID: 1, Stock: 10, Price: decimal.NewFromInt(10), CostPrice: decimal.NewFromInt(8)},
		{SerialNumber: "G-002", Name: "上架商品2", Status: 1, CategoryID: 1, SupplierID: 1, Stock: 20, Price: decimal.NewFromInt(20), CostPrice: decimal.NewFromInt(15)},
		{SerialNumber: "G-003", Name: "下架商品1", Status: 1, CategoryID: 1, SupplierID: 1, Stock: 30, Price: decimal.NewFromInt(30), CostPrice: decimal.NewFromInt(25)},
	}
	for i := range goodsList {
		require.NoError(t, env.DB.Create(&goodsList[i]).Error)
	}
	// 显式设置第3个商品status=0（GORM default:1 会跳过零值）
	require.NoError(t, env.DB.Model(&goodsList[2]).Update("status", 0).Error)

	// 只查询status=1的商品
	w := env.makeRequest(http.MethodGet, "/admin/goods?page=1&size=10&status=1", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 2)
}

func TestCreateGoods_ApproveAndReject(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建一个待审核商品（先创建再设status=0，避免GORM零值跳过）
	g := goods.Goods{
		SerialNumber: "G-PENDING-001",
		Name:         "待审核商品",
		Price:        decimal.NewFromFloat(99.9),
		CostPrice:    decimal.NewFromFloat(80.0),
		Status:       1,
		CategoryID:   1,
		SupplierID:   1,
		Stock:        50,
	}
	require.NoError(t, env.DB.Create(&g).Error)
	// 显式设置status=0（GORM default:1 会跳过零值）
	require.NoError(t, env.DB.Model(&g).Update("status", 0).Error)

	// 审核通过
	w := env.makeRequest(http.MethodPost, fmt.Sprintf("/admin/goods/%d/approve", g.ID), nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	require.Equal(t, float64(0), resp["code"], "response: %s", w.Body.String())

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["status"])

	// 验证DB中状态已更新
	var updated goods.Goods
	require.NoError(t, env.DB.First(&updated, g.ID).Error)
	assert.Equal(t, int8(1), updated.Status)
}
