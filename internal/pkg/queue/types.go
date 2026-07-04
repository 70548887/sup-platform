package queue

// 任务类型常量
const (
	TypeWebhookDeliver     = "webhook:deliver"
	TypeDockingSubmit      = "docking:submit"
	TypeReconciliationRun  = "reconciliation:run"
	TypeAnalyticsAggregate = "analytics:aggregate"
)
