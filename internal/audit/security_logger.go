// Package audit 提供安全日志功能
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

// SecurityEvent 安全事件类型.
type SecurityEvent string

const (
	// EventLoginSuccess 登录成功事件.
	EventLoginSuccess SecurityEvent = "login_success"
	// EventLoginFailed 登录失败事件.
	EventLoginFailed SecurityEvent = "login_failed"
	// EventLogout 登出事件.
	EventLogout SecurityEvent = "logout"
	// EventPasswordChange 密码修改事件.
	EventPasswordChange SecurityEvent = "password_change"
	// EventPermissionChange 权限变更事件.
	EventPermissionChange SecurityEvent = "permission_change"
	// EventAccessDenied 访问拒绝事件.
	EventAccessDenied SecurityEvent = "access_denied"
	// EventSuspiciousActivity 可疑活动事件.
	EventSuspiciousActivity SecurityEvent = "suspicious_activity"
	// EventAccountLocked 账户锁定事件.
	EventAccountLocked SecurityEvent = "account_locked"
	// EventAccountUnlocked 账户解锁事件.
	EventAccountUnlocked SecurityEvent = "account_unlocked"
	// EventMFAEnabled MFA启用事件.
	EventMFAEnabled SecurityEvent = "mfa_enabled"
	// EventMFADisabled MFA禁用事件.
	EventMFADisabled SecurityEvent = "mfa_disabled"
)

// SecurityLogLevel 安全日志级别.
type SecurityLogLevel string

const (
	// LogLevelInfo 信息级别.
	LogLevelInfo SecurityLogLevel = "info"
	// LogLevelWarning 警告级别.
	LogLevelWarning SecurityLogLevel = "warning"
	// LogLevelError 错误级别.
	LogLevelError SecurityLogLevel = "error"
	// LogLevelCritical 严重级别.
	LogLevelCritical SecurityLogLevel = "critical"
)

// SecurityLogEntry 安全日志条目.
type SecurityLogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     SecurityLogLevel       `json:"level"`
	Event     SecurityEvent          `json:"event"`
	UserID    string                 `json:"user_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SecurityLogger 安全日志器.
type SecurityLogger struct {
	mu      sync.RWMutex
	logDir  string
	entries []SecurityLogEntry
	maxSize int
}

// NewSecurityLogger 创建安全日志器.
func NewSecurityLogger(logDir string, maxSize int) (*SecurityLogger, error) {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	return &SecurityLogger{
		logDir:  logDir,
		entries: make([]SecurityLogEntry, 0),
		maxSize: maxSize,
	}, nil
}

// Log 记录安全日志.
func (l *SecurityLogger) Log(ctx context.Context, level SecurityLogLevel, event SecurityEvent, message string, details map[string]interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := SecurityLogEntry{
		ID:        generateSecurityLogID(),
		Timestamp: time.Now(),
		Level:     level,
		Event:     event,
		Message:   message,
		Details:   details,
	}

	l.entries = append(l.entries, entry)

	if err := l.writeToFile(entry); err != nil {
		return err
	}

	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	return nil
}

// LogWithContext 带用户上下文的安全日志.
func (l *SecurityLogger) LogWithContext(ctx context.Context, level SecurityLogLevel, event SecurityEvent, userID, username, ip, message string, details map[string]interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := SecurityLogEntry{
		ID:        generateSecurityLogID(),
		Timestamp: time.Now(),
		Level:     level,
		Event:     event,
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Message:   message,
		Details:   details,
	}

	l.entries = append(l.entries, entry)

	if err := l.writeToFile(entry); err != nil {
		return err
	}

	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	return nil
}

// writeToFile 写入安全日志文件.
func (l *SecurityLogger) writeToFile(entry SecurityLogEntry) error {
	filename := filepath.Join(l.logDir, fmt.Sprintf("security_%s.log", entry.Timestamp.Format("2006-01-02")))
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.WriteString(string(data) + "\n")
	return err
}

// Query 查询安全日志.
func (l *SecurityLogger) Query(ctx context.Context, filter SecurityLogFilter) ([]SecurityLogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []SecurityLogEntry
	for _, entry := range l.entries {
		if filter.Match(entry) {
			result = append(result, entry)
		}
	}
	return result, nil
}

// GetRecentEvents 获取最近的安全事件.
func (l *SecurityLogger) GetRecentEvents(ctx context.Context, count int) []SecurityLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if count > len(l.entries) {
		count = len(l.entries)
	}

	start := len(l.entries) - count
	if start < 0 {
		start = 0
	}

	return l.entries[start:]
}

// SecurityLogFilter 安全日志过滤器.
type SecurityLogFilter struct {
	Level     *SecurityLogLevel
	Event     *SecurityEvent
	UserID    string
	StartTime *time.Time
	EndTime   *time.Time
}

// Match 匹配过滤条件.
func (f SecurityLogFilter) Match(entry SecurityLogEntry) bool {
	if f.Level != nil && entry.Level != *f.Level {
		return false
	}
	if f.Event != nil && entry.Event != *f.Event {
		return false
	}
	if f.UserID != "" && entry.UserID != f.UserID {
		return false
	}
	if f.StartTime != nil && entry.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && entry.Timestamp.After(*f.EndTime) {
		return false
	}
	return true
}

func generateSecurityLogID() string {
	return fmt.Sprintf("sec_%d", time.Now().UnixNano())
}
