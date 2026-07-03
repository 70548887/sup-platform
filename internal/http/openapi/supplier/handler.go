package supplier

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
	"github.com/70548887/sup-platform/internal/module/order"
)

// Handler 供货端API处理器
type Handler struct {
	DB       *gorm.DB
	GoodsSvc *goods.GoodsService
	OrderSvc *order.OrderService
	CardSvc  *card.CardService
}

// GoodsPaging POST /openapi/supplier/Goods/Paging — 商品列表
func (h *Handler) GoodsPaging(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
		Status   *int8  `json:"status"`
		Name     string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	supplierID := userID
	filter := goods.GoodsFilter{
		SupplierID: &supplierID,
		Name:       req.Name,
		Status:     req.Status,
		Page:       req.Page,
		PageSize:   req.PageSize,
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
		})
	}

	response.Success(c, gin.H{
		"list":     items,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// GoodsShow POST /openapi/supplier/Goods/Show — 商品详情
func (h *Handler) GoodsShow(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

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

	// 权限校验：只能查看自己的商品
	if g.SupplierID != userID {
		response.Error(c, "商品不存在")
		return
	}

	// 解析buy_params JSON
	var buyParams interface{}
	if g.BuyParams != "" {
		if err := json.Unmarshal([]byte(g.BuyParams), &buyParams); err != nil {
			log.Printf("[WARN] supplier GoodsShow: json.Unmarshal buy_params failed, goods_sn=%s, err=%v", g.SerialNumber, err)
		}
	}
	if buyParams == nil {
		buyParams = []interface{}{}
	}

	// 解析images JSON
	var imageURLs interface{}
	if g.Images != "" {
		if err := json.Unmarshal([]byte(g.Images), &imageURLs); err != nil {
			log.Printf("[WARN] supplier GoodsShow: json.Unmarshal images failed, goods_sn=%s, err=%v", g.SerialNumber, err)
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
		"buy_min":       g.BuyMin,
		"buy_max":       g.BuyMax,
		"buy_base":      g.BuyBase,
		"is_close":      g.IsClose,
		"is_repeat":     g.IsRepeat,
		"buy_params":    buyParams,
		"image_urls":    imageURLs,
		"description":   g.Description,
	})
}

// GoodsEdit POST /openapi/supplier/Goods/Edit — 修改商品
func (h *Handler) GoodsEdit(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		GoodsSN  string           `json:"goods_sn"`
		Price    *decimal.Decimal `json:"price"`
		BuyMin   *int             `json:"buy_min"`
		BuyMax   *int             `json:"buy_max"`
		IsClose  *bool            `json:"is_close"`
		IsRepeat *bool            `json:"is_repeat"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.GoodsSN == "" {
		response.ParamError(c, "goods_sn", "商品编号不能为空")
		return
	}

	// 验证权限
	g, err := h.GoodsSvc.GetGoods(context.Background(), req.GoodsSN)
	if err != nil {
		response.Error(c, err.Error())
		return
	}
	if g.SupplierID != userID {
		response.Error(c, "商品不存在")
		return
	}

	params := goods.UpdateGoodsParams{
		Price:    req.Price,
		BuyMin:   req.BuyMin,
		BuyMax:   req.BuyMax,
		IsClose:  req.IsClose,
		IsRepeat: req.IsRepeat,
	}

	if err := h.GoodsSvc.UpdateGoods(context.Background(), req.GoodsSN, params); err != nil {
		response.Error(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"goods_sn": req.GoodsSN,
		"updated":  true,
	})
}

// GoodsEditPrice POST /openapi/supplier/Goods/EditPrice — 修改价格
func (h *Handler) GoodsEditPrice(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		GoodsSN string          `json:"goods_sn"`
		Price   decimal.Decimal `json:"price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.GoodsSN == "" {
		response.ParamError(c, "goods_sn", "商品编号不能为空")
		return
	}
	if req.Price.LessThanOrEqual(decimal.Zero) {
		response.ParamError(c, "price", "价格必须大于0")
		return
	}

	// 验证权限
	g, err := h.GoodsSvc.GetGoods(context.Background(), req.GoodsSN)
	if err != nil {
		response.Error(c, err.Error())
		return
	}
	if g.SupplierID != userID {
		response.Error(c, "商品不存在")
		return
	}

	params := goods.UpdateGoodsParams{
		Price: &req.Price,
	}

	if err := h.GoodsSvc.UpdateGoods(context.Background(), req.GoodsSN, params); err != nil {
		response.Error(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"goods_sn": req.GoodsSN,
		"price":    req.Price.StringFixed(2),
		"updated":  true,
	})
}

// OrderPaging POST /openapi/supplier/Order/Paging — 订单列表
func (h *Handler) OrderPaging(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
		Status   *int8  `json:"status"`
		GoodsSN  string `json:"goods_sn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	list, total, err := h.OrderSvc.ListBySupplier(context.Background(), userID, req.Status, req.GoodsSN, req.Page, req.PageSize)
	if err != nil {
		response.Error(c, "获取订单列表失败")
		return
	}

	type orderItem struct {
		OrderSN   string `json:"order_sn"`
		GoodsSN   string `json:"goods_sn"`
		BuyNumber int    `json:"buy_number"`
		Amount    string `json:"amount"`
		Status    int8   `json:"status"`
		CreatedAt int64  `json:"created_at"`
	}

	items := make([]orderItem, 0, len(list))
	for _, o := range list {
		items = append(items, orderItem{
			OrderSN:   o.OrderSN,
			GoodsSN:   o.GoodsSN,
			BuyNumber: o.BuyNumber,
			Amount:    o.Amount.StringFixed(2),
			Status:    o.Status,
			CreatedAt: o.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":     items,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// OrderShow POST /openapi/supplier/Order/Show — 订单详情
func (h *Handler) OrderShow(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		OrderSN string `json:"order_sn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderSN == "" {
		response.ParamError(c, "order_sn", "订单编号不能为空")
		return
	}

	ctx := context.Background()
	ord, err := h.OrderSvc.GetOrder(ctx, req.OrderSN)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	// 权限校验：供货商只能查看自己的订单
	if ord.SupplierID != userID {
		response.Error(c, "订单不存在")
		return
	}

	// 获取购买参数快照
	buyParamsValue := make(map[string]string)
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
		"order_sn":             ord.OrderSN,
		"goods_sn":             ord.GoodsSN,
		"goods_name":           ord.GoodsName,
		"buy_number":           ord.BuyNumber,
		"amount":               ord.Amount.StringFixed(2),
		"refund_amount":        ord.RefundAmount.StringFixed(2),
		"status":               ord.Status,
		"buy_params_value":     buyParamsValue,
		"callback_current_num": ord.CallbackCurrentNum,
		"callback_start_num":   ord.CallbackStartNum,
		"customer_sn":          strconv.FormatUint(uint64(ord.CustomerID), 10),
		"created_at":           ord.CreatedAt,
	}
	if ord.PaidAt != nil {
		data["paid_at"] = *ord.PaidAt
	}

	response.Success(c, data)
}

