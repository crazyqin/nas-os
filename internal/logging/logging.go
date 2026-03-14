// Package logging 提供结构化日志功能
package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// ParseLevel 解析日志级别
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
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
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
}

// Logger 日志记录器
type Logger struct {
	level     Level
	output    io.Writer
	formatter Formatter
	fields    map[string]interface{}
	source    string
	mu        sync.Mutex
}

// Formatter 日志格式化器
type Formatter interface {
	Format(entry *LogEntry) ([]byte, error)
}

// JSONFormatter JSON 格式化器
type JSONFormatter struct {
	PrettyPrint bool
}

func (f *JSONFormatter) Format(entry *LogEntry) ([]byte, error) {
	if f.PrettyPrint {
		return json.MarshalIndent(entry, "", "  ")
	}
	return json.Marshal(entry)
}

// TextFormatter 文本格式化器
type TextFormatter struct {
	DisableColors bool
	TimeFormat    string
}

func (f *TextFormatter) Format(entry *LogEntry) ([]byte, error) {
	timeFormat := f.TimeFormat
	if timeFormat == "" {
		timeFormat = "2006-01-02 15:04:05.000"
	}

	var prefix string
	if !f.DisableColors {
		prefix = levelColors[entry.Level]
	}

	var fieldsStr string
	if len(entry.Fields) > 0 {
		parts := make([]string, 0, len(entry.Fields))
		for k, v := range entry.Fields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		fieldsStr = " " + strings.Join(parts, " ")
	}

	caller := ""
	if entry.Caller != "" {
		caller = " [" + entry.Caller + "]"
	}

	return []byte(fmt.Sprintf("%s[%s] %s%s%s%s %s%s\n",
		prefix,
		entry.Timestamp.Format(timeFormat),
		entry.Level,
		resetColor,
		caller,
		fieldsStr,
		entry.Message,
		resetColor,
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
	Level       Level
	Output      io.Writer
	Formatter   Formatter
	JSONFormat  bool
	Source      string
	TimeFormat  string
	PrettyPrint bool
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
			config.Formatter = &JSONFormatter{PrettyPrint: config.PrettyPrint}
		} else {
			config.Formatter = &TextFormatter{
				DisableColors: false,
				TimeFormat:    config.TimeFormat,
			}
		}
	}

	return &Logger{
		level:     config.Level,
		output:    config.Output,
		formatter: config.Formatter,
		fields:    make(map[string]interface{}),
		source:    config.Source,
	}
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    make(map[string]interface{}),
		source:    l.source,
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
		source:    l.source,
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// WithSource 设置来源
func (l *Logger) WithSource(source string) *Logger {
	newLogger := l.WithField("source", source)
	newLogger.source = source
	return newLogger
}

// WithCaller 添加调用者信息
func (l *Logger) WithCaller(caller string) *Logger {
	return l.WithField("caller", caller)
}

// WithRequestID 设置请求 ID
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// WithTrace 设置追踪信息
func (l *Logger) WithTrace(traceID, spanID string) *Logger {
	return l.WithFields(map[string]interface{}{
		"trace_id": traceID,
		"span_id":  spanID,
	})
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
		Source:    l.source,
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

	// 将数据和换行符合并为一次原子写入
	output := append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	l.output.Write(output)
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

// GetLevel 获取日志级别
func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// ========== 日志管理器 ==========

// LogManager 日志管理器
type LogManager struct {
	loggers map[string]*Logger
	rotator *LogRotator
	config  *LogConfig
	mu      sync.RWMutex
}

// NewLogManager 创建日志管理器
func NewLogManager() *LogManager {
	return &LogManager{
		loggers: make(map[string]*Logger),
	}
}

// NewLogManagerWithConfig 使用配置创建日志管理器
func NewLogManagerWithConfig(config *LogConfig) *LogManager {
	return &LogManager{
		loggers: make(map[string]*Logger),
		config:  config,
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

	// 双重检查
	if logger, exists := m.loggers[name]; exists {
		return logger
	}

	config := m.config
	if config == nil {
		config = &LogConfig{Level: LevelInfo}
	}
	config.Source = name

	logger = NewLogger(config)
	m.loggers[name] = logger
	return logger
}

// SetRotator 设置日志轮转器
func (m *LogManager) SetRotator(rotator *LogRotator) {
	m.rotator = rotator
}

// GetRotator 获取日志轮转器
func (m *LogManager) GetRotator() *LogRotator {
	return m.rotator
}

// ListLoggers 列出所有日志记录器
func (m *LogManager) ListLoggers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.loggers))
	for name := range m.loggers {
		names = append(names, name)
	}
	return names
}

// ========== 日志轮转 ==========

// RotateConfig 轮转配置
type RotateConfig struct {
	Path       string // 日志文件路径
	MaxSize    int64  // 最大文件大小（字节）
	MaxBackups int    // 最大备份数量
	MaxAge     int    // 最大保留天数
	Compress   bool   // 是否压缩旧日志
	LocalTime  bool   // 使用本地时间
}

