package sftp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TransferLog 传输日志记录.
type TransferLog struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Username   string    `json:"username"`
	ClientIP   string    `json:"client_ip"`
	SessionID  string    `json:"session_id"`
	Direction  string    `json:"direction"` // "upload" or "download"
	FilePath   string    `json:"file_path"`
	FileSize   int64     `json:"file_size"`
	BytesTrans int64     `json:"bytes_transferred"`
	Duration   int64     `json:"duration_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Bandwidth  int64     `json:"bandwidth_bps"`
	Method     string    `json:"method"` // sftp
}

// TransferLogger 传输日志管理器.
type TransferLogger struct {
	mu        sync.RWMutex
	logs      []*TransferLog
	logFile   *os.File
	logPath   string
	maxLogs   int
	maxSize   int64
	retention time.Duration
	enabled   bool
}

// TransferLoggerConfig 日志配置.
type TransferLoggerConfig struct {
	LogPath   string        `json:"log_path"`
	MaxLogs   int           `json:"max_logs"`
	MaxSize   int64         `json:"max_size"`
	Retention time.Duration `json:"retention"`
	Enabled   bool          `json:"enabled"`
}

// DefaultTransferLoggerConfig 默认配置.
func DefaultTransferLoggerConfig() TransferLoggerConfig {
	return TransferLoggerConfig{
		LogPath:   "/var/log/nas-os/sftp-transfers.jsonl",
		MaxLogs:   10000,
		MaxSize:   100 * 1024 * 1024,
		Retention: 30 * 24 * time.Hour,
		Enabled:   true,
	}
}

// NewTransferLogger 创建传输日志管理器.
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

// initLogFile 初始化日志文件.
func (l *TransferLogger) initLogFile() error {
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0750); err != nil {
		return err
	}

	file, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	l.logFile = file
	return nil
}

// Log 记录传输日志.
func (l *TransferLogger) Log(log *TransferLog) {
	if !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	l.logs = append(l.logs, log)

	if l.maxLogs > 0 && len(l.logs) > l.maxLogs {
		l.logs = l.logs[len(l.logs)-l.maxLogs:]
	}

	if l.logFile != nil {
		data, err := json.Marshal(log)
		if err == nil {
			_, _ = l.logFile.Write(data)
			_, _ = l.logFile.Write([]byte("\n"))
		}

		if l.maxSize > 0 {
			if info, err := l.logFile.Stat(); err == nil && info.Size() > l.maxSize {
				l.rotateLog()
			}
		}
	}
}

// StartTransfer 开始传输跟踪.
func (l *TransferLogger) StartTransfer(username, clientIP, sessionID, direction, filePath string, fileSize int64) *TransferLog {
	return &TransferLog{
		ID:        generateTransferID(),
		Timestamp: time.Now(),
		Username:  username,
		ClientIP:  clientIP,
		SessionID: sessionID,
		Direction: direction,
		FilePath:  filePath,
		FileSize:  fileSize,
		Method:    "sftp",
	}
}

// CompleteTransfer 完成传输记录.
func (l *TransferLogger) CompleteTransfer(log *TransferLog, bytesTrans int64, duration time.Duration, success bool, errMsg string) {
	log.BytesTrans = bytesTrans
	log.Duration = duration.Milliseconds()
	log.Success = success
	log.Error = errMsg

	if duration > 0 && bytesTrans > 0 {
		log.Bandwidth = int64(float64(bytesTrans) / duration.Seconds() * 8)
	}

	l.Log(log)
}

// GetLogs 获取日志列表.
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

// GetStats 获取传输统计.
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

		stats.AvgBandwidth += log.Bandwidth
	}

	if stats.TotalTransfers > 0 {
		stats.AvgBandwidth /= int64(stats.TotalTransfers)
	}

	return stats
}

// Clear 清除日志.
func (l *TransferLogger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logs = make([]*TransferLog, 0)

	if l.logFile != nil {
		_ = l.logFile.Close()
		_ = os.Truncate(l.logPath, 0)
		_ = l.initLogFile()
	}

	return nil
}

// rotateLog 轮转日志文件.
func (l *TransferLogger) rotateLog() {
	if l.logFile == nil {
		return
	}

	_ = l.logFile.Close()
	backup := l.logPath + "." + time.Now().Format("20060102-150405")
	_ = os.Rename(l.logPath, backup)
	_ = l.initLogFile()
}

// Close 关闭日志管理器.
func (l *TransferLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// SetEnabled 设置是否启用.
func (l *TransferLogger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled 检查是否启用.
func (l *TransferLogger) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// TransferLogFilter 日志过滤器.
type TransferLogFilter struct {
	Username  string    `json:"username,omitempty"`
	Direction string    `json:"direction,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// Match 检查日志是否匹配过滤条件.
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
	return true
}

// TransferStats 传输统计.
type TransferStats struct {
	StartTime           time.Time `json:"start_time"`
	TotalTransfers      int       `json:"total_transfers"`
	Uploads             int       `json:"uploads"`
	Downloads           int       `json:"downloads"`
	SuccessfulTransfers int       `json:"successful_transfers"`
	FailedTransfers     int       `json:"failed_transfers"`
	TotalUploadBytes    int64     `json:"total_upload_bytes"`
	TotalDownloadBytes  int64     `json:"total_download_bytes"`
	AvgBandwidth        int64     `json:"avg_bandwidth_bps"`
}

// generateTransferID 生成传输 ID.
func generateTransferID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
