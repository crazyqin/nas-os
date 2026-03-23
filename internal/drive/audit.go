package drive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu      sync.RWMutex
	enabled bool
	logDir  string
	buffer  []AuditEntry
	flushCh chan struct{}
}

// AuditAction 审计动作类型
type AuditAction string

const (
	AuditActionSync    AuditAction = "sync"
	AuditActionLock    AuditAction = "lock"
	AuditActionUnlock  AuditAction = "unlock"
	AuditActionRead    AuditAction = "read"
	AuditActionWrite   AuditAction = "write"
	AuditActionDelete  AuditAction = "delete"
	AuditActionRename  AuditAction = "rename"
	AuditActionShare   AuditAction = "share"
	AuditActionUpload  AuditAction = "upload"
	AuditActionDownload AuditAction = "download"
	AuditActionVersion AuditAction = "version"
)

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Action    AuditAction `json:"action"`
	Path      string      `json:"path"`
	UserID    string      `json:"userId"`
	UserName  string      `json:"userName"`
	ClientIP  string      `json:"clientIp"`
	UserAgent string      `json:"userAgent"`
	Details   string      `json:"details"`
	Status    string      `json:"status"` // success, failed, denied
	Error     string      `json:"error,omitempty"`
}

// AuditFilter 审计日志查询过滤器
type AuditFilter struct {
	StartTime *time.Time  `json:"startTime"`
	EndTime   *time.Time  `json:"endTime"`
	Action    AuditAction `json:"action"`
	UserID    string      `json:"userId"`
	Path      string      `json:"path"`
	Status    string      `json:"status"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(enabled bool) *AuditLogger {
	return &AuditLogger{
		enabled: enabled,
		logDir:  "logs/audit",
		buffer:  make([]AuditEntry, 0, 100),
		flushCh: make(chan struct{}, 1),
	}
}

// Log 记录审计日志
func (a *AuditLogger) Log(action AuditAction, path, details string) {
	if !a.enabled {
		return
	}

	entry := AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Path:      path,
		Details:   details,
		Status:    "success",
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	shouldFlush := len(a.buffer) >= 100
	a.mu.Unlock()

	if shouldFlush {
		go a.Flush()
	}
}

// LogWithUser 记录审计日志 (带用户信息)
func (a *AuditLogger) LogWithUser(action AuditAction, path, userID, userName, clientIP, details string) {
	if !a.enabled {
		return
	}

	entry := AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Path:      path,
		UserID:    userID,
		UserName:  userName,
		ClientIP:  clientIP,
		Details:   details,
		Status:    "success",
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.mu.Unlock()
}

// LogError 记录错误日志
func (a *AuditLogger) LogError(action AuditAction, path, errMsg string) {
	if !a.enabled {
		return
	}

	entry := AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Path:      path,
		Status:    "failed",
		Error:     errMsg,
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.mu.Unlock()
}

// Query 查询审计日志
func (a *AuditLogger) Query(filter AuditFilter) ([]AuditEntry, error) {
	// 确定日志文件范围
	var files []string

	if filter.StartTime != nil && filter.EndTime != nil {
		// 按时间范围查找日志文件
		for t := filter.StartTime.Truncate(24 * time.Hour); t.Before(*filter.EndTime); t = t.Add(24 * time.Hour) {
			file := filepath.Join(a.logDir, t.Format("2006-01-02")+".json")
			if _, err := os.Stat(file); err == nil {
				files = append(files, file)
			}
		}
	} else {
		// 读取今天的日志
		today := time.Now().Format("2006-01-02")
		file := filepath.Join(a.logDir, today+".json")
		files = []string{file}
	}

	var results []AuditEntry

	for _, file := range files {
		entries, err := a.readLogFile(file)
		if err != nil {
			continue
		}

		// 应用过滤条件
		for _, entry := range entries {
			if !a.matchFilter(entry, filter) {
				continue
			}
			results = append(results, entry)
		}
	}

	// 应用分页
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// matchFilter 检查条目是否匹配过滤条件
func (a *AuditLogger) matchFilter(entry AuditEntry, filter AuditFilter) bool {
	if filter.Action != "" && entry.Action != filter.Action {
		return false
	}
	if filter.UserID != "" && entry.UserID != filter.UserID {
		return false
	}
	if filter.Path != "" && entry.Path != filter.Path {
		return false
	}
	if filter.Status != "" && entry.Status != filter.Status {
		return false
	}
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}
	return true
}

// readLogFile 读取日志文件
func (a *AuditLogger) readLogFile(path string) ([]AuditEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []AuditEntry
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Flush 刷新缓冲区到文件
func (a *AuditLogger) Flush() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.buffer) == 0 {
		return nil
	}

	// 确保目录存在
	if err := os.MkdirAll(a.logDir, 0755); err != nil {
		return err
	}

	// 按日期写入文件
	today := time.Now().Format("2006-01-02")
	file := filepath.Join(a.logDir, today+".json")

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range a.buffer {
		data, _ := json.Marshal(entry)
		f.WriteString(string(data) + "\n")
	}

	a.buffer = a.buffer[:0]
	return nil
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// splitLines 分割行
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}