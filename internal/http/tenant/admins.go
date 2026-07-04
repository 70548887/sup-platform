package tenant

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	tenantModule "github.com/70548887/sup-platform/internal/module/tenant"
)

// ListAdmins GET /tenant-admin/admins — 管理员列表
func (h *Handler) ListAdmins(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	ctx := c.Request.Context()
	db := h.DB.WithContext(ctx)

	var admins []tenantModule.TenantAdmin
	if err := db.Where("tenant_id = ?", tenantID).Find(&admins).Error; err != nil {
		response.Error(c, "查询管理员列表失败")
		return
	}

	type adminItem struct {
		ID          uint   `json:"id"`
		TenantID    uint   `json:"tenant_id"`
		UserID      uint   `json:"user_id"`
		AdminRole   string `json:"admin_role"`
		Permissions string `json:"permissions"`
		Status      int8   `json:"status"`
		CreatedAt   int64  `json:"created_at"`
	}

	list := make([]adminItem, 0, len(admins))
	for _, a := range admins {
		list = append(list, adminItem{
			ID:          a.ID,
			TenantID:    a.TenantID,
			UserID:      a.UserID,
			AdminRole:   a.AdminRole,
			Permissions: a.Permissions,
			Status:      a.Status,
			CreatedAt:   a.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": len(list),
	})
}

// AddAdmin POST /tenant-admin/admins — 添加管理员
func (h *Handler) AddAdmin(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	var req struct {
		UserID      uint   `json:"user_id" binding:"required"`
		Role        string `json:"role" binding:"required"`
		Permissions string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "user_id/role", "请求参数格式错误")
		return
	}

	ctx := c.Request.Context()
	if err := h.TenantSvc.AddAdmin(ctx, tenantID, req.UserID, req.Role, req.Permissions); err != nil {
		response.Error(c, fmt.Sprintf("添加管理员失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(ctx, audit.NewEntry(
		adminID, adminName, "tenant.admin.add", "tenant_admin", req.UserID,
		fmt.Sprintf("添加租户管理员 userID=%d role=%s", req.UserID, req.Role),
	))

	response.Success(c, gin.H{
		"user_id": req.UserID,
		"role":    req.Role,
	})
}

// RemoveAdmin DELETE /tenant-admin/admins/:id — 移除管理员
func (h *Handler) RemoveAdmin(c *gin.Context) {
	tenantID := getTenantID(c)
	if tenantID == 0 {
		response.Error(c, "无法获取租户信息")
		return
	}

	userID, ok := parseID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := h.TenantSvc.RemoveAdmin(ctx, tenantID, userID); err != nil {
		response.Error(c, fmt.Sprintf("移除管理员失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(ctx, audit.NewEntry(
		adminID, adminName, "tenant.admin.remove", "tenant_admin", userID,
		fmt.Sprintf("移除租户管理员 userID=%d", userID),
	))

	response.Success(c, gin.H{
		"user_id": userID,
	})
}
