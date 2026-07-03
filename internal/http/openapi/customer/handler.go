package customer

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/internal/http/middleware"
	"github.com/70548887/sup-platform/internal/http/response"
	"github.com/70548887/sup-platform/internal/module/card"
	"github.com/70548887/sup-platform/internal/module/goods"
	"github.com/70548887/sup-platform/internal/module/ledger"
	"github.com/70548887/sup-platform/internal/module/order"
)

// Handler 客户端API处理器
type Handler struct {
	DB        *gorm.DB
	GoodsSvc  *goods.GoodsService
	OrderSvc  *order.OrderService
	CardSvc   *card.CardService
	LedgerSvc *ledger.LedgerService
}

// AccountShow GET /openapi/customer/CustomerAccount/Show — 账户信息
func (h *Handler) AccountShow(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	balance, err := h.LedgerSvc.GetBalance(context.Background(), userID)
	if err != nil {
		response.Error(c, "获取余额失败")
		return
	}

	// 查询用户基本信息
	response.Success(c, gin.H{
		"serial_number": strconv.FormatUint(uint64(userID), 10),
		"name":          "",
		"balance":       balance.StringFixed(2),
	})
}

// GoodsCategoryList GET /openapi/customer/Goods/CategoryList — 商品分类
func (h *Handler) GoodsCategoryList(c *gin.Context) {
	categories, err := h.GoodsSvc.GetCategories(context.Background(), nil)
	if err != nil {
		response.Error(c, "获取分类列表失败")
		return
	}

	type categoryItem struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		ParentID uint   `json:"parent_id"`
	}

	list := make([]categoryItem, 0, len(categories))
	for _, cat := range categories {
		list = append(list, categoryItem{
			ID:       cat.ID,
			Name:     cat.Name,
			ParentID: cat.ParentID,
		})
	}

	response.Success(c, gin.H{
		"list": list,
	})
}

// GoodsList POST /openapi/customer/Goods/List — 商品列表
func (h *Handler) GoodsList(c *gin.Context) {
	var req struct {
		Page         int    `json:"page"`
		PageSize     int    `json:"pageSize"`
		CategoryID   *uint  `json:"category_id"`
		Name         string `json:"name"`
		SerialNumber string `json:"serial_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 也允许GET参数
		req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
		req.PageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		req.Name = c.Query("name")
		req.SerialNumber = c.Query("serial_number")
		if catStr := c.Query("category_id"); catStr != "" {
			catID, _ := strconv.ParseUint(catStr, 10, 64)
			catIDUint := uint(catID)
			req.CategoryID = &catIDUint
		}
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	// 仅显示上架商品
	status := int8(1)
	filter := goods.GoodsFilter{
		CategoryID:   req.CategoryID,
		Name:         req.Name,
		SerialNumber: req.SerialNumber,
		Status:       &status,
		Page:         req.Page,
		PageSize:     req.PageSize,
	}

	list, total, err := h.GoodsSvc.ListGoods(context.Background(), filter)
	if err != nil {
		response.Error(c, "获取商品列表失败")
		return
	}

	type goodsItem struct {
		ID           uint   `json:"id"`
		SerialNumber string `json:"serial_number"`
		Name         string `json:"name"`
		Price        string `json:"price"`
		Stock        int    `json:"stock"`
		Status       int8   `json:"status"`
		CategoryID   uint   `json:"category_id"`
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
		})
	}

	response.Success(c, gin.H{
		"list":     items,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// GoodsShow POST /openapi/customer/Goods/Show — 商品详情
func (h *Handler) GoodsShow(c *gin.Context) {
	var req struct {
		GoodsSN string `json:"goods_sn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GoodsSN == "" {
		response.ParamError(c, "goods_sn", "商品编号不能为空")
		return
	}

	g, err := h.GoodsSvc.GetGoods(context.Background(), req.GoodsSN)
	if err != nil {
		response.Error(c, err.Error())
		return
	}

	// 解析buy_params JSON
	var buyParams interface{}
	if g.BuyParams != "" {
		if err := json.Unmarshal([]byte(g.BuyParams), &buyParams); err != nil {
			log.Printf("[WARN] customer GoodsShow: json.Unmarshal buy_params failed, goods_sn=%s, err=%v", g.SerialNumber, err)
		}
	}
	if buyParams == nil {
		buyParams = []interface{}{}
	}

	// 解析images JSON
	var imageURLs interface{}
	if g.Images != "" {
		if err := json.Unmarshal([]byte(g.Images), &imageURLs); err != nil {
			log.Printf("[WARN] customer GoodsShow: json.Unmarshal images failed, goods_sn=%s, err=%v", g.SerialNumber, err)
		}
	}
	if imageURLs == nil {
		imageURLs = []interface{}{}
	}

	response.Success(c, gin.H{
		"id":            g.ID,
		"serial_number": g.SerialNumber,
		"name":          g.Name,
		"price":         g.Price.StringFixed(2),
		"stock":         g.Stock,
		"status":        g.Status,
		"buy_params":    buyParams,
		"image_urls":    imageURLs,
		"description":   g.Description,
		"buy_min":       g.BuyMin,
		"buy_max":       g.BuyMax,
		"buy_base":      g.BuyBase,
		"is_repeat":     g.IsRepeat,
	})
}

