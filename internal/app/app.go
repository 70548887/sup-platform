package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/adapter"
	"github.com/70548887/sup-platform/internal/adapter/yike"
	"github.com/70548887/sup-platform/internal/adapter/yile"
	"github.com/70548887/sup-platform/internal/adapter/zhike"
	"github.com/70548887/sup-platform/internal/config"
	apphttp "github.com/70548887/sup-platform/internal/http"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/module/billing"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/docking"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/notify"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/module/pricing"
	"github.com/70548887/sup-platform/internal/module/recharge"
	"github.com/70548887/sup-platform/internal/module/reconciliation"
	"github.com/70548887/sup-platform/internal/module/refund"
	"github.com/70548887/sup-platform/internal/module/settlement"
	"github.com/70548887/sup-platform/internal/module/tenant"
	"github.com/70548887/sup-platform/internal/pkg/logger"
	"github.com/70548887/sup-platform/internal/pkg/queue"
	"github.com/70548887/sup-platform/internal/pkg/ratelimit"
	pkgtenant "github.com/70548887/sup-platform/internal/pkg/tenant"
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

	// 3.5 连接Redis（降级模式：连接失败不影响启动）
	redisClient := connectRedis(cfg)

	// 3.6 多租户初始化
	if cfg.MultiTenant.Enabled {
		pkgtenant.RegisterTenantScope(db)
		logger.Default().Info(context.Background(), "multi-tenant mode enabled")
	}

	// 4. 初始化各Service
	ledgerSvc := ledger.NewLedgerService(db)
	goodsSvc := goods.NewGoodsService(db)

	// 解析卡密加密密钥
	var cardEncryptKey []byte
	if cfg.Security.CardEncryptKey != "" {
		key, err := hex.DecodeString(cfg.Security.CardEncryptKey)
		if err != nil {
			logger.Default().Warn(context.Background(), "CARD_ENCRYPT_KEY hex decode failed, card encryption disabled", "error", err)
		} else if len(key) != 32 {
			logger.Default().Warn(context.Background(), "CARD_ENCRYPT_KEY must be 32 bytes, card encryption disabled", "got", len(key))
		} else {
			cardEncryptKey = key
			logger.Default().Info(context.Background(), "card content encryption enabled", "algorithm", "AES-256-GCM")
		}
	}

	cardSvc := card.NewCardService(db, cardEncryptKey)
	orderSvc := order.NewOrderService(db, ledgerSvc)

	// 4.1 初始化通知服务并注入OrderService
	notifySvc := notify.NewNotifyService(db)
	orderSvc.SetNotifier(notifySvc)

	// 4.2 初始化审计日志服务
	auditSvc := audit.NewAuditService(db)

	// 4.3 初始化退款服务
	refundSvc := refund.NewRefundService(db, orderSvc, ledgerSvc)

	// 4.4 初始化认证服务
	authSvc := auth.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.Expire)

	// 4.5 Phase 3 服务初始化
	rechargeSvc := recharge.NewRechargeService(db, ledgerSvc)
	adapterFactory := adapter.NewFactory()
	// 注册亿乐适配器（如果配置了YILE_APP_ID）
	yileCfg := yile.LoadFromEnv()
	if yileCfg.AppId != "" {
		yileAdapter := yile.NewYileAdapter(yileCfg)
		// 供货商ID=1暂时硬编码，后续从数据库读取
		adapterFactory.Register(1, yileAdapter)
	}
	// 注册易客适配器
	yikeCfg := yike.LoadFromEnv()
	if yikeCfg.AppId != "" {
		yikeAdapter := yike.NewYikeAdapter(yikeCfg)
		adapterFactory.Register(2, yikeAdapter) // 供货商ID=2
	}
	// 注册直客适配器
	zhikeCfg := zhike.LoadFromEnv()
	if zhikeCfg.AppId != "" {
		zhikeAdapter := zhike.NewZhikeAdapter(zhikeCfg)
		adapterFactory.Register(3, zhikeAdapter) // 供货商ID=3
	}
	dockingSvc := docking.NewDockingService(db, adapterFactory)

	// 4.6 Phase 4A 服务初始化
	pricingSvc := pricing.NewPricingService(db, redisClient, cfg.Redis.Prefix)
	analyticsSvc := analytics.NewAnalyticsService(db, redisClient, cfg.Redis.Prefix)
	reconciliationSvc := reconciliation.NewReconciliationService(db)
	rateLimiter := ratelimit.NewRateLimiter(redisClient)
	if redisClient != nil {
		ctx := context.Background()
		if err := rateLimiter.LoadScript(ctx); err != nil {
			logger.Default().Warn(ctx, "rate limiter script load failed", "error", err)
		}
	}

	// 4.7 Phase 4B 服务初始化
	tenantSvc := tenant.NewTenantService(db)
	billingSvc := billing.NewBillingService(db, redisClient, cfg.Redis.Prefix)
	if cfg.MultiTenant.Enabled {
		tenantSvc.InitDefaultTenant(context.Background())
		billingSvc.InitDefaultPlans(context.Background())
	}

	// 4.8 Phase 5 结算服务初始化
	settlementSvc := settlement.NewSettlementService(db)

	// 4.9 Phase 5 队列客户端初始化
	var queueClient *queue.QueueClient
	if cfg.Redis.Enabled && cfg.Redis.Addr != "" {
		queueClient = queue.NewQueueClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		notifySvc.SetQueueClient(queueClient)
		dockingSvc.SetQueueClient(queueClient)
		cardSvc.SetQueueClient(queueClient)
		logger.Default().Info(context.Background(), "queue client initialized", "addr", cfg.Redis.Addr)
	} else {
		logger.Default().Warn(context.Background(), "queue client disabled: Redis not available")
	}

	// 5. 设置路由
	router := apphttp.SetupRouter(apphttp.RouterDeps{
		DB:                 db,
		Config:             cfg,
		GoodsSvc:           goodsSvc,
		OrderSvc:           orderSvc,
		CardSvc:            cardSvc,
		LedgerSvc:          ledgerSvc,
		AuditSvc:           auditSvc,
		RechargeSvc:        rechargeSvc,
		DockingSvc:         dockingSvc,
		RefundSvc:          refundSvc,
		AuthSvc:            authSvc,
		RedisClient:        redisClient,
		PricingSvc:         pricingSvc,
		AnalyticsSvc:       analyticsSvc,
		ReconciliationSvc:  reconciliationSvc,
		RateLimiter:        rateLimiter,
		TenantSvc:          tenantSvc,
		BillingSvc:         billingSvc,
		SettlementSvc:      settlementSvc,
		MultiTenantEnabled: cfg.MultiTenant.Enabled,
	})

	return &App{
		DB:        db,
		Router:    router,
		Config:    cfg,
		RefundSvc: refundSvc,
	}, nil
}

