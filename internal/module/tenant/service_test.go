package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTenantTestDB 创建隔离的SQLite内存数据库并迁移租户相关表
func setupTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	// SQLite内存模式必须限制为单连接，否则每个连接各自独立数据库
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = Migrate(db)
	require.NoError(t, err, "failed to migrate tenant tables")

	return db
}

// newTenantTestService 创建测试用的TenantService
func newTenantTestService(t *testing.T) (*TenantService, *gorm.DB) {
	t.Helper()
	db := setupTenantTestDB(t)
	svc := NewTenantService(db)
	return svc, db
}

var testTenantSeq uint32

// createTestTenant 辅助函数：创建一个测试租户并返回
func createTestTenant(t *testing.T, svc *TenantService) *Tenant {
	t.Helper()
	ctx := context.Background()
	seq := atomic.AddUint32(&testTenantSeq, 1)
	domain := fmt.Sprintf("test-%d.example.com", seq)
	tenant, err := svc.CreateTenant(ctx, "测试租户", domain, 100, "saas")
	require.NoError(t, err)
	require.NotNil(t, tenant)
	return tenant
}

// featureEnabled 从Features JSON中读取指定布尔开关
func featureEnabled(t *testing.T, featuresJSON, feature string) bool {
	t.Helper()
	if featuresJSON == "" {
		return false
	}
	var features map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(featuresJSON), &features))
	v, ok := features[feature]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	require.True(t, ok, "feature %s is not bool", feature)
	return b
}

// ---------- TestCreateTenant_Success ----------

func TestCreateTenant_Success(t *testing.T) {
	svc, db := newTenantTestService(t)
	ctx := context.Background()

	ownerUserID := uint(100)
	tenant, err := svc.CreateTenant(ctx, "测试租户", "create.example.com", ownerUserID, "saas")
	require.NoError(t, err)
	require.NotNil(t, tenant)

	assert.NotZero(t, tenant.ID)
	assert.Equal(t, "测试租户", tenant.Name)
	assert.Equal(t, "create.example.com", tenant.Domain)
	assert.Equal(t, "saas", tenant.Type)
	assert.Equal(t, ownerUserID, tenant.OwnerUserID)
	assert.Equal(t, int8(1), tenant.Status)
	assert.Equal(t, 5, tenant.MaxAdmins)

	// 验证owner被自动设置为boss管理员
	var admin TenantAdmin
	err = db.Where("tenant_id = ? AND user_id = ?", tenant.ID, ownerUserID).First(&admin).Error
	require.NoError(t, err)
	assert.Equal(t, AdminRoleBoss, admin.AdminRole)
	assert.Equal(t, int8(1), admin.Status)
}

// ---------- TestGetTenantByID ----------

func TestGetTenantByID(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	t.Run("存在的租户", func(t *testing.T) {
		tenant := createTestTenant(t, svc)

		got, err := svc.GetTenant(ctx, tenant.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, tenant.ID, got.ID)
		assert.Equal(t, tenant.Name, got.Name)
		assert.Equal(t, tenant.Domain, got.Domain)
	})

	t.Run("不存在的租户", func(t *testing.T) {
		got, err := svc.GetTenant(ctx, 9999)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrTenantNotFound))
		assert.Nil(t, got)
	})
}

// ---------- TestListAdmins ----------

func TestListAdmins(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)

	// 添加其他角色的管理员
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, 101, AdminRoleFinance, `[]`))
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, 102, AdminRoleOps, `[]`))
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, 103, AdminRoleSupport, `[]`))

	admins, err := svc.ListAdmins(ctx, tenant.ID)
	require.NoError(t, err)
	require.Len(t, admins, 4)

	roleCount := map[string]int{}
	for _, admin := range admins {
		roleCount[admin.AdminRole]++
	}
	assert.Equal(t, 1, roleCount[AdminRoleBoss])
	assert.Equal(t, 1, roleCount[AdminRoleFinance])
	assert.Equal(t, 1, roleCount[AdminRoleOps])
	assert.Equal(t, 1, roleCount[AdminRoleSupport])
}

// ---------- TestCheckPermission_Boss ----------

func TestCheckPermission_Boss(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)

	cases := []struct {
		resource string
		action   string
	}{
		{"billing", "POST"},
		{"orders", "DELETE"},
		{"goods", "GET"},
		{"system", "PATCH"},
	}

	for _, c := range cases {
		got := svc.CheckAdminPermission(ctx, tenant.ID, tenant.OwnerUserID, c.resource, c.action)
		assert.True(t, got, "boss should have all permissions on %s %s", c.resource, c.action)
	}
}

// ---------- TestCheckPermission_Finance ----------