// GoodsBuy POST /openapi/customer/Goods/Buy — 购买商品
func (h *Handler) GoodsBuy(c *gin.Context) {
	var req struct {
		GoodsSN         string            `json:"goods_sn"`
		BuyNumber       int               `json:"buy_number"`
		BuyParams       map[string]string `json:"buy_params"`
		CustomerOrderID string            `json:"customer_order_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.GoodsSN == "" {
		response.ParamError(c, "goods_sn", "商品编号不能为空")
		return
	}
	if req.BuyNumber <= 0 {
		response.ParamError(c, "buy_number", "购买数量必须大于0")
		return
	}

	// 从Context获取认证信息
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}
	appID, _ := middleware.GetAppIDFromContext(c)

	ctx := context.Background()

	// 1. 查询商品
	g, err := h.GoodsSvc.GetGoods(ctx, req.GoodsSN)
	if err != nil {
		response.Error(c, err.Error())
		return
	}

	// 2. 验证购买规则
	if err := h.GoodsSvc.ValidatePurchase(g, req.BuyNumber, userID); err != nil {
		response.Error(c, err.Error())
		return
	}

	// 3. 验证购买参数
	if err := h.GoodsSvc.ValidateBuyParams(g, req.BuyParams); err != nil {
		response.Error(c, err.Error())
		return
	}

	// 4. 计算AppID（数据库中存的是uint）—— 需要从string找到对应的记录ID
	// 中间件注入的app_id是string类型的AppId，这里需要用数字作为appID
	// 由于CreateOrderParams.AppID是uint，使用userID所属的app
	var appIDUint uint
	if appID != "" {
		// AppID存储在api_apps表中，这里简单使用userID
		appIDUint = userID
	}

	// 5. 创建订单
	orderParams := order.CreateOrderParams{
		AppID:           appIDUint,
		CustomerID:      userID,
		SupplierID:      g.SupplierID,
		CustomerOrderID: req.CustomerOrderID,
		GoodsID:         g.ID,
		GoodsSN:         g.SerialNumber,
		GoodsName:       g.Name,
		BuyNumber:       req.BuyNumber,
		UnitPrice:       g.Price,
		BuyParams:       req.BuyParams,
	}

	ord, err := h.OrderSvc.CreateOrder(ctx, orderParams)
	if err != nil {
		response.Error(c, err.Error())
		return
	}

	// 6. 如果是卡密商品，发放卡密
	var cardContents []string
	if g.IsCardProduct {
		cards, err := h.CardSvc.IssueCards(ctx, h.DB, g.ID, ord.ID, req.BuyNumber)
		if err != nil {
			// 卡密发放失败，订单已创建但需记录异常
			response.Success(c, gin.H{
				"order_sn":      ord.OrderSN,
				"goods_sn":      ord.GoodsSN,
				"buy_number":    ord.BuyNumber,
				"amount":        ord.Amount.StringFixed(2),
				"status":        ord.Status,
				"card_code_ids": []string{},
				"created_at":    ord.CreatedAt,
				"error":         "卡密发放失败: " + err.Error(),
			})
			return
		}
		for _, c := range cards {
			cardContents = append(cardContents, c.Content)
		}
	}

	response.Success(c, gin.H{
		"order_sn":      ord.OrderSN,
		"goods_sn":      ord.GoodsSN,
		"buy_number":    ord.BuyNumber,
		"amount":        ord.Amount.StringFixed(2),
		"status":        ord.Status,
		"card_code_ids": cardContents,
		"created_at":    ord.CreatedAt,
	})
}

// OrderShow POST /openapi/customer/Order/Show — 订单查询
func (h *Handler) OrderShow(c *gin.Context) {
	var req struct {
		OrderSN string `json:"order_sn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderSN == "" {
		response.ParamError(c, "order_sn", "订单编号不能为空")
		return
	}

	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	ctx := context.Background()
	ord, err := h.OrderSvc.GetOrder(ctx, req.OrderSN)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	// 权限校验：只能查自己的订单
	if ord.CustomerID != userID {
		response.Error(c, "订单不存在")
		return
	}

	// 获取卡密内容
	var cardContents []string
	cards, err := h.CardSvc.GetOrderCards(ctx, ord.ID)
	if err == nil && len(cards) > 0 {
		for _, c := range cards {
			cardContents = append(cardContents, c.Content)
		}
	}
	if cardContents == nil {
		cardContents = []string{}
	}

	// 获取购买参数快照（从order_buy_params表获取）
	buyParamsValue := make(map[string]string)
	// 简单查询
	type buyParamRow struct {
		Name  string
		Value string
	}
	var rows []buyParamRow
	if err := h.DB.Table("order_buy_params").
		Where("order_id = ?", ord.ID).
		Find(&rows).Error; err != nil {
		response.Error(c, "获取订单参数失败")
		return
	}
	for _, r := range rows {
		buyParamsValue[r.Name] = r.Value
	}

	data := gin.H{
		"order_sn":        ord.OrderSN,
		"goods_sn":        ord.GoodsSN,
		"buy_number":      ord.BuyNumber,
		"amount":          ord.Amount.StringFixed(2),
		"refund_amount":   ord.RefundAmount.StringFixed(2),
		"status":          ord.Status,
		"card_code_ids":   cardContents,
		"buy_params_value": buyParamsValue,
		"created_at":      ord.CreatedAt,
	}
	if ord.PaidAt != nil {
		data["paid_at"] = *ord.PaidAt
	}

	response.Success(c, data)
}

// OrderStatusHandle POST /openapi/customer/Order/StatusHandle — 订单操作（退单申请）
func (h *Handler) OrderStatusHandle(c *gin.Context) {
	var req struct {
		OrderSN string `json:"order_sn"`
		Status  int8   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.OrderSN == "" {
		response.ParamError(c, "order_sn", "订单编号不能为空")
		return
	}

	// 客户仅允许申请退单（状态5-退单中）
	if req.Status != order.StatusRefunding {
		response.Error(c, "客户仅允许申请退单操作")
		return
	}

	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	ctx := context.Background()
	ord, err := h.OrderSvc.GetOrder(ctx, req.OrderSN)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	// 权限校验
	if ord.CustomerID != userID {
		response.Error(c, "订单不存在")
		return
	}

	// 执行状态转移
	if err := h.OrderSvc.TransitionStatus(ctx, ord.ID, req.Status, "customer", "客户申请退单"); err != nil {
		response.Error(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"order_sn": req.OrderSN,
		"status":   req.Status,
	})
}

// ensure decimal is imported
var _ = decimal.Zero
