// Package apikey 使用日志记录
package apikey

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageLogger API 密钥使用日志记录器.
type UsageLogger struct {
	mu       sync.Mutex
	logPath  string
	maxSize  int64
	maxFiles int
}

// NewUsageLogger 创建使用日志记录器.
func NewUsageLogger(logPath string) *UsageLogger {
	return &UsageLogger{
		logPath:  logPath,
		maxSize:  100 * 1024 * 1024, // 100MB
		maxFiles: 10,
	}
}

// Log 记录使用日志.
func (l *UsageLogger) Log(usage APIKeyUsage) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 序列化
	data, err := json.Marshal(usage)
	if err != nil {
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0750); err != nil {
		return
	}

	// 检查轮转
	l.rotateIfNeeded()

	// 写入日志
	f, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return
	}
	if _, err := f.WriteString("\n"); err != nil {
		return
	}
}

// rotateIfNeeded 检查并执行日志轮转.
func (l *UsageLogger) rotateIfNeeded() {
	info, err := os.Stat(l.logPath)
	if err != nil {
		return
	}

	if info.Size() >= l.maxSize {
		l.rotateLog()
	}
}

// rotateLog 执行日志轮转.
func (l *UsageLogger) rotateLog() {
	// 删除最旧文件
	for i := l.maxFiles - 1; i >= 0; i-- {
		oldPath := l.logPath
		if i > 0 {
			oldPath = l.logPath + "." + string(rune('0'+i))
		}
		newPath := l.logPath + "." + string(rune('0'+i+1))
		_ = os.Rename(oldPath, newPath) // 忽略轮转错误，继续处理下一个
	}
}

// GetUsageStats 获取使用统计.
func (l *UsageLogger) GetUsageStats(keyID string, since time.Time) (map[string]interface{}, error) {
	// 读取日志文件并统计（简化实现）
	f, err := os.Open(l.logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := map[string]interface{}{
		"total_requests":  0,
		"success_count":   0,
		"failure_count":   0,
		"avg_response_ms": 0,
		"actions":         make(map[string]int),
	}

	// 实际实现应逐行解析 JSON 日志
	return stats, nil
}
