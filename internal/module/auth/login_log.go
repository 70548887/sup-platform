package auth

// LoginLog 登录审计日志
type LoginLog struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	UserID     uint   `gorm:"index;not null" json:"user_id"`
	TenantID   uint   `gorm:"index;default:1" json:"tenant_id"`
	AuthType   string `gorm:"size:20;default:jwt" json:"auth_type"` // jwt/legacy
	ClientIP   string `gorm:"size:45;not null" json:"client_ip"`
	UserAgent  string `gorm:"size:500" json:"user_agent"`
	Status     string `gorm:"size:20;default:success" json:"status"` // success/failed/locked
	FailReason string `gorm:"size:255" json:"fail_reason"`
	CreatedAt  int64  `gorm:"index;autoCreateTime" json:"created_at"`
}

func (LoginLog) TableName() string {
	return "login_logs"
}
