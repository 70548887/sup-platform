package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// ListRecharges GET /admin/recharges — 充值列表
func (h *Handler) ListRecharges(c *gin.Context) {
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

	// 可选user_id过滤
	var userID uint
	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err == nil {
			userID = uint(uid)
		}
	}

	// 可选status过滤
	var statusPtr *int8
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := strconv.ParseInt(statusStr, 10, 8)
		if err == nil {
			status := int8(s)
			statusPtr = &status
		}
	}

	// 调用RechargeSvc.List
	list, total, err := h.RechargeSvc.List(context.Background(), userID, statusPtr, page, size)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询充值列表失败: %v", err))
		return
	}

	// 构造返回数据
	type rechargeItem struct {
		ID           uint   `json:"id"`
		RechargeSN   string `json:"recharge_sn"`
		UserID       uint   `json:"user_id"`
		Amount       string `json:"amount"`
		Status       int8   `json:"status"`
		ApproverID   *uint  `json:"approver_id"`
		ApprovalNote string `json:"approval_note"`
		ApprovedAt   *int64 `json:"approved_at"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
	}

	items := make([]rechargeItem, 0, len(list))
	for _, r := range list {
		items = append(items, rechargeItem{
			ID:           r.ID,
			RechargeSN:   r.RechargeSN,
			UserID:       r.UserID,
			Amount:       r.Amount.StringFixed(2),
			Status:       r.Status,
			ApproverID:   r.ApproverID,
			ApprovalNote: r.ApprovalNote,
			ApprovedAt:   r.ApprovedAt,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ApproveRecharge POST /admin/recharges/:id/approve — 批准充值（到账）
func (h *Handler) ApproveRecharge(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)

	if err := h.RechargeSvc.Approve(context.Background(), id, adminID); err != nil {
		response.Error(c, fmt.Sprintf("批准充值失败: %v", err))
		return
	}

	// 记录审计日志
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "recharge.approve", "recharge", id,
		"批准充值到账",
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "approved",
	})
}

// RejectRecharge POST /admin/recharges/:id/reject — 拒绝充值
func (h *Handler) RejectRecharge(c *gin.Context) {
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

	if err := h.RechargeSvc.Reject(context.Background(), id, adminID, req.Note); err != nil {
		response.Error(c, fmt.Sprintf("拒绝充值失败: %v", err))
		return
	}

	// 记录审计日志
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "recharge.reject", "recharge", id,
		fmt.Sprintf("拒绝充值, 原因: %s", req.Note),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "rejected",
	})
}
