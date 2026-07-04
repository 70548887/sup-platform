package tenant

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/analytics"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/billing"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
	tenantModule "github.com/70548887/sup-platform/internal/module/tenant"
	pkgtenant "github.com/70548887/sup-platform/internal/pkg/tenant"
)

// Handler 租户Admin后台Handler
type Handler struct {
	DB           *gorm.DB
	TenantSvc    *tenantModule.TenantService
	GoodsSvc     *goods.GoodsService
	OrderSvc     *order.OrderService
	LedgerSvc    *ledger.LedgerService
	AuditSvc     *audit.AuditService
	AnalyticsSvc *analytics.AnalyticsService
	BillingSvc   *billing.BillingService
}

// getTenantID 从Gin Context获取租户ID
func getTenantID(c *gin.Context) uint {
	return pkgtenant.TenantIDFromGin(c)
}

// getAdminUserID 从JWT Context中获取管理员用户ID
func getAdminUserID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	if id, ok := userID.(uint); ok {
		return id
	}
	return 0
}

// getAdminUsername 从JWT Context中获取管理员用户名
func getAdminUsername(c *gin.Context) string {
	username, _ := c.Get("username")
	if name, ok := username.(string); ok {
		return name
	}
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
