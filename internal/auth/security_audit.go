package auth

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SecurityAuditLogger 安全审计日志记录器.
type SecurityAuditLogger struct {
	mu       sync.RWMutex
	logPath  string
	enabled  bool
	maxSize  int64 // 最大日志文件大小 (字节)
	maxFiles int   // 最大保留日志文件数
}

// SecurityAuditEntry 安全审计日志条目.
type SecurityAuditEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Category  string                 `json:"category"` // auth, mfa, password, session
	Event     string                 `json:"event"`
	UserID    string                 `json:"user_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Status    string                 `json:"status"` // success, failure
	Reason    string                 `json:"reason,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// DefaultSecurityAuditLogger 默认安全审计日志记录器.
var DefaultSecurityAuditLogger = &SecurityAuditLogger{
	logPath:  "/var/log/nas-os/security-audit.log",
	enabled:  true,
	maxSize:  100 * 1024 * 1024, // 100MB
	maxFiles: 10,
}

// NewSecurityAuditLogger 创建安全审计日志记录器.
func NewSecurityAuditLogger(logPath string) *SecurityAuditLogger {
	return &SecurityAuditLogger{
		logPath:  logPath,
		enabled:  true,
		maxSize:  100 * 1024 * 1024,
		maxFiles: 10,
	}
}

// SetEnabled 设置是否启用审计日志.
func (l *SecurityAuditLogger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// Log 记录安全审计日志.
func (l *SecurityAuditLogger) Log(entry SecurityAuditEntry) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if !l.enabled {
		return
	}

	// 设置 ID 和时间戳
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 序列化日志条目
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal security audit entry: %v", err)
		return
	}

	// 确保日志目录存在
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0750); err != nil {
		log.Printf("[ERROR] Failed to create security audit log directory: %v", err)
		return
	}

	// 检查日志文件大小并轮转
	l.rotateIfNeeded()

	// 写入日志文件
	f, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		log.Printf("[ERROR] Failed to open security audit log: %v", err)
		return
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		log.Printf("[ERROR] Failed to write security audit log: %v", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		log.Printf("[ERROR] Failed to write security audit log newline: %v", err)
	}
}

// rotateIfNeeded 检查并执行日志轮转.
func (l *SecurityAuditLogger) rotateIfNeeded() {
	info, err := os.Stat(l.logPath)
	if err != nil {
		return // 文件不存在，无需轮转
	}

	if info.Size() >= l.maxSize {
		// 执行日志轮转
		l.rotateLog()
	}
}

// rotateLog 执行日志轮转.
func (l *SecurityAuditLogger) rotateLog() {
	// 删除最旧的日志文件
	oldestLog := l.logPath + "." + string(rune(l.maxFiles-1))
	if err := os.Remove(oldestLog); err != nil && !os.IsNotExist(err) {
		log.Printf("[WARN] Failed to remove oldest log file: %v", err)
	}

	// 重命名现有日志文件
	for i := l.maxFiles - 2; i >= 0; i-- {
		oldPath := l.logPath
		if i > 0 {
			oldPath = l.logPath + "." + string(rune('0'+i))
		}
		newPath := l.logPath + "." + string(rune('0'+i+1))
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[WARN] Failed to rotate log file %s: %v", oldPath, err)
		}
	}
}

// ========== 便捷日志记录方法 ==========

// LogLogin 记录登录事件.
func (l *SecurityAuditLogger) LogLogin(userID, username, ip, userAgent, status, reason string) {
	l.Log(SecurityAuditEntry{
		Category:  "auth",
		Event:     "login",
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Reason:    reason,
	})
}

// LogLogout 记录登出事件.
func (l *SecurityAuditLogger) LogLogout(userID, username, ip string) {
	l.Log(SecurityAuditEntry{
		Category: "auth",
		Event:    "logout",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Status:   "success",
	})
}

// LogLoginAttempt 记录登录尝试.
func (l *SecurityAuditLogger) LogLoginAttempt(username, ip, userAgent, status, reason string) {
	l.Log(SecurityAuditEntry{
		Category:  "auth",
		Event:     "login_attempt",
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Reason:    reason,
	})
}

