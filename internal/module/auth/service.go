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

// Login 登录（管理后台/供货商后台）
func (s *AuthService) Login(username, password, role string) (string, error) {
	// 根据role查询不同的用户表
	type UserCredential struct {
		ID       uint
		Password string
	}

	var cred UserCredential

	switch role {
	case "admin":
		err := s.db.Table("admins").
			Select("id, password").
			Where("username = ? AND status = 1", username).
			First(&cred).Error
		if err != nil {
			return "", fmt.Errorf("用户名或密码错误")
		}
	case "supplier":
		err := s.db.Table("suppliers").
			Select("id, password").
			Where("username = ? AND status = 1", username).
			First(&cred).Error
		if err != nil {
			return "", fmt.Errorf("用户名或密码错误")
		}
	default:
		return "", fmt.Errorf("不支持的角色类型: %s", role)
	}

	if !VerifyPassword(cred.Password, password) {
		return "", fmt.Errorf("用户名或密码错误")
	}

	return s.GenerateJWT(cred.ID, role, 0)
}
