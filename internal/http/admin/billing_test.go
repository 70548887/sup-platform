package admin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/billing"
)

func TestListBillingPlans(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试套餐
	plans := []billing.SubscriptionPlan{
		{Name: "basic", DisplayName: "基础版", Status: 1},
		{Name: "pro", DisplayName: "专业版", Status: 1},
	}
	for i := range plans {
		require.NoError(t, env.DB.Create(&plans[i]).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/billing/plans", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	list := data["list"].([]interface{})
	assert.Len(t, list, 2)
}

func TestListBillingInvoices(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建测试账单
	invoices := []billing.Invoice{
		{TenantID: 1, Month: "2026-06", Status: "pending"},
		{TenantID: 1, Month: "2026-05", Status: "paid"},
		{TenantID: 2, Month: "2026-06", Status: "pending"},
	}
	for i := range invoices {
		require.NoError(t, env.DB.Create(&invoices[i]).Error)
	}

	// 查询所有
	w := env.makeRequest(http.MethodGet, "/admin/billing/invoices?page=1&size=10", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total"])

	// 按tenant_id过滤
	w = env.makeRequest(http.MethodGet, "/admin/billing/invoices?tenant_id=1", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp = parseResponse(t, w)
	data = resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	// 按status过滤
	w = env.makeRequest(http.MethodGet, "/admin/billing/invoices?status=pending", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp = parseResponse(t, w)
	data = resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])
}
