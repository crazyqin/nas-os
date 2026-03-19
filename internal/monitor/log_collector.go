// Package monitor 提供日志收集与查询系统
// 支持系统日志、应用日志、审计日志的收集和查询
package monitor

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogCollector 日志收集器
type LogCollector struct {
	mu      sync.RWMutex
	config  *LogCollectorConfig
	logDirs map[string]*LogSource
	storage LogStorage
	indexer *LogIndexer
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// LogCollectorConfig 日志收集器配置
type LogCollectorConfig struct {
	DataDir       string        `json:"dataDir"`       // 数据存储目录
	MaxSize       int64         `json:"maxSize"`       // 单文件最大大小 (字节)
	MaxAge        time.Duration `json:"maxAge"`        // 最大保留时间
	MaxBackups    int           `json:"maxBackups"`    // 最大备份文件数
	Compress      bool          `json:"compress"`      // 是否压缩
	FlushInterval time.Duration `json:"flushInterval"` // 刷新间隔
	BufferSize    int           `json:"bufferSize"`    // 缓冲区大小
	IndexEnabled  bool          `json:"indexEnabled"`  // 是否启用索引
	IndexInterval time.Duration `json:"indexInterval"` // 索引间隔
}

// LogSource 日志源
type LogSource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        LogType   `json:"type"`
	Format      LogFormat `json:"format"`
	Enabled     bool      `json:"enabled"`
	LastReadPos int64     `json:"lastReadPos"`
	LastReadAt  time.Time `json:"lastReadAt"`
}

// LogType 日志类型
type LogType string

// 日志类型常量
const (
	// LogTypeSystem 系统日志
	LogTypeSystem LogType = "system"
	// LogTypeApp 应用日志
	LogTypeApp LogType = "app"
	// LogTypeAudit 审计日志
	LogTypeAudit LogType = "audit"
	// LogTypeError 错误日志
	LogTypeError LogType = "error"
	// LogTypeAccess 访问日志
	LogTypeAccess LogType = "access"
	// LogTypeCustom 自定义日志
	LogTypeCustom LogType = "custom"
)

// LogFormat 日志格式
type LogFormat string

// 日志格式常量
const (
	// LogFormatJSON JSON 格式
	LogFormatJSON   LogFormat = "json"
	// LogFormatText 纯文本格式
	LogFormatText   LogFormat = "text"
	// LogFormatSyslog Syslog 格式
	LogFormatSyslog LogFormat = "syslog"
	// LogFormatApache Apache 格式
	LogFormatApache LogFormat = "apache"
	// LogFormatNginx Nginx 格式
	LogFormatNginx  LogFormat = "nginx"
)

// LogEntry 日志条目
type LogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Type      LogType                `json:"type"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Raw       string                 `json:"raw,omitempty"`
}

// LogStorage 日志存储接口
type LogStorage interface {
	Write(entry *LogEntry) error
	Query(filter *LogQueryFilter) ([]*LogEntry, int64, error)
	DeleteOlderThan(before time.Time) error
	Close() error
}

// LogQueryFilter 日志查询过滤条件
type LogQueryFilter struct {
	StartTime    *time.Time             `json:"startTime,omitempty"`
	EndTime      *time.Time             `json:"endTime,omitempty"`
	Level        string                 `json:"level,omitempty"`
	Source       string                 `json:"source,omitempty"`
	Type         LogType                `json:"type,omitempty"`
	Message      string                 `json:"message,omitempty"`
	MessageRegex string                 `json:"messageRegex,omitempty"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
	Offset       int                    `json:"offset"`
	Limit        int                    `json:"limit"`
	Order        string                 `json:"order"` // "asc" or "desc"
}

// LogIndexer 日志索引器
type LogIndexer struct {
	mu        sync.RWMutex
	index     map[string][]*LogEntry
	timeIndex map[string][]string // 时间索引
}

