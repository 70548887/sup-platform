package validate

import (
	"fmt"
	"html"
	"regexp"

	"github.com/shopspring/decimal"
)

// ValidatePageSize 验证分页大小（1-100）
func ValidatePageSize(size int) int {
	if size < 1 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}

// ValidatePage 验证页码
func ValidatePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

// ValidateDecimalAmount 验证金额合法性
func ValidateDecimalAmount(amount string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return decimal.Zero, fmt.Errorf("validate: invalid decimal amount: %w", err)
	}
	if d.IsNegative() {
		return decimal.Zero, fmt.Errorf("validate: amount cannot be negative")
	}
	if d.GreaterThan(decimal.NewFromInt(99999999)) {
		return decimal.Zero, fmt.Errorf("validate: amount exceeds maximum")
	}
	return d, nil
}

// ValidateOrderSN 验证订单号格式
var orderSNPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func ValidateOrderSN(sn string) error {
	if sn == "" {
		return fmt.Errorf("validate: order_sn cannot be empty")
	}
	if !orderSNPattern.MatchString(sn) {
		return fmt.Errorf("validate: invalid order_sn format")
	}
	return nil
}

// SanitizeString HTML转义字符串
func SanitizeString(s string) string {
	return html.EscapeString(s)
}
