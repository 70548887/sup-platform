package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/module/account"
)

func main() {
	host := envOr("DB_HOST", "localhost")
	port := envOr("DB_PORT", "3306")
	user := envOr("DB_USER", "root")
	pass := envOr("DB_PASSWORD", "root")
	dbname := envOr("DB_NAME", "sup_platform")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, pass, host, port, dbname)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	db.AutoMigrate(&account.User{})

	// 创建管理员
	createUser(db, "admin", "admin123456", "admin")
	// 创建供货商
	createUser(db, "supplier1", "123456", "supplier")
	// 创建客户
	createUser(db, "customer1", "123456", "customer")

	fmt.Println("\n=== 账号创建完成 ===")
	fmt.Println("管理端:   admin / admin123456")
	fmt.Println("供货商端: supplier1 / 123456")
	fmt.Println("客户端:   customer1 / 123456")
}

func createUser(db *gorm.DB, username, password, role string) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &account.User{
		Username: username,
		Password: string(hash),
		Role:     role,
		Status:   1,
		TenantID: 1,
	}

	var existing account.User
	if db.Where("username = ?", username).First(&existing).Error == nil {
		fmt.Printf("[跳过] 用户 %s 已存在 (ID=%d)\n", username, existing.ID)
		return
	}

	if err := db.Create(user).Error; err != nil {
		fmt.Printf("[失败] 创建用户 %s: %v\n", username, err)
		return
	}
	fmt.Printf("[成功] 创建用户 %s (ID=%d, role=%s)\n", username, user.ID, role)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
