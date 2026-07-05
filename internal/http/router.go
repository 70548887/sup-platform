package http

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/config"
	"github.com/70548887/sup-platform/internal/http/admin"
	"github.com/70548887/sup-platform/internal/http/middleware"
	"github.com/70548887/sup-platform/internal/http/openapi/customer"
	"github.com/70548887/sup-platform/internal/http/openapi/supplier"
	"github.com/70548887/sup-platform/internal/http/response"
	tenanthttp "github.com/70548887/sup-platform/internal/http/tenant"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/module/billing"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/docking"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/module/pricing"
	"github.com/70548887/sup-platform/internal/module/recharge"
	"github.com/70548887/sup-platform/internal/module/reconciliation"
	"github.com/70548887/sup-platform/internal/module/refund"
	"github.com/70548887/sup-platform/internal/module/settlement"
	"github.com/70548887/sup-platform/internal/module/tenant"
	"github.com/70548887/sup-platform/internal/pkg/ratelimit"

	_ "github.com/70548887/sup-platform/docs" // swagger docs
)

// RouterDeps 路由依赖（解决参数膨胀问题）
type RouterDeps struct {
	DB                *gorm.DB
	Config            *config.Config
	GoodsSvc          *goods.GoodsService
	OrderSvc          *order.OrderService
	CardSvc           *card.CardService
	LedgerSvc         *ledger.LedgerService
	AuditSvc          *audit.AuditService
	RechargeSvc       *recharge.RechargeService
	DockingSvc        *docking.DockingService
	RefundSvc         *refund.RefundService
	AuthSvc           *auth.AuthService
	RedisClient       *redis.Client
	PricingSvc        *pricing.PricingService
	AnalyticsSvc      *analytics.AnalyticsService
	ReconciliationSvc  *reconciliation.ReconciliationService
	RateLimiter        *ratelimit.RateLimiter
	TenantSvc          *tenant.TenantService
	BillingSvc         *billing.BillingService
	SettlementSvc      *settlement.SettlementService
	MultiTenantEnabled bool
}

