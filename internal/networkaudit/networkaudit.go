package networkaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Protocol 审计协议
type Protocol string

const (
	ProtocolSMB Protocol = "SMB"
	ProtocolNFS Protocol = "NFS"
	ProtocolFTP Protocol = "FTP"
	ProtocolWeb Protocol = "WEB"
	ProtocolAPI Protocol = "API"
)

// AuditAction 审计动作
type AuditAction string

const (
	ActionLogin       AuditAction = "LOGIN"
	ActionLogout      AuditAction = "LOGOUT"
	ActionFileRead    AuditAction = "FILE_READ"
	ActionFileWrite   AuditAction = "FILE_WRITE"
	ActionFileDelete  AuditAction = "FILE_DELETE"
	ActionFileCreate  AuditAction = "FILE_CREATE"
	ActionShareAccess AuditAction = "SHARE_ACCESS"
	ActionPermission  AuditAction = "PERMISSION"
	ActionConfig      AuditAction = "CONFIG"
	ActionAdmin       AuditAction = "ADMIN"
)

// AuditSeverity 严重级别
type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// AuditEntry 审计条目
type AuditEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Protocol  Protocol    `json:"protocol"`
	Action    AuditAction `json:"action"`
	User      string      `json:"user"`
	SourceIP  string      `json:"source_ip"`
	Target    string      `json:"target"`
	Resource  string      `json:"resource"`
	Details   string      `json:"details"`
	Severity  Severity    `json:"severity"`
	SessionID string      `json:"session_id"`
	Status    string      `json:"status"`
	Duration  int64       `json:"duration_ms"`
}

// AuditFilter 审计过滤器
type AuditFilter struct {
	StartTime  *time.Time
	EndTime    *time.Time
	Protocol   *Protocol
	Action     *AuditAction
	User       *string
	SourceIP   *string
	Severity   *Severity
	SearchText string
	Page       int
	PageSize   int
}

// AuditStats 审计统计
type AuditStats struct {
	TotalEntries int                 `json:"total_entries"`
	ByProtocol   map[Protocol]int    `json:"by_protocol"`
	ByAction     map[AuditAction]int `json:"by_action"`
	BySeverity   map[Severity]int    `json:"by_severity"`
	TopUsers     []UserCount         `json:"top_users"`
	TopIPs       []IPCount           `json:"top_ips"`
	FailedLogins int                 `json:"failed_logins"`
	AlertCount   int                 `json:"alert_count"`
}

type UserCount struct {
	User  string `json:"user"`
	Count int    `json:"count"`
}

type IPCount struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Action     AuditAction `json:"action"`
	Threshold  int         `json:"threshold"`
	TimeWindow int         `json:"time_window_minutes"`
	Severity   Severity    `json:"severity"`
	Enabled    bool        `json:"enabled"`
}

// AuditLogger 审计日志系统
type AuditLogger struct {
	logPath        string
	entries        []*AuditEntry
	alerts         []*AlertRule
	mu             sync.RWMutex
	maxEntries     int
	alertCallbacks []func(*AuditEntry)
}

// NewAuditLogger 创建审计日志系统
func NewAuditLogger(logPath string, maxEntries int) *AuditLogger {
	os.MkdirAll(logPath, 0755)
	return &AuditLogger{
		logPath:    logPath,
		maxEntries: maxEntries,
	}
}

// Log 记录审计事件
func (l *AuditLogger) Log(entry *AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	l.persistEntry(entry)
	l.checkAlerts(entry)
}

// Query 查询审计日志
func (l *AuditLogger) Query(filter AuditFilter) ([]*AuditEntry, int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var filtered []*AuditEntry
	for _, e := range l.entries {
		if !l.matchesFilter(e, filter) {
			continue
		}
		filtered = append(filtered, e)
	}
	total := len(filtered)
	if filter.PageSize == 0 {
		filter.PageSize = 50
	}
	start := filter.Page * filter.PageSize
	if start >= total {
		return nil, total
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// GetStats 获取统计信息
func (l *AuditLogger) GetStats(hours int) AuditStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	stats := AuditStats{
		ByProtocol: make(map[Protocol]int),
		ByAction:   make(map[AuditAction]int),
		BySeverity: make(map[Severity]int),
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	userMap := make(map[string]int)
	ipMap := make(map[string]int)
	for _, e := range l.entries {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		stats.TotalEntries++
		stats.ByProtocol[e.Protocol]++
		stats.ByAction[e.Action]++
		stats.BySeverity[e.Severity]++
		userMap[e.User]++
		ipMap[e.SourceIP]++
		if e.Action == ActionLogin && e.Status == "FAILED" {
			stats.FailedLogins++
		}
	}
	for u, c := range userMap {
		stats.TopUsers = append(stats.TopUsers, UserCount{User: u, Count: c})
	}
	for ip, c := range ipMap {
		stats.TopIPs = append(stats.TopIPs, IPCount{IP: ip, Count: c})
	}
	return stats
}

// AddAlertRule 添加告警规则
func (l *AuditLogger) AddAlertRule(rule *AlertRule) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.alerts = append(l.alerts, rule)
}

// OnAlert 注册告警回调
func (l *AuditLogger) OnAlert(cb func(*AuditEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.alertCallbacks = append(l.alertCallbacks, cb)
}

func (l *AuditLogger) checkAlerts(entry *AuditEntry) {
	for _, rule := range l.alerts {
		if !rule.Enabled {
			continue
		}
		if rule.Action == entry.Action && entry.Severity >= rule.Severity {
			for _, cb := range l.alertCallbacks {
				go cb(entry)
			}
		}
	}
}

func (l *AuditLogger) matchesFilter(e *AuditEntry, f AuditFilter) bool {
	if f.StartTime != nil && e.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && e.Timestamp.After(*f.EndTime) {
		return false
	}
	if f.Protocol != nil && e.Protocol != *f.Protocol {
		return false
	}
	if f.Action != nil && e.Action != *f.Action {
		return false
	}
	if f.User != nil && e.User != *f.User {
		return false
	}
	if f.SourceIP != nil && e.SourceIP != *f.SourceIP {
		return false
	}
	if f.Severity != nil && e.Severity < *f.Severity {
		return false
	}
	return true
}

func (l *AuditLogger) persistEntry(entry *AuditEntry) {
	dateStr := entry.Timestamp.Format("2006-01-02")
	logFile := filepath.Join(l.logPath, fmt.Sprintf("audit-%s.jsonl", dateStr))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(entry)
	f.Write(append(data, '\n'))
}

// ExportCSV 导出CSV
func (l *AuditLogger) ExportCSV(filter AuditFilter, destPath string) error {
	entries, _ := l.Query(filter)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString("ID,Timestamp,Protocol,Action,User,SourceIP,Target,Resource,Severity,Status\n")
	for _, e := range entries {
		f.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			e.ID, e.Timestamp.Format(time.RFC3339), e.Protocol, e.Action,
			e.User, e.SourceIP, e.Target, e.Resource, e.Severity, e.Status))
	}
	return nil
}
