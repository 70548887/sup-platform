package http

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/config"
	"github.com/70548887/sup-platform/internal/http/middleware"
	"github.com/70548887/sup-platform/internal/http/openapi/customer"
	"github.com/70548887/sup-platform/internal/http/openapi/supplier"
	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
)

// SetupRouter 初始化并返回配置好的路由引擎
func SetupRouter(
	db *gorm.DB,
	goodsSvc *goods.GoodsService,
	orderSvc *order.OrderService,
	cardSvc *card.CardService,
	ledgerSvc *ledger.LedgerService,
	cfg *config.Config,
) *gin.Engine {
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	// Legacy OpenAPI路由组（应用签名认证中间件）
	legacyAuth := middleware.LegacyAuth(db, nil)

	// 客户端API
	customerGroup := r.Group("/openapi/customer")
	customerGroup.Use(legacyAuth)
	{
		customerHandler := &customer.Handler{
			DB:        db,
			GoodsSvc:  goodsSvc,
			OrderSvc:  orderSvc,
			CardSvc:   cardSvc,
			LedgerSvc: ledgerSvc,
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
			DB:       db,
			GoodsSvc: goodsSvc,
			OrderSvc: orderSvc,
			CardSvc:  cardSvc,
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

	return r
}