// SetupRouter 初始化并返回配置好的路由引擎
func SetupRouter(deps RouterDeps) *gin.Engine {
	if deps.Config.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 安全响应头中间件
	r.Use(middleware.SecurityHeaders())

	// CORS跨域中间件
	r.Use(middleware.CORSMiddleware(deps.Config.Security.AllowedOrigins))

	// 输入净化中间件
	r.Use(middleware.InputSanitizeMiddleware())

	// 审计中间件：记录所有写操作(POST/PUT/DELETE/PATCH)
	r.Use(audit.AuditMiddleware(deps.AuditSvc))

	// 健康检查 - liveness probe（轻量级）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

	// 就绪检查 - readiness probe（检测依赖服务）
	r.GET("/readiness", func(c *gin.Context) {
		health := gin.H{"status": "ready", "db": "down", "redis": "down", "time": time.Now().Unix()}

		// 检查数据库
		if sqlDB, err := deps.DB.DB(); err == nil {
			if err := sqlDB.Ping(); err == nil {
				health["db"] = "up"
			}
		}

		// 检查 Redis
		if deps.RedisClient != nil {
			if err := deps.RedisClient.Ping(c).Err(); err == nil {
				health["redis"] = "up"
			}
		}

		if health["db"] == "up" {
			c.JSON(200, health)
		} else {
			c.JSON(503, health)
		}
	})

	// Swagger API文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 公开认证路由（无需JWT）
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", handleLogin(deps.AuthSvc, deps.Config))
		authGroup.POST("/refresh", handleRefresh(deps.AuthSvc, deps.Config, deps.RedisClient))
		authGroup.POST("/logout", handleLogout(deps.AuthSvc))
		authGroup.POST("/register", handleRegister(deps.AuthSvc, deps.DB, deps.RedisClient))
		authGroup.POST("/forgot-password", handleForgotPassword(deps.DB, deps.RedisClient))
		authGroup.POST("/reset-password", handleResetPassword(deps.DB, deps.RedisClient))

		// 需要JWT认证的端点
		authProtected := authGroup.Group("")
		authProtected.Use(middleware.JWTAuth(deps.AuthSvc, deps.RedisClient))
		{
			authProtected.GET("/profile", handleProfile(deps.DB))
		}
	}

	// Legacy OpenAPI路由组（应用签名认证中间件，兼容JWT）
	legacyAuth := middleware.LegacyAuth(deps.DB, &middleware.LegacyAuthConfig{
		RedisClient: deps.RedisClient,
		AuthService: deps.AuthSvc,
	})

	// 客户端API（JWT认证 - 供前端管理面板使用）
	customerGroup := r.Group("/openapi/customer")
	customerGroup.Use(middleware.TenantContextMiddleware(deps.Config))
	customerGroup.Use(middleware.JWTAuth(deps.AuthSvc, deps.RedisClient))
	customerGroup.Use(middleware.RateLimitMiddleware(deps.RateLimiter, deps.DB, deps.RedisClient))
	if deps.MultiTenantEnabled {
		customerGroup.Use(middleware.QuotaCheckMiddleware(deps.BillingSvc, deps.MultiTenantEnabled))
	}
	customerHandler := &customer.Handler{
		DB:         deps.DB,
		GoodsSvc:   deps.GoodsSvc,
		OrderSvc:   deps.OrderSvc,
		CardSvc:    deps.CardSvc,
		LedgerSvc:  deps.LedgerSvc,
		PricingSvc: deps.PricingSvc,
	}
	{
		customerGroup.GET("/CustomerAccount/Show", customerHandler.AccountShow)
		customerGroup.GET("/Goods/CategoryList", customerHandler.GoodsCategoryList)
		customerGroup.POST("/Goods/List", customerHandler.GoodsList)
		customerGroup.POST("/Goods/Show", customerHandler.GoodsShow)
		customerGroup.POST("/Goods/Buy", customerHandler.GoodsBuy)
		customerGroup.POST("/Order/Show", customerHandler.OrderShow)
		customerGroup.POST("/Order/InShow", customerHandler.OrderInShow)
		customerGroup.POST("/Order/StatusHandle", customerHandler.OrderStatusHandle)
		customerGroup.POST("/Callback/Test", customerHandler.CallbackTest)

		// App自助管理
		customerGroup.POST("/App/Create", customerHandler.AppCreate)
		customerGroup.GET("/App/List", customerHandler.AppList)
		customerGroup.GET("/App/Detail", customerHandler.AppDetail)
		customerGroup.POST("/App/RotateKey", customerHandler.AppRotateKey)
		customerGroup.PATCH("/App/Status", customerHandler.AppUpdateStatus)
		customerGroup.DELETE("/App/Delete", customerHandler.AppDelete)
	}

	// 供货端API（JWT认证 - 供前端管理面板使用）
	supplierGroup := r.Group("/openapi/supplier")
	supplierGroup.Use(middleware.TenantContextMiddleware(deps.Config))
	supplierGroup.Use(middleware.JWTAuth(deps.AuthSvc, deps.RedisClient))
	supplierGroup.Use(middleware.RateLimitMiddleware(deps.RateLimiter, deps.DB, deps.RedisClient))
	if deps.MultiTenantEnabled {
		supplierGroup.Use(middleware.QuotaCheckMiddleware(deps.BillingSvc, deps.MultiTenantEnabled))
	}
	supplierHandler := &supplier.Handler{
		DB:       deps.DB,
		GoodsSvc: deps.GoodsSvc,
		OrderSvc: deps.OrderSvc,
		CardSvc:  deps.CardSvc,
	}
	{
		supplierGroup.POST("/Goods/Paging", supplierHandler.GoodsPaging)
		supplierGroup.POST("/Goods/Show", supplierHandler.GoodsShow)
		supplierGroup.POST("/Goods/Edit", supplierHandler.GoodsEdit)
		supplierGroup.POST("/Goods/EditPrice", supplierHandler.GoodsEditPrice)
		supplierGroup.POST("/Order/Paging", supplierHandler.OrderPaging)
		supplierGroup.POST("/Order/Show", supplierHandler.OrderShow)
		supplierGroup.POST("/Order/StatusHandle", supplierHandler.OrderStatusHandle)
		supplierGroup.POST("/Order/ScheduleHandle", supplierHandler.OrderScheduleHandle)

		// App自助管理
		supplierGroup.POST("/App/Create", supplierHandler.AppCreate)
		supplierGroup.GET("/App/List", supplierHandler.AppList)
		supplierGroup.GET("/App/Detail", supplierHandler.AppDetail)
		supplierGroup.POST("/App/RotateKey", supplierHandler.AppRotateKey)
		supplierGroup.PATCH("/App/Status", supplierHandler.AppUpdateStatus)
		supplierGroup.DELETE("/App/Delete", supplierHandler.AppDelete)
	}

	// Admin管理API（JWT认证 + role=admin校验）
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.CSRFProtection())
	adminGroup.Use(middleware.JWTAuthWithRole(deps.AuthSvc, deps.RedisClient, "admin"))
	{
		adminHandler := &admin.Handler{
			DB:                deps.DB,
			GoodsSvc:          deps.GoodsSvc,
			OrderSvc:          deps.OrderSvc,
			LedgerSvc:         deps.LedgerSvc,
			AuditSvc:          deps.AuditSvc,
			RefundSvc:         deps.RefundSvc,
			RechargeSvc:       deps.RechargeSvc,
			DockingSvc:        deps.DockingSvc,
			PricingSvc:        deps.PricingSvc,
			ReconciliationSvc: deps.ReconciliationSvc,
			AnalyticsSvc:      deps.AnalyticsSvc,
			BillingSvc:        deps.BillingSvc,
			TenantSvc:         deps.TenantSvc,
			SettlementSvc:     deps.SettlementSvc,
		}

		// 用户管理
		adminGroup.POST("/users", adminHandler.CreateUser)
		adminGroup.GET("/users", adminHandler.ListUsers)
		adminGroup.GET("/users/:id", adminHandler.GetUser)
		adminGroup.PATCH("/users/:id/status", adminHandler.UpdateUserStatus)

		// 商品管理
		adminGroup.GET("/goods", adminHandler.ListGoods)
		adminGroup.PATCH("/goods/:id/status", adminHandler.UpdateGoodsStatus)
		adminGroup.GET("/goods/pending", adminHandler.ListPendingGoods)
		adminGroup.POST("/goods/:id/approve", adminHandler.ApproveGoods)
		adminGroup.POST("/goods/:id/reject", adminHandler.RejectGoods)

		// 订单管理
		adminGroup.GET("/orders", adminHandler.ListOrders)
		adminGroup.GET("/orders/:id", adminHandler.GetOrder)
		adminGroup.POST("/orders/:id/status", adminHandler.UpdateOrderStatus)

		// 退款审核
		adminGroup.GET("/refunds", adminHandler.ListRefunds)
		adminGroup.POST("/refunds/:id/approve", adminHandler.ApproveRefund)
		adminGroup.POST("/refunds/:id/reject", adminHandler.RejectRefund)

		// 充值审核
		adminGroup.GET("/recharges", adminHandler.ListRecharges)
		adminGroup.POST("/recharges/:id/approve", adminHandler.ApproveRecharge)
		adminGroup.POST("/recharges/:id/reject", adminHandler.RejectRecharge)

		// 对接任务管理
		adminGroup.GET("/docking-tasks/failed", adminHandler.ListFailedDockingTasks)
		adminGroup.POST("/docking-tasks/:id/retry", adminHandler.RetryDockingTask)
		adminGroup.GET("/docking-tasks/stats", adminHandler.GetDockingStats)

		// 审计日志
		adminGroup.GET("/audit", adminHandler.ListAuditLogs)

		// 定价规则管理
		adminGroup.POST("/pricing/rules", adminHandler.CreatePricingRule)
		adminGroup.GET("/pricing/rules", adminHandler.ListPricingRules)
		adminGroup.PUT("/pricing/rules/:id", adminHandler.UpdatePricingRule)
		adminGroup.DELETE("/pricing/rules/:id", adminHandler.DeletePricingRule)
		adminGroup.POST("/pricing/calc-preview", adminHandler.CalcPricePreview)

		// 客户分组管理
		adminGroup.POST("/customer-groups", adminHandler.CreateCustomerGroup)
		adminGroup.GET("/customer-groups", adminHandler.ListCustomerGroups)
		adminGroup.POST("/customer-groups/:id/members", adminHandler.AddGroupMember)
		adminGroup.DELETE("/customer-groups/:id/members/:memberId", adminHandler.RemoveGroupMember)

		// API应用管理
		adminGroup.POST("/api-apps", adminHandler.CreateApiApp)
		adminGroup.GET("/api-apps", adminHandler.ListApiApps)
		adminGroup.PATCH("/api-apps/:id/rate-limit", adminHandler.UpdateRateLimit)
		adminGroup.PATCH("/api-apps/:id/status", adminHandler.UpdateApiAppStatus)
		adminGroup.DELETE("/api-apps/:id", adminHandler.DeleteApiApp)
		adminGroup.POST("/api-apps/:id/reset", adminHandler.ResetApiAppSecret)
		adminGroup.GET("/api-apps/:id/usage", adminHandler.GetAppUsage)

		// 对账系统
		adminGroup.POST("/reconciliation/run", adminHandler.RunReconciliation)
		adminGroup.GET("/reconciliation/tasks", adminHandler.ListReconciliationTasks)
		adminGroup.GET("/reconciliation/tasks/:id", adminHandler.GetReconciliationTask)
		adminGroup.PATCH("/reconciliation/errors/:id", adminHandler.ResolveReconciliationError)

		// 数据统计
		adminGroup.GET("/analytics/dashboard", adminHandler.GetDashboard)
		adminGroup.GET("/analytics/revenue-trend", adminHandler.GetRevenueTrend)
		adminGroup.GET("/analytics/hot-goods", adminHandler.GetHotGoods)
		adminGroup.GET("/analytics/order-stats", adminHandler.GetOrderStats)
		adminGroup.GET("/analytics/customer-stats", adminHandler.GetCustomerStats)
		adminGroup.POST("/analytics/aggregate", adminHandler.TriggerAggregate)

		// 计费管理
		adminGroup.GET("/billing/plans", adminHandler.ListBillingPlans)
		adminGroup.POST("/billing/plans", adminHandler.CreateBillingPlan)
		adminGroup.GET("/billing/subscriptions", adminHandler.ListBillingSubscriptions)
		adminGroup.GET("/billing/invoices", adminHandler.ListBillingInvoices)
		adminGroup.POST("/billing/invoices/generate", adminHandler.GenerateInvoice)
		adminGroup.POST("/billing/invoices/:id/mark-paid", adminHandler.MarkInvoicePaid)

		// 结算管理
		adminGroup.GET("/settlements", adminHandler.ListSettlements)
		adminGroup.GET("/settlements/:id", adminHandler.GetSettlement)
		adminGroup.POST("/settlements/generate", adminHandler.GenerateSettlement)
		adminGroup.POST("/settlements/:id/confirm", adminHandler.ConfirmSettlement)
		adminGroup.POST("/settlements/:id/paid", adminHandler.MarkSettlementPaid)
	}

	// 租户管理后台（仅多租户模式启用）
	if deps.MultiTenantEnabled {
		tenantAdminGroup := r.Group("/tenant-admin")
		tenantAdminGroup.Use(middleware.CSRFProtection())
		tenantAdminGroup.Use(middleware.JWTAuth(deps.AuthSvc, deps.RedisClient))
		tenantAdminGroup.Use(middleware.TenantContextMiddleware(deps.Config))
		tenantAdminGroup.Use(middleware.TenantRBACMiddleware(deps.TenantSvc))
		{
			tenantHandler := &tenanthttp.Handler{
				DB:           deps.DB,
				TenantSvc:    deps.TenantSvc,
				GoodsSvc:     deps.GoodsSvc,
				OrderSvc:     deps.OrderSvc,
				LedgerSvc:    deps.LedgerSvc,
				AuditSvc:     deps.AuditSvc,
				AnalyticsSvc: deps.AnalyticsSvc,
				BillingSvc:   deps.BillingSvc,
			}
			tenantAdminGroup.GET("/dashboard", tenantHandler.GetDashboard)
			tenantAdminGroup.GET("/orders", tenantHandler.ListOrders)
			tenantAdminGroup.GET("/orders/:id", tenantHandler.GetOrder)
			tenantAdminGroup.GET("/goods", tenantHandler.ListGoods)
			tenantAdminGroup.PATCH("/goods/:id/status", tenantHandler.UpdateGoodsStatus)
			tenantAdminGroup.GET("/admins", tenantHandler.ListAdmins)
			tenantAdminGroup.POST("/admins", tenantHandler.AddAdmin)
			tenantAdminGroup.DELETE("/admins/:id", tenantHandler.RemoveAdmin)
			tenantAdminGroup.GET("/subscription", tenantHandler.GetSubscription)
			tenantAdminGroup.GET("/usage", tenantHandler.GetUsage)
			tenantAdminGroup.POST("/subscription/upgrade", tenantHandler.UpgradeSubscription)
		}
	}

	// 第三方API v1（Legacy签名认证 - 供第三方系统使用AppId+AppSecret对接）
	apiV1 := r.Group("/openapi/v1")
	apiV1.Use(middleware.TenantContextMiddleware(deps.Config))
	apiV1.Use(legacyAuth)
	apiV1.Use(middleware.RateLimitMiddleware(deps.RateLimiter, deps.DB, deps.RedisClient))
	if deps.MultiTenantEnabled {
		apiV1.Use(middleware.QuotaCheckMiddleware(deps.BillingSvc, deps.MultiTenantEnabled))
	}
	{
		// Customer V1 API（复用已创建的 customerHandler）
		apiV1Customer := apiV1.Group("/customer")
		apiV1Customer.GET("/CustomerAccount/Show", customerHandler.AccountShow)
		apiV1Customer.GET("/Goods/CategoryList", customerHandler.GoodsCategoryList)
		apiV1Customer.POST("/Goods/List", customerHandler.GoodsList)
		apiV1Customer.POST("/Goods/Show", customerHandler.GoodsShow)
		apiV1Customer.POST("/Goods/Buy", customerHandler.GoodsBuy)
		apiV1Customer.POST("/Order/Show", customerHandler.OrderShow)
		apiV1Customer.POST("/Order/InShow", customerHandler.OrderInShow)
		apiV1Customer.POST("/Order/StatusHandle", customerHandler.OrderStatusHandle)
		apiV1Customer.POST("/Callback/Test", customerHandler.CallbackTest)

		// Supplier V1 API（复用已创建的 supplierHandler）
		apiV1Supplier := apiV1.Group("/supplier")
		apiV1Supplier.POST("/Goods/Paging", supplierHandler.GoodsPaging)
		apiV1Supplier.POST("/Goods/Show", supplierHandler.GoodsShow)
		apiV1Supplier.POST("/Goods/Edit", supplierHandler.GoodsEdit)
		apiV1Supplier.POST("/Goods/EditPrice", supplierHandler.GoodsEditPrice)
		apiV1Supplier.POST("/Order/Paging", supplierHandler.OrderPaging)
		apiV1Supplier.POST("/Order/Show", supplierHandler.OrderShow)
		apiV1Supplier.POST("/Order/StatusHandle", supplierHandler.OrderStatusHandle)
		apiV1Supplier.POST("/Order/ScheduleHandle", supplierHandler.OrderScheduleHandle)
	}

	return r
}

