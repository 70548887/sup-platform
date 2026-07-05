package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger 结构化日志接口
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...interface{})
	Info(ctx context.Context, msg string, fields ...interface{})
	Warn(ctx context.Context, msg string, fields ...interface{})
	Error(ctx context.Context, msg string, fields ...interface{})
}

// defaultLogger 基于标准库的默认实现
type defaultLogger struct {
	level  Level
	output *log.Logger
}

var std Logger = &defaultLogger{
	level:  INFO,
	output: log.New(os.Stdout, "", log.LstdFlags),
}

// Default 获取默认logger
func Default() Logger {
	return std
}

// SetLevel 设置日志级别
func SetLevel(l Level) {
	if dl, ok := std.(*defaultLogger); ok {
		dl.level = l
	}
}

func (l *defaultLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	if l.level <= DEBUG {
		l.output.Printf("[DEBUG] %s %s", msg, formatFields(fields))
	}
}

func (l *defaultLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	if l.level <= INFO {
		l.output.Printf("[INFO] %s %s", msg, formatFields(fields))
	}
}

func (l *defaultLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	if l.level <= WARN {
		l.output.Printf("[WARN] %s %s", msg, formatFields(fields))
	}
}

func (l *defaultLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	l.output.Printf("[ERROR] %s %s", msg, formatFields(fields))
}

func formatFields(fields []interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i < len(fields)-1; i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
	}
	return strings.Join(parts, " ")
}
