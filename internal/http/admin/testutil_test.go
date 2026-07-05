package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/70548887/sup-platform/internal/module/account"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/module/billing"
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
)

const testJWTSecret = "test-secret-key-for-admin-handler"

// TestEnv 集成测试环境
type TestEnv struct {
	DB      *gorm.DB
	Handler *Handler
	Router  *gin.Engine
	AuthSvc *auth.AuthService
}

// setupTestEnv 初始化完整的测试环境
func setupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 1. SQLite内存DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "failed to open test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	// 2. AutoMigrate所有model
	require.NoError(t, account.AutoMigrate(db))
	require.NoError(t, ledger.AutoMigrate(db))
	require.NoError(t, order.AutoMigrate(db))
	require.NoError(t, goods.AutoMigrate(db))
	require.NoError(t, audit.Migrate(db))
	require.NoError(t, refund.Migrate(db))
	require.NoError(t, recharge.Migrate(db))
	require.NoError(t, docking.Migrate(db))
	require.NoError(t, pricing.Migrate(db))
	require.NoError(t, reconciliation.Migrate(db))
	require.NoError(t, analytics.Migrate(db))
	require.NoError(t, billing.Migrate(db))
	require.NoError(t, tenant.Migrate(db))
	require.NoError(t, settlement.Migrate(db))

	// 3. 初始化全部Service
	authSvc := auth.NewAuthService(db, testJWTSecret, 72)
	ledgerSvc := ledger.NewLedgerService(db)
	orderSvc := order.NewOrderService(db, ledgerSvc)
	goodsSvc := goods.NewGoodsService(db)
	auditSvc := audit.NewAuditService(db)
	refundSvc := refund.NewRefundService(db, orderSvc, ledgerSvc)
	rechargeSvc := recharge.NewRechargeService(db, ledgerSvc)
	dockingSvc := docking.NewDockingService(db, nil)
	pricingSvc := pricing.NewPricingService(db, nil, "test")
	reconciliationSvc := reconciliation.NewReconciliationService(db)
	analyticsSvc := analytics.NewAnalyticsService(db, nil, "test")
	billingSvc := billing.NewBillingService(db, nil, "test")
	tenantSvc := tenant.NewTenantService(db)
	settlementSvc := settlement.NewSettlementService(db)

	// 4. 构建Handler
	handler := &Handler{
		DB:                db,
		GoodsSvc:          goodsSvc,
		OrderSvc:          orderSvc,
		LedgerSvc:         ledgerSvc,
		AuditSvc:          auditSvc,
		RefundSvc:         refundSvc,
		RechargeSvc:       rechargeSvc,
		DockingSvc:        dockingSvc,
		PricingSvc:        pricingSvc,
		ReconciliationSvc: reconciliationSvc,
		AnalyticsSvc:      analyticsSvc,
		BillingSvc:        billingSvc,
		TenantSvc:         tenantSvc,
		SettlementSvc:     settlementSvc,
	}

	// 5. 创建gin.Engine并注册admin路由（使用测试中间件注入user context）
	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(testAuthMiddleware(authSvc))
	registerAdminRoutes(adminGroup, handler)

	return &TestEnv{
		DB:      db,
		Handler: handler,
		Router:  r,
		AuthSvc: authSvc,
	}
}

// testAuthMiddleware 测试用JWT认证中间件（与生产中间件逻辑一致）
func testAuthMiddleware(authSvc *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := bytes.SplitN([]byte(authHeader), []byte(" "), 2)
			if len(parts) == 2 {
				tokenString = string(parts[1])
			}
		}
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 100, "message": "缺少认证凭证"})
			return
		}

		claims, err := authSvc.VerifyJWT(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 100, "message": "token无效或已过期"})
			return
		}

		if claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 100, "message": "权限不足"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Next()
	}
}