func TestCheckPermission_Finance(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)
	financeUserID := uint(201)
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, financeUserID, AdminRoleFinance, `[]`))

	allowed := []string{"billing", "analytics", "ledger", "recharge", "BILLING_REPORTS"}
	for _, res := range allowed {
		assert.True(t, svc.CheckAdminPermission(ctx, tenant.ID, financeUserID, res, "POST"),
			"finance should allow %s", res)
	}

	denied := []string{"orders", "goods", "docking", "users"}
	for _, res := range denied {
		assert.False(t, svc.CheckAdminPermission(ctx, tenant.ID, financeUserID, res, "GET"),
			"finance should deny %s", res)
	}
}

// ---------- TestCheckPermission_Ops ----------

func TestCheckPermission_Ops(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)
	opsUserID := uint(202)
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, opsUserID, AdminRoleOps, `[]`))

	allowed := []string{"orders", "goods", "docking", "pricing", "ORDER_LIST"}
	for _, res := range allowed {
		assert.True(t, svc.CheckAdminPermission(ctx, tenant.ID, opsUserID, res, "POST"),
			"ops should allow %s", res)
	}

	denied := []string{"billing", "ledger", "analytics", "users"}
	for _, res := range denied {
		assert.False(t, svc.CheckAdminPermission(ctx, tenant.ID, opsUserID, res, "GET"),
			"ops should deny %s", res)
	}
}

// ---------- TestCheckPermission_Support ----------

func TestCheckPermission_Support(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)
	supportUserID := uint(203)
	require.NoError(t, svc.AddAdmin(ctx, tenant.ID, supportUserID, AdminRoleSupport, `[]`))

	// support 只读权限：任何资源的 GET 都允许
	assert.True(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "billing", "GET"))
	assert.True(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "orders", "get"))
	assert.True(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "system", "Get"))

	// 非 GET 操作一律拒绝
	assert.False(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "billing", "POST"))
	assert.False(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "orders", "DELETE"))
	assert.False(t, svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "goods", "PATCH"))
}

// ---------- TestCheckPermission_Denied ----------

func TestCheckPermission_Denied(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	tenant := createTestTenant(t, svc)

	t.Run("不存在的管理员", func(t *testing.T) {
		got := svc.CheckAdminPermission(ctx, tenant.ID, 9999, "billing", "GET")
		assert.False(t, got)
	})

	t.Run("finance越权访问ops资源", func(t *testing.T) {
		financeUserID := uint(301)
		require.NoError(t, svc.AddAdmin(ctx, tenant.ID, financeUserID, AdminRoleFinance, `[]`))
		got := svc.CheckAdminPermission(ctx, tenant.ID, financeUserID, "orders", "GET")
		assert.False(t, got)
	})

	t.Run("support尝试写操作", func(t *testing.T) {
		supportUserID := uint(302)
		require.NoError(t, svc.AddAdmin(ctx, tenant.ID, supportUserID, AdminRoleSupport, `[]`))
		got := svc.CheckAdminPermission(ctx, tenant.ID, supportUserID, "billing", "POST")
		assert.False(t, got)
	})
}

// ---------- TestToggleFeature ----------

func TestToggleFeature(t *testing.T) {
	svc, _ := newTenantTestService(t)
	ctx := context.Background()

	t.Run("开关切换", func(t *testing.T) {
		tenant := createTestTenant(t, svc)

		// 首次开启
		updated, err := svc.ToggleFeature(ctx, tenant.ID, "api_access")
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.True(t, featureEnabled(t, updated.Features, "api_access"))

		// 从数据库再查一次验证已持久化
		got, err := svc.GetTenant(ctx, tenant.ID)
		require.NoError(t, err)
		assert.True(t, featureEnabled(t, got.Features, "api_access"))

		// 再次切换关闭
		updated, err = svc.ToggleFeature(ctx, tenant.ID, "api_access")
		require.NoError(t, err)
		assert.False(t, featureEnabled(t, updated.Features, "api_access"))
	})

	t.Run("多个功能开关互不干扰", func(t *testing.T) {
		tenant := createTestTenant(t, svc)

		_, err := svc.ToggleFeature(ctx, tenant.ID, "feature_a")
		require.NoError(t, err)
		_, err = svc.ToggleFeature(ctx, tenant.ID, "feature_b")
		require.NoError(t, err)

		got, err := svc.GetTenant(ctx, tenant.ID)
		require.NoError(t, err)
		assert.True(t, featureEnabled(t, got.Features, "feature_a"))
		assert.True(t, featureEnabled(t, got.Features, "feature_b"))
	})

	t.Run("租户不存在", func(t *testing.T) {
		updated, err := svc.ToggleFeature(ctx, 9999, "api_access")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrTenantNotFound))
		assert.Nil(t, updated)
	})
}
