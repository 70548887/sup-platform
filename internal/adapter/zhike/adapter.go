package zhike

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/70548887/sup-platform/internal/adapter"
)

// ZhikeAdapter 直客供货商适配器（直接对接供货商API，form-urlencoded格式）
type ZhikeAdapter struct {
	config *Config
	client *http.Client
}

// NewZhikeAdapter 创建直客适配器实例
func NewZhikeAdapter(cfg *Config) *ZhikeAdapter {
	return &ZhikeAdapter{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 返回适配器名称
func (a *ZhikeAdapter) Name() string {
	return "zhike"
}

// SubmitOrder 提交订单到直客上游
func (a *ZhikeAdapter) SubmitOrder(ctx context.Context, orderSN string, params adapter.SubmitParams) (*adapter.SubmitResult, error) {
	formData := url.Values{
		"order_sn":          {orderSN},
		"goods_sn":          {params.GoodsSN},
		"goods_name":        {params.GoodsName},
		"buy_number":        {strconv.Itoa(params.BuyNumber)},
		"notify_url":        {params.NotifyURL},
		"customer_order_id": {params.CustomerOrderID},
	}
	// 将buy_params序列化为JSON放入表单
	if len(params.BuyParams) > 0 {
		bp, _ := json.Marshal(params.BuyParams)
		formData.Set("buy_params", string(bp))
	}

	respBody, err := a.doWithRetry(ctx, "/api/order/create", formData)
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
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("zhike: 解析响应失败: %v", err))
	}

	if resp.Code != 0 {
		return nil, adapter.NewBusinessError(resp.Code, fmt.Sprintf("zhike: %s", resp.Message))
	}

	return &adapter.SubmitResult{
		ExternalOrderID: resp.Data.ExternalOrderID,
		Status:          resp.Data.Status,
		Message:         resp.Message,
	}, nil
}

// QueryOrder 查询直客上游订单状态
func (a *ZhikeAdapter) QueryOrder(ctx context.Context, externalOrderID string) (*adapter.QueryResult, error) {
	formData := url.Values{
		"external_order_id": {externalOrderID},
	}

	respBody, err := a.doWithRetry(ctx, "/api/order/query", formData)
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
		return nil, adapter.NewBusinessError(0, fmt.Sprintf("zhike: 解析查询响应失败: %v", err))
	}

	if resp.Code != 0 {
		return nil, adapter.NewBusinessError(resp.Code, fmt.Sprintf("zhike: %s", resp.Message))
	}

	return &adapter.QueryResult{
		ExternalOrderID: externalOrderID,
		Status:          resp.Data.Status,
		Cards:           resp.Data.Cards,
		Message:         resp.Message,
	}, nil
}

// ParseCallback 解析直客回调数据
func (a *ZhikeAdapter) ParseCallback(body []byte) (*adapter.CallbackData, error) {
	var cb struct {
		OrderSN         string   `json:"order_sn"`
		ExternalOrderID string   `json:"external_order_id"`
		Status          int8     `json:"status"`
		Cards           []string `json:"cards"`
		Message         string   `json:"message"`
	}
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("zhike: 解析回调数据失败: %w", err)
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
func (a *ZhikeAdapter) doWithRetry(ctx context.Context, path string, formData url.Values) ([]byte, error) {
	backoffs := []time.Duration{100 * time.Millisecond, 400 * time.Millisecond, 1600 * time.Millisecond}
	maxAttempts := 3

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		respBody, err := a.doRequest(ctx, path, formData)
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
					return nil, adapter.NewTimeoutError("zhike: 请求被取消")
				case <-time.After(backoffs[attempt]):
				}
			}
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

// doRequest 执行HTTP请求（form-urlencoded POST + MD5签名）
func (a *ZhikeAdapter) doRequest(ctx context.Context, path string, formData url.Values) ([]byte, error) {
	reqURL := a.config.BaseURL + path

	// 签名: MD5(appId + timestamp + appSecret)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sign := a.sign(timestamp)

	// 附加签名参数到表单
	formData.Set("app_id", a.config.AppId)
	formData.Set("timestamp", timestamp)
	formData.Set("sign", sign)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("zhike: 构造请求失败: %v", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, adapter.NewTimeoutError(fmt.Sprintf("zhike: 请求超时: %v", err))
		}
		return nil, adapter.NewNetworkError(fmt.Sprintf("zhike: 网络错误: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, adapter.NewNetworkError(fmt.Sprintf("zhike: 读取响应体失败: %v", err))
	}

	// HTTP状态码分类
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, adapter.NewAuthError(fmt.Sprintf("zhike: 认证失败, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, adapter.NewBusinessError(resp.StatusCode, fmt.Sprintf("zhike: 客户端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	case resp.StatusCode >= 500:
		return nil, adapter.NewNetworkError(fmt.Sprintf("zhike: 服务端错误, HTTP %d: %s", resp.StatusCode, string(respBody)))
	}

	return respBody, nil
}

// sign 计算MD5签名: MD5(appId + timestamp + appSecret)
func (a *ZhikeAdapter) sign(timestamp string) string {
	raw := a.config.AppId + timestamp + a.config.AppSecret
	h := md5.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

// 编译期接口合规检查
var _ adapter.DockingAdapter = (*ZhikeAdapter)(nil)