// OrderStatusHandle POST /openapi/supplier/Order/StatusHandle — 修改订单状态
func (h *Handler) OrderStatusHandle(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

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

	ctx := context.Background()
	ord, err := h.OrderSvc.GetOrder(ctx, req.OrderSN)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	// 权限校验
	if ord.SupplierID != userID {
		response.Error(c, "订单不存在")
		return
	}

	// 执行状态转移
	if err := h.OrderSvc.TransitionStatus(ctx, ord.ID, req.Status, "supplier", "供货商修改状态"); err != nil {
		response.Error(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"order_sn": req.OrderSN,
		"status":   req.Status,
	})
}

// OrderScheduleHandle POST /openapi/supplier/Order/ScheduleHandle — 修改订单进度
func (h *Handler) OrderScheduleHandle(c *gin.Context) {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		response.AuthError(c, "无法获取用户信息")
		return
	}

	var req struct {
		OrderSN            string `json:"order_sn"`
		CallbackCurrentNum int    `json:"callback_current_num"`
		CallbackStartNum   int    `json:"callback_start_num"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "", "请求参数格式错误")
		return
	}

	if req.OrderSN == "" {
		response.ParamError(c, "order_sn", "订单编号不能为空")
		return
	}

	ctx := context.Background()
	ord, err := h.OrderSvc.GetOrder(ctx, req.OrderSN)
	if err != nil {
		response.Error(c, "订单不存在")
		return
	}

	// 权限校验
	if ord.SupplierID != userID {
		response.Error(c, "订单不存在")
		return
	}

	// 更新进度
	result := h.DB.Table("orders").
		Where("id = ?", ord.ID).
		Updates(map[string]interface{}{
			"callback_start_num":   req.CallbackStartNum,
			"callback_current_num": req.CallbackCurrentNum,
		})
	if result.Error != nil {
		response.Error(c, "更新进度失败")
		return
	}

	response.Success(c, gin.H{
		"order_sn":             req.OrderSN,
		"callback_current_num": req.CallbackCurrentNum,
		"callback_start_num":   req.CallbackStartNum,
	})
}

// SendNotification 通知投递（内部方法，由订单创建时触发）
func (h *Handler) SendNotification(ord *order.Order) error {
	// TODO: 实现HTTP回调通知供货商
	// 当订单创建成功后，向供货商的NotifyURL发送通知
	if ord.NotifyURL == "" {
		return nil
	}
	// 后续实现异步通知投递
	return nil
}

// ensure imports are used
var _ = decimal.Zero
