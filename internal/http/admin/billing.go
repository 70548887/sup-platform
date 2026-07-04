package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/billing"
)

// ListBillingPlans GET /admin/billing/plans — 套餐列表
func (h *Handler) ListBillingPlans(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	plans, err := h.BillingSvc.ListPlans(context.Background())
	if err != nil {
		response.Error(c, "查询套餐列表失败")
		return
	}

	type planItem struct {
		ID                  uint   `json:"id"`
		Name                string `json:"name"`
		DisplayName         string `json:"display_name"`
		MonthlyPrice        string `json:"monthly_price"`
		MaxAPICallsPerMonth int    `json:"max_api_calls_per_month"`
		MaxOrders           int    `json:"max_orders"`
		MaxAdmins           int    `json:"max_admins"`
		Features            string `json:"features"`
		Status              int8   `json:"status"`
	}

	list := make([]planItem, 0, len(plans))
	for _, p := range plans {
		list = append(list, planItem{
			ID:                  p.ID,
			Name:                p.Name,
			DisplayName:         p.DisplayName,
			MonthlyPrice:        p.MonthlyPrice.StringFixed(2),
			MaxAPICallsPerMonth: p.MaxAPICallsPerMonth,
			MaxOrders:           p.MaxOrders,
			MaxAdmins:           p.MaxAdmins,
			Features:            p.Features,
			Status:              p.Status,
		})
	}

	response.Success(c, gin.H{"list": list})
}