// NewLogCollector 创建日志收集器
func NewLogCollector(config *LogCollectorConfig) (*LogCollector, error) {
	if config.DataDir == "" {
		config.DataDir = "/var/log/nas-os"
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30 * 24 * time.Hour // 30 天
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 10
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}
	if config.IndexInterval == 0 {
		config.IndexInterval = 1 * time.Minute
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	storage := NewFileLogStorage(config.DataDir)

	collector := &LogCollector{
		config:  config,
		logDirs: make(map[string]*LogSource),
		storage: storage,
		indexer: NewLogIndexer(),
		stopCh:  make(chan struct{}),
	}

	// 添加默认日志源
	collector.addDefaultSources()

	return collector, nil
}

// addDefaultSources 添加默认日志源
func (lc *LogCollector) addDefaultSources() {
	sources := []*LogSource{
		{
			ID:      "syslog",
			Name:    "系统日志",
			Path:    "/var/log/syslog",
			Type:    LogTypeSystem,
			Format:  LogFormatSyslog,
			Enabled: true,
		},
		{
			ID:      "kern",
			Name:    "内核日志",
			Path:    "/var/log/kern.log",
			Type:    LogTypeSystem,
			Format:  LogFormatSyslog,
			Enabled: true,
		},
		{
			ID:      "auth",
			Name:    "认证日志",
			Path:    "/var/log/auth.log",
			Type:    LogTypeAudit,
			Format:  LogFormatSyslog,
			Enabled: true,
		},
		{
			ID:      "dmesg",
			Name:    "启动日志",
			Path:    "/var/log/dmesg",
			Type:    LogTypeSystem,
			Format:  LogFormatText,
			Enabled: true,
		},
	}

	for _, source := range sources {
		lc.logDirs[source.ID] = source
	}
}

// Start 启动日志收集
func (lc *LogCollector) Start() {
	lc.wg.Add(1)
	go lc.collectLoop()

	lc.wg.Add(1)
	go lc.cleanupLoop()

	if lc.config.IndexEnabled {
		lc.wg.Add(1)
		go lc.indexLoop()
	}
}

// Stop 停止日志收集
func (lc *LogCollector) Stop() {
	close(lc.stopCh)
	lc.wg.Wait()

	if lc.storage != nil {
		_ = lc.storage.Close()
	}
}

// AddSource 添加日志源
func (lc *LogCollector) AddSource(source *LogSource) error {
	if source.ID == "" {
		return fmt.Errorf("日志源 ID 不能为空")
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	source.LastReadAt = time.Now()
	lc.logDirs[source.ID] = source

	return nil
}

// RemoveSource 移除日志源
func (lc *LogCollector) RemoveSource(id string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	delete(lc.logDirs, id)
}

// GetSources 获取日志源列表
func (lc *LogCollector) GetSources() []*LogSource {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	sources := make([]*LogSource, 0, len(lc.logDirs))
	for _, source := range lc.logDirs {
		sources = append(sources, source)
	}

	return sources
}

// collectLoop 收集循环
func (lc *LogCollector) collectLoop() {
	defer lc.wg.Done()

	ticker := time.NewTicker(lc.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lc.stopCh:
			return
		case <-ticker.C:
			lc.collectLogs()
		}
	}
}

// collectLogs 收集日志
func (lc *LogCollector) collectLogs() {
	lc.mu.RLock()
	sources := make([]*LogSource, 0, len(lc.logDirs))
	for _, source := range lc.logDirs {
		if source.Enabled {
			sources = append(sources, source)
		}
	}
	lc.mu.RUnlock()

	for _, source := range sources {
		entries, err := lc.readLogFile(source)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if err := lc.storage.Write(entry); err != nil {
				continue
			}

			// 索引
			if lc.config.IndexEnabled {
				lc.indexer.Index(entry)
			}
		}
	}
}

