package queue

// WebhookPayload Webhook投递任务载荷
type WebhookPayload struct {
	CallbackID uint   `json:"callback_id"`
	URL        string `json:"url"`
	Body       string `json:"body"`
}

// DockingPayload 对接提交任务载荷
type DockingPayload struct {
	TaskID  uint   `json:"task_id"`
	OrderSN string `json:"order_sn"`
}

// ReconciliationPayload 对账任务载荷
type ReconciliationPayload struct {
	Type string `json:"type"` // balance_check, cross_verify
}

// AnalyticsPayload 统计聚合任务载荷
type AnalyticsPayload struct {
	Date string `json:"date"` // "2026-07-04"
}
