package admin

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/reconciliation"
)

// RunReconciliation POST /admin/reconciliation/run — 触发对账任务
// body: {"type": "balance_check"} 或 {"type": "cross_verify"}
func (h *Handler) RunReconciliation(c *gin.Context) {
	var req struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "type", "请求参数格式错误")
		return
	}

	svc := reconciliation.NewReconciliationService(h.DB)
	ctx := c.Request.Context()

	var task *reconciliation.ReconciliationTask
	var err error

	switch req.Type {
	case reconciliation.TypeBalanceCheck:
		task, err = svc.RunBalanceCheck(ctx)
	case reconciliation.TypeCrossVerify:
		task, err = svc.RunCrossVerify(ctx)
	default:
		response.ParamError(c, "type", fmt.Sprintf("无效的对账类型: %s", req.Type))
		return
	}

	if err != nil {
		response.Error(c, fmt.Sprintf("创建对账任务失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(ctx, audit.NewEntry(
		adminID, adminName, "reconciliation.run", "reconciliation", task.ID,
		fmt.Sprintf("手动触发对账任务, 类型: %s, 任务ID: %d", req.Type, task.ID),
	))

	response.Success(c, gin.H{
		"id":           task.ID,
		"type":         task.Type,
		"status":       task.Status,
		"started_at":   task.StartedAt,
		"total_checked": task.TotalChecked,
		"error_count":  task.ErrorCount,
	})
}

// ListReconciliationTasks GET /admin/reconciliation/tasks — 对账任务分页列表
func (h *Handler) ListReconciliationTasks(c *gin.Context) {
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

	svc := reconciliation.NewReconciliationService(h.DB)
	ctx := c.Request.Context()

	tasks, total, err := svc.ListTasks(ctx, page, size)
	if err != nil {
		response.Error(c, "查询对账任务列表失败")
		return
	}

	type taskItem struct {
		ID           uint   `json:"id"`
		Type         string `json:"type"`
		Status       string `json:"status"`
		TotalChecked int    `json:"total_checked"`
		ErrorCount   int    `json:"error_count"`
		StartedAt    int64  `json:"started_at"`
		CompletedAt  *int64 `json:"completed_at"`
		CreatedAt    int64  `json:"created_at"`
	}

	list := make([]taskItem, 0, len(tasks))
	for _, t := range tasks {
		list = append(list, taskItem{
			ID:           t.ID,
			Type:         t.Type,
			Status:       t.Status,
			TotalChecked: t.TotalChecked,
			ErrorCount:   t.ErrorCount,
			StartedAt:    t.StartedAt,
			CompletedAt:  t.CompletedAt,
			CreatedAt:    t.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetReconciliationTask GET /admin/reconciliation/tasks/:id — 对账任务详情（含关联异常列表）
func (h *Handler) GetReconciliationTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	svc := reconciliation.NewReconciliationService(h.DB)
	ctx := c.Request.Context()

	task, err := svc.GetTask(ctx, id)
	if err != nil {
		response.Error(c, "对账任务不存在")
		return
	}

	// 同时获取该任务下的异常列表（前100条）
	errs, total, err := svc.ListErrors(ctx, id, 1, 100)
	if err != nil {
		response.Error(c, "查询对账异常列表失败")
		return
	}

	type errorItem struct {
		ID         uint   `json:"id"`
		TaskID     uint   `json:"task_id"`
		ErrorType  string `json:"error_type"`
		UserID     uint   `json:"user_id"`
		Expected   string `json:"expected"`
		Actual     string `json:"actual"`
		Difference string `json:"difference"`
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
		ResolvedBy string `json:"resolved_by"`
		CreatedAt  int64  `json:"created_at"`
		ResolvedAt *int64 `json:"resolved_at"`
	}

	errList := make([]errorItem, 0, len(errs))
	for _, e := range errs {
		errList = append(errList, errorItem{
			ID:         e.ID,
			TaskID:     e.TaskID,
			ErrorType:  e.ErrorType,
			UserID:     e.UserID,
			Expected:   e.Expected.StringFixed(6),
			Actual:     e.Actual.StringFixed(6),
			Difference: e.Difference.StringFixed(6),
			Status:     e.Status,
			Resolution: e.Resolution,
			ResolvedBy: e.ResolvedBy,
			CreatedAt:  e.CreatedAt,
			ResolvedAt: e.ResolvedAt,
		})
	}

	response.Success(c, gin.H{
		"id":            task.ID,
		"type":          task.Type,
		"status":        task.Status,
		"total_checked": task.TotalChecked,
		"error_count":   task.ErrorCount,
		"started_at":    task.StartedAt,
		"completed_at":  task.CompletedAt,
		"created_at":    task.CreatedAt,
		"errors":        errList,
		"errors_total":  total,
	})
}

// ResolveReconciliationError PATCH /admin/reconciliation/errors/:id — 手动处理对账异常
// body: {"action": "manual_fixed|ignored", "note": "..."}
func (h *Handler) ResolveReconciliationError(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Action string `json:"action" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "action", "请求参数格式错误")
		return
	}

	svc := reconciliation.NewReconciliationService(h.DB)
	ctx := c.Request.Context()

	// 从context获取操作者信息
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	operator := fmt.Sprintf("admin:%s", adminName)

	if err := svc.ResolveError(ctx, id, req.Action, req.Note, operator); err != nil {
		response.Error(c, fmt.Sprintf("处理对账异常失败: %v", err))
		return
	}

	// 记录审计日志
	h.AuditSvc.Log(ctx, audit.NewEntry(
		adminID, adminName, "reconciliation.resolve", "reconciliation", id,
		fmt.Sprintf("处理对账异常, action: %s, note: %s", req.Action, req.Note),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": req.Action,
	})
}
