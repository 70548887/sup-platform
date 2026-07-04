package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/module/billing"
	"github.com/70548887/sup-platform/internal/module/tenant"
	"github.com/70548887/sup-platform/migrations"
)

func main() {
	log.Println("[init-tenant] 开始初始化...")

	// 1. 连接数据库
	db, err := connectDB()
	if err != nil {
		log.Fatalf("[init-tenant] 数据库连接失败: %v", err)
	}

	// 2. 执行核心模块迁移
	log.Println("[init-tenant] 执行数据库迁移...")
	if err := migrations.RunAll(db); err != nil {
		log.Fatalf("[init-tenant] 核心迁移失败: %v", err)
	}
	// 执行租户和计费模块迁移
	if err := tenant.Migrate(db); err != nil {
		log.Fatalf("[init-tenant] 租户模块迁移失败: %v", err)
	}
	if err := billing.Migrate(db); err != nil {
		log.Fatalf("[init-tenant] 计费模块迁移失败: %v", err)
	}
	log.Println("[init-tenant] 迁移完成")

	// 3. 检查是否已初始化（Tenant表有数据则跳过）
	var count int64
	db.Model(&tenant.Tenant{}).Count(&count)
	if count > 0 {
		log.Println("[init-tenant] 已初始化，跳过")
		return
	}

	// 4. 创建管理员用户
	username := getEnv("ADMIN_USERNAME", "admin")
	password := getEnv("ADMIN_PASSWORD", "admin123")

	hashedPwd, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("[init-tenant] 密码加密失败: %v", err)
	}

	adminUser := &account.User{
		TenantID: 1,
		Username: username,
		Password: hashedPwd,
		Nickname: "超级管理员",
		Role:     "admin",
		Status:   1,
	}
	if err := db.Create(adminUser).Error; err != nil {
		log.Fatalf("[init-tenant] 创建管理员失败: %v", err)
	}
	log.Printf("[init-tenant] 管理员创建成功: ID=%d, Username=%s", adminUser.ID, adminUser.Username)

	// 5. 创建默认租户（自动将admin添加为boss角色）
	tenantName := getEnv("INIT_TENANT_NAME", "默认租户")
	ctx := context.Background()
	tenantSvc := tenant.NewTenantService(db)

	t, err := tenantSvc.CreateTenant(ctx, tenantName, "default", adminUser.ID, "private")
	if err != nil {
		log.Fatalf("[init-tenant] 创建租户失败: %v", err)
	}
	log.Printf("[init-tenant] 租户创建成功: ID=%d, Name=%s", t.ID, t.Name)

	// 更新用户的TenantID为实际租户ID
	db.Model(adminUser).Update("tenant_id", t.ID)

	// 6. 初始化计费套餐
	billingSvc := billing.NewBillingService(db, nil, "")
	if err := billingSvc.InitDefaultPlans(ctx); err != nil {
		log.Fatalf("[init-tenant] 初始化套餐失败: %v", err)
	}
	log.Println("[init-tenant] 默认套餐初始化完成")

	// 7. 为租户创建企业版订阅（无限制配额）
	plans, err := billingSvc.ListPlans(ctx)
	if err != nil {
		log.Fatalf("[init-tenant] 获取套餐列表失败: %v", err)
	}
	var enterprisePlanID uint
	for _, p := range plans {
		if p.Name == "enterprise" {
			enterprisePlanID = p.ID
			break
		}
	}
	if enterprisePlanID == 0 {
		log.Println("[init-tenant] [WARN] 未找到企业版套餐，跳过订阅创建")
	} else {
		_, err := billingSvc.CreateSubscription(ctx, t.ID, enterprisePlanID)
		if err != nil {
			log.Fatalf("[init-tenant] 创建订阅失败: %v", err)
		}
		log.Println("[init-tenant] 企业版订阅创建成功")
	}

	// 8. 输出初始化结果
	log.Println("[init-tenant] ===== 初始化完成 =====")
	fmt.Printf("  租户: %s (ID=%d)\n", t.Name, t.ID)
	fmt.Printf("  管理员: %s\n", username)
	fmt.Printf("  密码: %s\n", password)
	fmt.Println("  请及时修改默认密码！")
}

func connectDB() (*gorm.DB, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnvInt("DB_PORT", 3306)
	user := getEnv("DB_USER", "root")
	password := getEnv("DB_PASSWORD", "")
	dbname := getEnv("DB_NAME", "sup_platform")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname,
	)

	var db *gorm.DB
	var err error

	// 重试连接（等待MySQL启动）
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, _ := db.DB()
			if pingErr := sqlDB.Ping(); pingErr == nil {
				return db, nil
			}
		}
		log.Printf("[init-tenant] 等待数据库就绪... (%d/30)", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("数据库连接超时: %v", err)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
