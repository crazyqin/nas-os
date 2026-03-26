// Package search 提供全局搜索服务
// 包含日志搜索支持
package search

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel 日志级别.
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
)

// LogEntry 日志条目.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Source    string    `json:"source"`    // 日志来源: system, audit, access, container
	Service   string    `json:"service"`   // 服务名称
	Message   string    `json:"message"`   // 日志消息
	File      string    `json:"file"`      // 日志文件路径
	Line      int       `json:"line"`      // 行号
	RawLine   string    `json:"rawLine"`   // 原始行内容
	Metadata  map[string]string `json:"metadata,omitempty"` // 元数据
}

// LogSearchRequest 日志搜索请求.
type LogSearchRequest struct {
	Query      string     `json:"query"`                // 搜索关键词
	Level      LogLevel   `json:"level,omitempty"`      // 日志级别过滤
	Source     string     `json:"source,omitempty"`     // 日志来源过滤
	Service    string     `json:"service,omitempty"`    // 服务过滤
	StartTime  *time.Time `json:"startTime,omitempty"` // 开始时间
	EndTime    *time.Time `json:"endTime,omitempty"`    // 结束时间
	Paths      []string   `json:"paths,omitempty"`      // 日志文件路径
	Limit      int        `json:"limit,omitempty"`      // 结果数量限制
	IgnoreCase bool       `json:"ignoreCase,omitempty"` // 忽略大小写
}

// LogSearchResult 日志搜索结果.
type LogSearchResult struct {
	Entry      LogEntry `json:"entry"`
	Score      float64  `json:"score"`
	MatchField string   `json:"matchField"`
}

// LogSearchResponse 日志搜索响应.
type LogSearchResponse struct {
	Query    string             `json:"query"`
	Total    int                `json:"total"`
	Took     time.Duration      `json:"took"`
	Results  []LogSearchResult  `json:"results"`
	Sources  map[string]int     `json:"sources"`  // 各来源日志数量
	Levels   map[LogLevel]int   `json:"levels"`   // 各级别日志数量
	Services map[string]int     `json:"services"` // 各服务日志数量
}

// LogRegistry 日志注册表.
type LogRegistry struct {
	logDirs   []string
	maxLines  int
	mu        sync.RWMutex
}

// NewLogRegistry 创建日志注册表.
func NewLogRegistry(logDirs []string) *LogRegistry {
	if len(logDirs) == 0 {
		logDirs = []string{
			"/var/log/nas-os",
			"/var/log",
			"/opt/nas-os/logs",
		}
	}
	return &LogRegistry{
		logDirs:  logDirs,
		maxLines: 10000, // 默认最多读取10000行
	}
}

// SearchLogs 搜索日志.
func (r *LogRegistry) SearchLogs(req LogSearchRequest) (*LogSearchResponse, error) {
	startTime := time.Now()

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	response := &LogSearchResponse{
		Query:    req.Query,
		Sources:  make(map[string]int),
		Levels:   make(map[LogLevel]int),
		Services: make(map[string]int),
		Results:  make([]LogSearchResult, 0),
	}

	query := req.Query
	if req.IgnoreCase {
		query = strings.ToLower(query)
	}

	// 确定搜索路径
	paths := req.Paths
	if len(paths) == 0 {
		paths = r.getLogFiles()
	}

	// 搜索日志文件
	for _, path := range paths {
		entries := r.searchLogFile(path, query, req)
		for _, result := range entries {
			response.Results = append(response.Results, result)
			response.Sources[result.Entry.Source]++
			response.Levels[result.Entry.Level]++
			if result.Entry.Service != "" {
				response.Services[result.Entry.Service]++
			}
		}
	}

	// 按分数和时间排序
	sortLogResults(response.Results)

	// 限制数量
	response.Total = len(response.Results)
	if len(response.Results) > req.Limit {
		response.Results = response.Results[:req.Limit]
	}

	response.Took = time.Since(startTime)
	return response, nil
}

