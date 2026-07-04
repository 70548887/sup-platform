package adapter

import "context"

// DockingAdapter 供货商对接适配器统一接口
type DockingAdapter interface {
	// Name 适配器名称
	Name() string
	// SubmitOrder 提交订单到上游供货商
	SubmitOrder(ctx context.Context, orderSN string, params SubmitParams) (*SubmitResult, error)
	// QueryOrder 查询上游订单状态
	QueryOrder(ctx context.Context, externalOrderID string) (*QueryResult, error)
	// ParseCallback 解析上游回调数据
	ParseCallback(body []byte) (*CallbackData, error)
}