package tenant

import (
	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	tenantModule "github.com/70548887/sup-platform/internal/module/tenant"
)

// GetSubscription GET /tenant-admin/subscription — 查询租户订阅信息
func (h *Handler) GetSubscription(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	ctx := c.Request.Context()
	db := h.DB.WithContext(ctx)

	var t tenantModule.Tenant
	if err := db.First(&t, tenantID).Error; err != nil {
		response.Error(c, "租户信息不存在")
		return
	}

	response.Success(c, gin.H{
		"tenant_id":  t.ID,
		"name":       t.Name,
		"domain":     t.Domain,
		"type":       t.Type,
		"status":     t.Status,
		"max_admins": t.MaxAdmins,
		"created_at": t.CreatedAt,
	})
}

// GetUsage GET /tenant-admin/usage — 查询使用量
func (h *Handler) GetUsage(c *gin.Context) {
	// 后续集成BillingService后完善
	response.Success(c, gin.H{
		"message": "使用量统计功能开发中",
		"usage":   nil,
	})
}

// UpgradeSubscription POST /tenant-admin/subscription/upgrade — 升级订阅
func (h *Handler) UpgradeSubscription(c *gin.Context) {
	// 后续实现BillingService后完善
	response.Error(c, "订阅升级功能开发中")
}
