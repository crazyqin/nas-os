// Package logcenter 日志管理器
package logcenter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 日志管理器.
type Manager struct {
	mu        sync.RWMutex
	config    *LogConfig
	logger    *zap.Logger
	entries   []LogEntry
	listeners []chan LogStreamMessage
	hostname  string
}

// NewManager 创建日志管理器.
func NewManager(logger *zap.Logger, config *LogConfig) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	hostname, _ := os.Hostname()

	m := &Manager{
		config:   config,
		logger:   logger,
		entries:  make([]LogEntry, 0, config.MaxEntries),
		hostname: hostname,
	}

	// 启动日志收集
	if config.EnableSyslog {
		go m.collectSyslog()
	}
	if config.EnableAuth {
		go m.collectAuthLog()
	}

	// 启动清理任务
	go m.cleanupLoop()

	return m
}

// Add 添加日志条目.
func (m *Manager) Add(entry LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Hostname == "" {
		entry.Hostname = m.hostname
	}

	m.entries = append(m.entries, entry)

	// 超过上限则裁剪
	if len(m.entries) > m.config.MaxEntries {
		m.entries = m.entries[len(m.entries)-m.config.MaxEntries:]
	}

	// 通知监听者
	msg := LogStreamMessage{Type: "log", Log: &entry}
	for _, ch := range m.listeners {
		select {
		case ch <- msg:
		default:
			// 队列满了跳过
		}
	}

	m.logger.Debug("日志已添加",
		zap.String("level", string(entry.Level)),
		zap.String("source", string(entry.Source)),
		zap.String("message", entry.Message[:min(50, len(entry.Message))]),
	)
}

// Query 查询日志.
func (m *Manager) Query(query LogQuery) LogQueryResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 默认值
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}
	if query.PageSize > 500 {
		query.PageSize = 500
	}

	// 过滤
	var filtered []LogEntry
	for _, entry := range m.entries {
		if !m.matchQuery(entry, query) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 排序
	if query.SortDesc {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Timestamp.After(filtered[j].Timestamp)
		})
	} else {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Timestamp.Before(filtered[j].Timestamp)
		})
	}

	// 分页
	total := len(filtered)
	totalPages := (total + query.PageSize - 1) / query.PageSize
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return LogQueryResult{
		Logs:       filtered[start:end],
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}
}

// matchQuery 匹配查询条件.
func (m *Manager) matchQuery(entry LogEntry, query LogQuery) bool {
	if query.Level != "" && entry.Level != query.Level {
		return false
	}
	if query.Source != "" && entry.Source != query.Source {
		return false
	}
	if query.Category != "" && entry.Category != query.Category {
		return false
	}
	if query.Hostname != "" && entry.Hostname != query.Hostname {
		return false
	}
	if query.Service != "" && entry.Service != query.Service {
		return false
	}
	if !query.StartTime.IsZero() && entry.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && entry.Timestamp.After(query.EndTime) {
		return false
	}
	if query.Keywords != "" {
		msg := strings.ToLower(entry.Message + " " + entry.Details)
		if !strings.Contains(msg, strings.ToLower(query.Keywords)) {
			return false
		}
	}
	return true
}

// GetStats 获取日志统计.
func (m *Manager) GetStats() LogStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := LogStats{
		LevelCounts:  make(map[string]int),
		SourceCounts: make(map[string]int),
		RecentErrors: make([]LogEntry, 0),
	}

	if len(m.entries) == 0 {
		return stats
	}

	stats.TotalCount = len(m.entries)
	stats.OldestLog = m.entries[0].Timestamp
	stats.NewestLog = m.entries[0].Timestamp

	today := time.Now().Truncate(24 * time.Hour)
	errorCount24h := 0
	totalCount24h := 0

	for _, entry := range m.entries {
		// 级别统计
		stats.LevelCounts[string(entry.Level)]++

		// 来源统计
		stats.SourceCounts[string(entry.Source)]++

		// 时间范围
		if entry.Timestamp.Before(stats.OldestLog) {
			stats.OldestLog = entry.Timestamp
		}
		if entry.Timestamp.After(stats.NewestLog) {
			stats.NewestLog = entry.Timestamp
		}

		// 今日统计
		if entry.Timestamp.After(today) {
			stats.TodayCount++
		}

		// 24小时内错误率
		if entry.Timestamp.After(time.Now().Add(-24 * time.Hour)) {
			totalCount24h++
			if entry.Level == LogLevelError || entry.Level == LogLevelFatal {
				errorCount24h++
			}
		}

		// 最近错误（保留最近10条）
		if entry.Level == LogLevelError || entry.Level == LogLevelFatal {
			stats.RecentErrors = append(stats.RecentErrors, entry)
			if len(stats.RecentErrors) > 10 {
				stats.RecentErrors = stats.RecentErrors[1:]
			}
		}
	}

	if totalCount24h > 0 {
		stats.ErrorRate24h = float64(errorCount24h) / float64(totalCount24h) * 100
	}

	return stats
}