// LogMFASetup 记录 MFA 设置事件.
func (l *SecurityAuditLogger) LogMFASetup(userID, username, ip, mfaType, status string) {
	l.Log(SecurityAuditEntry{
		Category: "mfa",
		Event:    "mfa_setup",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Action:   mfaType,
		Status:   status,
	})
}

// LogMFAVerify 记录 MFA 验证事件.
func (l *SecurityAuditLogger) LogMFAVerify(userID, username, ip, mfaType, status, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "mfa",
		Event:    "mfa_verify",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Action:   mfaType,
		Status:   status,
		Reason:   reason,
	})
}

// LogPasswordChange 记录密码变更事件.
func (l *SecurityAuditLogger) LogPasswordChange(userID, username, ip, status, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "password",
		Event:    "password_change",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Status:   status,
		Reason:   reason,
	})
}

// LogPasswordReset 记录密码重置事件.
func (l *SecurityAuditLogger) LogPasswordReset(userID, username, ip, status, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "password",
		Event:    "password_reset",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Status:   status,
		Reason:   reason,
	})
}

// LogSessionCreate 记录会话创建事件.
func (l *SecurityAuditLogger) LogSessionCreate(userID, username, ip, userAgent, deviceID string) {
	l.Log(SecurityAuditEntry{
		Category:  "session",
		Event:     "session_create",
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Resource:  deviceID,
		Status:    "success",
	})
}

// LogSessionInvalidate 记录会话失效事件.
func (l *SecurityAuditLogger) LogSessionInvalidate(userID, username, ip, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "session",
		Event:    "session_invalidate",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Status:   "success",
		Reason:   reason,
	})
}

// LogAccountLock 记录账户锁定事件.
func (l *SecurityAuditLogger) LogAccountLock(username, ip, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "auth",
		Event:    "account_lock",
		Username: username,
		IP:       ip,
		Status:   "success",
		Reason:   reason,
	})
}

// LogAccountUnlock 记录账户解锁事件.
func (l *SecurityAuditLogger) LogAccountUnlock(username, adminUser, reason string) {
	l.Log(SecurityAuditEntry{
		Category: "auth",
		Event:    "account_unlock",
		Username: username,
		Details: map[string]interface{}{
			"unlocked_by": adminUser,
		},
		Status: "success",
		Reason: reason,
	})
}

// LogPermissionChange 记录权限变更事件.
func (l *SecurityAuditLogger) LogPermissionChange(userID, username, ip, resource, action, status string) {
	l.Log(SecurityAuditEntry{
		Category: "auth",
		Event:    "permission_change",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Status:   status,
	})
}

// LogSecurityAlert 记录安全告警.
func (l *SecurityAuditLogger) LogSecurityAlert(alertType, severity, message string, details map[string]interface{}) {
	l.Log(SecurityAuditEntry{
		Category: "security",
		Event:    "security_alert",
		Status:   "warning",
		Reason:   message,
		Details: map[string]interface{}{
			"alert_type": alertType,
			"severity":   severity,
			"details":    details,
		},
	})
}

// ========== 全局日志记录函数 ==========

// AuditLog 使用默认记录器记录审计日志.
func AuditLog(entry SecurityAuditEntry) {
	DefaultSecurityAuditLogger.Log(entry)
}

// AuditLogin 记录登录事件.
func AuditLogin(userID, username, ip, userAgent, status, reason string) {
	DefaultSecurityAuditLogger.LogLogin(userID, username, ip, userAgent, status, reason)
}

// AuditMFASetup 记录 MFA 设置事件.
func AuditMFASetup(userID, username, ip, mfaType, status string) {
	DefaultSecurityAuditLogger.LogMFASetup(userID, username, ip, mfaType, status)
}

// AuditPasswordChange 记录密码变更事件.
func AuditPasswordChange(userID, username, ip, status, reason string) {
	DefaultSecurityAuditLogger.LogPasswordChange(userID, username, ip, status, reason)
}
