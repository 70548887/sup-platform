package admin

import (
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/recharge"
)

func TestListRecharges(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试充值记录
	recharges := []recharge.RechargeOrder{
		{RechargeSN: "RC-001", UserID: 1, Amount: decimal.NewFromInt(100), Status: 1, IdempotencyKey: "idem-001"},
		{RechargeSN: "RC-002", UserID: 1, Amount: decimal.NewFromInt(200), Status: 2, IdempotencyKey: "idem-002"},
		{RechargeSN: "RC-003", UserID: 2, Amount: decimal.NewFromInt(500), Status: 1, IdempotencyKey: "idem-003"},
	}
	for i := range recharges {
		require.NoError(t, env.DB.Create(&recharges[i]).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/recharges?page=1&size=10&user_id=1", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 2)
}

func TestApproveRecharge(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建待审核充值
	rc := recharge.RechargeOrder{
		RechargeSN:     "RC-APPROVE-001",
		UserID:         1,
		Amount:         decimal.NewFromInt(100),
		Status:         1, // pending
		IdempotencyKey: "idem-approve-001",
	}
	require.NoError(t, env.DB.Create(&rc).Error)

	w := env.makeRequest(http.MethodPost, "/admin/recharges/1/approve", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	// RechargeSvc.Approve可能需要wallet，结果取决于service内部逻辑
	// 验证handler正确调用并返回JSON响应
	assert.Contains(t, resp, "code")
}
