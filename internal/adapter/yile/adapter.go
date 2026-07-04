package yile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/70548887/sup-platform/internal/adapter"
	"github.com/70548887/sup-platform/internal/pkg/signature"
)

// YileAdapter 亿乐供货商适配器
type YileAdapter struct {
	config *Config
	client *http.Client
}

// NewYileAdapter 创建亿乐适配器实例
func NewYileAdapter(cfg *Config) *YileAdapter {
	return &YileAdapter{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 返回适配器名称
func (a *YileAdapter) Name() string {
	return "yile"
}

// SubmitOrder 提交订单到亿乐上游
func (a *YileAdapter) SubmitOrder(ctx context.Context, orderSN string, params adapter.SubmitParams) (*adapter.SubmitResult, error) {
	body := map[string]interface{}{
		"order_sn":          orderSN,
		"goods_sn":          params.GoodsSN,
		"goods_name":        params.GoodsName,
		"buy_number":        params.BuyNumber,
		"notify_url":        params.NotifyURL,
		"buy_params":        params.BuyParams,
		"customer_order_id": params.CustomerOrderID,
	}

	respBody, err := a.doWithRetry(ctx, "/api/order/create", body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ExternalOrderID string `json:"external_order_id"`
			Status          int8   `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("解析响应失败: %v", err))
	}

	if resp.Code != 0 {
		return nil, adapter.NewBusinessError(resp.Code, resp.Message)
	}

	return &adapter.SubmitResult{
		ExternalOrderID: resp.Data.ExternalOrderID,
		Status:          resp.Data.Status,
		Message:         resp.Message,
	}, nil
}

// QueryOrder 查询亿乐上游订单状态
func (a *YileAdapter) QueryOrder(ctx context.Context, externalOrderID string) (*adapter.QueryResult, error) {
	body := map[string]interface{}{
		"external_order_id": externalOrderID,
	}

	respBody, err := a.doWithRetry(ctx, "/api/order/query", body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status int8     `json:"status"`
			Cards  []string `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("解析响应失败: %v", err))
	}

	if resp.Code != 0 {
		return nil, adapter.NewBusinessError(resp.Code, resp.Message)
	}

	return &adapter.QueryResult{
		ExternalOrderID: externalOrderID,
		Status:          resp.Data.Status,
		Cards:           resp.Data.Cards,
		Message:         resp.Message,
	}, nil
}

// ParseCallback 解析亿乐回调数据
func (a *YileAdapter) ParseCallback(body []byte) (*adapter.CallbackData, error) {
	var cb struct {
		OrderSN         string   `json:"order_sn"`
		ExternalOrderID string   `json:"external_order_id"`
		Status          int8     `json:"status"`
		Cards           []string `json:"cards"`
	}
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("解析回调数据失败: %w", err)
	}

	return &adapter.CallbackData{
		ExternalOrderID: cb.ExternalOrderID,
		OrderSN:         cb.OrderSN,
		Status:          cb.Status,
		Cards:           cb.Cards,
	}, nil
}

// doWithRetry 带重试的请求执行，最多3次，指数退避
func (a *YileAdapter) doWithRetry(ctx context.Context, path string, body interface{}) ([]byte, error) {
	// 重试间隔: 100ms, 400ms, 1600ms
	backoffs := []time.Duration{100 * time.Millisecond, 400 * time.Millisecond, 1600 * time.Millisecond}
	maxAttempts := 3

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		respBody, err := a.doRequest(ctx, path, body)
		if err == nil {
			return respBody, nil
		}
		lastErr = err

		// 仅在错误可重试时继续
		var submitErr *adapter.SubmitError
		if errors.As(err, &submitErr) && submitErr.Retryable {
			if attempt < maxAttempts-1 {
				select {
				case <-ctx.Done():
					return nil, adapter.NewTimeoutError("请求被取消")
				case <-time.After(backoffs[attempt]):
					// 继续重试
				}
			}
			continue
		}
		// 不可重试错误，立即返回
		return nil, err
	}
	return nil, lastErr
}

// doRequest 统一HTTP请求执行（构造签名+发送+错误分类）
func (a *YileAdapter) doRequest(ctx context.Context, path string, body interface{}) ([]byte, error) {
	// 序列化请求体
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("序列化请求体失败: %v", err))
	}

	// 构造HTTP请求
	url := a.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("构造请求失败: %v", err))
	}

	// 生成签名
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	token := signature.LegacySHA1(a.config.AppId, a.config.AppSecret, path, timestamp)

	// 设置Header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Appid", a.config.AppId)
	req.Header.Set("AppTimestamp", timestamp)
	req.Header.Set("AppToken", token)

	// 发送请求
	resp, err := a.client.Do(req)
	if err != nil {
		// 错误分类
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, adapter.NewTimeoutError(fmt.Sprintf("请求超时: %v", err))
		}
		return nil, adapter.NewNetworkError(fmt.Sprintf("网络错误: %v", err))
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("读取响应体失败: %v", err))
	}

	// HTTP状态码分类
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, adapter.NewAuthError(fmt.Sprintf("认证失败, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, adapter.NewBusinessError(resp.StatusCode, fmt.Sprintf("客户端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 500:
		return nil, adapter.NewNetworkError(fmt.Sprintf("服务端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	}

	return respBody, nil
}

// 编译期接口合规检查
var _ adapter.DockingAdapter = (*YileAdapter)(nil)