// Subscribe 订阅实时日志流.
func (m *Manager) Subscribe() chan LogStreamMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan LogStreamMessage, 100)
	m.listeners = append(m.listeners, ch)
	return ch
}

// Unsubscribe 取消订阅.
func (m *Manager) Unsubscribe(ch chan LogStreamMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, listener := range m.listeners {
		if listener == ch {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// Clear 清空日志.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make([]LogEntry, 0, m.config.MaxEntries)
	m.logger.Info("日志已清空")
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config LogConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = &config
	m.logger.Info("日志配置已更新")
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() LogConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return *m.config
}

// collectSyslog 收集系统日志.
func (m *Manager) collectSyslog() {
	logPaths := []string{
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/kern.log",
	}

	for _, path := range logPaths {
		if _, err := os.Stat(path); err == nil {
			go m.tailLog(path, SourceSystem)
		}
	}
}

// collectAuthLog 收集认证日志.
func (m *Manager) collectAuthLog() {
	logPaths := []string{
		"/var/log/auth.log",
		"/var/log/secure",
	}

	for _, path := range logPaths {
		if _, err := os.Stat(path); err == nil {
			go m.tailLog(path, SourceAuth)
		}
	}
}

// tailLog 实时追踪日志文件.
func (m *Manager) tailLog(path string, source LogSource) {
	file, err := os.Open(path)
	if err != nil {
		m.logger.Warn("无法打开日志文件", zap.String("path", path), zap.Error(err))
		return
	}
	defer file.Close()

	// 跳到文件末尾
	file.Seek(0, 2)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		level := m.parseLogLevel(line)
		category := filepath.Base(path)

		m.Add(LogEntry{
			Level:    level,
			Source:   source,
			Category: category,
			Message:  line,
		})
	}
}

// parseLogLevel 从日志行解析级别.
func (m *Manager) parseLogLevel(line string) LogLevel {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
		return LogLevelError
	case strings.Contains(lower, "warn"):
		return LogLevelWarn
	case strings.Contains(lower, "info"):
		return LogLevelInfo
	case strings.Contains(lower, "debug"):
		return LogLevelDebug
	case strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
		return LogLevelFatal
	default:
		return LogLevelInfo
	}
}

// cleanupLoop 定期清理过期日志.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期日志.
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	var kept []LogEntry

	for _, entry := range m.entries {
		if entry.Timestamp.After(cutoff) {
			kept = append(kept, entry)
		}
	}

	removed := len(m.entries) - len(kept)
	m.entries = kept

	if removed > 0 {
		m.logger.Info("清理过期日志", zap.Int("removed", removed), zap.Int("kept", len(kept)))
	}
}

// GetSources 获取所有日志来源.
func (m *Manager) GetSources() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sourceSet := make(map[string]bool)
	for _, entry := range m.entries {
		sourceSet[string(entry.Source)] = true
	}

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	return sources
}

// GetCategories 获取所有日志分类.
func (m *Manager) GetCategories() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	catSet := make(map[string]bool)
	for _, entry := range m.entries {
		if entry.Category != "" {
			catSet[entry.Category] = true
		}
	}

	categories := make([]string, 0, len(catSet))
	for c := range catSet {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return categories
}

// Export 导出日志.
func (m *Manager) Export(query LogQuery, format string) ([]byte, error) {
	result := m.Query(query)

	switch format {
	case "json":
		return m.exportJSON(result.Logs)
	case "csv":
		return m.exportCSV(result.Logs)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// exportJSON 导出为 JSON.
func (m *Manager) exportJSON(logs []LogEntry) ([]byte, error) {
	import_json := `{"logs":[`
	for i, log := range logs {
		if i > 0 {
			import_json += ","
		}
		import_json += fmt.Sprintf(`{"timestamp":"%s","level":"%s","source":"%s","message":"%s"}`,
			log.Timestamp.Format(time.RFC3339), log.Level, log.Source, strings.ReplaceAll(log.Message, `"`, `\"`))
	}
	import_json += `]}`
	return []byte(import_json), nil
}

// exportCSV 导出为 CSV.
func (m *Manager) exportCSV(logs []LogEntry) ([]byte, error) {
	csv := "timestamp,level,source,category,message\n"
	for _, log := range logs {
		csv += fmt.Sprintf("%s,%s,%s,%s,\"%s\"\n",
			log.Timestamp.Format(time.RFC3339), log.Level, log.Source, log.Category,
			strings.ReplaceAll(log.Message, `"`, `""`))
	}
	return []byte(csv), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
