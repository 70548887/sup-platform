package admin

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/order"
)

func TestListOrders_Success(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建供货商用户（为了 supplierNames 查询）
	supplier := account.User{Username: "supplier1", Password: "hash", Nickname: "供货商A", Role: "supplier", Status: 1}
	require.NoError(t, env.DB.Create(&supplier).Error)

	// 创建测试订单
	for i := 1; i <= 3; i++ {
		o := order.Order{
			OrderSN:    fmt.Sprintf("ORD-%03d", i),
			CustomerID: 1,
			SupplierID: supplier.ID,
			GoodsID:    uint(i),
			GoodsSN:    fmt.Sprintf("G-%03d", i),
			GoodsName:  fmt.Sprintf("商品%d", i),
			BuyNumber:  1,
			UnitPrice:  decimal.NewFromInt(100),
			Amount:     decimal.NewFromInt(100),
			Status:     1,
		}
		require.NoError(t, env.DB.Create(&o).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/orders?page=1&size=10", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 3)

	// 验证supplierName被正确填充
	firstOrder := list[0].(map[string]interface{})
	assert.Equal(t, "供货商A", firstOrder["supplier_name"])
}

func TestListOrders_WithFilter(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建不同状态的订单
	orders := []order.Order{
		{OrderSN: "ORD-001", CustomerID: 1, SupplierID: 1, GoodsID: 1, GoodsSN: "G1", GoodsName: "商品1", BuyNumber: 1, UnitPrice: decimal.NewFromInt(10), Amount: decimal.NewFromInt(10), Status: 1},
		{OrderSN: "ORD-002", CustomerID: 1, SupplierID: 1, GoodsID: 2, GoodsSN: "G2", GoodsName: "商品2", BuyNumber: 1, UnitPrice: decimal.NewFromInt(20), Amount: decimal.NewFromInt(20), Status: 2},
		{OrderSN: "ORD-003", CustomerID: 2, SupplierID: 1, GoodsID: 3, GoodsSN: "G3", GoodsName: "商品3", BuyNumber: 1, UnitPrice: decimal.NewFromInt(30), Amount: decimal.NewFromInt(30), Status: 1},
	}
	for i := range orders {
		require.NoError(t, env.DB.Create(&orders[i]).Error)
	}

	// 按status=1过滤
	w := env.makeRequest(http.MethodGet, "/admin/orders?status=1", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	// 按customer_id=2过滤
	w = env.makeRequest(http.MethodGet, "/admin/orders?customer_id=2", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp = parseResponse(t, w)
	data = resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])
}

func TestGetOrderDetail(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试订单
	o := order.Order{
		OrderSN:         "ORD-DETAIL-001",
		CustomerOrderID: "CUST-001",
		CustomerID:      1,
		SupplierID:      2,
		GoodsID:         10,
		GoodsSN:         "G-010",
		GoodsName:       "详情测试商品",
		BuyNumber:       3,
		UnitPrice:       decimal.NewFromFloat(50.5),
		Amount:          decimal.NewFromFloat(151.5),
		Status:          1,
	}
	require.NoError(t, env.DB.Create(&o).Error)

	w := env.makeRequest(http.MethodGet, fmt.Sprintf("/admin/orders/%d", o.ID), nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "ORD-DETAIL-001", data["order_sn"])
	assert.Equal(t, "CUST-001", data["customer_order_id"])
	assert.Equal(t, "详情测试商品", data["goods_name"])
	assert.Equal(t, float64(3), data["buy_number"])
	assert.Equal(t, "50.50", data["unit_price"])
	assert.Equal(t, "151.50", data["amount"])
}