// getLogFiles 获取日志文件列表.
func (r *LogRegistry) getLogFiles() []string {
	var files []string

	for _, dir := range r.logDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			// 支持普通日志文件和gzip压缩的日志
			if strings.HasSuffix(path, ".log") ||
				strings.HasSuffix(path, ".log.gz") ||
				strings.HasSuffix(path, ".txt") {
				files = append(files, path)
			}
			return nil
		})
	}

	return files
}

// searchLogFile 搜索单个日志文件.
func (r *LogRegistry) searchLogFile(path, query string, req LogSearchRequest) []LogSearchResult {
	var results []LogSearchResult

	var reader io.ReadCloser
	var err error

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		return results
	}
	defer file.Close()

	// 检查是否是gzip文件
	if strings.HasSuffix(path, ".gz") {
		reader, err = gzip.NewReader(file)
		if err != nil {
			return results
		}
		defer reader.Close()
	} else {
		reader = file
	}

	scanner := bufio.NewScanner(reader)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		entry := r.parseLogLine(line, path, lineNum)
		if entry == nil {
			continue
		}

		// 应用过滤器
		if !r.matchFilters(entry, req) {
			continue
		}

		// 搜索匹配
		score, matchField := r.matchQuery(entry, query, req.IgnoreCase)
		if score > 0 {
			results = append(results, LogSearchResult{
				Entry:      *entry,
				Score:      score,
				MatchField: matchField,
			})
		}
	}

	return results
}

// parseLogLine 解析日志行.
func (r *LogRegistry) parseLogLine(line string, file string, lineNum int) *LogEntry {
	if line == "" {
		return nil
	}

	entry := &LogEntry{
		RawLine: line,
		File:    file,
		Line:    lineNum,
		Level:   LogLevelInfo, // 默认级别
	}

	// 尝试解析标准格式日志
	// 格式1: 2024-01-15 10:30:45 [INFO] [storage] message
	// 格式2: 2024/01/15 10:30:45 INFO storage: message
	// 格式3: Jan 15 10:30:45 hostname service[pid]: message

	// 解析时间戳
	if len(line) >= 19 {
		// 尝试解析 "2006-01-02 15:04:05" 格式
		if t, err := time.Parse("2006-01-02 15:04:05", line[:19]); err == nil {
			entry.Timestamp = t
			line = line[20:]
		}
	}

	// 解析日志级别
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[") {
		end := strings.Index(line, "]")
		if end > 0 {
			levelStr := strings.ToLower(line[1:end])
			entry.Level = LogLevel(levelStr)
			line = strings.TrimSpace(line[end+1:])
		}
	} else {
		// 格式2: INFO service: message
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			levelStr := strings.ToLower(parts[0])
			if isValidLevel(levelStr) {
				entry.Level = LogLevel(levelStr)
				if len(parts) > 1 {
					line = parts[1]
				}
			}
		}
	}

	// 解析服务/来源
	if strings.HasPrefix(line, "[") {
		end := strings.Index(line, "]")
		if end > 0 {
			entry.Source = line[1:end]
			line = strings.TrimSpace(line[end+1:])
		}
	} else {
		// 格式: service: message
		if idx := strings.Index(line, ":"); idx > 0 && idx < 30 {
			entry.Service = strings.TrimSpace(line[:idx])
			line = strings.TrimSpace(line[idx+1:])
		}
	}

	entry.Message = line

	// 根据文件路径推断来源
	if entry.Source == "" {
		entry.Source = r.inferSourceFromPath(file)
	}

	return entry
}

// inferSourceFromPath 从文件路径推断日志来源.
func (r *LogRegistry) inferSourceFromPath(path string) string {
	filename := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))

	switch {
	case strings.Contains(path, "audit"):
		return "audit"
	case strings.Contains(path, "access"):
		return "access"
	case strings.Contains(path, "container") || strings.Contains(path, "docker"):
		return "container"
	case strings.Contains(dir, "nas-os"):
		return "system"
	default:
		return dir
	}
}

