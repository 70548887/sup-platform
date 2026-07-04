package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const cardEncryptPrefix = "ENC:" // 加密内容前缀标识

var ErrDecryptFailed = errors.New("crypto: card content decryption failed")

// EncryptCardContent 使用 AES-GCM 加密卡密内容，返回 "ENC:" + base64(nonce+ciphertext)
func EncryptCardContent(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return cardEncryptPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCardContent 解密卡密内容。如果内容没有加密前缀（旧数据），直接返回原文。
func DecryptCardContent(content string, key []byte) (string, error) {
	// 兼容旧数据：没有加密前缀的直接返回
	if !strings.HasPrefix(content, cardEncryptPrefix) {
		return content, nil
	}

	encoded := strings.TrimPrefix(content, cardEncryptPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrDecryptFailed
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrDecryptFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptFailed
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptFailed
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}

	return string(plaintext), nil
}

// IsEncrypted 检查内容是否已加密
func IsEncrypted(content string) bool {
	return strings.HasPrefix(content, cardEncryptPrefix)
}
