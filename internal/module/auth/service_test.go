package auth

import (
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/pkg/signature"
)

// setupAuthTestDB 创建隔离SQLite内存数据库，迁移auth表
func setupAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 迁移auth模块的ApiApp表
	err = db.AutoMigrate(&ApiApp{})
	require.NoError(t, err, "failed to migrate auth tables")

	return db
}

// newAuthTestService 创建测试用的AuthService
func newAuthTestService(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	db := setupAuthTestDB(t)
	svc := NewAuthService(db, "test-jwt-secret-key", 24)
	return svc, db
}

// ---------- TestGenerateJWT ----------

func TestGenerateJWT(t *testing.T) {
	svc, _ := newAuthTestService(t)

	userID := uint(42)
	role := "admin"
	tenantID := uint(0)

	token, err := svc.GenerateJWT(userID, role, tenantID)
	require.NoError(t, err)
	assert.NotEmpty(t, token, "JWT token不应为空")

	// 验证token
	claims, err := svc.VerifyJWT(token)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// 验证claims字段
	assert.Equal(t, userID, claims.UserID, "UserID应匹配")
	assert.Equal(t, role, claims.Role, "Role应匹配")
	assert.Equal(t, tenantID, claims.TenantID, "TenantID应匹配")
	assert.Equal(t, "sup-platform", claims.Issuer, "Issuer应为sup-platform")

	// 验证时间字段
	assert.NotNil(t, claims.ExpiresAt, "ExpiresAt不应为nil")
	assert.NotNil(t, claims.IssuedAt, "IssuedAt不应为nil")
	assert.NotNil(t, claims.NotBefore, "NotBefore不应为nil")

	// ExpiresAt应在未来（24小时后）
	assert.True(t, claims.ExpiresAt.Time.After(time.Now()), "token应未过期")

	// IssuedAt应在当前时间附近
	assert.True(t, claims.IssuedAt.Time.Before(time.Now().Add(5*time.Second)),
		"IssuedAt应接近当前时间")
}

// ---------- TestValidateJWT_Expired ----------

