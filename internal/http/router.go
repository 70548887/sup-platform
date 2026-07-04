package http

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	"github.com/70548887/sup-platform/internal/module/tenant"
	"github.com/70548887/sup-platform/internal/pkg/ratelimit"
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
	MultiTenantEnabled bool
}

// SetupRouter 初始化并返回配置好的路由引擎
func SetupRouter(deps RouterDeps) *gin.Engine {
	if deps.Config.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 审计中间件：记录所有写操作(POST/PUT/DELETE/PATCH)
	r.Use(audit.AuditMiddleware(deps.AuditSvc))

	// 租户上下文中间件：注入tenantID到Context
	r.Use(middleware.TenantContextMiddleware(deps.Config))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	// Legacy OpenAPI路由组（应用签名认证中间件）
	legacyAuth := middleware.LegacyAuth(deps.DB, nil)

	// 客户端API
	customerGroup := r.Group("/openapi/customer")
	customerGroup.Use(legacyAuth)
	customerGroup.Use(middleware.RateLimitMiddleware(deps.RateLimiter, deps.DB))
	if deps.MultiTenantEnabled {
		customerGroup.Use(middleware.QuotaCheckMiddleware(deps.BillingSvc, deps.MultiTenantEnabled))
	}
	{
		customerHandler := &customer.Handler{
			DB:         deps.DB,
			GoodsSvc:   deps.GoodsSvc,
			OrderSvc:   deps.OrderSvc,
			CardSvc:    deps.CardSvc,
			LedgerSvc:  deps.LedgerSvc,
			PricingSvc: deps.PricingSvc,
		}
		customerGroup.GET("/CustomerAccount/Show", customerHandler.AccountShow)
		customerGroup.GET("/Goods/CategoryList", customerHandler.GoodsCategoryList)
		customerGroup.POST("/Goods/List", customerHandler.GoodsList)
		customerGroup.POST("/Goods/Show", customerHandler.GoodsShow)
		customerGroup.POST("/Goods/Buy", customerHandler.GoodsBuy)
		customerGroup.POST("/Order/Show", customerHandler.OrderShow)
		customerGroup.POST("/Order/StatusHandle", customerHandler.OrderStatusHandle)
	}

	// 供货端API
	supplierGroup := r.Group("/openapi/supplier")
	supplierGroup.Use(legacyAuth)
	supplierGroup.Use(middleware.RateLimitMiddleware(deps.RateLimiter, deps.DB))
	if deps.MultiTenantEnabled {
		supplierGroup.Use(middleware.QuotaCheckMiddleware(deps.BillingSvc, deps.MultiTenantEnabled))
	}
	{
		supplierHandler := &supplier.Handler{
			DB:       deps.DB,
			GoodsSvc: deps.GoodsSvc,
			OrderSvc: deps.OrderSvc,
			CardSvc:  deps.CardSvc,
		}
		supplierGroup.POST("/Goods/Paging", supplierHandler.GoodsPaging)
		supplierGroup.POST("/Goods/Show", supplierHandler.GoodsShow)
		supplierGroup.POST("/Goods/Edit", supplierHandler.GoodsEdit)
		supplierGroup.POST("/Goods/EditPrice", supplierHandler.GoodsEditPrice)
		supplierGroup.POST("/Order/Paging", supplierHandler.OrderPaging)
		supplierGroup.POST("/Order/Show", supplierHandler.OrderShow)
		supplierGroup.POST("/Order/StatusHandle", supplierHandler.OrderStatusHandle)
		supplierGroup.POST("/Order/ScheduleHandle", supplierHandler.OrderScheduleHandle)
	}

	// Admin管理API（JWT认证 + role=admin校验）
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.JWTAuthWithRole(deps.AuthSvc, "admin"))
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

		// API限流管理
		adminGroup.GET("/api-apps", adminHandler.ListApiApps)
		adminGroup.PATCH("/api-apps/:id/rate-limit", adminHandler.UpdateRateLimit)
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
		adminGroup.POST("/billing/invoices/:id/mark-paid", adminHandler.MarkInvoicePaid)
	}

	// 租户管理后台（仅多租户模式启用）
	if deps.MultiTenantEnabled {
		tenantAdminGroup := r.Group("/tenant-admin")
		tenantAdminGroup.Use(middleware.JWTAuth(deps.AuthSvc))
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

	return r
}
