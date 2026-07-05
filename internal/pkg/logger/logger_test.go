package logger

import (
	"bytes"
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestLogger(level Level) (*defaultLogger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := &defaultLogger{
		level:  level,
		output: log.New(buf, "", 0),
	}
	return l, buf
}

func TestLogger_DebugLevel_ShowsAll(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	ctx := context.Background()

	l.Debug(ctx, "debug msg")
	assert.Contains(t, buf.String(), "[DEBUG]")
	assert.Contains(t, buf.String(), "debug msg")
	buf.Reset()

	l.Info(ctx, "info msg")
	assert.Contains(t, buf.String(), "[INFO]")
	buf.Reset()

	l.Warn(ctx, "warn msg")
	assert.Contains(t, buf.String(), "[WARN]")
	buf.Reset()

	l.Error(ctx, "error msg")
	assert.Contains(t, buf.String(), "[ERROR]")
}

func TestLogger_InfoLevel_FiltersDebug(t *testing.T) {
	l, buf := newTestLogger(INFO)
	ctx := context.Background()

	l.Debug(ctx, "should not appear")
	assert.Empty(t, buf.String())

	l.Info(ctx, "info visible")
	assert.Contains(t, buf.String(), "[INFO]")
	buf.Reset()

	l.Warn(ctx, "warn visible")
	assert.Contains(t, buf.String(), "[WARN]")
	buf.Reset()

	l.Error(ctx, "error visible")
	assert.Contains(t, buf.String(), "[ERROR]")
}

func TestLogger_WarnLevel_FiltersDebugAndInfo(t *testing.T) {
	l, buf := newTestLogger(WARN)
	ctx := context.Background()

	l.Debug(ctx, "no debug")
	assert.Empty(t, buf.String())

	l.Info(ctx, "no info")
	assert.Empty(t, buf.String())

	l.Warn(ctx, "warn visible")
	assert.Contains(t, buf.String(), "[WARN]")
	buf.Reset()

	l.Error(ctx, "error visible")
	assert.Contains(t, buf.String(), "[ERROR]")
}

func TestLogger_ErrorLevel_OnlyError(t *testing.T) {
	l, buf := newTestLogger(ERROR)
	ctx := context.Background()

	l.Debug(ctx, "no")
	l.Info(ctx, "no")
	l.Warn(ctx, "no")
	assert.Empty(t, buf.String())

	l.Error(ctx, "error visible")
	assert.Contains(t, buf.String(), "[ERROR]")
}

func TestLogger_WithFields(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	ctx := context.Background()

	l.Info(ctx, "user login", "user_id", 123, "ip", "192.168.1.1")
	output := buf.String()
	assert.Contains(t, output, "user login")
	assert.Contains(t, output, "user_id=123")
	assert.Contains(t, output, "ip=192.168.1.1")
}

func TestLogger_Default(t *testing.T) {
	l := Default()
	assert.NotNil(t, l)
}

// mask_test
func TestMaskPassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"short password", "ab", "***"},
		{"exactly 3 chars", "abc", "***"},
		{"normal password", "mypassword123", "***23"},
		{"longer password", "verylongpassword!", "***d!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskPassword(tt.input))
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"short token", "abcdefgh", "***"},
		{"exactly 8 chars", "12345678", "***"},
		{"normal token", "abcdefghijklmnop", "abcd********mnop"},
		{"9 chars", "123456789", "1234*6789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskToken(tt.input))
		})
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal email", "john.doe@example.com", "jo***@example.com"},
		{"short name", "ab@test.com", "**@test.com"},
		{"single char name", "a@test.com", "**@test.com"},
		{"three char name", "abc@test.com", "ab***@test.com"},
		{"no at sign", "notanemail", "***"},
		{"empty", "", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskEmail(tt.input))
		})
	}
}
