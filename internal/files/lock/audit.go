package lock

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// ========== 锁审计日志存储 ==========

// LockAuditStorage 锁审计日志存储
type LockAuditStorage struct {
	logPath   string
	maxSize   int64
	maxCount  int
	maxAge    int
	signKey   []byte
	entries   []*LockAuditEntry
	mu        sync.RWMutex
	stopCh    chan struct{}
	flushCh   chan struct{}
}

// LockAuditStorageConfig 审计存储配置
type LockAuditStorageConfig struct {
	LogPath   string        // 日志存储路径
	MaxSize   int64         // 单文件最大大小(MB)
	MaxCount  int           // 最大文件数
	MaxAge    int           // 最大保留天数
	SignKey   []byte        // 签名密钥
	FlushInterval time.Duration // 刷新间隔
}

// DefaultLockAuditStorageConfig 默认配置
func DefaultLockAuditStorageConfig() LockAuditStorageConfig {
	return LockAuditStorageConfig{
		LogPath:       "/var/log/nas-os/lock-audit",
		MaxSize:       100, // 100MB
		MaxCount:      30,  // 保留30个文件
		MaxAge:        90,  // 保留90天
		SignKey:       []byte(uuid.New().String()),
		FlushInterval: time.Minute,
	}
}

// NewLockAuditStorage 创建审计存储
func NewLockAuditStorage(config LockAuditStorageConfig) (*LockAuditStorage, error) {
	// 创建日志目录
	if err := os.MkdirAll(config.LogPath, 0750); err != nil {
		return nil, fmt.Errorf("创建审计日志目录失败: %w", err)
	}

	storage := &LockAuditStorage{
		logPath:  config.LogPath,
		maxSize:  config.MaxSize * 1024 * 1024, // 转换为字节
		maxCount: config.MaxCount,
		maxAge:   config.MaxAge,
		signKey:  config.SignKey,
		entries:  make([]*LockAuditEntry, 0),
		stopCh:   make(chan struct{}),
		flushCh:  make(chan struct{}, 1),
	}

	// 启动后台刷新
	go storage.flushLoop(config.FlushInterval)

	// 启动清理任务
	go storage.cleanupLoop()

	return storage, nil
}

// LogLockAudit 记录锁审计日志
func (s *LockAuditStorage) LogLockAudit(entry *LockAuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 添加签名
	s.signEntry(entry)

	s.entries = append(s.entries, entry)

	// 触发异步刷新
	select {
	case s.flushCh <- struct{}{}:
	default:
	}
}

// signEntry 为条目签名
func (s *LockAuditStorage) signEntry(entry *LockAuditEntry) {
	signData := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Event,
		entry.FilePath,
		entry.Owner,
		entry.LockType,
		entry.ID,
	)

	h := hmac.New(sha256.New, s.signKey)
	h.Write([]byte(signData))
	if entry.Details == nil {
		entry.Details = make(map[string]interface{})
	}
	entry.Details["_sig"] = hex.EncodeToString(h.Sum(nil))
}

// VerifyEntry 验证条目签名
func (s *LockAuditStorage) VerifyEntry(entry *LockAuditEntry) bool {
	if entry.Details == nil {
		return false
	}

	sig, ok := entry.Details["_sig"].(string)
	if !ok {
		return false
	}

	// 保存原始签名
	originalSig := sig

	// 移除签名后重新计算
	delete(entry.Details, "_sig")
	s.signEntry(entry)
	newSig, _ := entry.Details["_sig"].(string)

	// 恢复原始签名
	entry.Details["_sig"] = originalSig

	return hmac.Equal([]byte(originalSig), []byte(newSig))
}

// flushLoop 刷新循环
func (s *LockAuditStorage) flushLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			s.flush()
			return
		case <-s.flushCh:
			s.flush()
		case <-ticker.C:
			s.flush()
		}
	}
}

// flush 刷新到文件
func (s *LockAuditStorage) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) == 0 {
		return
	}

	// 按日期分组
	entriesByDate := make(map[string][]*LockAuditEntry)
	for _, entry := range s.entries {
		date := entry.Timestamp.Format("2006-01-02")
		entriesByDate[date] = append(entriesByDate[date], entry)
	}

	// 写入文件
	for date, entries := range entriesByDate {
		filename := filepath.Join(s.logPath, fmt.Sprintf("lock-audit-%s.jsonl", date))
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			data, _ := json.Marshal(entry)
			_, _ = f.Write(data)
			_, _ = f.Write([]byte("\n"))
		}
		_ = f.Close()
	}

	// 清空内存
	s.entries = s.entries[:0]
}

