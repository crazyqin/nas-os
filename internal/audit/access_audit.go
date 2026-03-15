// Package audit 提供访问审计功能
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AccessAuditEntry 访问审计条目
type AccessAuditEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Method    string                 `json:"method"`
	IP        string                 `json:"ip"`
	UserAgent string                 `json:"user_agent"`
	Status    int                    `json:"status"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AccessAuditLogger 访问审计日志器
type AccessAuditLogger struct {
	mu      sync.RWMutex
	logDir  string
	entries []AccessAuditEntry
	maxSize int
	enabled bool
}

// NewAccessAuditLogger 创建访问审计日志器
func NewAccessAuditLogger(logDir string, maxSize int) (*AccessAuditLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	return &AccessAuditLogger{
		logDir:  logDir,
		entries: make([]AccessAuditEntry, 0),
		maxSize: maxSize,
		enabled: true,
	}, nil
}

// Log 记录访问审计日志
func (l *AccessAuditLogger) Log(ctx context.Context, entry AccessAuditEntry) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry.ID = generateAuditID()
	entry.Timestamp = time.Now()

	l.entries = append(l.entries, entry)

	// 写入文件
	if err := l.writeToFile(entry); err != nil {
		return err
	}

	// 检查是否需要清理
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	return nil
}

// writeToFile 写入日志文件
func (l *AccessAuditLogger) writeToFile(entry AccessAuditEntry) error {
	filename := filepath.Join(l.logDir, fmt.Sprintf("audit_%s.log", entry.Timestamp.Format("2006-01-02")))
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.WriteString(string(data) + "\n")
	return err
}

// Query 查询审计日志
func (l *AccessAuditLogger) Query(ctx context.Context, filter AuditFilter) ([]AccessAuditEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []AccessAuditEntry
	for _, entry := range l.entries {
		if filter.Match(entry) {
			result = append(result, entry)
		}
	}
	return result, nil
}

// Enable 启用审计
func (l *AccessAuditLogger) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = true
}

// Disable 禁用审计
func (l *AccessAuditLogger) Disable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = false
}

// AuditFilter 审计过滤器
type AuditFilter struct {
	UserID    string
	Action    string
	Resource  string
	StartTime *time.Time
	EndTime   *time.Time
	Status    *int
}

// Match 匹配过滤条件
func (f AuditFilter) Match(entry AccessAuditEntry) bool {
	if f.UserID != "" && entry.UserID != f.UserID {
		return false
	}
	if f.Action != "" && entry.Action != f.Action {
		return false
	}
	if f.Resource != "" && entry.Resource != f.Resource {
		return false
	}
	if f.StartTime != nil && entry.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && entry.Timestamp.After(*f.EndTime) {
		return false
	}
	if f.Status != nil && entry.Status != *f.Status {
		return false
	}
	return true
}

func generateAuditID() string {
	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}
