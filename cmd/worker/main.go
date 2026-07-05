package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/adapter"
	"github.com/70548887/sup-platform/internal/adapter/yike"
	"github.com/70548887/sup-platform/internal/adapter/yile"
	"github.com/70548887/sup-platform/internal/adapter/zhike"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/docking"
	"github.com/70548887/sup-platform/internal/module/notify"
	"github.com/70548887/sup-platform/internal/module/reconciliation"
	"github.com/70548887/sup-platform/internal/pkg/logger"
	"github.com/70548887/sup-platform/internal/pkg/queue"
)

// 包级别服务变量
var (
	notifySvc    *notify.NotifyService
	dockingSvc   *docking.DockingService
	reconcileSvc *reconciliation.ReconciliationService
	analyticsSvc *analytics.AnalyticsService
	cardSvc      *card.CardService
)

func main() {
	// Redis 配置
	redisAddr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := getEnvInt("REDIS_DB", 0)
	redisPrefix := getEnv("REDIS_PREFIX", "sup")
	concurrency := getEnvInt("WORKER_CONCURRENCY", 10)

	// 初始化数据库
	db, err := connectDB()
	if err != nil {
		log.Fatalf("[FATAL] Worker DB connect failed: %v", err)
	}
	logger.Default().Info(context.Background(), "worker DB connected successfully")

	// 初始化Redis客户端（供AnalyticsService使用）
	redisClient := connectRedis(redisAddr, redisPassword, redisDB)

	// 初始化服务依赖
	initServices(db, redisClient, redisPrefix)

	// 创建队列服务器（使用与API相同的Redis DB）
	srv := queue.NewQueueServer(redisAddr, redisPassword, redisDB, concurrency)

	// 注册任务处理函数
	srv.HandleFunc(queue.TypeWebhookDeliver, handleWebhookDeliver)
	srv.HandleFunc(queue.TypeDockingSubmit, handleDockingSubmit)
	srv.HandleFunc(queue.TypeReconciliationRun, handleReconciliation)
	srv.HandleFunc(queue.TypeAnalyticsAggregate, handleAnalyticsAggregate)
	srv.HandleFunc(queue.TypeCardImport, handleCardImport)

	logger.Default().Info(context.Background(), "worker starting", "concurrency", concurrency, "redis", redisAddr, "db", redisDB)

	// 非阻塞启动Worker
	if err := srv.Start(); err != nil {
		log.Fatalf("[FATAL] Worker start failed: %v", err)
	}

	// 监听系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Default().Info(context.Background(), "worker shutting down gracefully")
	srv.Shutdown()
	logger.Default().Info(context.Background(), "worker exited gracefully")
}

// initServices 初始化所有服务
func initServices(db *gorm.DB, redisClient *redis.Client, redisPrefix string) {
	// 通知服务
	notifySvc = notify.NewNotifyService(db)

	// 对接服务（需要适配器工厂）
	adapterFactory := adapter.NewFactory()
	yileCfg := yile.LoadFromEnv()
	if yileCfg.AppId != "" {
		yileAdapter := yile.NewYileAdapter(yileCfg)
		adapterFactory.Register(1, yileAdapter)
	}
	yikeCfg := yike.LoadFromEnv()
	if yikeCfg.AppId != "" {
		yikeAdapter := yike.NewYikeAdapter(yikeCfg)
		adapterFactory.Register(2, yikeAdapter)
	}
	zhikeCfg := zhike.LoadFromEnv()
	if zhikeCfg.AppId != "" {
		zhikeAdapter := zhike.NewZhikeAdapter(zhikeCfg)
		adapterFactory.Register(3, zhikeAdapter)
	}
	dockingSvc = docking.NewDockingService(db, adapterFactory)

	// 对账服务
	reconcileSvc = reconciliation.NewReconciliationService(db)

	// 统计服务
	analyticsSvc = analytics.NewAnalyticsService(db, redisClient, redisPrefix)

	// 卡密服务
	var cardEncryptKey []byte
	if keyHex := os.Getenv("CARD_ENCRYPT_KEY"); keyHex != "" {
		key, err := hex.DecodeString(keyHex)
		if err == nil && len(key) == 32 {
			cardEncryptKey = key
		}
	}
	cardSvc = card.NewCardService(db, cardEncryptKey)

	logger.Default().Info(context.Background(), "worker services initialized")
}

// connectDB 连接MySQL
func connectDB() (*gorm.DB, error) {
	dsn := getEnv("MYSQL_DSN", "")
	if dsn == "" {
		// 从分离的环境变量构造DSN
		host := getEnv("DB_HOST", "127.0.0.1")
		port := getEnvInt("DB_PORT", 3306)
		user := getEnv("DB_USER", "root")
		password := getEnv("DB_PASSWORD", "")
		dbName := getEnv("DB_NAME", "sup_platform")
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, dbName)
	}

	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}

	db, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// connectRedis 连接Redis，失败返回nil
func connectRedis(addr, password string, db int) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Default().Warn(ctx, "worker Redis connection failed, analytics cache disabled", "error", err)
		return nil
	}
	logger.Default().Info(ctx, "worker Redis connected", "addr", addr, "db", db)
	return client
}

// handleWebhookDeliver 调用NotifyService执行回调投递
func handleWebhookDeliver(ctx context.Context, t *asynq.Task) error {
	var p queue.WebhookPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("webhook: unmarshal payload: %w", err)
	}
	logger.Default().Info(ctx, "webhook deliver", "callback_id", p.CallbackID, "url", p.URL)
	return notifySvc.DeliverCallback(ctx, p.CallbackID)
}

// handleDockingSubmit 调用DockingService提交对接订单
func handleDockingSubmit(ctx context.Context, t *asynq.Task) error {
	var p queue.DockingPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("docking: unmarshal payload: %w", err)
	}
	logger.Default().Info(ctx, "docking submit", "task_id", p.TaskID, "order_sn", p.OrderSN)
	return dockingSvc.ExecuteTask(ctx, p.TaskID)
}

// handleReconciliation 调用ReconciliationService执行对账
func handleReconciliation(ctx context.Context, t *asynq.Task) error {
	var p queue.ReconciliationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("reconciliation: unmarshal payload: %w", err)
	}
	logger.Default().Info(ctx, "reconciliation", "type", p.Type)
	switch p.Type {
	case "balance_check":
		_, err := reconcileSvc.RunBalanceCheck(ctx)
		return err
	case "cross_verify":
		_, err := reconcileSvc.RunCrossVerify(ctx)
		return err
	default:
		return fmt.Errorf("unknown reconciliation type: %s", p.Type)
	}
}

// handleAnalyticsAggregate 调用AnalyticsService执行聚合
func handleAnalyticsAggregate(ctx context.Context, t *asynq.Task) error {
	var p queue.AnalyticsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("analytics: unmarshal payload: %w", err)
	}
	logger.Default().Info(ctx, "analytics aggregate", "date", p.Date)
	return analyticsSvc.AggregateDaily(ctx, p.Date)
}

// handleCardImport 处理卡密批量导入任务
func handleCardImport(ctx context.Context, t *asynq.Task) error {
	var p queue.CardImportPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("card_import: unmarshal payload: %w", err)
	}
	logger.Default().Info(ctx, "card import", "goods_id", p.GoodsID, "batch", p.BatchName, "count", len(p.Contents))
	_, err := cardSvc.ImportCards(ctx, p.GoodsID, p.BatchName, p.Contents)
	return err
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
