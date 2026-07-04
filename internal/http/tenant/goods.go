package tenant

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/goods"
)

// ListGoods GET /tenant-admin/goods — 商品分页列表
func (h *Handler) ListGoods(c *gin.Context) {
	ctx := c.Request.Context()
	db := h.DB.WithContext(ctx)

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

	query := db.Model(&goods.Goods{})

	// 可选status过滤
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := strconv.ParseInt(statusStr, 10, 8)
		if err == nil {
			query = query.Where("status = ?", int8(s))
		}
	}

	// 可选分类过滤
	if catStr := c.Query("category_id"); catStr != "" {
		catID, err := strconv.ParseUint(catStr, 10, 32)
		if err == nil {
			query = query.Where("category_id = ?", uint(catID))
		}
	}

	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Error(c, "查询商品总数失败")
		return
	}

	// 分页查询
	var goodsList []goods.Goods
	if err := query.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&goodsList).Error; err != nil {
		response.Error(c, "查询商品列表失败")
		return
	}

	// 构造返回数据
	type goodsItem struct {
		ID           uint   `json:"id"`
		SerialNumber string `json:"serial_number"`
		CategoryID   uint   `json:"category_id"`
		SupplierID   uint   `json:"supplier_id"`
		Name         string `json:"name"`
		Price        string `json:"price"`
		CostPrice    string `json:"cost_price"`
		Stock        int    `json:"stock"`
		Status       int8   `json:"status"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
	}

	list := make([]goodsItem, 0, len(goodsList))
	for _, g := range goodsList {
		list = append(list, goodsItem{
			ID:           g.ID,
			SerialNumber: g.SerialNumber,
			CategoryID:   g.CategoryID,
			SupplierID:   g.SupplierID,
			Name:         g.Name,
			Price:        g.Price.StringFixed(2),
			CostPrice:    g.CostPrice.StringFixed(2),
			Stock:        g.Stock,
			Status:       g.Status,
			CreatedAt:    g.CreatedAt,
			UpdatedAt:    g.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// UpdateGoodsStatus PATCH /tenant-admin/goods/:id/status — 更新商品状态
func (h *Handler) UpdateGoodsStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "status", "请求参数格式错误")
		return
	}

	ctx := c.Request.Context()
	db := h.DB.WithContext(ctx)

	// 先确认商品存在且属于本租户（Scope自动过滤）
	var g goods.Goods
	if err := db.First(&g, id).Error; err != nil {
		response.Error(c, "商品不存在")
		return
	}

	// 更新状态
	if err := db.Model(&goods.Goods{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		response.Error(c, "更新商品状态失败")
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(ctx, audit.NewEntry(
		adminID, adminName, "tenant.goods.status_change", "goods", id,
		fmt.Sprintf("更新商品状态为%d", req.Status),
	))

	response.Success(c, gin.H{
		"id":     id,
		"status": req.Status,
	})
}
