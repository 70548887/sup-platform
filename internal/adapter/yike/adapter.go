package yike

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/70548887/sup-platform/internal/adapter"
)

// YikeAdapter 易客供货商适配器
type YikeAdapter struct {
	config *Config
	client *http.Client
}

// NewYikeAdapter 创建易客适配器实例
func NewYikeAdapter(cfg *Config) *YikeAdapter {
	return &YikeAdapter{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 返回适配器名称
func (a *YikeAdapter) Name() string {
	return "yike"
}

// SubmitOrder 提交订单到易客上游
func (a *YikeAdapter) SubmitOrder(ctx context.Context, orderSN string, params adapter.SubmitParams) (*adapter.SubmitResult, error) {
	body := map[string]interface{}{
		"order_sn":          orderSN,
		"goods_sn":          params.GoodsSN,
		"goods_name":        params.GoodsName,
		"buy_number":        params.BuyNumber,
		"notify_url":        params.NotifyURL,
		"buy_params":        params.BuyParams,
		"customer_order_id": params.CustomerOrderID,
	}

	respBody, err := a.doWithRetry(ctx, "/order/submit", body)
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
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("yike: 解析响应失败: %v", err))
	}

	if resp.Code != 0 {
		return nil, adapter.NewBusinessError(resp.Code, fmt.Sprintf("yike: %s", resp.Message))
	}

	return &adapter.SubmitResult{
		ExternalOrderID: resp.Data.ExternalOrderID,
		Status:          resp.Data.Status,
		Message:         resp.Message,
	}, nil
}

// QueryOrder 查询易客上游订单状态
func (a *YikeAdapter) QueryOrder(ctx context.Context, externalOrderID string) (*adapter.QueryResult, error) {
	url := a.config.BaseURL + "/order/query?order_id=" + externalOrderID
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sign := a.sign(externalOrderID, timestamp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("yike: 构造查询请求失败: %w", err)
	}
	req.Header.Set("X-App-Id", a.config.AppId)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Sign", sign)

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, adapter.NewTimeoutError(fmt.Sprintf("yike: 查询超时: %v", err))
		}
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 网络错误: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 读取响应失败: %v", err))
	}

	// HTTP状态码分类
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, adapter.NewAuthError(fmt.Sprintf("yike: 认证失败, HTTP %d", resp.StatusCode))
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, adapter.NewBusinessError(resp.StatusCode, fmt.Sprintf("yike: 客户端错误, HTTP %d", resp.StatusCode))
	case resp.StatusCode >= 500:
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 服务端错误, HTTP %d", resp.StatusCode))
	}

	// 解析响应
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status int8     `json:"status"`
			Cards  []string `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("yike: 解析查询响应失败: %v", err))
	}

	if result.Code != 0 {
		return nil, adapter.NewBusinessError(result.Code, fmt.Sprintf("yike: %s", result.Message))
	}

	return &adapter.QueryResult{
		ExternalOrderID: externalOrderID,
		Status:          result.Data.Status,
		Cards:           result.Data.Cards,
		Message:         result.Message,
	}, nil
}

// ParseCallback 解析易客回调数据
func (a *YikeAdapter) ParseCallback(body []byte) (*adapter.CallbackData, error) {
	var cb struct {
		OrderSN         string   `json:"order_sn"`
		ExternalOrderID string   `json:"external_order_id"`
		Status          int8     `json:"status"`
		Cards           []string `json:"cards"`
		Message         string   `json:"message"`
	}
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("yike: 解析回调数据失败: %w", err)
	}

	return &adapter.CallbackData{
		ExternalOrderID: cb.ExternalOrderID,
		OrderSN:         cb.OrderSN,
		Status:          cb.Status,
		Cards:           cb.Cards,
		Message:         cb.Message,
	}, nil
}

// doWithRetry 带重试的请求执行，最多3次，指数退避
func (a *YikeAdapter) doWithRetry(ctx context.Context, path string, body interface{}) ([]byte, error) {
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
					return nil, adapter.NewTimeoutError("yike: 请求被取消")
				case <-time.After(backoffs[attempt]):
				}
			}
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

// doRequest 执行HTTP请求（JSON POST + HMAC-SHA256签名）
func (a *YikeAdapter) doRequest(ctx context.Context, path string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("yike: 序列化请求体失败: %v", err))
	}

	url := a.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 构造请求失败: %v", err))
	}

	// 签名: HMAC-SHA256(appSecret, requestBody + timestamp)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sign := a.sign(string(jsonBody), timestamp)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", a.config.AppId)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Sign", sign)

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, adapter.NewTimeoutError(fmt.Sprintf("yike: 请求超时: %v", err))
		}
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 网络错误: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 读取响应体失败: %v", err))
	}

	// HTTP状态码分类
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, adapter.NewAuthError(fmt.Sprintf("yike: 认证失败, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, adapter.NewBusinessError(resp.StatusCode, fmt.Sprintf("yike: 客户端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 500:
		return nil, adapter.NewNetworkError(fmt.Sprintf("yike: 服务端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	}

	return respBody, nil
}

// sign 计算HMAC-SHA256签名
func (a *YikeAdapter) sign(data, timestamp string) string {
	raw := data + timestamp
	mac := hmac.New(sha256.New, []byte(a.config.AppSecret))
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}

// 编译期接口合规检查
var _ adapter.DockingAdapter = (*YikeAdapter)(nil)