// readLogFile 读取日志文件
func (lc *LogCollector) readLogFile(source *LogSource) ([]*LogEntry, error) {
	file, err := os.Open(source.Path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// 定位到上次读取位置
	if source.LastReadPos > 0 {
		_, _ = file.Seek(source.LastReadPos, io.SeekStart)
	}

	var entries []*LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry := lc.parseLine(line, source)
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	// 更新读取位置
	if pos, err := file.Seek(0, io.SeekCurrent); err == nil {
		source.LastReadPos = pos
	}
	source.LastReadAt = time.Now()

	return entries, scanner.Err()
}

// parseLine 解析日志行
func (lc *LogCollector) parseLine(line string, source *LogSource) *LogEntry {
	switch source.Format {
	case LogFormatJSON:
		return lc.parseJSONLine(line, source)
	case LogFormatSyslog:
		return lc.parseSyslogLine(line, source)
	default:
		return lc.parseTextLine(line, source)
	}
}

// parseJSONLine 解析 JSON 格式日志
func (lc *LogCollector) parseJSONLine(line string, source *LogSource) *LogEntry {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil
	}

	entry := &LogEntry{
		ID:     generateLogID(),
		Source: source.ID,
		Type:   source.Type,
		Raw:    line,
		Fields: make(map[string]interface{}),
	}

	// 提取时间戳
	if ts, ok := data["timestamp"]; ok {
		switch v := ts.(type) {
		case string:
			entry.Timestamp, _ = time.Parse(time.RFC3339, v)
		case float64:
			entry.Timestamp = time.Unix(int64(v), 0)
		}
	}

	// 提取级别
	if level, ok := data["level"]; ok {
		entry.Level = fmt.Sprintf("%v", level)
	} else {
		entry.Level = "info"
	}

	// 提取消息
	if msg, ok := data["message"]; ok {
		entry.Message = fmt.Sprintf("%v", msg)
	} else if msg, ok := data["msg"]; ok {
		entry.Message = fmt.Sprintf("%v", msg)
	}

	// 复制其他字段
	for k, v := range data {
		switch k {
		case "timestamp", "level", "message", "msg":
			continue
		default:
			entry.Fields[k] = v
		}
	}

	return entry
}

// parseSyslogLine 解析 Syslog 格式日志
func (lc *LogCollector) parseSyslogLine(line string, source *LogSource) *LogEntry {
	// Syslog 格式: Mon Jan 2 15:04:05 hostname process[pid]: message
	// 示例: Mar 15 10:00:00 nas-os sshd[1234]: Accepted password for user

	entry := &LogEntry{
		ID:     generateLogID(),
		Source: source.ID,
		Type:   source.Type,
		Raw:    line,
		Fields: make(map[string]interface{}),
	}

	// 解析时间戳 (使用当前年份)
	if len(line) >= 15 {
		monthStr := line[0:3]
		dayStr := strings.TrimSpace(line[4:6])
		timeStr := line[7:15]

		// 构建完整时间
		year := time.Now().Year()
		timeStr = fmt.Sprintf("%s %s %d %s", monthStr, dayStr, year, timeStr)
		t, err := time.Parse("Jan 2 2006 15:04:05", timeStr)
		if err == nil {
			entry.Timestamp = t
		}
	}

	// 解析主机名和进程
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) >= 2 {
		entry.Message = parts[1]

		// 解析主机名和进程名
		header := parts[0]
		headerParts := strings.Fields(header)
		if len(headerParts) >= 4 {
			hostname := headerParts[3]
			processInfo := ""
			if len(headerParts) >= 5 {
				processInfo = headerParts[4]
			}

			entry.Fields["hostname"] = hostname
			entry.Fields["process_info"] = processInfo

			// 提取进程名和 PID
			if processInfo != "" {
				re := regexp.MustCompile(`(\w+)(?:\[(\d+)\])?`)
				matches := re.FindStringSubmatch(processInfo)
				if len(matches) >= 2 {
					entry.Fields["process"] = matches[1]
					if len(matches) >= 3 && matches[2] != "" {
						entry.Fields["pid"] = matches[2]
					}
				}
			}
		}
	}

	// 判断日志级别
	entry.Level = lc.detectLevel(entry.Message)

	return entry
}

// parseTextLine 解析纯文本日志
func (lc *LogCollector) parseTextLine(line string, source *LogSource) *LogEntry {
	entry := &LogEntry{
		ID:        generateLogID(),
		Timestamp: time.Now(),
		Source:    source.ID,
		Type:      source.Type,
		Message:   line,
		Raw:       line,
		Fields:    make(map[string]interface{}),
	}

	entry.Level = lc.detectLevel(line)

	return entry
}

// detectLevel 检测日志级别
func (lc *LogCollector) detectLevel(message string) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "error") || strings.Contains(lower, "err") {
		return "error"
	}
	if strings.Contains(lower, "warning") || strings.Contains(lower, "warn") {
		return "warning"
	}
	if strings.Contains(lower, "critical") || strings.Contains(lower, "fatal") {
		return "critical"
	}
	if strings.Contains(lower, "debug") {
		return "debug"
	}

	return "info"
}

// Query 查询日志
func (lc *LogCollector) Query(filter *LogQueryFilter) ([]*LogEntry, int64, error) {
	if filter.Limit == 0 {
		filter.Limit = 100
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}

	return lc.storage.Query(filter)
}