// handleLogin 登录Handler，验证用户名密码并通过Cookie+JSON双模式返回token
func handleLogin(authSvc *auth.AuthService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
			Role     string `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamError(c, "", "请求参数格式错误")
			return
		}

		// 调用AuthService登录
		token, err := authSvc.Login(req.Username, req.Password, req.Role, c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Error(c, err.Error())
			return
		}

		// 通过HttpOnly Cookie设置token
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			"auth_token",          // name
			token,                 // value
			3600*72,               // maxAge: 72小时
			"/",                   // path
			"",                    // domain (空=当前域名)
			cfg.Security.CookieSecure, // secure (生产环境设为true)
			true,                  // httpOnly
		)

		// 同时在JSON中返回token（兼容API调用方式）
		response.Success(c, gin.H{
			"token": token,
		})
	}
}

// handleRefresh 刷新token，从Header中获取旧token，验证后签发新token，并将旧token加入黑名单
func handleRefresh(authSvc *auth.AuthService, cfg *config.Config, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header提取旧token
		oldToken := extractBearerToken(c)
		if oldToken == "" {
			response.AuthError(c, "缺少认证凭证")
			return
		}

		// 验证旧token（即使快过期也可刷新）
		claims, err := authSvc.VerifyJWT(oldToken)
		if err != nil {
			response.AuthError(c, "token无效或已过期")
			return
		}

		// 签发新token
		newToken, err := authSvc.GenerateJWT(claims.UserID, claims.Role, claims.TenantID)
		if err != nil {
			response.Error(c, "刷新token失败")
			return
		}

		// 将旧token加入黑名单（Redis可用时）
		if redisClient != nil {
			tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(oldToken)))[:16]
			blacklistKey := fmt.Sprintf("token_blacklist:%s", tokenHash)
			// TTL设为JWT过期时间（72小时）
			redisClient.Set(c, blacklistKey, "1", 72*time.Hour)
		}

		// 设置新Cookie
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			"auth_token",
			newToken,
			3600*72,
			"/",
			"",
			cfg.Security.CookieSecure,
			true,
		)

		response.Success(c, gin.H{
			"token": newToken,
		})
	}
}

// handleLogout 登出（暂不实现Redis黑名单，仅清除Cookie）
func handleLogout(_ *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 清除Cookie
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie("auth_token", "", -1, "/", "", false, true)

		response.Success(c, nil)
	}
}

// handleProfile 获取当前登录用户信息（不含密码）
func handleProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.AuthError(c, "未获取到用户信息")
			return
		}

		var user struct {
			ID        uint   `json:"id"`
			TenantID  uint   `json:"tenant_id"`
			Username  string `json:"username"`
			Nickname  string `json:"nickname"`
			Email     string `json:"email"`
			Phone     string `json:"phone"`
			Role      string `json:"role"`
			Status    int8   `json:"status"`
			CreatedAt int64  `json:"created_at"`
		}

		if err := db.Table("users").
			Select("id, tenant_id, username, nickname, email, phone, role, status, created_at").
			Where("id = ?", userID).
			First(&user).Error; err != nil {
			response.Error(c, "用户不存在")
			return
		}

		response.Success(c, user)
	}
}

// ---------- 注册与密码重置 ----------

// resetCodeStore 验证码内存降级存储（Redis不可用时使用，带定时清理）
var (
	resetCodes   = make(map[string]resetCodeEntry)
	resetCodesMu sync.RWMutex
)

type resetCodeEntry struct {
	Code      string
	ExpiresAt time.Time
}

func init() {
	// 启动定时清理goroutine，每60秒清除过期验证码
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			resetCodesMu.Lock()
			for key, entry := range resetCodes {
				if now.After(entry.ExpiresAt) {
					delete(resetCodes, key)
				}
			}
			resetCodesMu.Unlock()
		}
	}()
}

// storeResetCode 存储验证码（内存降级时使用）
func storeResetCode(email, code string) {
	resetCodesMu.Lock()
	resetCodes[email] = resetCodeEntry{Code: code, ExpiresAt: time.Now().Add(5 * time.Minute)}
	resetCodesMu.Unlock()
}

// getResetCode 获取验证码（内存降级时使用）
func getResetCode(email string) (string, bool) {
	resetCodesMu.RLock()
	entry, ok := resetCodes[email]
	resetCodesMu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return "", false
	}
	return entry.Code, true
}

// deleteResetCode 删除验证码（内存降级时使用）
func deleteResetCode(email string) {
	resetCodesMu.Lock()
	delete(resetCodes, email)
	resetCodesMu.Unlock()
}

// handleRegister 用户注册
func handleRegister(authSvc *auth.AuthService, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// IP限流：每IP每小时最多10次注册
		clientIP := c.ClientIP()
		registerKey := fmt.Sprintf("register_attempt:%s", clientIP)

		if redisClient != nil {
			count, _ := redisClient.Incr(c, registerKey).Result()
			if count == 1 {
				redisClient.Expire(c, registerKey, 1*time.Hour)
			}
			if count > 10 {
				response.Error(c, "注册操作过于频繁，请稍后再试")
				return
			}
		}

		var req struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
			Role     string `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamError(c, "", "请求参数格式错误")
			return
		}

		// 1. 验证role
		if req.Role != "supplier" && req.Role != "customer" {
			response.ParamError(c, "role", "角色只能是supplier或customer")
			return
		}

		// 2. 验证密码策略
		if err := auth.ValidatePassword(req.Password, req.Username); err != nil {
			response.ParamError(c, "password", err.Error())
			return
		}

		// 3. 检查username唯一性
		var count int64
		db.Table("users").Where("username = ?", req.Username).Count(&count)
		if count > 0 {
			response.Error(c, "用户名已存在")
			return
		}

		// 检查email唯一性
		db.Table("users").Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			response.Error(c, "邮箱已被注册")
			return
		}

		// 4. bcrypt哈希密码
		hashed, err := auth.HashPassword(req.Password)
		if err != nil {
			response.Error(c, "系统错误")
			return
		}

		// 5. 创建用户
		now := time.Now().Unix()
		user := map[string]interface{}{
			"username":   req.Username,
			"email":      req.Email,
			"password":   hashed,
			"role":       req.Role,
			"status":     1,
			"tenant_id":  1,
			"created_at": now,
			"updated_at": now,
		}
		result := db.Table("users").Create(user)
		if result.Error != nil {
			// 检测MySQL唯一键冲突
			if strings.Contains(result.Error.Error(), "Duplicate entry") || strings.Contains(result.Error.Error(), "duplicate key") {
				response.Error(c, "用户名或邮箱已被注册")
				return
			}
			response.Error(c, "注册失败，请重试")
			return
		}

		// 获取新用户ID
		var newUser struct {
			ID uint
		}
		db.Table("users").Where("username = ?", req.Username).Select("id").First(&newUser)

		// 6. 生成JWT token
		token, err := authSvc.GenerateJWT(newUser.ID, req.Role, 1)
		if err != nil {
			response.Error(c, "注册成功但生成token失败")
			return
		}

		// 7. 返回结果
		c.JSON(http.StatusOK, response.Response{
			Code:    0,
			Message: "注册成功",
			Data: gin.H{
				"token": token,
				"user": gin.H{
					"id":       newUser.ID,
					"username": req.Username,
					"email":    req.Email,
					"role":     req.Role,
				},
			},
		})
	}
}

