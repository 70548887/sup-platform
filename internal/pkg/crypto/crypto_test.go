package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptCardContent_Symmetric(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes for AES-256

	tests := []struct {
		name      string
		plaintext string
	}{
		{"normal content", "CARD-SECRET-12345"},
		{"empty string", ""},
		{"unicode content", "中文卡密内容🎉"},
		{"long content", strings.Repeat("A", 1024)},
		{"special chars", "abc!@#$%^&*()_+-=[]{}|;':\",./<>?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptCardContent(tt.plaintext, key)
			require.NoError(t, err)

			// 加密结果应以 ENC: 开头
			assert.True(t, strings.HasPrefix(encrypted, "ENC:"))

			// 解密应还原明文
			decrypted, err := DecryptCardContent(encrypted, key)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptCardContent_DifferentCiphertexts(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	plaintext := "same-content"

	enc1, err := EncryptCardContent(plaintext, key)
	require.NoError(t, err)

	enc2, err := EncryptCardContent(plaintext, key)
	require.NoError(t, err)

	// 由于nonce随机性，两次加密结果应不同
	assert.NotEqual(t, enc1, enc2)
}

func TestDecryptCardContent_UnencryptedData(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	// 没有ENC:前缀的旧数据应直接返回原文
	plain := "old-unencrypted-card-data"
	result, err := DecryptCardContent(plain, key)
	require.NoError(t, err)
	assert.Equal(t, plain, result)
}

func TestDecryptCardContent_InvalidData(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	tests := []struct {
		name    string
		content string
	}{
		{"invalid base64", "ENC:not-valid-base64!!!"},
		{"too short data", "ENC:YQ=="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptCardContent(tt.content, key)
			assert.ErrorIs(t, err, ErrDecryptFailed)
		})
	}
}

func TestDecryptCardContent_WrongKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("fedcba9876543210fedcba9876543210")

	encrypted, err := EncryptCardContent("secret", key1)
	require.NoError(t, err)

	_, err = DecryptCardContent(encrypted, key2)
	assert.ErrorIs(t, err, ErrDecryptFailed)
}

func TestEncryptCardContent_InvalidKeyLength(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"too short", []byte("short")},
		{"15 bytes", []byte("123456789012345")},
		{"17 bytes", []byte("12345678901234567")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncryptCardContent("test", tt.key)
			assert.Error(t, err)
		})
	}
}

func TestEncryptCardContent_ValidKeyLengths(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"16 bytes (AES-128)", []byte("0123456789abcdef")},
		{"24 bytes (AES-192)", []byte("0123456789abcdef01234567")},
		{"32 bytes (AES-256)", []byte("0123456789abcdef0123456789abcdef")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptCardContent("test-content", tt.key)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(encrypted, "ENC:"))

			decrypted, err := DecryptCardContent(encrypted, tt.key)
			require.NoError(t, err)
			assert.Equal(t, "test-content", decrypted)
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	assert.True(t, IsEncrypted("ENC:somedata"))
	assert.True(t, IsEncrypted("ENC:"))
	assert.False(t, IsEncrypted("plain-text"))
	assert.False(t, IsEncrypted(""))
	assert.False(t, IsEncrypted("enc:lowercase"))
}

func TestGenerateAppId(t *testing.T) {
	id, err := GenerateAppId()
	require.NoError(t, err)
	assert.Len(t, id, 16) // 8 bytes = 16 hex chars

	// 生成多个确保唯一性
	id2, err := GenerateAppId()
	require.NoError(t, err)
	assert.NotEqual(t, id, id2)
}

func TestGenerateAppSecret(t *testing.T) {
	secret, err := GenerateAppSecret()
	require.NoError(t, err)
	assert.Len(t, secret, 32) // 16 bytes = 32 hex chars

	secret2, err := GenerateAppSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret, secret2)
}

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"zero length", 0},
		{"1 byte", 1},
		{"16 bytes", 16},
		{"32 bytes", 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := GenerateRandomBytes(tt.n)
			require.NoError(t, err)
			assert.Len(t, b, tt.n)
		})
	}
}
