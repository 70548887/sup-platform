package app

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/config"
	apphttp "github.com/70548887/sup-platform/internal/http"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/notify"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/module/refund"
	"github.com/70548887/sup-platform/migrations"
)

// App 应用实例
type App struct {
	DB        *gorm.DB
	Router    *gin.Engine
	Config    *config.Config
	RefundSvc *refund.RefundService
}

// New 初始化应用
func New() (*App, error) {
	// 1. 加载配置
	cfg := loadConfig()

	// 2. 连接数据库
	db, err := connectDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database failed: %w", err)
	}

	// 3. 自动迁移
	if err := migrations.RunAll(db); err != nil {
		return nil, fmt.Errorf("auto migrate failed: %w", err)
	}

	// 4. 初始化各Service
	ledgerSvc := ledger.NewLedgerService(db)
	goodsSvc := goods.NewGoodsService(db)
	cardSvc := card.NewCardService(db)
	orderSvc := order.NewOrderService(db, ledgerSvc)

	// 4.1 初始化通知服务并注入OrderService
	notifySvc := notify.NewNotifyService(db)
	orderSvc.SetNotifier(notifySvc)

	// 4.2 初始化审计日志服务
	auditSvc := audit.NewAuditService(db)

	// 4.3 初始化退款服务
	refundSvc := refund.NewRefundService(db, orderSvc, ledgerSvc)

	// 5. 设置路由
	router := apphttp.SetupRouter(db, goodsSvc, orderSvc, cardSvc, ledgerSvc, auditSvc, cfg)

	return &App{
		DB:        db,
		Router:    router,
		Config:    cfg,
		RefundSvc: refundSvc,
	}, nil
}

// Run 启动HTTP服务
func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.Config.App.Port)
	fmt.Printf("SUP Platform API Server starting on %s\n", addr)
	return a.Router.Run(addr)
}

// loadConfig 从环境变量加载配置
func loadConfig() *config.Config {
	cfg := &config.Config{
		App: config.AppConfig{
			Name: getEnv("APP_NAME", "sup-platform"),
			Port: getEnvInt("APP_PORT", 8080),
			Mode: getEnv("APP_MODE", "debug"),
		},
		Database: config.DatabaseConfig{
			Driver:   "mysql",
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnvInt("DB_PORT", 3306),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "sup_platform"),
		},
		Redis: config.RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: config.JWTConfig{
			Secret: getEnv("JWT_SECRET", "sup-platform-secret-key"),
			Expire: getEnvInt("JWT_EXPIRE", 72),
		},
	}
	return cfg
}

// connectDB 连接MySQL数据库
func connectDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)

	gormCfg := &gorm.Config{}
	if cfg.App.Mode == "debug" {
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormCfg.Logger = logger.Default.LogMode(logger.Warn)
	}

	db, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// getEnv 获取环境变量，不存在则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt 获取int类型环境变量
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
