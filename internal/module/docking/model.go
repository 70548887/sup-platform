package docking

// OrderDockingTask 订单对接任务
type OrderDockingTask struct {
	ID              uint   `gorm:"primarykey"`
	OrderID         uint   `gorm:"uniqueIndex;not null"`     // 唯一索引防重复提交
	SupplierID      uint   `gorm:"not null;index"`
	ExternalOrderID string `gorm:"size:64"`                  // 可空（上游可能不返回）
	Status          int8   `gorm:"not null;default:0;index"` // 0=pending,1=locked,2=submitted,3=failed,4=cancelled
	RetryCount      int    `gorm:"default:0"`
	MaxRetry        int    `gorm:"default:5"`                // 可配置最大重试次数
	NextRetryAt     *int64
	LastFailureAt   *int64
	ErrorMessage    string `gorm:"size:500"`      // 最后一次错误信息
	IsManualRetry   bool   `gorm:"default:false"` // 人工重试标记
	RequestPayload  string `gorm:"type:text"`     // 请求体JSON
	ResponsePayload string `gorm:"type:text"`     // 响应体JSON
	SubmittedAt     *int64
	CreatedAt       int64 `gorm:"autoCreateTime"`
	UpdatedAt       int64 `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (OrderDockingTask) TableName() string {
	return "order_docking_tasks"
}

// 状态常量
const (
	TaskPending   int8 = 0 // 待提交
	TaskLocked    int8 = 1 // 已锁定（正在提交中）
	TaskSubmitted int8 = 2 // 已提交成功
	TaskFailed    int8 = 3 // 提交失败（超过重试次数）
	TaskCancelled int8 = 4 // 已取消
)
