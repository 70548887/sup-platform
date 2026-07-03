package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateAppId 生成AppId（16字符hex，即8字节随机数）
func GenerateAppId() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成AppId失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateAppSecret 生成AppSecret（32字符hex，即16字节随机数）
func GenerateAppSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成AppSecret失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandomBytes 生成指定长度的安全随机字节
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("生成随机字节失败: %w", err)
	}
	return b, nil
}
