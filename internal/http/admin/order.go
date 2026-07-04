package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/order"
)

// ListOrders GET /admin/orders — 订单分页列表
func (h *Handler) ListOrders(c *gin.Context) {
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
	query := h.DB.Model(&order.Order{})

	// 可选status过滤
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := strconv.ParseInt(statusStr, 10, 8)
		if err == nil {
			query = query.Where("status = ?", int8(s))
		}
	}

	// 可选customer_id过滤
	if customerStr := c.Query("customer_id"); customerStr != "" {
		cid, err := strconv.ParseUint(customerStr, 10, 32)
		if err == nil {
			query = query.Where("customer_id = ?", uint(cid))
		}
	}

	// 可选supplier_id过滤
	if supplierStr := c.Query("supplier_id"); supplierStr != "" {
		sid, err := strconv.ParseUint(supplierStr, 10, 32)
		if err == nil {
			query = query.Where("supplier_id = ?", uint(sid))
		}
	}

	// 可选时间范围过滤
	if startStr := c.Query("start_time"); startStr != "" {
		st, err := strconv.ParseInt(startStr, 10, 64)
		if err == nil {
			query = query.Where("created_at >= ?", st)
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		et, err := strconv.ParseInt(endStr, 10, 64)
		if err == nil {
			query = query.Where("created_at <= ?", et)
		}
	}

	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询订单总数失败")
		return
	}

	// 分页查询
	var orders []order.Order
	if err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&orders).Error; err != nil {
		response.Error(c, "查询订单列表失败")
		return
	}

	// 构造返回数据
	type orderItem struct {
		ID              uint   `json:"id"`
		OrderSN         string `json:"order_sn"`
		CustomerOrderID string `json:"customer_order_id"`
		CustomerID      uint   `json:"customer_id"`
		SupplierID      uint   `json:"supplier_id"`
		GoodsID         uint   `json:"goods_id"`
		GoodsSN         string `json:"goods_sn"`
		GoodsName       string `json:"goods_name"`
		BuyNumber       int    `json:"buy_number"`
		UnitPrice       string `json:"unit_price"`
		Amount          string `json:"amount"`
		RefundAmount    string `json:"refund_amount"`
		Status          int8   `json:"status"`
		CreatedAt       int64  `json:"created_at"`
		UpdatedAt       int64  `json:"updated_at"`
	}

	list := make([]orderItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, orderItem{
			ID:              o.ID,
			OrderSN:         o.OrderSN,
			CustomerOrderID: o.CustomerOrderID,
			CustomerID:      o.CustomerID,
			SupplierID:      o.SupplierID,
			GoodsID:         o.GoodsID,
			GoodsSN:         o.GoodsSN,
			GoodsName:       o.GoodsName,
			BuyNumber:       o.BuyNumber,
			UnitPrice:       o.UnitPrice.StringFixed(2),
			Amount:          o.Amount.StringFixed(2),
			RefundAmount:    o.RefundAmount.StringFixed(2),
			Status:          o.Status,
			CreatedAt:       o.CreatedAt,
			UpdatedAt:       o.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetOrder GET /admin/orders/:id — 订单详情
func (h *Handler) GetOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	ord, err := h.OrderSvc.GetOrderByID(context.Background(), id)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	response.Success(c, gin.H{
		"id":                ord.ID,
		"order_sn":          ord.OrderSN,
		"customer_order_id": ord.CustomerOrderID,
		"app_id":            ord.AppID,
		"customer_id":       ord.CustomerID,
		"supplier_id":       ord.SupplierID,
		"goods_id":          ord.GoodsID,
		"goods_sn":          ord.GoodsSN,
		"goods_name":        ord.GoodsName,
		"buy_number":        ord.BuyNumber,
		"unit_price":        ord.UnitPrice.StringFixed(2),
		"amount":            ord.Amount.StringFixed(2),
		"refund_amount":     ord.RefundAmount.StringFixed(2),
		"status":            ord.Status,
		"version":           ord.Version,
		"notify_url":        ord.NotifyURL,
		"remark":            ord.Remark,
		"paid_at":           ord.PaidAt,
		"completed_at":      ord.CompletedAt,
		"created_at":        ord.CreatedAt,
		"updated_at":        ord.UpdatedAt,
	})
}

// UpdateOrderStatus POST /admin/orders/:id/status — 手动状态变更
func (h *Handler) UpdateOrderStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Status int8   `json:"status" binding:"required"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "status", "请求参数格式错误")
		return
	}

	// 调用OrderService的TransitionStatus
	adminName := getAdminUsername(c)
	operator := "admin:" + adminName
	if err := h.OrderSvc.TransitionStatus(context.Background(), id, req.Status, operator, req.Remark); err != nil {
		response.Error(c, fmt.Sprintf("状态变更失败: %v", err))
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "order.status_change", "order", id,
		fmt.Sprintf("手动变更订单状态为%d, 备注: %s", req.Status, req.Remark),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": req.Status,
	})
}