func TestValidateJWT_Expired(t *testing.T) {
	db := setupAuthTestDB(t)

	// 创建过期配置的AuthService（jwtExpireHours=-1，token生成时即已过期1小时）
	svc := NewAuthService(db, "test-jwt-secret-key", -1)

	userID := uint(1)
	role := "admin"
	tenantID := uint(0)

	token, err := svc.GenerateJWT(userID, role, tenantID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// 验证过期token应失败
	claims, err := svc.VerifyJWT(token)
	require.Error(t, err, "过期token应验证失败")
	assert.Nil(t, claims, "验证失败时claims应为nil")
	assert.Contains(t, err.Error(), "JWT验证失败")

	// 用正常配置的service也验证不了（不同secret但这里相同）
	normalSvc := NewAuthService(db, "test-jwt-secret-key", 24)
	_, err = normalSvc.VerifyJWT(token)
	require.Error(t, err, "过期token即使换service也应验证失败")
}

// ---------- TestCreateAPIApp ----------

func TestCreateAPIApp(t *testing.T) {
	svc, db := newAuthTestService(t)

	userID := uint(10)
	appName := "test-api-app"

	appId, appSecret, err := svc.GenerateApiCredentials(userID, appName)
	require.NoError(t, err)

	// AppId应为16字符hex（8字节）
	assert.Len(t, appId, 16, "AppId应为16字符hex")
	assert.True(t, isHexString(appId), "AppId应为hex字符串")

	// AppSecret应为32字符hex（16字节）
	assert.Len(t, appSecret, 32, "AppSecret应为32字符hex")
	assert.True(t, isHexString(appSecret), "AppSecret应为hex字符串")

	// AppId和AppSecret不应相同
	assert.NotEqual(t, appId, appSecret, "AppId和AppSecret不应相同")

	// 验证已持久化到数据库
	var app ApiApp
	err = db.Where("app_id = ?", appId).First(&app).Error
	require.NoError(t, err)

	assert.Equal(t, userID, app.UserID)
	assert.Equal(t, appId, app.AppId)
	assert.Equal(t, appSecret, app.AppSecret)
	assert.Equal(t, appName, app.AppName)
	assert.Equal(t, 1, app.Status, "新创建的应用状态应为1(启用)")

	// 通过VerifyApiApp验证
	appInfo, err := svc.VerifyApiApp(appId)
	require.NoError(t, err)
	assert.Equal(t, appId, appInfo.AppId)
	assert.Equal(t, appSecret, appInfo.AppSecret)
	assert.Equal(t, userID, appInfo.UserID)
	assert.Equal(t, 1, appInfo.Status)

	// 多次创建AppId不同
	appId2, _, err := svc.GenerateApiCredentials(userID, "second-app")
	require.NoError(t, err)
	assert.NotEqual(t, appId, appId2, "两次生成的AppId应不同")
}

// ---------- TestVerifySignature ----------

func TestVerifySignature(t *testing.T) {
	svc, _ := newAuthTestService(t)

	// 1. 创建API应用
	userID := uint(5)
	appId, appSecret, err := svc.GenerateApiCredentials(userID, "sig-test-app")
	require.NoError(t, err)

	// 2. 通过VerifyApiApp获取应用信息（模拟中间件流程）
	appInfo, err := svc.VerifyApiApp(appId)
	require.NoError(t, err)
	require.NotNil(t, appInfo)

	// AppSecret应与生成时一致
	assert.Equal(t, appSecret, appInfo.AppSecret)

	// 3. 计算正确的Legacy签名
	requestURI := "/api/v1/orders"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	correctToken := signature.LegacySHA1(appId, appInfo.AppSecret, requestURI, timestamp)

	// 4. 正确签名验证通过
	ok := signature.VerifyLegacy(appId, appInfo.AppSecret, requestURI, timestamp, correctToken)
	assert.True(t, ok, "正确签名应验证通过")

	// 5. 错误签名验证失败
	wrongToken := "0123456789abcdef0123456789abcdef01234567"
	ok = signature.VerifyLegacy(appId, appInfo.AppSecret, requestURI, timestamp, wrongToken)
	assert.False(t, ok, "错误签名应验证失败")

	// 6. 篡改请求URI验证失败
	ok = signature.VerifyLegacy(appId, appInfo.AppSecret, "/api/v1/different", timestamp, correctToken)
	assert.False(t, ok, "篡改URI后签名应验证失败")

	// 7. 篡改时间戳验证失败
	ok = signature.VerifyLegacy(appId, appInfo.AppSecret, requestURI, "1700000000", correctToken)
	assert.False(t, ok, "篡改时间戳后签名应验证失败")

	// 8. 使用错误AppSecret验证失败
	ok = signature.VerifyLegacy(appId, "wrongsecret", requestURI, timestamp, correctToken)
	assert.False(t, ok, "错误AppSecret应验证失败")

	// 9. 带Nonce的Legacy签名验证
	nonce := "random-nonce-123"
	correctTokenWithNonce := signature.LegacySHA1WithNonce(appId, appInfo.AppSecret, requestURI, timestamp, nonce)
	ok = signature.VerifyLegacyWithNonce(appId, appInfo.AppSecret, requestURI, timestamp, nonce, correctTokenWithNonce)
	assert.True(t, ok, "带Nonce的正确签名应验证通过")

	// 带Nonce的错误签名验证失败
	ok = signature.VerifyLegacyWithNonce(appId, appInfo.AppSecret, requestURI, timestamp, "wrong-nonce", correctTokenWithNonce)
	assert.False(t, ok, "带Nonce的错误签名应验证失败")
}

// isHexString 检查字符串是否为纯hex字符
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
