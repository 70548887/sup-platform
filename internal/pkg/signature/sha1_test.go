package signature

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegacySHA1_Deterministic(t *testing.T) {
	appId := "app001"
	appSecret := "secretkey"
	requestURI := "/api/order/query"
	timestamp := "1700000000"

	sig1 := LegacySHA1(appId, appSecret, requestURI, timestamp)
	sig2 := LegacySHA1(appId, appSecret, requestURI, timestamp)

	assert.Equal(t, sig1, sig2)
	assert.Len(t, sig1, 40) // SHA1 = 20 bytes = 40 hex chars
}

func TestLegacySHA1_DifferentInputs(t *testing.T) {
	base := LegacySHA1("app", "secret", "/api", "1000")

	assert.NotEqual(t, base, LegacySHA1("app2", "secret", "/api", "1000"))
	assert.NotEqual(t, base, LegacySHA1("app", "secret2", "/api", "1000"))
	assert.NotEqual(t, base, LegacySHA1("app", "secret", "/api2", "1000"))
	assert.NotEqual(t, base, LegacySHA1("app", "secret", "/api", "2000"))
}

func TestVerifyLegacy_CorrectSignature(t *testing.T) {
	appId := "testapp"
	appSecret := "testsecret"
	uri := "/api/order"
	ts := "1700000000"

	sig := LegacySHA1(appId, appSecret, uri, ts)
	assert.True(t, VerifyLegacy(appId, appSecret, uri, ts, sig))
}

func TestVerifyLegacy_WrongSignature(t *testing.T) {
	assert.False(t, VerifyLegacy("app", "secret", "/api", "1000", "wrong"))
	assert.False(t, VerifyLegacy("app", "secret", "/api", "1000", ""))
}

func TestLegacySHA1WithNonce_Deterministic(t *testing.T) {
	sig1 := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce1")
	sig2 := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce1")

	assert.Equal(t, sig1, sig2)
	assert.Len(t, sig1, 40)
}

func TestLegacySHA1WithNonce_DifferentNonce(t *testing.T) {
	sig1 := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce1")
	sig2 := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce2")

	assert.NotEqual(t, sig1, sig2)
}

func TestVerifyLegacyWithNonce_CorrectSignature(t *testing.T) {
	sig := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce1")
	assert.True(t, VerifyLegacyWithNonce("app", "secret", "/api", "1000", "nonce1", sig))
}

func TestVerifyLegacyWithNonce_CaseInsensitive(t *testing.T) {
	sig := LegacySHA1WithNonce("app", "secret", "/api", "1000", "nonce1")
	// strings.EqualFold in implementation means case-insensitive
	assert.True(t, VerifyLegacyWithNonce("app", "secret", "/api", "1000", "nonce1", strings.ToUpper(sig)))
}

func TestVerifyLegacyWithNonce_WrongSignature(t *testing.T) {
	assert.False(t, VerifyLegacyWithNonce("app", "secret", "/api", "1000", "nonce1", "wrong"))
}