// matchFilters 检查是否匹配过滤器.
func (r *LogRegistry) matchFilters(entry *LogEntry, req LogSearchRequest) bool {
	// 级别过滤
	if req.Level != "" && entry.Level != req.Level {
		return false
	}

	// 来源过滤
	if req.Source != "" && entry.Source != req.Source {
		return false
	}

	// 服务过滤
	if req.Service != "" && entry.Service != req.Service {
		return false
	}

	// 时间过滤
	if req.StartTime != nil && entry.Timestamp.Before(*req.StartTime) {
		return false
	}
	if req.EndTime != nil && entry.Timestamp.After(*req.EndTime) {
		return false
	}

	return true
}

// matchQuery 匹配查询.
func (r *LogRegistry) matchQuery(entry *LogEntry, query string, ignoreCase bool) (float64, string) {
	if query == "" {
		return 1.0, "all"
	}

	message := entry.Message
	level := string(entry.Level)
	source := entry.Source
	service := entry.Service

	if ignoreCase {
		message = strings.ToLower(message)
		level = strings.ToLower(level)
		source = strings.ToLower(source)
		service = strings.ToLower(service)
	}

	// 消息匹配
	if strings.Contains(message, query) {
		return 1.0, "message"
	}

	// 级别匹配
	if strings.Contains(level, query) {
		return 0.9, "level"
	}

	// 来源匹配
	if strings.Contains(source, query) {
		return 0.8, "source"
	}

	// 服务匹配
	if strings.Contains(service, query) {
		return 0.7, "service"
	}

	return 0, ""
}

// isValidLevel 检查是否是有效的日志级别.
func isValidLevel(level string) bool {
	switch LogLevel(level) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError, LogLevelFatal:
		return true
	default:
		return false
	}
}

// sortLogResults 排序日志搜索结果.
func sortLogResults(results []LogSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			// 先按分数排序，再按时间排序
			if results[i].Score < results[j].Score ||
				(results[i].Score == results[j].Score && results[i].Entry.Timestamp.After(results[j].Entry.Timestamp)) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// GetLogSources 获取日志来源列表.
func (r *LogRegistry) GetLogSources() []string {
	return []string{
		"system",
		"audit",
		"access",
		"container",
		"backup",
		"network",
		"storage",
	}
}

// GetLogLevels 获取日志级别列表.
func (r *LogRegistry) GetLogLevels() []LogLevel {
	return []LogLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarning,
		LogLevelError,
		LogLevelFatal,
	}
}

// GetLogFiles 获取日志文件列表.
func (r *LogRegistry) GetLogFiles() []map[string]interface{} {
	files := r.getLogFiles()
	result := make([]map[string]interface{}, 0, len(files))

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		result = append(result, map[string]interface{}{
			"path":     path,
			"size":     info.Size(),
			"modTime":  info.ModTime(),
			"source":   r.inferSourceFromPath(path),
		})
	}

	return result
}

// TailLog 实时跟踪日志.
func (r *LogRegistry) TailLog(path string, lines int) ([]LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	lineNum := 0
	allLines := make([]string, 0)

	// 读取所有行
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
		lineNum++
	}

	// 获取最后N行
	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}

	for i, line := range allLines[start:] {
		entry := r.parseLogLine(line, path, start+i+1)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}

// GetLogStats 获取日志统计.
func (r *LogRegistry) GetLogStats() map[string]interface{} {
	files := r.getLogFiles()
	totalSize := int64(0)

	for _, path := range files {
		info, err := os.Stat(path)
		if err == nil {
			totalSize += info.Size()
		}
	}

	return map[string]interface{}{
		"fileCount": len(files),
		"totalSize": totalSize,
		"logDirs":   r.logDirs,
	}
}