package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// ListFailedDockingTasks GET /admin/docking-tasks/failed — 失败任务列表
func (h *Handler) ListFailedDockingTasks(c *gin.Context) {
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

	list, total, err := h.DockingSvc.GetFailedTasks(context.Background(), page, size)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询失败任务列表失败: %v", err))
		return
	}

	// 构造返回数据
	type taskItem struct {
		ID              uint   `json:"id"`
		OrderID         uint   `json:"order_id"`
		SupplierID      uint   `json:"supplier_id"`
		ExternalOrderID string `json:"external_order_id"`
		Status          int8   `json:"status"`
		RetryCount      int    `json:"retry_count"`
		MaxRetry        int    `json:"max_retry"`
		ErrorMessage    string `json:"error_message"`
		IsManualRetry   bool   `json:"is_manual_retry"`
		LastFailureAt   *int64 `json:"last_failure_at"`
		CreatedAt       int64  `json:"created_at"`
		UpdatedAt       int64  `json:"updated_at"`
	}

	items := make([]taskItem, 0, len(list))
	for _, t := range list {
		items = append(items, taskItem{
			ID:              t.ID,
			OrderID:         t.OrderID,
			SupplierID:      t.SupplierID,
			ExternalOrderID: t.ExternalOrderID,
			Status:          t.Status,
			RetryCount:      t.RetryCount,
			MaxRetry:        t.MaxRetry,
			ErrorMessage:    t.ErrorMessage,
			IsManualRetry:   t.IsManualRetry,
			LastFailureAt:   t.LastFailureAt,
			CreatedAt:       t.CreatedAt,
			UpdatedAt:       t.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// RetryDockingTask POST /admin/docking-tasks/:id/retry — 手动重试
func (h *Handler) RetryDockingTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.DockingSvc.ManualRetry(context.Background(), id); err != nil {
		response.Error(c, fmt.Sprintf("重试失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "docking.manual_retry", "docking_task", id,
		fmt.Sprintf("手动重试对接任务 #%d", id),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "retrying",
	})
}

// GetDockingStats GET /admin/docking-tasks/stats — 对接统计
func (h *Handler) GetDockingStats(c *gin.Context) {
	// 可选supplier_id过滤
	var supplierID uint
	if sidStr := c.Query("supplier_id"); sidStr != "" {
		sid, err := strconv.ParseUint(sidStr, 10, 32)
		if err == nil {
			supplierID = uint(sid)
		}
	}

	total, failed, err := h.DockingSvc.GetFailureStats(context.Background(), supplierID)
	if err != nil {
		response.Error(c, fmt.Sprintf("查询对接统计失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"total":  total,
		"failed": failed,
	})
}
