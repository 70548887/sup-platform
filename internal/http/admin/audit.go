package admin

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
)

// ListAuditLogs GET /admin/audit — 分页查询审计日志
func (h *Handler) ListAuditLogs(c *gin.Context) {
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

	// 构建过滤条件
	filter := audit.AuditFilter{}

	// 可选user_id过滤
	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err == nil {
			filter.UserID = uint(uid)
		}
	}

	// 可选action过滤
	if action := c.Query("action"); action != "" {
		filter.Action = action
	}

	// 可选resource过滤
	if resource := c.Query("resource"); resource != "" {
		filter.Resource = resource
	}

	// 可选时间范围过滤
	if startStr := c.Query("start_time"); startStr != "" {
		st, err := strconv.ParseInt(startStr, 10, 64)
		if err == nil {
			filter.StartTime = st
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		et, err := strconv.ParseInt(endStr, 10, 64)
		if err == nil {
			filter.EndTime = et
		}
	}

	// 调用AuditSvc.Query
	list, total, err := h.AuditSvc.Query(context.Background(), filter, page, size)
	if err != nil {
		response.Error(c, "查询审计日志失败")
		return
	}

	// 构造返回数据
	type auditItem struct {
		ID         uint   `json:"id"`
		UserID     uint   `json:"user_id"`
		Username   string `json:"username"`
		Action     string `json:"action"`
		Resource   string `json:"resource"`
		ResourceID uint   `json:"resource_id"`
		Detail     string `json:"detail"`
		IP         string `json:"ip"`
		CreatedAt  int64  `json:"created_at"`
	}

	items := make([]auditItem, 0, len(list))
	for _, l := range list {
		items = append(items, auditItem{
			ID:         l.ID,
			UserID:     l.UserID,
			Username:   l.Username,
			Action:     l.Action,
			Resource:   l.Resource,
			ResourceID: l.ResourceID,
			Detail:     l.Detail,
			IP:         l.IP,
			CreatedAt:  l.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}
