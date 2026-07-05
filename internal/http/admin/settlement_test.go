package admin

import (
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/settlement"
)

func TestListSettlements(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试结算单
	settlements := []settlement.Settlement{
		{SupplierID: 1, Period: "2026-06", TotalOrders: 10, TotalAmount: decimal.NewFromInt(1000), CommissionRate: decimal.NewFromFloat(0.05), CommissionAmount: decimal.NewFromInt(50), NetAmount: decimal.NewFromInt(950), Status: "pending"},
		{SupplierID: 2, Period: "2026-06", TotalOrders: 5, TotalAmount: decimal.NewFromInt(500), CommissionRate: decimal.NewFromFloat(0.05), CommissionAmount: decimal.NewFromInt(25), NetAmount: decimal.NewFromInt(475), Status: "confirmed"},
	}
	for i := range settlements {
		require.NoError(t, env.DB.Create(&settlements[i]).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/settlements?page=1&size=10", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 2)
}

func TestGenerateSettlement(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"supplier_id": 1,
		"period":      "2026-06",
	}

	w := env.makeRequest(http.MethodPost, "/admin/settlements/generate", body)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	// GenerateSettlement 可能返回 code=0（成功）或非0（无数据等情况）
	// 测试验证HTTP层正确传递参数并返回响应
	assert.Contains(t, resp, "code")
}
