package smbaudit

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"
)

// AuditResult 审计结果类型
type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailure AuditResult = "failure"
)

// AuditEntry 审计条目
type AuditEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	User      string      `json:"user"`
	IP        string      `json:"ip"`
	Share     string      `json:"share"`
	Path      string      `json:"path"`
	Action    string      `json:"action"`
	Result    AuditResult `json:"result"`
	Details   string      `json:"details,omitempty"`
}

// AuditFilter 审计过滤条件
type AuditFilter struct {
	User      *string       `json:"user,omitempty"`
	IP        *string       `json:"ip,omitempty"`
	Share     *string       `json:"share,omitempty"`
	Action    *string       `json:"action,omitempty"`
	Result    *AuditResult  `json:"result,omitempty"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
}

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled       bool   `json:"enabled"`
	MaxEntries    int    `json:"max_entries"`
	RetentionDays int    `json:"retention_days"`
	LogPath       string `json:"log_path"`
}

// SMBAuditLogger SMB 审计日志管理器
type SMBAuditLogger struct {
	mu      sync.RWMutex
	entries []*AuditEntry
	config  *AuditConfig
	nextID  int
}

// NewSMBAuditLogger 创建新的 SMB 审计日志管理器
func NewSMBAuditLogger(config *AuditConfig) *SMBAuditLogger {
	if config == nil {
		config = &AuditConfig{
			Enabled:       true,
			MaxEntries:    10000,
			RetentionDays: 30,
		}
	}
	return &SMBAuditLogger{
		entries: make([]*AuditEntry, 0),
		config:  config,
		nextID:  1,
	}
}

// Record 记录审计事件
func (l *SMBAuditLogger) Record(entry *AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.ID == "" {
		entry.ID = strconv.Itoa(l.nextID)
		l.nextID++
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	l.entries = append(l.entries, entry)

	// 超过最大条目数时清理旧条目
	if l.config.MaxEntries > 0 && len(l.entries) > l.config.MaxEntries {
		l.entries = l.entries[len(l.entries)-l.config.MaxEntries:]
	}
}

// Query 查询审计事件
func (l *SMBAuditLogger) Query(filter *AuditFilter, limit, offset int) []*AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*AuditEntry
	for _, e := range l.entries {
		if l.matchFilter(e, filter) {
			result = append(result, e)
		}
	}

	if offset >= len(result) {
		return []*AuditEntry{}
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

// GetByUser 按用户获取事件
func (l *SMBAuditLogger) GetByUser(user string, limit int) []*AuditEntry {
	return l.Query(&AuditFilter{User: &user}, limit, 0)
}

// GetStats 获取统计信息
func (l *SMBAuditLogger) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.entries)
	failed := 0
	for _, e := range l.entries {
		if e.Result == AuditResultFailure {
			failed++
		}
	}

	return map[string]interface{}{
		"total_events":   total,
		"failed_events":  failed,
		"success_events": total - failed,
	}
}

// ExportJSON 导出为 JSON
func (l *SMBAuditLogger) ExportJSON() ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return json.Marshal(l.entries)
}

// Clear 清理所有事件
func (l *SMBAuditLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*AuditEntry, 0)
}

// GetConfig 获取配置
func (l *SMBAuditLogger) GetConfig() *AuditConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// UpdateConfig 更新配置
func (l *SMBAuditLogger) UpdateConfig(cfg *AuditConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = cfg
}

// matchFilter 匹配过滤条件
func (l *SMBAuditLogger) matchFilter(entry *AuditEntry, filter *AuditFilter) bool {
	if filter == nil {
		return true
	}
	if filter.User != nil && entry.User != *filter.User {
		return false
	}
	if filter.IP != nil && entry.IP != *filter.IP {
		return false
	}
	if filter.Share != nil && entry.Share != *filter.Share {
		return false
	}
	if filter.Action != nil && entry.Action != *filter.Action {
		return false
	}
	if filter.Result != nil && entry.Result != *filter.Result {
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
