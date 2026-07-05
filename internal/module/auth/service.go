package auth

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	pkgcrypto "github.com/70548887/sup-platform/internal/pkg/crypto"
)

// ApiApp API应用模型
type ApiApp struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"column:user_id;index" json:"user_id"`
	AppName     string    `gorm:"column:app_name;size:100" json:"app_name"`
	AppId       string    `gorm:"column:app_id;size:32;uniqueIndex" json:"app_id"`
	AppSecret   string    `gorm:"column:app_secret;size:64" json:"-"`
	IPWhitelist string    `gorm:"column:ip_whitelist;size:500" json:"ip_whitelist"` // 逗号分隔的IP/CIDR列表
	Status      int       `gorm:"column:status;default:1" json:"status"`           // 1=启用, 0=禁用

	// 商业化字段
	Environment    string `gorm:"column:environment;size:20;default:production" json:"environment"` // sandbox/production
	Description    string `gorm:"column:description;type:text" json:"description"`
	ExpiresAt      int64  `gorm:"column:expires_at" json:"expires_at"`
	WebhookURL     string `gorm:"column:webhook_url;size:500" json:"webhook_url"`
	LastCalledAt   int64  `gorm:"column:last_called_at" json:"last_called_at"`
	CallCountDay   int64  `gorm:"column:call_count_day;default:0" json:"call_count_day"`
	CallCountMonth int64  `gorm:"column:call_count_month;default:0" json:"call_count_month"`
	KeyRotatedAt   int64  `gorm:"column:key_rotated_at" json:"key_rotated_at"`

	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ApiApp) TableName() string {
	return "api_apps"
}

// ApiAppInfo 查询结果
type ApiAppInfo struct {
	ID          uint
	UserID      uint
	AppId       string
	AppSecret   string
	IPWhitelist string
	Status      int
}

// Claims 定义已移至 claims.go

// AuthService 认证服务
type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
	jwtExpire time.Duration
}

// NewAuthService 创建认证服务实例
func NewAuthService(db *gorm.DB, jwtSecret string, jwtExpireHours int) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		jwtExpire: time.Duration(jwtExpireHours) * time.Hour,
	}
}

// GenerateApiCredentials 生成API凭证
func (s *AuthService) GenerateApiCredentials(userID uint, name string) (appId, appSecret string, err error) {
	appId, err = pkgcrypto.GenerateAppId()
	if err != nil {
		return "", "", fmt.Errorf("生成AppId失败: %w", err)
	}

	appSecret, err = pkgcrypto.GenerateAppSecret()
	if err != nil {
		return "", "", fmt.Errorf("生成AppSecret失败: %w", err)
	}

	app := &ApiApp{
		UserID:    userID,
		AppName:   name,
		AppId:     appId,
		AppSecret: appSecret,
		Status:    1,
	}

	if err = s.db.Create(app).Error; err != nil {
		return "", "", fmt.Errorf("保存API应用失败: %w", err)
	}

	return appId, appSecret, nil
}

// VerifyApiApp 验证API应用，返回应用信息
func (s *AuthService) VerifyApiApp(appId string) (*ApiAppInfo, error) {
	var app ApiApp
	if err := s.db.Where("app_id = ?", appId).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("应用不存在")
		}
		return nil, fmt.Errorf("查询应用失败: %w", err)
	}

	return &ApiAppInfo{
		ID:          app.ID,
		UserID:      app.UserID,
		AppId:       app.AppId,
		AppSecret:   app.AppSecret,
		IPWhitelist: app.IPWhitelist,
		Status:      app.Status,
	}, nil
}

// CheckIPWhitelist 检查IP是否在白名单内
func (s *AuthService) CheckIPWhitelist(clientIP, whitelist string) bool {
	if whitelist == "" {
		return true // 未配置白名单则放行
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	items := strings.Split(whitelist, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		// 支持CIDR格式
		if strings.Contains(item, "/") {
			_, cidr, err := net.ParseCIDR(item)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return true
			}
		} else {
			// 单个IP
			if item == clientIP {
				return true
			}
		}
	}

	return false
}

// GenerateJWT 生成JWT token
// tenantID=0 表示平台超管，>0 表示租户管理员
func (s *AuthService) GenerateJWT(userID uint, role string, tenantID uint) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Role:     role,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "sup-platform",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// VerifyJWT 验证JWT token
func (s *AuthService) VerifyJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签名方法: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("JWT验证失败: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的JWT token")
	}

	return claims, nil
}

// HashPassword 密码哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("密码哈希失败: %w", err)
	}
	return string(bytes), nil
}

// VerifyPassword 验证密码
func VerifyPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// Login 登录（管理后台/供货商后台），含防暴力破解与登录日志
func (s *AuthService) Login(username, password, role, clientIP, userAgent string) (string, error) {
	// 1. 查询用户（包含安全字段）
	type UserCredential struct {
		ID            uint
		Password      string
		TenantID      uint
		LoginAttempts int
		LockedUntil   int64
	}
	var cred UserCredential

	query := s.db.Table("users").
		Select("id, password, tenant_id, login_attempts, locked_until").
		Where("username = ? AND status = 1", username)
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if err := query.First(&cred).Error; err != nil {
		// 用户不存在也记录日志（userID=0）
		s.RecordLoginLog(0, 0, "jwt", clientIP, userAgent, "failed", "用户不存在")
		return "", fmt.Errorf("用户名或密码错误")
	}

	// 2. 检查账户是否锁定
	if cred.LockedUntil > 0 && time.Now().Unix() < cred.LockedUntil {
		remaining := cred.LockedUntil - time.Now().Unix()
		s.RecordLoginLog(cred.ID, cred.TenantID, "jwt", clientIP, userAgent, "locked", "账户锁定中")
		return "", fmt.Errorf("账户已锁定，请%d分钟后重试", remaining/60+1)
	}

	// 3. 验证密码
	if !VerifyPassword(cred.Password, password) {
		attempts := cred.LoginAttempts + 1
		updates := map[string]interface{}{
			"login_attempts": attempts,
		}
		// 达到5次锁定15分钟
		if attempts >= 5 {
			updates["locked_until"] = time.Now().Add(15 * time.Minute).Unix()
		}
		s.db.Table("users").Where("id = ?", cred.ID).Updates(updates)

		if attempts >= 5 {
			s.RecordLoginLog(cred.ID, cred.TenantID, "jwt", clientIP, userAgent, "locked", "连续失败5次锁定")
			return "", fmt.Errorf("连续登录失败5次，账户已锁定15分钟")
		}
		s.RecordLoginLog(cred.ID, cred.TenantID, "jwt", clientIP, userAgent, "failed", "密码错误")
		return "", fmt.Errorf("用户名或密码错误")
	}

	// 4. 成功：重置失败计数，更新最后登录信息
	s.db.Table("users").Where("id = ?", cred.ID).Updates(map[string]interface{}{
		"login_attempts": 0,
		"locked_until":   0,
		"last_login_at":  time.Now().Unix(),
	})

	s.RecordLoginLog(cred.ID, cred.TenantID, "jwt", clientIP, userAgent, "success", "")

	return s.GenerateJWT(cred.ID, role, cred.TenantID)
}

// RecordLoginLog 记录登录日志
func (s *AuthService) RecordLoginLog(userID uint, tenantID uint, authType, clientIP, userAgent, status, failReason string) {
	log := &LoginLog{
		UserID:     userID,
		TenantID:   tenantID,
		AuthType:   authType,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		Status:     status,
		FailReason: failReason,
		CreatedAt:  time.Now().Unix(),
	}
	s.db.Create(log)
}
