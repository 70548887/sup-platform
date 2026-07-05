package auth

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidatePassword 验证密码强度
// 规则：至少8位，包含大写字母、小写字母和数字，不能与用户名相同
func ValidatePassword(password, username string) error {
	if len(password) < 8 {
		return fmt.Errorf("密码长度不能少于8位")
	}

	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("密码必须包含至少一个大写字母")
	}
	if !hasLower {
		return fmt.Errorf("密码必须包含至少一个小写字母")
	}
	if !hasDigit {
		return fmt.Errorf("密码必须包含至少一个数字")
	}

	// 密码不能与用户名相同
	if username != "" && strings.EqualFold(password, username) {
		return fmt.Errorf("密码不能与用户名相同")
	}

	// 简单密码黑名单
	weakPasswords := []string{"password", "12345678", "qwerty123", "admin123"}
	lower := strings.ToLower(password)
	for _, weak := range weakPasswords {
		if lower == weak || strings.Contains(lower, weak) {
			return fmt.Errorf("密码过于简单，请使用更复杂的密码")
		}
	}

	return nil
}