// cleanupLoop 清理循环
func (s *LockAuditStorage) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup 清理过期文件
func (s *LockAuditStorage) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -s.maxAge)

	entries, err := os.ReadDir(s.logPath)
	if err != nil {
		return
	}

	// 按文件名排序（时间升序）
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	// 删除过期文件
	for i, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "lock-audit-") {
			continue
		}

		// 检查文件数量
		if i < len(entries)-s.maxCount {
			_ = os.Remove(filepath.Join(s.logPath, entry.Name()))
			continue
		}

		// 检查文件时间
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(s.logPath, entry.Name()))
		}
	}
}

// Close 关闭存储
func (s *LockAuditStorage) Close() {
	close(s.stopCh)
}

// Query 查询审计日志
func (s *LockAuditStorage) Query(opts LockAuditQueryOptions) (*LockAuditQueryResult, error) {
	s.mu.RLock()
	// 复制内存中的条目
	memoryEntries := make([]*LockAuditEntry, len(s.entries))
	copy(memoryEntries, s.entries)
	s.mu.RUnlock()

	// 合并文件中的条目
	allEntries := memoryEntries

	// 从文件加载（无论是否指定时间范围，当内存为空时也尝试）
	if len(allEntries) == 0 || opts.StartTime != nil || opts.EndTime != nil {
		fileEntries, err := s.loadFromFile(opts)
		if err == nil {
			allEntries = append(allEntries, fileEntries...)
		}
	}

	// 过滤
	filtered := make([]*LockAuditEntry, 0)
	for _, entry := range allEntries {
		if !s.matchesFilter(entry, opts) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 排序（时间降序）
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 分页
	total := len(filtered)
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if end > total {
		end = total
	}

	return &LockAuditQueryResult{
		Total:   total,
		Entries: filtered[start:end],
	}, nil
}

// loadFromFile 从文件加载日志
func (s *LockAuditStorage) loadFromFile(opts LockAuditQueryOptions) ([]*LockAuditEntry, error) {
	var entries []*LockAuditEntry

	// 确定要读取的文件
	var files []string
	if opts.StartTime != nil {
		start := opts.StartTime.Format("2006-01-02")
		end := time.Now().Format("2006-01-02")
		if opts.EndTime != nil {
			end = opts.EndTime.Format("2006-01-02")
		}

		// 列出范围内的文件
		dirEntries, err := os.ReadDir(s.logPath)
		if err != nil {
			return nil, err
		}

		for _, entry := range dirEntries {
			name := entry.Name()
			if !strings.HasPrefix(name, "lock-audit-") {
				continue
			}
			// 提取日期
			date := strings.TrimPrefix(name, "lock-audit-")
			date = strings.TrimSuffix(date, ".jsonl")
			if date >= start && date <= end {
				files = append(files, filepath.Join(s.logPath, name))
			}
		}
	} else {
		// 没有时间范围时，读取所有审计日志文件
		dirEntries, err := os.ReadDir(s.logPath)
		if err != nil {
			return nil, err
		}

		for _, entry := range dirEntries {
			name := entry.Name()
			if strings.HasPrefix(name, "lock-audit-") && strings.HasSuffix(name, ".jsonl") {
				files = append(files, filepath.Join(s.logPath, name))
			}
		}
	}

	// 读取文件
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			var entry LockAuditEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			entries = append(entries, &entry)
		}
	}

	return entries, nil
}

// matchesFilter 检查是否匹配过滤条件
func (s *LockAuditStorage) matchesFilter(entry *LockAuditEntry, opts LockAuditQueryOptions) bool {
	// 时间范围
	if opts.StartTime != nil && entry.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && entry.Timestamp.After(*opts.EndTime) {
		return false
	}

	// 事件类型
	if opts.Event != "" && entry.Event != opts.Event {
		return false
	}

	// 文件路径
	if opts.FilePath != "" && !strings.Contains(entry.FilePath, opts.FilePath) {
		return false
	}

	// 持有者
	if opts.Owner != "" && entry.Owner != opts.Owner {
		return false
	}

	// 客户端ID
	if opts.ClientID != "" && entry.ClientID != opts.ClientID {
		return false
	}

	// 协议
	if opts.Protocol != "" && entry.Protocol != opts.Protocol {
		return false
	}

	return true
}

// LockAuditQueryOptions 查询选项
type LockAuditQueryOptions struct {
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	StartTime *time.Time    `json:"startTime,omitempty"`
	EndTime   *time.Time    `json:"endTime,omitempty"`
	Event     LockAuditEvent `json:"event,omitempty"`
	FilePath  string        `json:"filePath,omitempty"`
	Owner     string        `json:"owner,omitempty"`
	ClientID  string        `json:"clientId,omitempty"`
	Protocol  string        `json:"protocol,omitempty"`
}

