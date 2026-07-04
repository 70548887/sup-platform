package adapter

// SubmitParams 提交订单参数
type SubmitParams struct {
	GoodsSN         string            // 商品编号
	GoodsName       string            // 商品名称
	BuyNumber       int               // 购买数量
	NotifyURL       string            // 回调通知URL
	BuyParams       map[string]string // 购买参数（如充值账号等）
	CustomerOrderID string            // 客户方订单号
}

// SubmitResult 提交订单结果
type SubmitResult struct {
	ExternalOrderID string // 上游返回的订单号
	Status          int8   // 上游订单状态
	Message         string // 响应消息
}

// QueryResult 查询订单结果
type QueryResult struct {
	ExternalOrderID string   // 上游订单号
	Status          int8     // 上游状态
	Cards           []string // 卡密列表（如果有）
	Message         string
}

// CallbackData 回调数据解析结果
type CallbackData struct {
	ExternalOrderID string   // 上游订单号
	OrderSN         string   // 平台订单号
	Status          int8     // 状态
	Cards           []string // 卡密
	Message         string
}