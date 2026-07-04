package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
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

// Handler Admin管理后台处理器
type Handler struct {
	DB                *gorm.DB
	GoodsSvc          *goods.GoodsService
	OrderSvc          *order.OrderService
	LedgerSvc         *ledger.LedgerService
	AuditSvc          *audit.AuditService
	RefundSvc         *refund.RefundService
	RechargeSvc       *recharge.RechargeService
	DockingSvc        *docking.DockingService
	PricingSvc        *pricing.PricingService
	ReconciliationSvc *reconciliation.ReconciliationService
	AnalyticsSvc      *analytics.AnalyticsService
	BillingSvc        *billing.BillingService
	TenantSvc         *tenant.TenantService
	SettlementSvc     *settlement.SettlementService
}

// getAdminUserID 从JWT Context中获取管理员用户ID
func getAdminUserID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	if id, ok := userID.(uint); ok {
		return id
	}
	return 0
}

// getAdminUsername 从JWT Context中获取管理员用户名（审计日志用）
func getAdminUsername(c *gin.Context) string {
	username, _ := c.Get("username")
	if name, ok := username.(string); ok {
		return name
	}
	// 回退：使用用户ID作为标识
	return strconv.FormatUint(uint64(getAdminUserID(c)), 10)
}

// parseID 解析URL路径中的ID参数
func parseID(c *gin.Context) (uint, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.ParamError(c, "id", "无效的ID参数")
		return 0, false
	}
	return uint(id), true
}
