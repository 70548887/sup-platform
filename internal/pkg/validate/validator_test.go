package validate

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePageSize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"normal value", 10, 10},
		{"minimum boundary", 1, 1},
		{"maximum boundary", 100, 100},
		{"zero defaults to 20", 0, 20},
		{"negative defaults to 20", -1, 20},
		{"over max caps at 100", 101, 100},
		{"large value caps at 100", 9999, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ValidatePageSize(tt.input))
		})
	}
}

func TestValidatePage(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"normal page", 5, 5},
		{"first page", 1, 1},
		{"zero defaults to 1", 0, 1},
		{"negative defaults to 1", -1, 1},
		{"large page", 9999, 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ValidatePage(tt.input))
		})
	}
}

func TestValidateDecimalAmount_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"integer", "100", "100"},
		{"decimal", "99.99", "99.99"},
		{"zero", "0", "0"},
		{"small amount", "0.01", "0.01"},
		{"max valid", "99999999", "99999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ValidateDecimalAmount(tt.input)
			require.NoError(t, err)
			expected, _ := decimal.NewFromString(tt.expected)
			assert.True(t, d.Equal(expected))
		})
	}
}

func TestValidateDecimalAmount_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"negative", "-1"},
		{"exceeds max", "100000000"},
		{"not a number", "abc"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateDecimalAmount(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestValidateOrderSN_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"alphanumeric", "ORDER123"},
		{"with dash", "ORD-2024-001"},
		{"with underscore", "order_sn_123"},
		{"single char", "A"},
		{"max length 64", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrderSN(tt.input)
			assert.NoError(t, err)
		})
	}
}

func TestValidateOrderSN_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"contains space", "ORDER 123"},
		{"contains special char", "order@123"},
		{"too long", "aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeee1234567890123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrderSN(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no special chars", "hello", "hello"},
		{"html tags", "<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"ampersand", "a&b", "a&amp;b"},
		{"quotes", `"hello"`, "&#34;hello&#34;"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeString(tt.input))
		})
	}
}