// registerAdminRoutes 注册admin路由（与router.go保持一致）
func registerAdminRoutes(rg *gin.RouterGroup, h *Handler) {
	// 用户管理
	rg.POST("/users", h.CreateUser)
	rg.GET("/users", h.ListUsers)
	rg.GET("/users/:id", h.GetUser)
	rg.PATCH("/users/:id/status", h.UpdateUserStatus)

	// 商品管理
	rg.GET("/goods", h.ListGoods)
	rg.PATCH("/goods/:id/status", h.UpdateGoodsStatus)
	rg.GET("/goods/pending", h.ListPendingGoods)
	rg.POST("/goods/:id/approve", h.ApproveGoods)
	rg.POST("/goods/:id/reject", h.RejectGoods)

	// 订单管理
	rg.GET("/orders", h.ListOrders)
	rg.GET("/orders/:id", h.GetOrder)
	rg.POST("/orders/:id/status", h.UpdateOrderStatus)

	// 退款审核
	rg.GET("/refunds", h.ListRefunds)
	rg.POST("/refunds/:id/approve", h.ApproveRefund)
	rg.POST("/refunds/:id/reject", h.RejectRefund)

	// 充值审核
	rg.GET("/recharges", h.ListRecharges)
	rg.POST("/recharges/:id/approve", h.ApproveRecharge)
	rg.POST("/recharges/:id/reject", h.RejectRecharge)

	// 对接任务管理
	rg.GET("/docking-tasks/failed", h.ListFailedDockingTasks)
	rg.POST("/docking-tasks/:id/retry", h.RetryDockingTask)
	rg.GET("/docking-tasks/stats", h.GetDockingStats)

	// 审计日志
	rg.GET("/audit", h.ListAuditLogs)

	// 定价规则管理
	rg.POST("/pricing/rules", h.CreatePricingRule)
	rg.GET("/pricing/rules", h.ListPricingRules)
	rg.PUT("/pricing/rules/:id", h.UpdatePricingRule)
	rg.DELETE("/pricing/rules/:id", h.DeletePricingRule)
	rg.POST("/pricing/calc-preview", h.CalcPricePreview)

	// 客户分组管理
	rg.POST("/customer-groups", h.CreateCustomerGroup)
	rg.GET("/customer-groups", h.ListCustomerGroups)
	rg.POST("/customer-groups/:id/members", h.AddGroupMember)
	rg.DELETE("/customer-groups/:id/members/:memberId", h.RemoveGroupMember)

	// API限流管理
	rg.GET("/api-apps", h.ListApiApps)
	rg.PATCH("/api-apps/:id/rate-limit", h.UpdateRateLimit)
	rg.GET("/api-apps/:id/usage", h.GetAppUsage)

	// 对账系统
	rg.POST("/reconciliation/run", h.RunReconciliation)
	rg.GET("/reconciliation/tasks", h.ListReconciliationTasks)
	rg.GET("/reconciliation/tasks/:id", h.GetReconciliationTask)
	rg.PATCH("/reconciliation/errors/:id", h.ResolveReconciliationError)

	// 数据统计
	rg.GET("/analytics/dashboard", h.GetDashboard)
	rg.GET("/analytics/revenue-trend", h.GetRevenueTrend)
	rg.GET("/analytics/hot-goods", h.GetHotGoods)
	rg.GET("/analytics/order-stats", h.GetOrderStats)
	rg.GET("/analytics/customer-stats", h.GetCustomerStats)
	rg.POST("/analytics/aggregate", h.TriggerAggregate)

	// 计费管理
	rg.GET("/billing/plans", h.ListBillingPlans)
	rg.POST("/billing/plans", h.CreateBillingPlan)
	rg.GET("/billing/subscriptions", h.ListBillingSubscriptions)
	rg.GET("/billing/invoices", h.ListBillingInvoices)
	rg.POST("/billing/invoices/generate", h.GenerateInvoice)
	rg.POST("/billing/invoices/:id/mark-paid", h.MarkInvoicePaid)

	// 结算管理
	rg.GET("/settlements", h.ListSettlements)
	rg.GET("/settlements/:id", h.GetSettlement)
	rg.POST("/settlements/generate", h.GenerateSettlement)
	rg.POST("/settlements/:id/confirm", h.ConfirmSettlement)
	rg.POST("/settlements/:id/paid", h.MarkSettlementPaid)
}

// makeRequest 构建并执行HTTP请求（带JWT认证）
func (env *TestEnv) makeRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	return env.makeRequestWithToken(method, path, body, env.generateAdminToken(1))
}

// makeRequestWithToken 构建并执行HTTP请求（指定token）
func (env *TestEnv) makeRequestWithToken(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	env.Router.ServeHTTP(w, req)
	return w
}

// makeRequestNoAuth 构建并执行HTTP请求（不带认证）
func (env *TestEnv) makeRequestNoAuth(method, path string, body interface{}) *httptest.ResponseRecorder {
	return env.makeRequestWithToken(method, path, body, "")
}

// generateAdminToken 生成测试用admin JWT token
func (env *TestEnv) generateAdminToken(userID uint) string {
	token, _ := env.AuthSvc.GenerateJWT(userID, "admin", 0)
	return token
}

// generateNonAdminToken 生成非admin角色token（用于权限测试）
func (env *TestEnv) generateNonAdminToken(userID uint) string {
	token, _ := env.AuthSvc.GenerateJWT(userID, "customer", 0)
	return token
}

// parseResponse 解析JSON响应
func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "failed to parse response body: %s", w.Body.String())
	return resp
}

// createTestAdmin 创建测试管理员用户并返回ID
func (env *TestEnv) createTestAdmin(t *testing.T) uint {
	t.Helper()
	user := account.User{
		Username: "testadmin",
		Password: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", // "password123"
		Nickname: "Test Admin",
		Role:     "admin",
		Status:   1,
	}
	require.NoError(t, env.DB.Create(&user).Error)
	return user.ID
}

// createTestGoods 创建测试商品并返回ID
func (env *TestEnv) createTestGoods(t *testing.T) uint {
	t.Helper()
	g := goods.Goods{
		SerialNumber: "TEST-GOODS-001",
		Name:         "测试商品",
		Status:       1,
		CategoryID:   1,
		SupplierID:   1,
		Stock:        100,
	}
	require.NoError(t, env.DB.Create(&g).Error)
	return g.ID
}

// createTestOrder 创建测试订单并返回ID
func (env *TestEnv) createTestOrder(t *testing.T, goodsID uint) uint {
	t.Helper()
	o := order.Order{
		OrderSN:    "TEST-ORD-001",
		CustomerID: 1,
		SupplierID: 1,
		GoodsID:    goodsID,
		GoodsSN:    "TEST-GOODS-001",
		GoodsName:  "测试商品",
		BuyNumber:  1,
		Status:     1, // StatusPaid
	}
	require.NoError(t, env.DB.Create(&o).Error)
	return o.ID
}
