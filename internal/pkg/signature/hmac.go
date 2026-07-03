package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HMACSHA256 计算新API签名（Phase 2使用）
// 算法: HMAC-SHA256(AppSecret, AppId + RequestURI + Timestamp + Nonce)
func HMACSHA256(appId, appSecret, requestURI, timestamp, nonce string) string {
	raw := appId + requestURI + timestamp + nonce
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC 验证HMAC-SHA256签名
func VerifyHMAC(appId, appSecret, requestURI, timestamp, nonce, token string) bool {
	expected := HMACSHA256(appId, appSecret, requestURI, timestamp, nonce)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}