// QueryByTimeRange 按时间范围查询
func (lc *LogCollector) QueryByTimeRange(start, end time.Time, limit int) ([]*LogEntry, error) {
	filter := &LogQueryFilter{
		StartTime: &start,
		EndTime:   &end,
		Limit:     limit,
		Order:     "desc",
	}

	entries, _, err := lc.storage.Query(filter)
	return entries, err
}

// QueryByLevel 按级别查询
func (lc *LogCollector) QueryByLevel(level string, limit int) ([]*LogEntry, error) {
	filter := &LogQueryFilter{
		Level: level,
		Limit: limit,
		Order: "desc",
	}

	entries, _, err := lc.storage.Query(filter)
	return entries, err
}

// Search 搜索日志
func (lc *LogCollector) Search(query string, limit int) ([]*LogEntry, error) {
	filter := &LogQueryFilter{
		Message: query,
		Limit:   limit,
		Order:   "desc",
	}

	entries, _, err := lc.storage.Query(filter)
	return entries, err
}

// SearchRegex 正则搜索
func (lc *LogCollector) SearchRegex(pattern string, limit int) ([]*LogEntry, error) {
	filter := &LogQueryFilter{
		MessageRegex: pattern,
		Limit:        limit,
		Order:        "desc",
	}

	entries, _, err := lc.storage.Query(filter)
	return entries, err
}

// cleanupLoop 清理循环
func (lc *LogCollector) cleanupLoop() {
	defer lc.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-lc.stopCh:
			return
		case <-ticker.C:
			lc.cleanupOldLogs()
		}
	}
}

// cleanupOldLogs 清理旧日志
func (lc *LogCollector) cleanupOldLogs() {
	before := time.Now().Add(-lc.config.MaxAge)
	_ = lc.storage.DeleteOlderThan(before)
}

// indexLoop 索引循环
func (lc *LogCollector) indexLoop() {
	defer lc.wg.Done()

	ticker := time.NewTicker(lc.config.IndexInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lc.stopCh:
			return
		case <-ticker.C:
			lc.indexer.Optimize()
		}
	}
}

// GetStats 获取统计信息
func (lc *LogCollector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"sources":     len(lc.logDirs),
		"config":      lc.config,
		"index_stats": lc.indexer.GetStats(),
	}
}

// NewLogIndexer 创建日志索引器
func NewLogIndexer() *LogIndexer {
	return &LogIndexer{
		index:     make(map[string][]*LogEntry),
		timeIndex: make(map[string][]string),
	}
}

// Index 索引日志条目
func (li *LogIndexer) Index(entry *LogEntry) {
	li.mu.Lock()
	defer li.mu.Unlock()

	// 按级别索引
	li.index["level:"+entry.Level] = append(li.index["level:"+entry.Level], entry)

	// 按来源索引
	li.index["source:"+entry.Source] = append(li.index["source:"+entry.Source], entry)

	// 按时间索引 (按小时)
	hourKey := entry.Timestamp.Format("2006-01-02-15")
	li.timeIndex[hourKey] = append(li.timeIndex[hourKey], entry.ID)
}

// Optimize 优化索引
func (li *LogIndexer) Optimize() {
	li.mu.Lock()
	defer li.mu.Unlock()

	// 清理过期的索引
	now := time.Now()
	for key := range li.timeIndex {
		t, err := time.Parse("2006-01-02-15", key)
		if err != nil {
			continue
		}

		if now.Sub(t) > 24*time.Hour {
			delete(li.timeIndex, key)
		}
	}
}

// GetStats 获取索引统计
func (li *LogIndexer) GetStats() map[string]interface{} {
	li.mu.RLock()
	defer li.mu.RUnlock()

	totalEntries := 0
	for _, entries := range li.index {
		totalEntries += len(entries)
	}

	return map[string]interface{}{
		"index_count":   len(li.index),
		"total_entries": totalEntries,
		"time_keys":     len(li.timeIndex),
	}
}

// FileLogStorage 文件日志存储
type FileLogStorage struct {
	dataDir string
	mu      sync.RWMutex
}

// NewFileLogStorage 创建文件日志存储
func NewFileLogStorage(dataDir string) *FileLogStorage {
	return &FileLogStorage{
		dataDir: dataDir,
	}
}

