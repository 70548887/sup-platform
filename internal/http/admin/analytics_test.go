package admin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDashboard_Success(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	w := env.makeRequest(http.MethodGet, "/admin/analytics/dashboard", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	// 验证返回了四个时间段的统计数据
	assert.Contains(t, data, "today")
	assert.Contains(t, data, "yesterday")
	assert.Contains(t, data, "week")
	assert.Contains(t, data, "month")
}

func TestGetRevenueTrend(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 缺少参数应返回错误
	w := env.makeRequest(http.MethodGet, "/admin/analytics/revenue-trend", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.NotEqual(t, float64(0), resp["code"], "缺少date参数应返回错误")

	// 带正确参数
	w = env.makeRequest(http.MethodGet, "/admin/analytics/revenue-trend?start_date=2026-07-01&end_date=2026-07-05&granularity=day", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	resp = parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Contains(t, data, "list")
	assert.Equal(t, "day", data["granularity"])
	assert.Equal(t, "2026-07-01", data["start_date"])
	assert.Equal(t, "2026-07-05", data["end_date"])
}
