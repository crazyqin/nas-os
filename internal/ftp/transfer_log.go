package ftp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TransferLog 传输日志记录
type TransferLog struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Username    string    `json:"username"`
	ClientIP    string    `json:"client_ip"`
	Direction   string    `json:"direction"` // "upload" or "download"
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	BytesTrans  int64     `json:"bytes_transferred"`
	Duration    int64     `json:"duration_ms"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	Bandwidth   int64     `json:"bandwidth_bps"` // 实际传输速率
	Aborted     bool      `json:"aborted"`
	StartOffset int64     `json:"start_offset,omitempty"` // 断点续传偏移
}

// TransferLogger 传输日志管理器
type TransferLogger struct {
	mu        sync.RWMutex
	logs      []*TransferLog
	logFile   *os.File
	logPath   string
	maxLogs   int           // 内存中最大日志数
	maxSize   int64         // 日志文件最大大小 (字节)
	retention time.Duration // 日志保留时间
	enabled   bool
}

// TransferLoggerConfig 日志配置
type TransferLoggerConfig struct {
	LogPath   string        `json:"log_path"`
	MaxLogs   int           `json:"max_logs"`  // 0 = 无限制
	MaxSize   int64         `json:"max_size"`  // 日志文件最大大小 (字节), 0 = 无限制
	Retention time.Duration `json:"retention"` // 日志保留时间, 0 = 永久保留
	Enabled   bool          `json:"enabled"`
}

// DefaultTransferLoggerConfig 默认配置
func DefaultTransferLoggerConfig() TransferLoggerConfig {
	return TransferLoggerConfig{
		LogPath:   "/var/log/nas-os/ftp-transfers.jsonl",
		MaxLogs:   10000,
		MaxSize:   100 * 1024 * 1024,   // 100MB
		Retention: 30 * 24 * time.Hour, // 30 天
		Enabled:   true,
	}
}

// NewTransferLogger 创建传输日志管理器
func NewTransferLogger(config TransferLoggerConfig) (*TransferLogger, error) {
	logger := &TransferLogger{
		logs:      make([]*TransferLog, 0),
		logPath:   config.LogPath,
		maxLogs:   config.MaxLogs,
		maxSize:   config.MaxSize,
		retention: config.Retention,
		enabled:   config.Enabled,
	}

	if config.Enabled && config.LogPath != "" {
		if err := logger.initLogFile(); err != nil {
			return nil, err
		}
	}

	return logger, nil
}

// initLogFile 初始化日志文件
func (l *TransferLogger) initLogFile() error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0755); err != nil {
		return err
	}

	// 打开或创建日志文件
	file, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.logFile = file
	return nil
}

// Log 记录传输日志
func (l *TransferLogger) Log(log *TransferLog) {
	if !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置时间戳
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// 添加到内存
	l.logs = append(l.logs, log)

	// 检查内存限制
	if l.maxLogs > 0 && len(l.logs) > l.maxLogs {
		// 移除最旧的日志
		l.logs = l.logs[len(l.logs)-l.maxLogs:]
	}

	// 写入文件
	if l.logFile != nil {
		data, err := json.Marshal(log)
		if err == nil {
			if _, writeErr := l.logFile.Write(data); writeErr != nil {
				// 写入失败，忽略
			}
			if _, writeErr := l.logFile.Write([]byte("\n")); writeErr != nil {
				// 写入失败，忽略
			}
		}

		// 检查文件大小并轮转
		if l.maxSize > 0 {
			if info, err := l.logFile.Stat(); err == nil && info.Size() > l.maxSize {
				l.rotateLog()
			}
		}
	}
}

// StartTransfer 开始传输跟踪
func (l *TransferLogger) StartTransfer(username, clientIP, direction, filePath string, fileSize int64, startOffset int64) *TransferLog {
	return &TransferLog{
		ID:          generateTransferID(),
		Timestamp:   time.Now(),
		Username:    username,
		ClientIP:    clientIP,
		Direction:   direction,
		FilePath:    filePath,
		FileSize:    fileSize,
		StartOffset: startOffset,
	}
}

// CompleteTransfer 完成传输记录
func (l *TransferLogger) CompleteTransfer(log *TransferLog, bytesTrans int64, duration time.Duration, success bool, errMsg string) {
	log.BytesTrans = bytesTrans
	log.Duration = duration.Milliseconds()
	log.Success = success
	log.Error = errMsg

	// 计算带宽
	if duration > 0 && bytesTrans > 0 {
		log.Bandwidth = int64(float64(bytesTrans) / duration.Seconds() * 8)
	}

	l.Log(log)
}

// GetLogs 获取日志列表
func (l *TransferLogger) GetLogs(limit int, offset int, filter *TransferLogFilter) []*TransferLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*TransferLog

	for i := len(l.logs) - 1; i >= 0; i-- {
		log := l.logs[i]
		if filter != nil && !filter.Match(log) {
			continue
		}
		result = append(result, log)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	if offset > 0 && offset < len(result) {
		result = result[offset:]
	}

	return result
}

// GetStats 获取传输统计
func (l *TransferLogger) GetStats(period time.Duration) *TransferStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := &TransferStats{
		StartTime: time.Now().Add(-period),
	}

	cutoff := time.Now().Add(-period)

	for _, log := range l.logs {
		if log.Timestamp.Before(cutoff) {
			continue
		}

		stats.TotalTransfers++

		if log.Direction == "upload" {
			stats.Uploads++
			stats.TotalUploadBytes += log.BytesTrans
		} else {
			stats.Downloads++
			stats.TotalDownloadBytes += log.BytesTrans
		}

		if log.Success {
			stats.SuccessfulTransfers++
		} else {
			stats.FailedTransfers++
		}

		if log.Aborted {
			stats.AbortedTransfers++
		}

		// 累计带宽
		stats.AvgBandwidth += log.Bandwidth
	}

	if stats.TotalTransfers > 0 {
		stats.AvgBandwidth /= int64(stats.TotalTransfers)
	}

	return stats
}

// Clear 清除日志
func (l *TransferLogger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logs = make([]*TransferLog, 0)

	// 清空文件
	if l.logFile != nil {
		l.logFile.Close()
		os.Truncate(l.logPath, 0)
		l.initLogFile()
	}

	return nil
}

// rotateLog 轮转日志文件
func (l *TransferLogger) rotateLog() {
	if l.logFile == nil {
		return
	}

	// 关闭当前文件
	l.logFile.Close()

	// 重命名旧文件
	backup := l.logPath + "." + time.Now().Format("20060102-150405")
	_ = os.Rename(l.logPath, backup)

	// 创建新文件
	l.initLogFile()

	// 清理旧备份
	if l.retention > 0 {
		go l.cleanupOldBackups()
	}
}

// cleanupOldBackups 清理过期的备份
func (l *TransferLogger) cleanupOldBackups() {
	dir := filepath.Dir(l.logPath)
	base := filepath.Base(l.logPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-l.retention)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == base || !strings.HasPrefix(name, base+".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// Close 关闭日志管理器
func (l *TransferLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// SetEnabled 设置是否启用
func (l *TransferLogger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled 检查是否启用
func (l *TransferLogger) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// TransferLogFilter 日志过滤器
type TransferLogFilter struct {
	Username  string    `json:"username,omitempty"`
	Direction string    `json:"direction,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	MinSize   int64     `json:"min_size,omitempty"`
	MaxSize   int64     `json:"max_size,omitempty"`
}

