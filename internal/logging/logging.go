// Package logging 提供结构化日志功能
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Source    string                 `json:"source,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// Logger 日志记录器
type Logger struct {
	level     Level
	output    io.Writer
	formatter Formatter
	fields    map[string]interface{}
	mu        sync.Mutex
}

// Formatter 日志格式化器
type Formatter interface {
	Format(entry *LogEntry) ([]byte, error)
}

// JSONFormatter JSON 格式化器
type JSONFormatter struct{}

func (f *JSONFormatter) Format(entry *LogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// TextFormatter 文本格式化器
type TextFormatter struct {
	DisableColors bool
}

func (f *TextFormatter) Format(entry *LogEntry) ([]byte, error) {
	var prefix string
	if !f.DisableColors {
		prefix = levelColors[entry.Level]
	}
	return []byte(fmt.Sprintf("%s[%s] %s%s %v\n",
		prefix,
		entry.Timestamp.Format("2006-01-02 15:04:05.000"),
		entry.Level,
		resetColor,
		entry.Message,
	)), nil
}

var levelColors = map[string]string{
	"DEBUG": "\033[36m", // 青色
	"INFO":  "\033[32m", // 绿色
	"WARN":  "\033[33m", // 黄色
	"ERROR": "\033[31m", // 红色
	"FATAL": "\033[35m", // 紫色
}

const resetColor = "\033[0m"

// LogConfig 日志配置
type LogConfig struct {
	Level      Level
	Output     io.Writer
	Formatter  Formatter
	JSONFormat bool
}

// NewLogger 创建日志记录器
func NewLogger(config *LogConfig) *Logger {
	if config == nil {
		config = &LogConfig{
			Level: LevelInfo,
		}
	}

	if config.Output == nil {
		config.Output = os.Stdout
	}

	if config.Formatter == nil {
		if config.JSONFormat {
			config.Formatter = &JSONFormatter{}
		} else {
			config.Formatter = &TextFormatter{}
		}
	}

	return &Logger{
		level:     config.Level,
		output:    config.Output,
		formatter: config.Formatter,
		fields:    make(map[string]interface{}),
	}
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// log 记录日志
func (l *Logger) log(level Level, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Message:   message,
		Fields:    make(map[string]interface{}),
	}

	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	for k, v := range fields {
		entry.Fields[k] = v
	}

	data, err := l.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format log: %v\n", err)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.output.Write(data)
}

// Debug 记录调试日志
func (l *Logger) Debug(message string) {
	l.log(LevelDebug, message, nil)
}

// Debugf 记录格式化调试日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, fmt.Sprintf(format, args...), nil)
}

// Info 记录信息日志
func (l *Logger) Info(message string) {
	l.log(LevelInfo, message, nil)
}

// Infof 记录格式化信息日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, fmt.Sprintf(format, args...), nil)
}

// Warn 记录警告日志
func (l *Logger) Warn(message string) {
	l.log(LevelWarn, message, nil)
}

// Warnf 记录格式化警告日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, fmt.Sprintf(format, args...), nil)
}

// Error 记录错误日志
func (l *Logger) Error(message string) {
	l.log(LevelError, message, nil)
}

// Errorf 记录格式化错误日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, args...), nil)
}

// Fatal 记录致命错误日志并退出
func (l *Logger) Fatal(message string) {
	l.log(LevelFatal, message, nil)
	os.Exit(1)
}

// Fatalf 记录格式化致命错误日志并退出
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(LevelFatal, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// ========== 日志管理器 ==========

// LogManager 日志管理器
type LogManager struct {
	loggers map[string]*Logger
	rotator *LogRotator
	mu      sync.RWMutex
}

// NewLogManager 创建日志管理器
func NewLogManager() *LogManager {
	return &LogManager{
		loggers: make(map[string]*Logger),
	}
}

// GetLogger 获取日志记录器
func (m *LogManager) GetLogger(name string) *Logger {
	m.mu.RLock()
	logger, exists := m.loggers[name]
	m.mu.RUnlock()

	if exists {
		return logger
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	logger = NewLogger(&LogConfig{Level: LevelInfo})
	m.loggers[name] = logger
	return logger
}

// SetRotator 设置日志轮转器
func (m *LogManager) SetRotator(rotator *LogRotator) {
	m.rotator = rotator
}

// ========== 日志轮转 ==========

// LogRotator 日志轮转器
type LogRotator struct {
	path        string
	maxSize     int64 // 最大文件大小（字节）
	maxBackups  int   // 最大备份数量
	maxAge      int   // 最大保留天数
	currentSize int64
	file        *os.File
	mu          sync.Mutex
}

// NewLogRotator 创建日志轮转器
func NewLogRotator(path string, maxSize int64, maxBackups, maxAge int) (*LogRotator, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	rotator := &LogRotator{
		path:       path,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		maxAge:     maxAge,
	}

	if err := rotator.openFile(); err != nil {
		return nil, err
	}

	return rotator, nil
}

func (r *LogRotator) openFile() error {
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	r.file = file
	r.currentSize = info.Size()
	return nil
}

// Write 写入日志
func (r *LogRotator) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentSize+int64(len(p)) > r.maxSize {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = r.file.Write(p)
	r.currentSize += int64(n)
	return n, err
}

func (r *LogRotator) rotate() error {
	if r.file != nil {
		r.file.Close()
	}

	// 重命名当前日志文件
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", r.path, timestamp)
	if err := os.Rename(r.path, backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 清理旧日志
	r.cleanOldLogs()

	return r.openFile()
}

func (r *LogRotator) cleanOldLogs() {
	dir := filepath.Dir(r.path)
	pattern := filepath.Base(r.path) + ".*"

	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	if len(matches) <= r.maxBackups {
		return
	}

	// 删除最旧的文件
	for i := 0; i < len(matches)-r.maxBackups; i++ {
		os.Remove(matches[i])
	}
}

// Close 关闭轮转器
func (r *LogRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// ========== 上下文日志 ==========

type ctxKey struct{}

// WithContext 将日志记录器添加到上下文
func WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext 从上下文获取日志记录器
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return logger
	}
	return NewLogger(nil)
}