// handleForgotPassword 忘记密码（简化版：验证码打印到日志）
func handleForgotPassword(db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email string `json:"email" binding:"required,email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamError(c, "email", "邮箱格式不正确")
			return
		}

		// 验证email存在
		var count int64
		db.Table("users").Where("email = ? AND status = 1", req.Email).Count(&count)
		if count == 0 {
			// 为安全起见，即使不存在也返回成功
			c.JSON(http.StatusOK, response.Response{
				Code:    0,
				Message: "验证码已发送",
				Data:    nil,
			})
			return
		}

		// 生成6位数字验证码
		code := fmt.Sprintf("%06d", rand.Intn(1000000))
		key := "reset_code:" + req.Email

		// 存储验证码（Redis优先，降级到内存）
		if redisClient != nil {
			redisClient.Set(context.Background(), key, code, 5*time.Minute)
		} else {
			storeResetCode(key, code)
		}

		// 打印到日志（实际生产应发送邮件）
		log.Printf("[密码重置] 邮箱: %s, 验证码: %s", req.Email, code)

		c.JSON(http.StatusOK, response.Response{
			Code:    0,
			Message: "验证码已发送",
			Data:    nil,
		})
	}
}

// handleResetPassword 重置密码
func handleResetPassword(db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email       string `json:"email" binding:"required,email"`
			Code        string `json:"code" binding:"required"`
			NewPassword string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamError(c, "", "请求参数格式错误")
			return
		}

		// IP限流：每IP 5分钟最多5次重置尝试
		clientIP := c.ClientIP()
		rateLimitKey := fmt.Sprintf("reset_attempt:%s", clientIP)

		if redisClient != nil {
			count, _ := redisClient.Incr(c, rateLimitKey).Result()
			if count == 1 {
				redisClient.Expire(c, rateLimitKey, 5*time.Minute)
			}
			if count > 5 {
				response.Error(c, "操作过于频繁，请5分钟后再试")
				return
			}
		}

		// 1. 验证验证码
		key := "reset_code:" + req.Email
		var storedCode string

		if redisClient != nil {
			val, err := redisClient.Get(context.Background(), key).Result()
			if err != nil {
				response.Error(c, "验证码无效或已过期")
				return
			}
			storedCode = val
		} else {
			val, ok := getResetCode(key)
			if !ok {
				response.Error(c, "验证码无效或已过期")
				return
			}
			storedCode = val
		}

		if storedCode != req.Code {
			response.Error(c, "验证码错误")
			return
		}

		// 2. 验证新密码策略
		if err := auth.ValidatePassword(req.NewPassword, ""); err != nil {
			response.ParamError(c, "new_password", err.Error())
			return
		}

		// 3. 更新密码
		hashed, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			response.Error(c, "系统错误")
			return
		}

		result := db.Table("users").Where("email = ? AND status = 1", req.Email).Update("password", hashed)
		if result.RowsAffected == 0 {
			response.Error(c, "用户不存在")
			return
		}

		// 4. 删除验证码
		if redisClient != nil {
			redisClient.Del(context.Background(), key)
		} else {
			deleteResetCode(key)
		}

		c.JSON(http.StatusOK, response.Response{
			Code:    0,
			Message: "密码重置成功",
			Data:    nil,
		})
	}
}

// extractBearerToken 从Authorization Header中提取Bearer token
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// 回退：从Cookie提取
		if token, err := c.Cookie("auth_token"); err == nil && token != "" {
			return token
		}
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
