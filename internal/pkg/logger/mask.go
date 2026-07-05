package logger

import "strings"

// MaskPassword 密码脱敏
func MaskPassword(password string) string {
	if len(password) <= 3 {
		return "***"
	}
	return "***" + password[len(password)-2:]
}

// MaskToken Token脱敏（保留前4位和后4位）
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// MaskEmail 邮箱脱敏
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	name := parts[0]
	if len(name) <= 2 {
		return "**@" + parts[1]
	}
	return name[:2] + "***@" + parts[1]
}