// LogRotator 日志轮转器
type LogRotator struct {
	path        string
	maxSize     int64 // 最大文件大小（字节）
	maxBackups  int   // 最大备份数量
	maxAge      int   // 最大保留天数
	compress    bool
	currentSize int64
	file        *os.File
	mu          sync.Mutex
}

// NewLogRotator 创建日志轮转器
func NewLogRotator(path string, maxSize int64, maxBackups, maxAge int) (*LogRotator, error) {
	return NewLogRotatorWithConfig(&RotateConfig{
		Path:       path,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
	})
}

// NewLogRotatorWithConfig 使用配置创建日志轮转器
func NewLogRotatorWithConfig(config *RotateConfig) (*LogRotator, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if err := os.MkdirAll(filepath.Dir(config.Path), 0755); err != nil {
		return nil, err
	}

	rotator := &LogRotator{
		path:       config.Path,
		maxSize:    config.MaxSize,
		maxBackups: config.MaxBackups,
		maxAge:     config.MaxAge,
		compress:   config.Compress,
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

	// 按修改时间排序
	sort.Slice(matches, func(i, j int) bool {
		info1, _ := os.Stat(matches[i])
		info2, _ := os.Stat(matches[j])
		return info1.ModTime().Before(info2.ModTime())
	})

	// 删除最旧的文件
	for i := 0; i < len(matches)-r.maxBackups; i++ {
		os.Remove(matches[i])
	}

	// 删除过期的日志
	if r.maxAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -r.maxAge)
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(match)
			}
		}
	}
}

// ForceRotate 强制轮转
func (r *LogRotator) ForceRotate() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rotate()
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

// GetPath 获取日志文件路径
func (r *LogRotator) GetPath() string {
	return r.path
}

// GetCurrentSize 获取当前文件大小
func (r *LogRotator) GetCurrentSize() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentSize
}

// ListBackups 列出备份文件
func (r *LogRotator) ListBackups() ([]string, error) {
	dir := filepath.Dir(r.path)
	pattern := filepath.Base(r.path) + ".*"
	return filepath.Glob(filepath.Join(dir, pattern))
}

// ========== 日志搜索 ==========

// SearchConfig 搜索配置
type SearchConfig struct {
	Path      string    // 日志目录路径
	Pattern   string    // 搜索模式（支持简单的通配符）
	Level     string    // 日志级别过滤
	StartTime time.Time // 开始时间
	EndTime   time.Time // 结束时间
	Source    string    // 来源过滤
	RequestID string    // 请求 ID 过滤
	Keyword   string    // 关键词搜索
	Limit     int       // 结果限制
	Offset    int       // 偏移量
}

// SearchResult 搜索结果
type SearchResult struct {
	Total   int        `json:"total"`
	Entries []LogEntry `json:"entries"`
	Files   []string   `json:"files,omitempty"`
}

// LogSearcher 日志搜索器
type LogSearcher struct {
	path string
	mu   sync.RWMutex
}

// NewLogSearcher 创建日志搜索器
func NewLogSearcher(path string) *LogSearcher {
	return &LogSearcher{path: path}
}

// Search 搜索日志
func (s *LogSearcher) Search(ctx context.Context, config *SearchConfig) (*SearchResult, error) {
	if config == nil {
		config = &SearchConfig{}
	}

	result := &SearchResult{
		Entries: make([]LogEntry, 0),
		Files:   make([]string, 0),
	}

	// 确定搜索路径
	searchPath := config.Path
	if searchPath == "" {
		searchPath = s.path
	}

	// 获取日志文件列表
	var files []string
	if info, err := os.Stat(searchPath); err == nil && !info.IsDir() {
		files = []string{searchPath}
	} else {
		// 搜索目录下的所有日志文件
		matches, err := filepath.Glob(filepath.Join(searchPath, "*.log"))
		if err != nil {
			return nil, err
		}
		files = matches

		// 也包含备份文件
		backups, _ := filepath.Glob(filepath.Join(searchPath, "*.log.*"))
		files = append(files, backups...)
	}

	result.Files = files

	// 解析日志条目
	for _, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		entries, err := s.parseLogFile(file, config)
		if err != nil {
			continue
		}
		result.Entries = append(result.Entries, entries...)
	}

	// 按时间排序
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Timestamp.After(result.Entries[j].Timestamp)
	})

	result.Total = len(result.Entries)

	// 应用分页
	if config.Offset > 0 || config.Limit > 0 {
		start := config.Offset
		if start > len(result.Entries) {
			start = len(result.Entries)
		}

		end := len(result.Entries)
		if config.Limit > 0 && start+config.Limit < end {
			end = start + config.Limit
		}

		result.Entries = result.Entries[start:end]
	}

	return result, nil
}

