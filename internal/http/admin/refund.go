package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/refund"
)

// ListRefunds GET /admin/refunds — 退款列表
func (h *Handler) ListRefunds(c *gin.Context) {
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

	// 构建查询
	query := h.DB.Model(&refund.RefundOrder{})

	// 可选status过滤
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := strconv.ParseInt(statusStr, 10, 8)
		if err == nil {
			query = query.Where("status = ?", int8(s))
		}
	}

	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询退款列表总数失败")
		return
	}

	// 分页查询
	var refunds []refund.RefundOrder
	if err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&refunds).Error; err != nil {
		response.Error(c, "查询退款列表失败")
		return
	}

	// 构造返回数据
	type refundItem struct {
		ID         uint   `json:"id"`
		RefundSN   string `json:"refund_sn"`
		OrderID    uint   `json:"order_id"`
		OrderSN    string `json:"order_sn"`
		CustomerID uint   `json:"customer_id"`
		Amount     string `json:"amount"`
		Reason     string `json:"reason"`
		Status     int8   `json:"status"`
		ReviewerID *uint  `json:"reviewer_id"`
		ReviewNote string `json:"review_note"`
		ReviewedAt *int64 `json:"reviewed_at"`
		RefundedAt *int64 `json:"refunded_at"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
	}

	list := make([]refundItem, 0, len(refunds))
	for _, r := range refunds {
		list = append(list, refundItem{
			ID:         r.ID,
			RefundSN:   r.RefundSN,
			OrderID:    r.OrderID,
			OrderSN:    r.OrderSN,
			CustomerID: r.CustomerID,
			Amount:     r.Amount.StringFixed(2),
			Reason:     r.Reason,
			Status:     r.Status,
			ReviewerID: r.ReviewerID,
			ReviewNote: r.ReviewNote,
			ReviewedAt: r.ReviewedAt,
			RefundedAt: r.RefundedAt,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ApproveRefund POST /admin/refunds/:id/approve — 批准退款
func (h *Handler) ApproveRefund(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	// note是可选的，忽略绑定错误
	_ = c.ShouldBindJSON(&req)

	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)

	if err := h.RefundSvc.Approve(context.Background(), id, adminID, req.Note); err != nil {
		response.Error(c, fmt.Sprintf("批准退款失败: %v", err))
		return
	}

	// 记录审计日志
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "refund.approve", "refund", id,
		fmt.Sprintf("批准退款, 备注: %s", req.Note),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "approved",
	})
}

// RejectRefund POST /admin/refunds/:id/reject — 拒绝退款
func (h *Handler) RejectRefund(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "note", "拒绝原因不能为空")
		return
	}

	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)

	if err := h.RefundSvc.Reject(context.Background(), id, adminID, req.Note); err != nil {
		response.Error(c, fmt.Sprintf("拒绝退款失败: %v", err))
		return
	}

	// 记录审计日志
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "refund.reject", "refund", id,
		fmt.Sprintf("拒绝退款, 原因: %s", req.Note),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "rejected",
	})
}
