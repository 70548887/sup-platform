package apperror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_CreatesBusinessError(t *testing.T) {
	err := New(1001, "test error", 400)
	require.NotNil(t, err)
	assert.Equal(t, 1001, err.Code)
	assert.Equal(t, "test error", err.Message)
	assert.Equal(t, 400, err.HTTPStatus)
}

func TestBusinessError_Error(t *testing.T) {
	err := New(2001, "user not found", 404)
	assert.Equal(t, "user not found", err.Error())
}

func TestBusinessError_Is_SameCode(t *testing.T) {
	err1 := New(1001, "unauthorized", 401)
	err2 := New(1001, "different message", 403)

	// Same code should match
	assert.True(t, errors.Is(err1, err2))
}

func TestBusinessError_Is_DifferentCode(t *testing.T) {
	err1 := New(1001, "unauthorized", 401)
	err2 := New(1002, "forbidden", 403)

	assert.False(t, errors.Is(err1, err2))
}

func TestBusinessError_Is_NonBusinessError(t *testing.T) {
	bizErr := New(1001, "unauthorized", 401)
	stdErr := fmt.Errorf("standard error")

	assert.False(t, bizErr.Is(stdErr))
}

func TestWrap_NilError(t *testing.T) {
	result := Wrap(nil, ErrNotFound)
	assert.Nil(t, result)
}

func TestWrap_WrapsError(t *testing.T) {
	inner := fmt.Errorf("db connection failed")
	wrapped := Wrap(inner, ErrInternal)

	require.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "internal server error")
	assert.Contains(t, wrapped.Error(), "db connection failed")

	// Should be unwrappable to BusinessError
	var bizErr *BusinessError
	assert.True(t, errors.As(wrapped, &bizErr))
	assert.Equal(t, 1005, bizErr.Code)
}

func TestWrap_ErrorChain(t *testing.T) {
	inner := fmt.Errorf("record not found")
	wrapped := Wrap(inner, ErrNotFound)

	// errors.Is should work through the chain
	assert.True(t, errors.Is(wrapped, ErrNotFound))
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *BusinessError
		code       int
		httpStatus int
	}{
		{"ErrUnauthorized", ErrUnauthorized, 1001, 401},
		{"ErrForbidden", ErrForbidden, 1002, 403},
		{"ErrNotFound", ErrNotFound, 1003, 404},
		{"ErrBadRequest", ErrBadRequest, 1004, 400},
		{"ErrInternal", ErrInternal, 1005, 500},
		{"ErrTooManyRequests", ErrTooManyRequests, 1006, 429},
		{"ErrUserNotFound", ErrUserNotFound, 2001, 404},
		{"ErrUserExists", ErrUserExists, 2002, 409},
		{"ErrInvalidCredentials", ErrInvalidCredentials, 2003, 401},
		{"ErrInsufficientBalance", ErrInsufficientBalance, 2004, 400},
		{"ErrGoodsNotFound", ErrGoodsNotFound, 3001, 404},
		{"ErrInsufficientStock", ErrInsufficientStock, 3004, 400},
		{"ErrOrderNotFound", ErrOrderNotFound, 4001, 404},
		{"ErrOrderDuplicate", ErrOrderDuplicate, 4003, 409},
		{"ErrTenantNotFound", ErrTenantNotFound, 8001, 404},
		{"ErrTenantDisabled", ErrTenantDisabled, 8002, 403},
		{"ErrWalletCASConflict", ErrWalletCASConflict, 9001, 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
			assert.NotEmpty(t, tt.err.Message)
		})
	}
}

func TestBusinessError_ImplementsErrorInterface(t *testing.T) {
	var err error = New(1001, "test", 400)
	assert.NotNil(t, err)
	assert.Equal(t, "test", err.Error())
}