// CreateBillingPlan POST /admin/billing/plans — 创建套餐
func (h *Handler) CreateBillingPlan(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	var req struct {
		Name                string `json:"name" binding:"required"`
		DisplayName         string `json:"display_name" binding:"required"`
		MonthlyPrice        string `json:"monthly_price" binding:"required"`
		MaxAPICallsPerMonth int    `json:"max_api_calls_per_month" binding:"required"`
		MaxOrders           int    `json:"max_orders"`
		MaxAdmins           int    `json:"max_admins"`
		Features            string `json:"features"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	price, err := decimal.NewFromString(req.MonthlyPrice)
	if err != nil {
		response.ParamError(c, "monthly_price", "金额格式错误")
		return
	}

	plan := &billing.SubscriptionPlan{
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		MonthlyPrice:        price,
		MaxAPICallsPerMonth: req.MaxAPICallsPerMonth,
		MaxOrders:           req.MaxOrders,
		MaxAdmins:           req.MaxAdmins,
		Features:            req.Features,
		Status:              1,
	}

	if err := h.DB.Create(plan).Error; err != nil {
		response.Error(c, fmt.Sprintf("创建套餐失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "billing.plan_create", "subscription_plan", plan.ID,
		fmt.Sprintf("创建套餐: %s (%s)", plan.DisplayName, plan.Name),
	))

	response.Success(c, gin.H{
		"id":   plan.ID,
		"name": plan.Name,
	})
}

// ListBillingSubscriptions GET /admin/billing/subscriptions — 订阅分页列表
func (h *Handler) ListBillingSubscriptions(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
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

	var total int64
	query := h.DB.Model(&billing.TenantSubscription{})
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询订阅总数失败")
		return
	}

	var subs []billing.TenantSubscription
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&subs).Error; err != nil {
		response.Error(c, "查询订阅列表失败")
		return
	}

	type subItem struct {
		ID        uint   `json:"id"`
		TenantID  uint   `json:"tenant_id"`
		PlanID    uint   `json:"plan_id"`
		StartAt   int64  `json:"start_at"`
		EndAt     int64  `json:"end_at"`
		AutoRenew bool   `json:"auto_renew"`
		Status    string `json:"status"`
		CreatedAt int64  `json:"created_at"`
	}

	list := make([]subItem, 0, len(subs))
	for _, s := range subs {
		list = append(list, subItem{
			ID:        s.ID,
			TenantID:  s.TenantID,
			PlanID:    s.PlanID,
			StartAt:   s.StartAt,
			EndAt:     s.EndAt,
			AutoRenew: s.AutoRenew,
			Status:    s.Status,
			CreatedAt: s.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ListBillingInvoices GET /admin/billing/invoices — 账单分页列表
func (h *Handler) ListBillingInvoices(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
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

	var total int64
	query := h.DB.Model(&billing.Invoice{})

	// 可选tenant_id过滤
	if tenantStr := c.Query("tenant_id"); tenantStr != "" {
		tid, err := strconv.ParseUint(tenantStr, 10, 32)
		if err == nil {
			query = query.Where("tenant_id = ?", uint(tid))
		}
	}

	// 可选status过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询账单总数失败")
		return
	}

	var invoices []billing.Invoice
	offset := (page - 1) * size
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&invoices).Error; err != nil {
		response.Error(c, "查询账单列表失败")
		return
	}

	type invoiceItem struct {
		ID            uint   `json:"id"`
		TenantID      uint   `json:"tenant_id"`
		Month         string `json:"month"`
		PlanFee       string `json:"plan_fee"`
		OverageCharge string `json:"overage_charge"`
		TotalAmount   string `json:"total_amount"`
		Status        string `json:"status"`
		IssuedAt      int64  `json:"issued_at"`
		PaidAt        *int64 `json:"paid_at"`
		CreatedAt     int64  `json:"created_at"`
	}

	list := make([]invoiceItem, 0, len(invoices))
	for _, inv := range invoices {
		list = append(list, invoiceItem{
			ID:            inv.ID,
			TenantID:      inv.TenantID,
			Month:         inv.Month,
			PlanFee:       inv.PlanFee.StringFixed(2),
			OverageCharge: inv.OverageCharge.StringFixed(2),
			TotalAmount:   inv.TotalAmount.StringFixed(2),
			Status:        inv.Status,
			IssuedAt:      inv.IssuedAt,
			PaidAt:        inv.PaidAt,
			CreatedAt:     inv.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GenerateInvoice POST /admin/billing/invoices/generate — 生成月度账单
func (h *Handler) GenerateInvoice(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	var req struct {
		TenantID uint   `json:"tenant_id" binding:"required"`
		Month    string `json:"month" binding:"required"` // "2026-07"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "参数错误: tenant_id和month必填")
		return
	}

	// 校验month格式
	_, parseErr := time.Parse("2006-01", req.Month)
	if parseErr != nil {
		response.ParamError(c, "month", "月份格式错误，应为 2006-01")
		return
	}

	invoice, err := h.BillingSvc.GenerateMonthlyInvoice(c.Request.Context(), req.TenantID, req.Month)
	if err != nil {
		response.Error(c, fmt.Sprintf("生成账单失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "billing.invoice_generate", "invoice", invoice.ID,
		fmt.Sprintf("生成月度账单: tenant=%d month=%s amount=%s", req.TenantID, req.Month, invoice.TotalAmount.StringFixed(2)),
	))

	response.Success(c, gin.H{
		"id":           invoice.ID,
		"tenant_id":    invoice.TenantID,
		"month":        invoice.Month,
		"plan_fee":     invoice.PlanFee.StringFixed(2),
		"overage":      invoice.OverageCharge.StringFixed(2),
		"total_amount": invoice.TotalAmount.StringFixed(2),
		"status":       invoice.Status,
	})
}

// MarkInvoicePaid POST /admin/billing/invoices/:id/mark-paid — 标记账单已支付
func (h *Handler) MarkInvoicePaid(c *gin.Context) {
	if h.BillingSvc == nil {
		response.Error(c, "计费服务未启用")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	// 验证账单存在
	var invoice billing.Invoice
	if err := h.DB.First(&invoice, id).Error; err != nil {
		response.Error(c, "账单不存在")
		return
	}

	if invoice.Status == "paid" {
		response.Error(c, "账单已支付，无需重复操作")
		return
	}

	now := time.Now().Unix()
	if err := h.DB.Model(&billing.Invoice{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":  "paid",
		"paid_at": now,
	}).Error; err != nil {
		response.Error(c, fmt.Sprintf("标记支付失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "billing.invoice_mark_paid", "invoice", id,
		fmt.Sprintf("标记账单已支付: tenant=%d month=%s amount=%s", invoice.TenantID, invoice.Month, invoice.TotalAmount.StringFixed(2)),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": "paid",
	})
}
