package apperror

// 通用错误 (1xxx)
var (
	ErrUnauthorized    = New(1001, "unauthorized", 401)
	ErrForbidden       = New(1002, "forbidden", 403)
	ErrNotFound        = New(1003, "resource not found", 404)
	ErrBadRequest      = New(1004, "bad request", 400)
	ErrInternal        = New(1005, "internal server error", 500)
	ErrTooManyRequests = New(1006, "too many requests", 429)
)

// 用户/账户错误 (2xxx)
var (
	ErrUserNotFound        = New(2001, "user not found", 404)
	ErrUserExists          = New(2002, "username already exists", 409)
	ErrInvalidCredentials  = New(2003, "invalid username or password", 401)
	ErrInsufficientBalance = New(2004, "insufficient balance", 400)
)

// 商品错误 (3xxx)
var (
	ErrGoodsNotFound     = New(3001, "goods not found", 404)
	ErrGoodsInvalidPrice = New(3002, "goods price must be greater than 0", 400)
	ErrGoodsInvalidName  = New(3003, "goods name is required", 400)
	ErrInsufficientStock = New(3004, "insufficient stock", 400)
)

// 订单错误 (4xxx)
var (
	ErrOrderNotFound      = New(4001, "order not found", 404)
	ErrOrderStatusInvalid = New(4002, "invalid order status transition", 400)
	ErrOrderDuplicate     = New(4003, "duplicate order", 409)
)

// 退款错误 (5xxx)
var (
	ErrRefundExceeded      = New(5001, "refund amount exceeds refundable", 400)
	ErrRefundNotFound      = New(5002, "refund not found", 404)
	ErrRefundStatusInvalid = New(5003, "invalid refund status", 400)
)

// 卡密错误 (6xxx)
var (
	ErrCardNotFound = New(6001, "card not found", 404)
	ErrCardNoStock  = New(6002, "no available cards", 400)
)

// 定价错误 (7xxx)
var (
	ErrPricingRuleNotFound = New(7001, "pricing rule not found", 404)
	ErrQuotaExhausted      = New(7002, "API quota exhausted", 429)
)

// 租户错误 (8xxx)
var (
	ErrTenantNotFound  = New(8001, "tenant not found", 404)
	ErrTenantDisabled  = New(8002, "tenant is disabled", 403)
	ErrFeatureDisabled = New(8003, "feature is disabled", 403)
)

// 钱包错误 (9xxx)
var (
	ErrWalletCASConflict = New(9001, "wallet update conflict", 409)
	ErrWalletNotFound    = New(9002, "wallet not found", 404)
)
