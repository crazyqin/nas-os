// Package smbaudit SMB审计日志 - 文件操作追踪
// 对标群晖SMB审计功能
package smbaudit

import (
	"encoding/json"
	"sync"
	"time"
)

// AuditAction 审计动作
type AuditAction string

const (
	ActionCreate  AuditAction = "create"
	ActionRead    AuditAction = "read"
	ActionWrite   AuditAction = "write"
	ActionDelete  AuditAction = "delete"
	ActionRename  AuditAction = "rename"
	ActionMove    AuditAction = "move"
	ActionCopy    AuditAction = "copy"
	ActionOpen    AuditAction = "open"
	ActionClose   AuditAction = "close"
	ActionLock    AuditAction = "lock"
	ActionUnlock  AuditAction = "unlock"
)

// AuditResult 审计结果
type AuditResult string

const (
	ResultSuccess AuditResult = "success"
	ResultFailure AuditResult = "failure"
	ResultDenied  AuditResult = "denied"
)

// AuditSeverity 审计严重级别
type AuditSeverity string

const (
	SeverityInfo    AuditSeverity = "info"
	SeverityWarning AuditSeverity = "warning"
	SeverityError   AuditSeverity = "error"
	SeverityCritical AuditSeverity = "critical"
)

// AuditEntry 审计条目
type AuditEntry struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	UserID      string        `json:"user_id"`
	Username    string        `json:"username"`
	ClientIP    string        `json:"client_ip"`
	ClientName  string        `json:"client_name,omitempty"`
	ShareName   string        `json:"share_name"`
	FilePath    string        `json:"file_path"`
	Action      AuditAction   `json:"action"`
	Result      AuditResult   `json:"result"`
	Severity    AuditSeverity `json:"severity"`
	
	// 文件信息
	FileSize    int64     `json:"file_size,omitempty"`
	FileType    string    `json:"file_type,omitempty"`
	OldPath     string    `json:"old_path,omitempty"` // rename/move时
	
	// 会话信息
	SessionID   string `json:"session_id,omitempty"`
	TreeID      string `json:"tree_id,omitempty"`
	FID         string `json:"fid,omitempty"`
	
	// 详细信息
	Details     string `json:"details,omitempty"`
	ErrorMsg    string `json:"error_msg,omitempty"`
}

// AuditFilter 审计过滤器
type AuditFilter struct {
	Username   string         `json:"username,omitempty"`
	ClientIP   string         `json:"client_ip,omitempty"`
	ShareName  string         `json:"share_name,omitempty"`
	FilePath   string         `json:"file_path,omitempty"`
	Action     *AuditAction   `json:"action,omitempty"`
	Result     *AuditResult   `json:"result,omitempty"`
	Severity   *AuditSeverity `json:"severity,omitempty"`
	StartTime  *time.Time     `json:"start_time,omitempty"`
	EndTime    *time.Time     `json:"end_time,omitempty"`
}

// Match 检查是否匹配
func (f *AuditFilter) Match(entry *AuditEntry) bool {
	if f == nil {
		return true
	}

	if f.Username != "" && entry.Username != f.Username {
		return false
	}

	if f.ClientIP != "" && entry.ClientIP != f.ClientIP {
		return false
	}

	if f.ShareName != "" && entry.ShareName != f.ShareName {
		return false
	}

	if f.FilePath != "" && entry.FilePath != f.FilePath {
		return false
	}

	if f.Action != nil && entry.Action != *f.Action {
		return false
	}

	if f.Result != nil && entry.Result != *f.Result {
		return false
	}

	if f.Severity != nil && entry.Severity != *f.Severity {
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

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled         bool `json:"enabled"`
	MaxEntries      int  `json:"max_entries"`
	RetentionDays   int  `json:"retention_days"`
	LogToFile       bool `json:"log_file"`
	LogToSyslog     bool `json:"log_syslog"`
	LogToConsole    bool `json:"log_console"`
	LogPath         string `json:"log_path"`
	
	// 审计规则
	LogReads        bool `json:"log_reads"`
	LogWrites       bool `json:"log_writes"`
	LogDeletes      bool `json:"log_deletes"`
	LogFailedOps    bool `json:"log_failed_ops"`
	LogAnonymous    bool `json:"log_anonymous"`
	
	// 告警规则
	AlertOnDelete   bool `json:"alert_on_delete"`
	AlertOnFailure  bool `json:"alert_on_failure"`
	AlertThreshold  int  `json:"alert_threshold"` // 每分钟操作数阈值
}

// DefaultAuditConfig 默认配置
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:       true,
		MaxEntries:    100000,
		RetentionDays: 90,
		LogToFile:     true,
		LogToSyslog:   false,
		LogToConsole:  false,
		LogPath:       "/var/log/nas-os/smb-audit.log",
		LogReads:      false,
		LogWrites:     true,
		LogDeletes:    true,
		LogFailedOps:  true,
		LogAnonymous:  false,
		AlertOnDelete: true,
		AlertOnFailure: true,
		AlertThreshold: 100,
	}
}

