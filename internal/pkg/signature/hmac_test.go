package signature

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMACSHA256_Deterministic(t *testing.T) {
	appId := "app123"
	appSecret := "secret456"
	requestURI := "/api/v1/orders"
	timestamp := "1688888888"
	nonce := "abc123"

	sig1 := HMACSHA256(appId, appSecret, requestURI, timestamp, nonce)
	sig2 := HMACSHA256(appId, appSecret, requestURI, timestamp, nonce)

	assert.Equal(t, sig1, sig2)
	assert.Len(t, sig1, 64) // SHA256 = 32 bytes = 64 hex chars
}

func TestHMACSHA256_DifferentInputs(t *testing.T) {
	base := HMACSHA256("app", "secret", "/api", "1000", "nonce1")

	// 修改任意参数应得到不同签名
	assert.NotEqual(t, base, HMACSHA256("app2", "secret", "/api", "1000", "nonce1"))
	assert.NotEqual(t, base, HMACSHA256("app", "secret2", "/api", "1000", "nonce1"))
	assert.NotEqual(t, base, HMACSHA256("app", "secret", "/api2", "1000", "nonce1"))
	assert.NotEqual(t, base, HMACSHA256("app", "secret", "/api", "2000", "nonce1"))
	assert.NotEqual(t, base, HMACSHA256("app", "secret", "/api", "1000", "nonce2"))
}

func TestVerifyHMAC_CorrectSignature(t *testing.T) {
	appId := "testapp"
	appSecret := "testsecret"
	uri := "/api/orders"
	ts := "1700000000"
	nonce := "rand123"

	sig := HMACSHA256(appId, appSecret, uri, ts, nonce)
	assert.True(t, VerifyHMAC(appId, appSecret, uri, ts, nonce, sig))
}

func TestVerifyHMAC_WrongSignature(t *testing.T) {
	appId := "testapp"
	appSecret := "testsecret"
	uri := "/api/orders"
	ts := "1700000000"
	nonce := "rand123"

	assert.False(t, VerifyHMAC(appId, appSecret, uri, ts, nonce, "wrongsignature"))
	assert.False(t, VerifyHMAC(appId, appSecret, uri, ts, nonce, ""))
}

func TestVerifyHMAC_EmptyInputs(t *testing.T) {
	// 即使参数为空也能生成/验证签名
	sig := HMACSHA256("", "", "", "", "")
	assert.True(t, VerifyHMAC("", "", "", "", "", sig))
	assert.Len(t, sig, 64)
}
