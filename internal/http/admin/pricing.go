package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/audit"
	"github.com/70548887/sup-platform/internal/module/pricing"
)

// CreatePricingRule POST /admin/pricing/rules — 创建定价规则
func (h *Handler) CreatePricingRule(c *gin.Context) {
	var req struct {
		GoodsID         uint            `json:"goods_id" binding:"required"`
		RuleType        string          `json:"rule_type" binding:"required"`
		CustomerGroupID *uint           `json:"customer_group_id"`
		MinQuantity     int             `json:"min_quantity"`
		MaxQuantity     int             `json:"max_quantity"`
		Price           decimal.Decimal `json:"price"`
		DiscountPercent decimal.Decimal `json:"discount_percent"`
		PromotionName   string          `json:"promotion_name"`
		StartAt         *int64          `json:"start_at"`
		EndAt           *int64          `json:"end_at"`
		Priority        int             `json:"priority"`
		Status          int8            `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	// 默认值
	if req.MinQuantity <= 0 {
		req.MinQuantity = 1
	}
	if req.MaxQuantity <= 0 {
		req.MaxQuantity = 999999
	}
	if req.Status == 0 {
		req.Status = 1
	}

	rule := pricing.PricingRule{
		GoodsID:         req.GoodsID,
		RuleType:        req.RuleType,
		CustomerGroupID: req.CustomerGroupID,
		MinQuantity:     req.MinQuantity,
		MaxQuantity:     req.MaxQuantity,
		Price:           req.Price,
		DiscountPercent: req.DiscountPercent,
		PromotionName:   req.PromotionName,
		StartAt:         req.StartAt,
		EndAt:           req.EndAt,
		Priority:        req.Priority,
		Status:          req.Status,
	}

	if err := h.PricingSvc.CreateRule(c.Request.Context(), &rule); err != nil {
		if strings.Contains(err.Error(), "冲突") {
			c.JSON(http.StatusConflict, response.Response{
				Code:    409,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "create_rule", "pricing", rule.ID,
		fmt.Sprintf("创建定价规则 goods_id=%d type=%s price=%s priority=%d",
			rule.GoodsID, rule.RuleType, rule.Price.StringFixed(2), rule.Priority),
	))

	c.JSON(http.StatusCreated, response.Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"id":         rule.ID,
			"goods_id":   rule.GoodsID,
			"rule_type":  rule.RuleType,
			"price":      rule.Price.StringFixed(2),
			"priority":   rule.Priority,
			"version":    rule.Version,
			"created_at": rule.CreatedAt,
		},
	})
}

// ListPricingRules GET /admin/pricing/rules — 分页查询定价规则
func (h *Handler) ListPricingRules(c *gin.Context) {
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

	var goodsID *uint
	if gidStr := c.Query("goods_id"); gidStr != "" {
		gid, err := strconv.ParseUint(gidStr, 10, 32)
		if err == nil {
			g := uint(gid)
			goodsID = &g
		}
	}
	ruleType := c.Query("rule_type")

	rules, total, err := h.PricingSvc.ListRules(c.Request.Context(), goodsID, ruleType, page, size)
	if err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	type ruleItem struct {
		ID              uint   `json:"id"`
		GoodsID         uint   `json:"goods_id"`
		RuleType        string `json:"rule_type"`
		CustomerGroupID *uint  `json:"customer_group_id"`
		MinQuantity     int    `json:"min_quantity"`
		MaxQuantity     int    `json:"max_quantity"`
		Price           string `json:"price"`
		DiscountPercent string `json:"discount_percent"`
		PromotionName   string `json:"promotion_name"`
		Priority        int    `json:"priority"`
		Status          int8   `json:"status"`
		Version         int64  `json:"version"`
		CreatedAt       int64  `json:"created_at"`
		UpdatedAt       int64  `json:"updated_at"`
	}

	list := make([]ruleItem, 0, len(rules))
	for _, r := range rules {
		list = append(list, ruleItem{
			ID:              r.ID,
			GoodsID:         r.GoodsID,
			RuleType:        r.RuleType,
			CustomerGroupID: r.CustomerGroupID,
			MinQuantity:     r.MinQuantity,
			MaxQuantity:     r.MaxQuantity,
			Price:           r.Price.StringFixed(2),
			DiscountPercent: r.DiscountPercent.StringFixed(4),
			PromotionName:   r.PromotionName,
			Priority:        r.Priority,
			Status:          r.Status,
			Version:         r.Version,
			CreatedAt:       r.CreatedAt,
			UpdatedAt:       r.UpdatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// UpdatePricingRule PUT /admin/pricing/rules/:id — CAS更新定价规则
func (h *Handler) UpdatePricingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req struct {
		GoodsID         *uint            `json:"goods_id"`
		RuleType        *string          `json:"rule_type"`
		CustomerGroupID *uint            `json:"customer_group_id"`
		MinQuantity     *int             `json:"min_quantity"`
		MaxQuantity     *int             `json:"max_quantity"`
		Price           *decimal.Decimal `json:"price"`
		DiscountPercent *decimal.Decimal `json:"discount_percent"`
		PromotionName   *string          `json:"promotion_name"`
		StartAt         *int64           `json:"start_at"`
		EndAt           *int64           `json:"end_at"`
		Priority        *int             `json:"priority"`
		Status          *int8            `json:"status"`
		Version         int64            `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "version", "version字段必须传")
		return
	}

	// 先获取现有规则
	existing, err := h.PricingSvc.GetRule(c.Request.Context(), id)
	if err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 应用更新（仅更新非nil字段）
	updated := *existing
	if req.GoodsID != nil {
		updated.GoodsID = *req.GoodsID
	}
	if req.RuleType != nil {
		updated.RuleType = *req.RuleType
	}
	if req.CustomerGroupID != nil {
		updated.CustomerGroupID = req.CustomerGroupID
	}
	if req.MinQuantity != nil {
		updated.MinQuantity = *req.MinQuantity
	}
	if req.MaxQuantity != nil {
		updated.MaxQuantity = *req.MaxQuantity
	}
	if req.Price != nil {
		updated.Price = *req.Price
	}
	if req.DiscountPercent != nil {
		updated.DiscountPercent = *req.DiscountPercent
	}
	if req.PromotionName != nil {
		updated.PromotionName = *req.PromotionName
	}
	if req.StartAt != nil {
		updated.StartAt = req.StartAt
	}
	if req.EndAt != nil {
		updated.EndAt = req.EndAt
	}
	if req.Priority != nil {
		updated.Priority = *req.Priority
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	if err := h.PricingSvc.UpdateRule(c.Request.Context(), &updated, req.Version); err != nil {
		if strings.Contains(err.Error(), "版本冲突") {
			c.JSON(http.StatusConflict, response.Response{
				Code:    409,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "update_rule", "pricing", id,
		fmt.Sprintf("更新定价规则 id=%d version=%d->%d", id, req.Version, req.Version+1),
	))

	response.Success(c, gin.H{
		"id":         updated.ID,
		"goods_id":   updated.GoodsID,
		"rule_type":  updated.RuleType,
		"price":      updated.Price.StringFixed(2),
		"priority":   updated.Priority,
		"status":     updated.Status,
		"version":    updated.Version,
		"updated_at": updated.UpdatedAt,
	})
}

// DeletePricingRule DELETE /admin/pricing/rules/:id — 删除定价规则
func (h *Handler) DeletePricingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.PricingSvc.DeleteRule(c.Request.Context(), id); err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 审计日志
	adminID := getAdminUserID(c)
	adminName := getAdminUsername(c)
	h.AuditSvc.Log(context.Background(), audit.NewEntry(
		adminID, adminName, "delete_rule", "pricing", id,
		fmt.Sprintf("删除定价规则 id=%d", id),
	))

	response.Success(c, gin.H{
		"id": id,
	})
}

// CalcPricePreview POST /admin/pricing/calc-preview — 价格预览
func (h *Handler) CalcPricePreview(c *gin.Context) {
	var req struct {
		GoodsID    uint `json:"goods_id" binding:"required"`
		CustomerID uint `json:"customer_id"`
		Quantity   int  `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.Quantity <= 0 {
		response.ParamError(c, "quantity", "数量必须大于0")
		return
	}

	// 查询商品基础价
	var goods struct {
		Price decimal.Decimal
	}
	if err := h.DB.Table("goods").Select("price").Where("id = ?", req.GoodsID).Scan(&goods).Error; err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}
	if goods.Price.IsZero() {
		response.Error(c, "商品不存在或价格为0")
		return
	}

	basePrice := goods.Price
	finalPrice, err := h.PricingSvc.CalculatePrice(c.Request.Context(), req.GoodsID, req.CustomerID, req.Quantity, basePrice)
	if err != nil {
		response.Error(c, fmt.Errorf("admin: pricing: %w", err).Error())
		return
	}

	// 判断是否应用了规则
	ruleApplied := "none"
	if !finalPrice.Equal(basePrice) {
		ruleApplied = "unknown"
	}

	response.Success(c, gin.H{
		"base_price":   basePrice.StringFixed(2),
		"final_price":  finalPrice.StringFixed(2),
		"rule_applied": ruleApplied,
	})
}