// SMBAuditLogger SMB审计日志器
type SMBAuditLogger struct {
	mu      sync.RWMutex
	config  *AuditConfig
	entries []*AuditEntry
	index   map[string][]int // userID -> entry indices
	alerts  []*AuditAlert
}

// AuditAlert 审计告警
type AuditAlert struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Severity  AuditSeverity `json:"severity"`
	EntryID   string    `json:"entry_id,omitempty"`
}

// NewSMBAuditLogger 创建审计日志器
func NewSMBAuditLogger(config *AuditConfig) *SMBAuditLogger {
	if config == nil {
		config = DefaultAuditConfig()
	}

	return &SMBAuditLogger{
		config:  config,
		entries: make([]*AuditEntry, 0),
		index:   make(map[string][]int),
		alerts:  make([]*AuditAlert, 0),
	}
}

// Log 记录审计条目
func (l *SMBAuditLogger) Log(entry *AuditEntry) {
	if !l.config.Enabled {
		return
	}

	// 检查是否应该记录
	if !l.shouldLog(entry) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置默认值
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	if entry.Severity == "" {
		entry.Severity = l.determineSeverity(entry)
	}

	// 添加到条目列表
	idx := len(l.entries)
	l.entries = append(l.entries, entry)

	// 更新索引
	l.index[entry.UserID] = append(l.index[entry.UserID], idx)

	// 检查是否需要告警
	l.checkAlerts(entry)

	// 清理旧条目
	l.cleanup()
}

// shouldLog 检查是否应该记录
func (l *SMBAuditLogger) shouldLog(entry *AuditEntry) bool {
	// 检查操作类型
	switch entry.Action {
	case ActionRead:
		if !l.config.LogReads {
			return false
		}
	case ActionWrite, ActionCreate:
		if !l.config.LogWrites {
			return false
		}
	case ActionDelete:
		if !l.config.LogDeletes {
			return false
		}
	}

	// 检查失败操作
	if entry.Result != ResultSuccess && !l.config.LogFailedOps {
		return false
	}

	// 检查匿名用户
	if entry.Username == "anonymous" && !l.config.LogAnonymous {
		return false
	}

	return true
}

// determineSeverity 确定严重级别
func (l *SMBAuditLogger) determineSeverity(entry *AuditEntry) AuditSeverity {
	// 失败操作
	if entry.Result == ResultFailure || entry.Result == ResultDenied {
		return SeverityWarning
	}

	// 删除操作
	if entry.Action == ActionDelete {
		return SeverityWarning
	}

	// 大文件操作
	if entry.FileSize > 1024*1024*100 { // 100MB
		return SeverityInfo
	}

	return SeverityInfo
}