// Run 启动HTTP服务（支持优雅关闭）
func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.Config.App.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动HTTP服务（非阻塞）
	go func() {
		logger.Default().Info(context.Background(), "SUP Platform API Server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// 监听系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Default().Info(context.Background(), "shutting down gracefully")

	// 设置30秒超时进行优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logger.Default().Info(context.Background(), "server exited gracefully")
	return nil
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
			Enabled:  getEnv("REDIS_ENABLED", "true") == "true",
			Prefix:   getEnv("REDIS_PREFIX", "sup"),
		},
		JWT: config.JWTConfig{
			Secret: getEnv("JWT_SECRET", "sup-platform-secret-key"),
			Expire: getEnvInt("JWT_EXPIRE", 72),
		},
		MultiTenant: config.MultiTenantConfig{
			Enabled:         getEnv("MULTI_TENANT_ENABLED", "false") == "true",
			DefaultTenantID: uint(getEnvInt("DEFAULT_TENANT_ID", 1)),
		},
		Security: config.SecurityConfig{
			AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"), ","),
			TLSEnabled:     getEnv("TLS_ENABLED", "false") == "true",
			CardEncryptKey: getEnv("CARD_ENCRYPT_KEY", ""),
			CookieSecure:   getEnv("COOKIE_SECURE", "false") == "true",
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
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Info)
	} else {
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Warn)
	}

	db, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}

	// 配置数据库连接池
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	return db, nil
}

// connectRedis 连接Redis，失败返回nil（降级模式）
func connectRedis(cfg *config.Config) *redis.Client {
	if !cfg.Redis.Enabled {
		logger.Default().Warn(context.Background(), "Redis is disabled, running in degraded mode")
		return nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     200,
		MinIdleConns: 50,
		PoolTimeout:  30 * time.Second,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Default().Warn(context.Background(), "Redis connection failed, running in degraded mode", "error", err)
		return nil
	}
	logger.Default().Info(context.Background(), "Redis connected successfully", "addr", cfg.Redis.Addr)
	return client
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
