package account

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AccountService 账户服务
type AccountService struct {
	db *gorm.DB
}

// NewAccountService 创建账户服务实例
func NewAccountService(db *gorm.DB) *AccountService {
	return &AccountService{db: db}
}

// HashPassword 密码哈希（bcrypt）
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

// Register 注册新用户，返回创建的用户
func (s *AccountService) Register(username, password, role string) (*User, error) {
	// 检查用户名是否已存在
	var count int64
	if err := s.db.Model(&User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("用户名已存在")
	}

	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &User{
		Username: username,
		Password: hashed,
		Role:     role,
		Status:   1,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// Login 用户登录，验证用户名和密码
func (s *AccountService) Login(username, password string) (*User, error) {
	var user User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	if user.Status != 1 {
		return nil, fmt.Errorf("用户已被禁用")
	}

	if !VerifyPassword(user.Password, password) {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	return &user, nil
}
