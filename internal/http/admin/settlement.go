package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// ListSettlements 结算单列表
// @Summary 结算单分页列表
// @Description 获取结算单列表，支持按供货商筛选
// @Tags Admin-结算管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param supplier_id query int false "供货商ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/settlements [get]
func (h *Handler) ListSettlements(c *gin.Context) {
	if h.SettlementSvc == nil {
		response.Error(c, "结算服务未启用")
		return
	}

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

	var supplierID uint
	if s := c.Query("supplier_id"); s != "" {
		sid, err := strconv.ParseUint(s, 10, 32)
		if err == nil {
			supplierID = uint(sid)
		}
	}

	list, total, err := h.SettlementSvc.ListSettlements(c.Request.Context(), supplierID, page, size)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询结算单失败: %v", err))
		return
	}

	type settlementItem struct {
		ID               uint   `json:"id"`
		SupplierID       uint   `json:"supplier_id"`
		Period           string `json:"period"`
		TotalOrders      int    `json:"total_orders"`
		TotalAmount      string `json:"total_amount"`
		CommissionRate   string `json:"commission_rate"`
		CommissionAmount string `json:"commission_amount"`
		NetAmount        string `json:"net_amount"`
		Status           string `json:"status"`
		CreatedAt        int64  `json:"created_at"`
	}

	items := make([]settlementItem, 0, len(list))
	for _, s := range list {
		items = append(items, settlementItem{
			ID:               s.ID,
			SupplierID:       s.SupplierID,
			Period:           s.Period,
			TotalOrders:      s.TotalOrders,
			TotalAmount:      s.TotalAmount.StringFixed(2),
			CommissionRate:   s.CommissionRate.StringFixed(4),
			CommissionAmount: s.CommissionAmount.StringFixed(2),
			NetAmount:        s.NetAmount.StringFixed(2),
			Status:           s.Status,
			CreatedAt:        s.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetSettlement 结算单详情
// @Summary 获取结算单详情
// @Description 根据ID获取结算单详细信息
// @Tags Admin-结算管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "结算单ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/settlements/{id} [get]
func (h *Handler) GetSettlement(c *gin.Context) {
	if h.SettlementSvc == nil {
		response.Error(c, "结算服务未启用")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	s, err := h.SettlementSvc.GetSettlement(c.Request.Context(), id)
	if err != nil {
		response.Error(c, "结算单不存在")
		return
	}

	response.Success(c, gin.H{
		"id":                s.ID,
		"tenant_id":         s.TenantID,
		"supplier_id":       s.SupplierID,
		"period":            s.Period,
		"total_orders":      s.TotalOrders,
		"total_amount":      s.TotalAmount.StringFixed(2),
		"commission_rate":   s.CommissionRate.StringFixed(4),
		"commission_amount": s.CommissionAmount.StringFixed(2),
		"net_amount":        s.NetAmount.StringFixed(2),
		"status":            s.Status,
		"confirmed_at":      s.ConfirmedAt,
		"paid_at":           s.PaidAt,
		"created_at":        s.CreatedAt,
		"updated_at":        s.UpdatedAt,
	})
}

// GenerateSettlement 生成结算单
// @Summary 生成结算单
// @Description 根据供货商和月份生成结算单
// @Tags Admin-结算管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "生成参数{supplier_id,period}"
// @Success 200 {object} map[string]interface{}
// @Router /admin/settlements/generate [post]
func (h *Handler) GenerateSettlement(c *gin.Context) {
	if h.SettlementSvc == nil {
		response.Error(c, "结算服务未启用")
		return
	}

	var req struct {
		SupplierID uint   `json:"supplier_id" binding:"required"`
		Period     string `json:"period" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	s, err := h.SettlementSvc.GenerateSettlement(c.Request.Context(), req.SupplierID, req.Period)
	if err != nil {
		response.Error(c, fmt.Sprintf("生成结算单失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "settlement.generate", "settlement", s.ID,
		fmt.Sprintf("生成结算单: 供货商=%d, 月份=%s, 金额=%s", req.SupplierID, req.Period, s.TotalAmount.StringFixed(2)),
	))

	response.Success(c, gin.H{
		"id":               s.ID,
		"supplier_id":      s.SupplierID,
		"period":           s.Period,
		"total_orders":     s.TotalOrders,
		"total_amount":     s.TotalAmount.StringFixed(2),
		"commission_amount": s.CommissionAmount.StringFixed(2),
		"net_amount":       s.NetAmount.StringFixed(2),
		"status":           s.Status,
	})
}

// ConfirmSettlement 确认结算单
// @Summary 确认结算单
// @Description 将结算单状态从pending变更为confirmed
// @Tags Admin-结算管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "结算单ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/settlements/{id}/confirm [post]
func (h *Handler) ConfirmSettlement(c *gin.Context) {
	if h.SettlementSvc == nil {
		response.Error(c, "结算服务未启用")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.SettlementSvc.ConfirmSettlement(c.Request.Context(), id); err != nil {
		response.Error(c, fmt.Sprintf("确认结算单失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "settlement.confirm", "settlement", id,
		fmt.Sprintf("确认结算单: id=%d", id),
	))

	response.Success(c, gin.H{"id": id, "status": "confirmed"})
}

// MarkSettlementPaid 标记结算已付款
// @Summary 标记结算已付款
// @Description 将结算单状态从confirmed变更为paid
// @Tags Admin-结算管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "结算单ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/settlements/{id}/paid [post]
func (h *Handler) MarkSettlementPaid(c *gin.Context) {
	if h.SettlementSvc == nil {
		response.Error(c, "结算服务未启用")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.SettlementSvc.MarkPaid(c.Request.Context(), id); err != nil {
		response.Error(c, fmt.Sprintf("标记付款失败: %v", err))
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "settlement.mark_paid", "settlement", id,
		fmt.Sprintf("标记结算单已付款: id=%d", id),
	))

	response.Success(c, gin.H{"id": id, "status": "paid"})
}
