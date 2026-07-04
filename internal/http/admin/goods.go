package admin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/goods"
)

// ListGoods 商品列表
// @Summary 商品分页列表
// @Description 获取商品列表，支持状态/供货商/关键字筛选
// @Tags Admin-商品管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param status query int false "商品状态"
// @Param supplier_id query int false "供货商ID"
// @Param keyword query string false "关键字搜索"
// @Success 200 {object} map[string]interface{}
// @Router /admin/goods [get]
func (h *Handler) ListGoods(c *gin.Context) {
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
	filter := goods.GoodsFilter{
		Page:     page,
		PageSize: size,
	}

	// 可选status过滤
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := strconv.ParseInt(statusStr, 10, 8)
		if err == nil {
			status := int8(s)
			filter.Status = &status
		}
	}

	// 可选supplier_id过滤
	if supplierStr := c.Query("supplier_id"); supplierStr != "" {
		sid, err := strconv.ParseUint(supplierStr, 10, 32)
		if err == nil {
			supplierID := uint(sid)
			filter.SupplierID = &supplierID
		}
	}

	list, total, err := h.GoodsSvc.ListGoods(context.Background(), filter)
	if err != nil {
		response.Error(c, "获取商品列表失败")
		return
	}

	// 构造返回数据
	type goodsItem struct {
		ID           uint   `json:"id"`
		SerialNumber string `json:"serial_number"`
		Name         string `json:"name"`
		Price        string `json:"price"`
		CostPrice    string `json:"cost_price"`
		Stock        int    `json:"stock"`
		Status       int8   `json:"status"`
		CategoryID   uint   `json:"category_id"`
		SupplierID   uint   `json:"supplier_id"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
	}

	items := make([]goodsItem, 0, len(list))
	for _, g := range list {
		items = append(items, goodsItem{
			ID:           g.ID,
			SerialNumber: g.SerialNumber,
			Name:         g.Name,
			Price:        g.Price.StringFixed(2),
			CostPrice:    g.CostPrice.StringFixed(2),
			Stock:        g.Stock,
			Status:       g.Status,
			CategoryID:   g.CategoryID,
			SupplierID:   g.SupplierID,
			CreatedAt:    g.CreatedAt,
			UpdatedAt:    g.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// UpdateGoodsStatus PATCH /admin/goods/:id/status — 商品上下架
func (h *Handler) UpdateGoodsStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	// 校验status值
	if req.Status != 0 && req.Status != 1 {
		response.ParamError(c, "status", "status只能是0(下架)或1(上架)")
		return
	}

	// 检查商品是否存在
	var g goods.Goods
	if err := h.DB.Where("id = ?", id).First(&g).Error; err != nil {
		response.Error(c, "商品不存在")
		return
	}

	// 更新状态
	if err := h.DB.Model(&goods.Goods{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		response.Error(c, "更新商品状态失败")
		return
	}

	// 记录审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	statusText := "上架"
	if req.Status == 0 {
		statusText = "下架"
	}
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "goods.status_change", "goods", id,
		fmt.Sprintf("%s商品: %s(%s)", statusText, g.Name, g.SerialNumber),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": req.Status,
	})
}

// ListPendingGoods GET /admin/goods/pending — 待审核商品列表
func (h *Handler) ListPendingGoods(c *gin.Context) {
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

	// 查询status=0（下架/待审核状态）的商品
	status := int8(0)
	filter := goods.GoodsFilter{
		Status:   &status,
		Page:     page,
		PageSize: size,
	}

	list, total, err := h.GoodsSvc.ListGoods(context.Background(), filter)
	if err != nil {
		response.Error(c, "获取待审核商品列表失败")
		return
	}

	// 构造返回数据
	type goodsItem struct {
		ID           uint   `json:"id"`
		SerialNumber string `json:"serial_number"`
		Name         string `json:"name"`
		Price        string `json:"price"`
		Stock        int    `json:"stock"`
		Status       int8   `json:"status"`
		CategoryID   uint   `json:"category_id"`
		SupplierID   uint   `json:"supplier_id"`
		CreatedAt    int64  `json:"created_at"`
	}

	items := make([]goodsItem, 0, len(list))
	for _, g := range list {
		items = append(items, goodsItem{
			ID:           g.ID,
			SerialNumber: g.SerialNumber,
			Name:         g.Name,
			Price:        g.Price.StringFixed(2),
			Stock:        g.Stock,
			Status:       g.Status,
			CategoryID:   g.CategoryID,
			SupplierID:   g.SupplierID,
			CreatedAt:    g.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ApproveGoods POST /admin/goods/:id/approve — 批准商品（审核通过→上架）
func (h *Handler) ApproveGoods(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	// 检查商品是否存在
	var g goods.Goods
	if err := h.DB.Where("id = ?", id).First(&g).Error; err != nil {
		response.Error(c, "商品不存在")
		return
	}

	// 检查商品当前状态（只有status=0的商品可审核通过）
	if g.Status != 0 {
		response.Error(c, "该商品不在待审核状态")
		return
	}

	// 审核通过：设置status=1（上架）
	if err := h.DB.Model(&goods.Goods{}).Where("id = ?", id).Update("status", 1).Error; err != nil {
		response.Error(c, "审核操作失败")
		return
	}

	// 记录审计日志（审核人ID + 时间）
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "goods.approve", "goods", id,
		fmt.Sprintf("审核通过商品: %s(%s)", g.Name, g.SerialNumber),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": int8(1),
	})
}

// RejectGoods POST /admin/goods/:id/reject — 拒绝商品
func (h *Handler) RejectGoods(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "reason", "拒绝原因不能为空")
		return
	}

	// 检查商品是否存在
	var g goods.Goods
	if err := h.DB.Where("id = ?", id).First(&g).Error; err != nil {
		response.Error(c, "商品不存在")
		return
	}

	// 检查商品当前状态
	if g.Status != 0 {
		response.Error(c, "该商品不在待审核状态")
		return
	}

	// 保持status=0，记录拒绝原因到审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "goods.reject", "goods", id,
		fmt.Sprintf("拒绝商品: %s(%s), 原因: %s", g.Name, g.SerialNumber, req.Reason),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": g.Status,
		"reason": req.Reason,
	})
}
