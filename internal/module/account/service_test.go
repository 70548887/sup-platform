package account

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupAccountTestDB 创建隔离SQLite内存数据库，迁移account表
func setupAccountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = AutoMigrate(db)
	require.NoError(t, err, "failed to migrate account tables")

	return db
}

// newAccountTestService 创建测试用的AccountService
func newAccountTestService(t *testing.T) (*AccountService, *gorm.DB) {
	t.Helper()
	db := setupAccountTestDB(t)
	svc := NewAccountService(db)
	return svc, db
}

// ---------- TestRegister_Success ----------

func TestRegister_Success(t *testing.T) {
	svc, db := newAccountTestService(t)

	user, err := svc.Register("testuser", "P@ssw0rd!", "customer")
	require.NoError(t, err)
	require.NotNil(t, user)

	// 验证返回的用户字段
	assert.NotZero(t, user.ID, "用户ID应已生成")
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "customer", user.Role)
	assert.Equal(t, int8(1), user.Status, "新用户状态应为1(活跃)")

	// 密码应是bcrypt哈希，不是明文
	assert.NotEqual(t, "P@ssw0rd!", user.Password, "密码不应是明文")
	assert.True(t, len(user.Password) > 0, "密码哈希不应为空")

	// 验证已持久化到数据库
	var dbUser User
	err = db.First(&dbUser, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "testuser", dbUser.Username)
	assert.Equal(t, user.Password, dbUser.Password, "数据库中的密码哈希应一致")
}

// ---------- TestRegister_DuplicateUsername ----------

func TestRegister_DuplicateUsername(t *testing.T) {
	svc, _ := newAccountTestService(t)

	// 第一次注册成功
	_, err := svc.Register("dupuser", "password1", "customer")
	require.NoError(t, err)

	// 第二次注册相同用户名应失败
	_, err = svc.Register("dupuser", "password2", "supplier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名已存在")
}

// ---------- TestLogin_Success ----------

func TestLogin_Success(t *testing.T) {
	svc, _ := newAccountTestService(t)

	// 先注册用户
	_, err := svc.Register("loginuser", "correctpass", "admin")
	require.NoError(t, err)

	// 使用正确密码登录
	user, err := svc.Login("loginuser", "correctpass")
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Equal(t, "loginuser", user.Username)
	assert.Equal(t, "admin", user.Role)
	assert.Equal(t, int8(1), user.Status)
}

// ---------- TestLogin_WrongPassword ----------

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newAccountTestService(t)

	// 先注册用户
	_, err := svc.Register("loginuser2", "correctpass", "admin")
	require.NoError(t, err)

	// 使用错误密码登录应失败
	_, err = svc.Login("loginuser2", "wrongpassword")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名或密码错误")

	// 不存在的用户名也应失败
	_, err = svc.Login("nonexistent", "anypassword")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户名或密码错误")
}

// ---------- TestPasswordHash ----------

func TestPasswordHash(t *testing.T) {
	password := "mySecret123"

	// 相同密码哈希两次
	hash1, err := HashPassword(password)
	require.NoError(t, err)

	hash2, err := HashPassword(password)
	require.NoError(t, err)

	// 1. 哈希不可逆：哈希值不应等于明文
	assert.NotEqual(t, password, hash1, "哈希值不应等于明文密码")
	assert.NotEqual(t, password, hash2, "哈希值不应等于明文密码")

	// 2. 相同密码每次生成的哈希不同（bcrypt随机盐）
	assert.NotEqual(t, hash1, hash2, "相同密码的两次哈希应不同（bcrypt随机盐）")

	// 3. 两个哈希都能正确验证原密码
	assert.True(t, VerifyPassword(hash1, password), "hash1应能验证原密码")
	assert.True(t, VerifyPassword(hash2, password), "hash2应能验证原密码")

	// 4. 错误密码验证失败
	assert.False(t, VerifyPassword(hash1, "wrongpassword"), "错误密码应验证失败")
	assert.False(t, VerifyPassword(hash1, ""), "空密码应验证失败")
}
