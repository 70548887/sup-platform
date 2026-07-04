package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/pricing"
)

// CreateCustomerGroup POST /admin/customer-groups — 创建客户分组
func (h *Handler) CreateCustomerGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "name", "分组名称不能为空")
		return
	}

	group := pricing.CustomerGroup{
		Name:        req.Name,
		Description: req.Description,
		Status:      1,
	}

	if err := h.PricingSvc.CreateGroup(c.Request.Context(), &group); err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "create_group", "customer_group", group.ID,
		fmt.Sprintf("创建客户分组 name=%s", group.Name),
	))

	response.Success(c, gin.H{
		"id":          group.ID,
		"name":        group.Name,
		"description": group.Description,
		"status":      group.Status,
		"created_at":  group.CreatedAt,
	})
}

// ListCustomerGroups GET /admin/customer-groups — 分页查询客户分组
func (h *Handler) ListCustomerGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	groups, total, err := h.PricingSvc.ListGroups(c.Request.Context(), page, size)
	if err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	type groupItem struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      int8   `json:"status"`
		CreatedAt   int64  `json:"created_at"`
		UpdatedAt   int64  `json:"updated_at"`
	}

	list := make([]groupItem, 0, len(groups))
	for _, g := range groups {
		list = append(list, groupItem{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Status:      g.Status,
			CreatedAt:   g.CreatedAt,
			UpdatedAt:   g.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// AddGroupMember POST /admin/customer-groups/:id/members — 添加分组成员
func (h *Handler) AddGroupMember(c *gin.Context) {
	groupID, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		CustomerID uint `json:"customer_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "customer_id", "客户ID不能为空")
		return
	}

	member := pricing.CustomerGroupMember{
		GroupID:    groupID,
		CustomerID: req.CustomerID,
	}

	if err := h.PricingSvc.AddMember(c.Request.Context(), &member); err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "add_member", "customer_group", groupID,
		fmt.Sprintf("添加分组成员 group_id=%d customer_id=%d", groupID, req.CustomerID),
	))

	response.Success(c, gin.H{
		"id":          member.ID,
		"group_id":    member.GroupID,
		"customer_id": member.CustomerID,
		"created_at":  member.CreatedAt,
	})
}

// RemoveGroupMember DELETE /admin/customer-groups/:id/members/:memberId — 移除分组成员
func (h *Handler) RemoveGroupMember(c *gin.Context) {
	groupID, ok := parseID(c)
	if !ok {
		return
	}

	memberIDStr := c.Param("memberId")
	memberID, err := strconv.ParseUint(memberIDStr, 10, 32)
	if err != nil {
		response.ParamError(c, "memberId", "无效的成员ID参数")
		return
	}
	customerID := uint(memberID)

	if err := h.PricingSvc.RemoveMember(c.Request.Context(), groupID, customerID); err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "remove_member", "customer_group", groupID,
		fmt.Sprintf("移除分组成员 group_id=%d customer_id=%d", groupID, customerID),
	))

	response.Success(c, gin.H{
		"group_id":    groupID,
		"customer_id": customerID,
	})
}
