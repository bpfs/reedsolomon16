/**
 * Reed-Solomon 编码库 - 日志系统
 *
 * Copyright 2024
 */

package reedsolomon

import (
	"log"
	"os"
)

// 日志级别定义
const (
	LogLevelNone = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// Logger 接口定义了日志系统
type Logger interface {
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	SetLevel(level int)
}

// 默认日志实现
type defaultLogger struct {
	level  int
	logger *log.Logger
}

// 全局日志实例
var logger Logger = &defaultLogger{
	level:  LogLevelError,
	logger: log.New(os.Stderr, "[ReedSolomon] ", log.LstdFlags),
}

// SetLogger 设置全局日志实例
func SetLogger(l Logger) {
	if l != nil {
		logger = l
	}
}

// Error 记录错误级别日志
func (l *defaultLogger) Error(msg string, args ...interface{}) {
	if l.level >= LogLevelError {
		l.logger.Printf("ERROR: "+msg, args...)
	}
}

// Warn 记录警告级别日志
func (l *defaultLogger) Warn(msg string, args ...interface{}) {
	if l.level >= LogLevelWarn {
		l.logger.Printf("WARN: "+msg, args...)
	}
}

// Info 记录信息级别日志
func (l *defaultLogger) Info(msg string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("INFO: "+msg, args...)
	}
}

// Debug 记录调试级别日志
func (l *defaultLogger) Debug(msg string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("DEBUG: "+msg, args...)
	}
}

// SetLevel 设置日志级别
func (l *defaultLogger) SetLevel(level int) {
	l.level = level
}
