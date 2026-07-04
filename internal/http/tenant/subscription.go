package tenant

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
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

	// 查询租户基本信息
	var t tenantModule.Tenant
	if err := db.First(&t, tenantID).Error; err != nil {
		response.Error(c, "租户信息不存在")
		return
	}

	resp := gin.H{
		"tenant_id":  t.ID,
		"name":       t.Name,
		"domain":     t.Domain,
		"type":       t.Type,
		"status":     t.Status,
		"max_admins": t.MaxAdmins,
		"created_at": t.CreatedAt,
	}

	// 如果计费服务可用，查询订阅+套餐信息
	if h.BillingSvc != nil {
		sub, plan, err := h.BillingSvc.GetSubscription(ctx, tenantID)
		if err == nil && sub != nil {
			resp["subscription"] = gin.H{
				"id":         sub.ID,
				"plan_id":    sub.PlanID,
				"start_at":   sub.StartAt,
				"end_at":     sub.EndAt,
				"auto_renew": sub.AutoRenew,
				"status":     sub.Status,
			}
			if plan != nil {
				resp["plan"] = gin.H{
					"id":                     plan.ID,
					"name":                   plan.Name,
					"display_name":           plan.DisplayName,
					"monthly_price":          plan.MonthlyPrice,
					"max_api_calls_per_month": plan.MaxAPICallsPerMonth,
					"max_orders":             plan.MaxOrders,
					"max_admins":             plan.MaxAdmins,
					"features":               plan.Features,
				}
			}
		}
	}

	response.Success(c, resp)
}

// GetUsage GET /tenant-admin/usage — 查询使用量
func (h *Handler) GetUsage(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	usage, err := h.BillingSvc.GetUsage(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, fmt.Sprintf("获取用量失败: %v", err))
		return
	}

	response.Success(c, usage)
}

// UpgradeSubscription POST /tenant-admin/subscription/upgrade — 升级订阅
func (h *Handler) UpgradeSubscription(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	var req struct {
		PlanID uint `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, "请求参数错误: plan_id必填")
		return
	}

	sub, err := h.BillingSvc.UpgradeSubscription(c.Request.Context(), tenantID, req.PlanID)
	if err != nil {
		response.Error(c, fmt.Sprintf("订阅升级失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(c.Request.Context(), audit.NewEntry(
		adminID, adminName, "tenant.subscription.upgrade", "subscription", sub.ID,
		fmt.Sprintf("升级订阅套餐 planID=%d", req.PlanID),
	))

	response.Success(c, sub)
}
