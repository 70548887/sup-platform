package signature

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
)

// LegacySHA1 计算亿乐Legacy签名
// 算法: SHA1(AppId + AppSecret + RequestURI + Timestamp)
func LegacySHA1(appId, appSecret, requestURI, timestamp string) string {
	raw := appId + appSecret + requestURI + timestamp
	h := sha1.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyLegacy 验证Legacy签名（使用ConstantTimeCompare防时序攻击）
func VerifyLegacy(appId, appSecret, requestURI, timestamp, token string) bool {
	expected := LegacySHA1(appId, appSecret, requestURI, timestamp)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}