// LockAuditQueryResult 查询结果
type LockAuditQueryResult struct {
	Total   int               `json:"total"`
	Entries []*LockAuditEntry `json:"entries"`
}

// ========== 统计报告 ==========

// LockAuditStats 锁审计统计
type LockAuditStats struct {
	TotalEvents     int64             `json:"totalEvents"`
	ByEvent         map[string]int64  `json:"byEvent"`
	ByProtocol      map[string]int64  `json:"byProtocol"`
	ByOwner         map[string]int64  `json:"byOwner"`
	TopFiles        []FileAuditCount  `json:"topFiles"`
	ConflictRate    float64           `json:"conflictRate"`
	PreemptionRate  float64           `json:"preemptionRate"`
	AvgLockDuration int64             `json:"avgLockDuration"` // ms
}

// FileAuditCount 文件审计计数
type FileAuditCount struct {
	FilePath string `json:"filePath"`
	FileName string `json:"fileName"`
	Count    int64  `json:"count"`
}

// GetStats 获取统计信息
func (s *LockAuditStorage) GetStats(startTime, endTime *time.Time) (*LockAuditStats, error) {
	opts := LockAuditQueryOptions{
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     100000,
	}

	result, err := s.Query(opts)
	if err != nil {
		return nil, err
	}

	stats := &LockAuditStats{
		ByEvent:    make(map[string]int64),
		ByProtocol: make(map[string]int64),
		ByOwner:    make(map[string]int64),
	}

	fileCounts := make(map[string]int64)
	var totalDuration int64
	var durationCount int64
	var conflictCount int64

	for _, entry := range result.Entries {
		stats.TotalEvents++

		// 按事件类型统计
		stats.ByEvent[string(entry.Event)]++

		// 按协议统计
		if entry.Protocol != "" {
			stats.ByProtocol[entry.Protocol]++
		}

		// 按持有者统计
		if entry.Owner != "" {
			stats.ByOwner[entry.Owner]++
		}

		// 文件统计
		if entry.FilePath != "" {
			fileCounts[entry.FilePath]++
		}

		// 时长统计
		if entry.Duration > 0 {
			totalDuration += entry.Duration
			durationCount++
		}

		// 冲突统计
		if entry.Event == AuditEventLockConflict || entry.Event == AuditEventLockPreempted {
			conflictCount++
		}
	}

	// 计算平均时长
	if durationCount > 0 {
		stats.AvgLockDuration = totalDuration / durationCount
	}

	// 计算冲突率
	if stats.TotalEvents > 0 {
		stats.ConflictRate = float64(conflictCount) / float64(stats.TotalEvents) * 100
	}

	// Top 文件
	for path, count := range fileCounts {
		stats.TopFiles = append(stats.TopFiles, FileAuditCount{
			FilePath: path,
			FileName: filepath.Base(path),
			Count:    count,
		})
	}
	sort.Slice(stats.TopFiles, func(i, j int) bool {
		return stats.TopFiles[i].Count > stats.TopFiles[j].Count
	})
	if len(stats.TopFiles) > 10 {
		stats.TopFiles = stats.TopFiles[:10]
	}

	return stats, nil
}

// ========== 集成到锁管理器 ==========

// AuditEnabledManager 带审计的锁管理器
type AuditEnabledManager struct {
	*Manager
	storage *LockAuditStorage
}

// NewAuditEnabledManager 创建带审计的锁管理器
func NewAuditEnabledManager(config FileLockConfig, logger *zap.Logger, auditConfig LockAuditStorageConfig) (*AuditEnabledManager, error) {
	storage, err := NewLockAuditStorage(auditConfig)
	if err != nil {
		return nil, err
	}

	manager := NewManager(config, logger)
	manager.SetAuditLogger(storage)

	return &AuditEnabledManager{
		Manager: manager,
		storage: storage,
	}, nil
}

// GetAuditStorage 获取审计存储
func (m *AuditEnabledManager) GetAuditStorage() *LockAuditStorage {
	return m.storage
}

// QueryAuditLogs 查询审计日志
func (m *AuditEnabledManager) QueryAuditLogs(ctx context.Context, opts LockAuditQueryOptions) (*LockAuditQueryResult, error) {
	return m.storage.Query(opts)
}

// GetAuditStats 获取审计统计
func (m *AuditEnabledManager) GetAuditStats(ctx context.Context, startTime, endTime *time.Time) (*LockAuditStats, error) {
	return m.storage.GetStats(startTime, endTime)
}

// Close 关闭管理器
func (m *AuditEnabledManager) Close() {
	m.Manager.Close()
	m.storage.Close()
}