// checkAlerts 检查告警
func (l *SMBAuditLogger) checkAlerts(entry *AuditEntry) {
	// 删除告警
	if l.config.AlertOnDelete && entry.Action == ActionDelete {
		alert := &AuditAlert{
			ID:        generateAlertID(),
			Timestamp: time.Now(),
			Type:      "file_delete",
			Message:   "文件删除: " + entry.FilePath + " by " + entry.Username,
			Severity:  SeverityWarning,
			EntryID:   entry.ID,
		}
		l.alerts = append(l.alerts, alert)
	}

	// 失败告警
	if l.config.AlertOnFailure && entry.Result != ResultSuccess {
		alert := &AuditAlert{
			ID:        generateAlertID(),
			Timestamp: time.Now(),
			Type:      "operation_failure",
			Message:   "操作失败: " + string(entry.Action) + " " + entry.FilePath,
			Severity:  SeverityError,
			EntryID:   entry.ID,
		}
		l.alerts = append(l.alerts, alert)
	}
}

// cleanup 清理旧条目
func (l *SMBAuditLogger) cleanup() {
	// 检查条目数量限制
	if len(l.entries) > l.config.MaxEntries {
		// 移除最旧的条目
		excess := len(l.entries) - l.config.MaxEntries
		l.entries = l.entries[excess:]
		
		// 重建索引
		l.rebuildIndex()
	}
}

// rebuildIndex 重建索引
func (l *SMBAuditLogger) rebuildIndex() {
	l.index = make(map[string][]int)
	for i, entry := range l.entries {
		l.index[entry.UserID] = append(l.index[entry.UserID], i)
	}
}

// Query 查询审计条目
func (l *SMBAuditLogger) Query(filter *AuditFilter, limit, offset int) []*AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*AuditEntry, 0)

	for _, entry := range l.entries {
		if filter != nil && !filter.Match(entry) {
			continue
		}
		result = append(result, entry)
	}

	// 应用分页
	if offset > 0 && offset < len(result) {
		result = result[offset:]
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result
}

// GetByUser 获取用户审计条目
func (l *SMBAuditLogger) GetByUser(userID string, limit int) []*AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	indices, exists := l.index[userID]
	if !exists {
		return nil
	}

	result := make([]*AuditEntry, 0)
	for i := len(indices) - 1; i >= 0; i-- {
		if len(result) >= limit {
			break
		}
		result = append(result, l.entries[indices[i]])
	}

	return result
}

// GetAlerts 获取告警
func (l *SMBAuditLogger) GetAlerts(limit int) []*AuditAlert {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.alerts) {
		limit = len(l.alerts)
	}

	// 返回最近的告警
	start := len(l.alerts) - limit
	if start < 0 {
		start = 0
	}

	return l.alerts[start:]
}

// GetStats 获取统计信息
func (l *SMBAuditLogger) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := map[string]interface{}{
		"total_entries":   len(l.entries),
		"total_alerts":    len(l.alerts),
		"by_action":       make(map[AuditAction]int),
		"by_result":       make(map[AuditResult]int),
		"by_user":         make(map[string]int),
		"by_share":        make(map[string]int),
	}

	byAction := stats["by_action"].(map[AuditAction]int)
	byResult := stats["by_result"].(map[AuditResult]int)
	byUser := stats["by_user"].(map[string]int)
	byShare := stats["by_share"].(map[string]int)

	for _, entry := range l.entries {
		byAction[entry.Action]++
		byResult[entry.Result]++
		byUser[entry.Username]++
		byShare[entry.ShareName]++
	}

	return stats
}

// Clear 清除所有条目
func (l *SMBAuditLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = make([]*AuditEntry, 0)
	l.index = make(map[string][]int)
	l.alerts = make([]*AuditAlert, 0)
}

// GetConfig 获取配置
func (l *SMBAuditLogger) GetConfig() *AuditConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.config
}

// UpdateConfig 更新配置
func (l *SMBAuditLogger) UpdateConfig(config *AuditConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.config = config
}

// ExportJSON 导出为JSON
func (l *SMBAuditLogger) ExportJSON() ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return json.Marshal(l.entries)
}

// generateAlertID 生成告警ID
func generateAlertID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(4)
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, n)
	for i := range result {
		result[i] = hexChars[time.Now().UnixNano()%16]
	}
	return string(result)
}
