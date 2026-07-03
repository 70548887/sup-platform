package account

// User 统一用户表
type User struct {
	ID        uint   `gorm:"primarykey"`
	Username  string `gorm:"size:64;uniqueIndex;not null"`  // 登录名
	Password  string `gorm:"size:128;not null"`             // bcrypt哈希
	Nickname  string `gorm:"size:64"`
	Email     string `gorm:"size:128"`
	Phone     string `gorm:"size:20"`
	Role      string `gorm:"size:20;not null;index"`  // admin/supplier/customer
	Status    int8   `gorm:"not null;default:1"`      // 1活跃 0禁用
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// ApiApp API应用凭证
type ApiApp struct {
	ID          uint   `gorm:"primarykey"`
	UserID      uint   `gorm:"not null;index"`
	AppId       string `gorm:"size:64;uniqueIndex;not null"`
	AppSecret   string `gorm:"size:128;not null"`
	Name        string `gorm:"size:100"`
	IPWhitelist string `gorm:"type:text"`          // JSON数组
	RateLimit   int    `gorm:"default:100"`        // 每分钟请求上限
	Status      int8   `gorm:"not null;default:1"`
	CreatedAt   int64  `gorm:"autoCreateTime"`
	UpdatedAt   int64  `gorm:"autoUpdateTime"`
}

// Role 角色
type Role struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"size:50;uniqueIndex;not null"`
	Description string `gorm:"size:200"`
	Permissions string `gorm:"type:text"` // JSON权限列表
	CreatedAt   int64  `gorm:"autoCreateTime"`
}

// Permission 权限
type Permission struct {
	ID     uint   `gorm:"primarykey"`
	Code   string `gorm:"size:100;uniqueIndex;not null"`
	Name   string `gorm:"size:100;not null"`
	Module string `gorm:"size:50;index"`
}

// LoginSession 登录会话
type LoginSession struct {
	ID        uint   `gorm:"primarykey"`
	UserID    uint   `gorm:"not null;index"`
	Token     string `gorm:"size:500;not null"`
	IP        string `gorm:"size:45"`
	UserAgent string `gorm:"size:500"`
	ExpiredAt int64  `gorm:"not null"`
	CreatedAt int64  `gorm:"autoCreateTime"`
}
