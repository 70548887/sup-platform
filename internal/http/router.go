package http

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/config"
	"github.com/70548887/sup-platform/internal/http/admin"
	"github.com/70548887/sup-platform/internal/http/middleware"
	"github.com/70548887/sup-platform/internal/http/openapi/customer"
	"github.com/70548887/sup-platform/internal/http/openapi/supplier"
	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/auth"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/docking"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
	"github.com/70548887/sup-platform/internal/module/recharge"
	"github.com/70548887/sup-platform/internal/module/refund"
)

// RouterDeps 路由依赖（解决参数膨胀问题）
type RouterDeps struct {
	DB          *gorm.DB
	Config      *config.Config
	GoodsSvc    *goods.GoodsService
	OrderSvc    *order.OrderService
	CardSvc     *card.CardService
	LedgerSvc   *ledger.LedgerService
	AuditSvc    *audit.AuditService
	RechargeSvc *recharge.RechargeService
	DockingSvc  *docking.DockingService
	RefundSvc   *refund.RefundService
	AuthSvc     *auth.AuthService
}

// SetupRouter 初始化并返回配置好的路由引擎
func SetupRouter(deps RouterDeps) *gin.Engine {
	if deps.Config.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 审计中间件：记录所有写操作(POST/PUT/DELETE/PATCH)
	r.Use(audit.AuditMiddleware(deps.AuditSvc))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	// Legacy OpenAPI路由组（应用签名认证中间件）
	legacyAuth := middleware.LegacyAuth(deps.DB, nil)

	// 客户端API
	customerGroup := r.Group("/openapi/customer")
	customerGroup.Use(legacyAuth)
	{
		customerHandler := &customer.Handler{
			DB:        deps.DB,
			GoodsSvc:  deps.GoodsSvc,
			OrderSvc:  deps.OrderSvc,
			CardSvc:   deps.CardSvc,
			LedgerSvc: deps.LedgerSvc,
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
			DB:          deps.DB,
			GoodsSvc:    deps.GoodsSvc,
			OrderSvc:    deps.OrderSvc,
			LedgerSvc:   deps.LedgerSvc,
			AuditSvc:    deps.AuditSvc,
			RefundSvc:   deps.RefundSvc,
			RechargeSvc: deps.RechargeSvc,
			DockingSvc:  deps.DockingSvc,
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
	}

	return r
}