// Write 写入日志
func (s *FileLogStorage) Write(entry *LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 按日期分文件
	date := entry.Timestamp.Format("2006-01-02")
	filename := filepath.Join(s.dataDir, fmt.Sprintf("logs-%s.jsonl", date))

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

// Query 查询日志
func (s *FileLogStorage) Query(filter *LogQueryFilter) ([]*LogEntry, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []*LogEntry
	var total int64

	// 确定要查询的文件
	files := s.getLogFiles(filter)

	for _, file := range files {
		fileEntries, err := s.queryFile(file, filter)
		if err != nil {
			continue
		}

		entries = append(entries, fileEntries...)
		total += int64(len(fileEntries))
	}

	// 排序
	sort.Slice(entries, func(i, j int) bool {
		if filter.Order == "asc" {
			return entries[i].Timestamp.Before(entries[j].Timestamp)
		}
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// 分页
	start := filter.Offset
	end := filter.Offset + filter.Limit
	if start > len(entries) {
		start = len(entries)
	}
	if end > len(entries) {
		end = len(entries)
	}

	return entries[start:end], total, nil
}

// getLogFiles 获取日志文件列表
func (s *FileLogStorage) getLogFiles(filter *LogQueryFilter) []string {
	files, _ := filepath.Glob(filepath.Join(s.dataDir, "logs-*.jsonl"))

	// 按时间过滤
	var result []string
	for _, file := range files {
		// 从文件名提取日期
		base := filepath.Base(file)
		// logs-2006-01-02.jsonl -> 2006-01-02
		dateStr := strings.TrimPrefix(base, "logs-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")

		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// 检查时间范围
		if filter.StartTime != nil && t.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && t.After(*filter.EndTime) {
			continue
		}

		result = append(result, file)
	}

	// 排序（最新的在前）
	sort.Sort(sort.Reverse(sort.StringSlice(result)))

	return result
}

// queryFile 查询单个文件
func (s *FileLogStorage) queryFile(filename string, filter *LogQueryFilter) ([]*LogEntry, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []*LogEntry
	scanner := bufio.NewScanner(f)

	// 编译正则表达式
	var re *regexp.Regexp
	if filter.MessageRegex != "" {
		re, _ = regexp.Compile(filter.MessageRegex)
	}

	for scanner.Scan() {
		line := scanner.Text()
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// 应用过滤条件
		if !s.matchFilter(&entry, filter, re) {
			continue
		}

		entries = append(entries, &entry)
	}

	return entries, scanner.Err()
}

// matchFilter 匹配过滤条件
func (s *FileLogStorage) matchFilter(entry *LogEntry, filter *LogQueryFilter, re *regexp.Regexp) bool {
	// 时间过滤
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	// 级别过滤
	if filter.Level != "" && entry.Level != filter.Level {
		return false
	}

	// 来源过滤
	if filter.Source != "" && entry.Source != filter.Source {
		return false
	}

	// 类型过滤
	if filter.Type != "" && entry.Type != filter.Type {
		return false
	}

	// 消息过滤
	if filter.Message != "" && !strings.Contains(entry.Message, filter.Message) {
		return false
	}

	// 正则过滤
	if re != nil && !re.MatchString(entry.Message) {
		return false
	}

	return true
}

// DeleteOlderThan 删除旧日志
func (s *FileLogStorage) DeleteOlderThan(before time.Time) error {
	files, _ := filepath.Glob(filepath.Join(s.dataDir, "logs-*.jsonl"))

	for _, file := range files {
		// 从文件名提取日期
		base := filepath.Base(file)
		dateStr := strings.TrimPrefix(base, "logs-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")

		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if t.Before(before) {
			// 删除或压缩
			if err := os.Remove(file); err != nil {
				// 尝试压缩
				_ = s.compressFile(file)
			}
		}
	}

	return nil
}

// compressFile 压缩文件
func (s *FileLogStorage) compressFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gzFile := filename + ".gz"
	gz, err := os.Create(gzFile)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	w := gzip.NewWriter(gz)
	defer func() { _ = w.Close() }()

	_, err = io.Copy(w, f)
	if err != nil {
		return err
	}

	return os.Remove(filename)
}

// Close 关闭存储
func (s *FileLogStorage) Close() error {
	return nil
}

// generateLogID 生成日志 ID
func generateLogID() string {
	return fmt.Sprintf("log-%d", time.Now().UnixNano())
}