// Match 检查日志是否匹配过滤条件
func (f *TransferLogFilter) Match(log *TransferLog) bool {
	if f.Username != "" && log.Username != f.Username {
		return false
	}
	if f.Direction != "" && log.Direction != f.Direction {
		return false
	}
	if f.Success != nil && log.Success != *f.Success {
		return false
	}
	if !f.StartTime.IsZero() && log.Timestamp.Before(f.StartTime) {
		return false
	}
	if !f.EndTime.IsZero() && log.Timestamp.After(f.EndTime) {
		return false
	}
	if f.MinSize > 0 && log.BytesTrans < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && log.BytesTrans > f.MaxSize {
		return false
	}
	return true
}

// TransferStats 传输统计
type TransferStats struct {
	StartTime           time.Time `json:"start_time"`
	TotalTransfers      int       `json:"total_transfers"`
	Uploads             int       `json:"uploads"`
	Downloads           int       `json:"downloads"`
	SuccessfulTransfers int       `json:"successful_transfers"`
	FailedTransfers     int       `json:"failed_transfers"`
	AbortedTransfers    int       `json:"aborted_transfers"`
	TotalUploadBytes    int64     `json:"total_upload_bytes"`
	TotalDownloadBytes  int64     `json:"total_download_bytes"`
	AvgBandwidth        int64     `json:"avg_bandwidth_bps"`
}

// generateTransferID 生成传输 ID
func generateTransferID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
