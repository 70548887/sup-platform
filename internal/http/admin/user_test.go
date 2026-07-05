package admin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/70548887/sup-platform/internal/module/account"
)

func TestListUsers_Success(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	// 创建几个测试用户
	users := []account.User{
		{Username: "user1", Password: "hash1", Role: "admin", Status: 1},
		{Username: "user2", Password: "hash2", Role: "supplier", Status: 1},
		{Username: "user3", Password: "hash3", Role: "customer", Status: 1},
	}
	for i := range users {
		require.NoError(t, env.DB.Create(&users[i]).Error)
	}

	w := env.makeRequest(http.MethodGet, "/admin/users?page=1&size=10", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total"])

	list := data["list"].([]interface{})
	assert.Len(t, list, 3)
}

func TestListUsers_Unauthorized(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	w := env.makeRequestNoAuth(http.MethodGet, "/admin/users", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateUser_Success(t *testing.T) {
	t.Parallel()
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"username": "newuser",
		"password": "password123",
		"nickname": "New User",
		"email":    "new@example.com",
		"phone":    "13800138000",
		"role":     "supplier",
	}

	w := env.makeRequest(http.MethodPost, "/admin/users", body)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "newuser", data["username"])
	assert.Equal(t, "supplier", data["role"])
	assert.NotZero(t, data["id"])

	// 验证DB中已创建
	var user account.User
	err := env.DB.Where("username = ?", "newuser").First(&user).Error
	require.NoError(t, err)
	assert.Equal(t, "supplier", user.Role)
	assert.Equal(t, int8(1), user.Status)
}