// parseLogFile 解析日志文件
func (s *LogSearcher) parseLogFile(path string, config *SearchConfig) ([]LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make([]LogEntry, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 尝试解析 JSON 格式
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// 非 JSON 格式，创建简单条目
			entry = LogEntry{
				Timestamp: time.Now(),
				Message:   line,
				Level:     "INFO",
			}

			// 尝试从文本中提取时间和级别
			s.parseTextLog(line, &entry)
		}

		// 应用过滤条件
		if !s.matchFilter(&entry, config) {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// parseTextLog 从文本日志中解析信息
func (s *LogSearcher) parseTextLog(line string, entry *LogEntry) {
	// 尝试匹配常见格式：[时间] 级别 消息
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return
	}

	// 尝试解析时间
	for i, part := range parts {
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			timeStr := strings.Trim(part, "[]")
			if t, err := time.Parse("2006-01-02 15:04:05.000", timeStr); err == nil {
				entry.Timestamp = t
			} else if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
				entry.Timestamp = t
			}
			parts = append(parts[:i], parts[i+1:]...)
			break
		}
	}

	// 尝试解析级别
	if len(parts) > 0 {
		level := strings.ToUpper(parts[0])
		if level == "DEBUG" || level == "INFO" || level == "WARN" || level == "ERROR" || level == "FATAL" {
			entry.Level = level
			entry.Message = strings.Join(parts[1:], " ")
		} else {
			entry.Message = strings.Join(parts, " ")
		}
	}
}

// matchFilter 检查是否匹配过滤条件
func (s *LogSearcher) matchFilter(entry *LogEntry, config *SearchConfig) bool {
	// 级别过滤
	if config.Level != "" && !strings.EqualFold(entry.Level, config.Level) {
		return false
	}

	// 时间过滤
	if !config.StartTime.IsZero() && entry.Timestamp.Before(config.StartTime) {
		return false
	}
	if !config.EndTime.IsZero() && entry.Timestamp.After(config.EndTime) {
		return false
	}

	// 来源过滤
	if config.Source != "" && entry.Source != config.Source {
		return false
	}

	// 请求 ID 过滤
	if config.RequestID != "" && entry.RequestID != config.RequestID {
		return false
	}

	// 关键词搜索
	if config.Keyword != "" {
		keyword := strings.ToLower(config.Keyword)
		if !strings.Contains(strings.ToLower(entry.Message), keyword) {
			return false
		}
	}

	return true
}

// Stream 实时流式日志
func (s *LogSearcher) Stream(ctx context.Context, config *SearchConfig) (<-chan LogEntry, error) {
	ch := make(chan LogEntry, 100)

	go func() {
		defer close(ch)

		// 获取最新日志文件
		logFile := s.path
		if info, err := os.Stat(s.path); err == nil && info.IsDir() {
			files, _ := filepath.Glob(filepath.Join(s.path, "*.log"))
			if len(files) > 0 {
				// 选择最新的文件
				sort.Slice(files, func(i, j int) bool {
					info1, _ := os.Stat(files[i])
					info2, _ := os.Stat(files[j])
					return info1.ModTime().After(info2.ModTime())
				})
				logFile = files[0]
			}
		}

		file, err := os.Open(logFile)
		if err != nil {
			return
		}
		defer file.Close()

		// 跳到文件末尾
		file.Seek(0, io.SeekEnd)

		reader := bufio.NewReader(file)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var entry LogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				entry = LogEntry{
					Timestamp: time.Now(),
					Message:   line,
					Level:     "INFO",
				}
			}

			if s.matchFilter(&entry, config) {
				ch <- entry
			}
		}
	}()

	return ch, nil
}

// GetStats 获取日志统计
func (s *LogSearcher) GetStats(path string) (map[string]interface{}, error) {
	if path == "" {
		path = s.path
	}

	stats := map[string]interface{}{
		"path":  path,
		"files": 0,
		"size":  int64(0),
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		stats["files"] = 1
		stats["size"] = info.Size()
		return stats, nil
	}

	var totalSize int64
	var fileCount int

	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	stats["files"] = fileCount
	stats["size"] = totalSize
	stats["size_mb"] = totalSize / 1024 / 1024

	return stats, nil
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

// ========== 全局日志 ==========

var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// SetGlobalLogger 设置全局日志记录器
func SetGlobalLogger(logger *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = logger
}

// GetGlobalLogger 获取全局日志记录器
func GetGlobalLogger() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		globalLogger = NewLogger(nil)
	}
	return globalLogger
}

// 全局日志函数
func Debug(message string)                             { GetGlobalLogger().Debug(message) }
func Debugf(format string, args ...interface{})        { GetGlobalLogger().Debugf(format, args...) }
func Info(message string)                              { GetGlobalLogger().Info(message) }
func Infof(format string, args ...interface{})         { GetGlobalLogger().Infof(format, args...) }
func Warn(message string)                              { GetGlobalLogger().Warn(message) }
func Warnf(format string, args ...interface{})         { GetGlobalLogger().Warnf(format, args...) }
func Error(message string)                             { GetGlobalLogger().Error(message) }
func Errorf(format string, args ...interface{})        { GetGlobalLogger().Errorf(format, args...) }
func Fatal(message string)                             { GetGlobalLogger().Fatal(message) }
func Fatalf(format string, args ...interface{})        { GetGlobalLogger().Fatalf(format, args...) }
func WithField(key string, value interface{}) *Logger  { return GetGlobalLogger().WithField(key, value) }
func WithFields(fields map[string]interface{}) *Logger { return GetGlobalLogger().WithFields(fields) }
