package testutil

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/ledger"
)

// SetupTestDB 创建SQLite内存数据库用于测试
// 每次调用返回独立的内存数据库，避免测试间干扰
func SetupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to open test database: " + err.Error())
	}

	// 自动迁移所有需要的模型
	if err := ledger.AutoMigrate(db); err != nil {
		panic("failed to auto migrate: " + err.Error())
	}

	return db
}

// SetupIsolatedTestDB 创建完全隔离的SQLite内存数据库
// 使用不同DSN确保每个测试完全独立
func SetupIsolatedTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to open test database: " + err.Error())
	}

	// 自动迁移所有需要的模型
	if err := ledger.AutoMigrate(db); err != nil {
		panic("failed to auto migrate: " + err.Error())
	}

	return db
}
