package smbaudit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditEvent 表示一条 SMB 访问审计事件
type AuditEvent struct {
	EventID   string `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	Username  string `json:"username"`
	ClientIP  string `json:"client_ip"`
	ShareName string `json:"share_name"`
	FilePath  string `json:"file_path"`
	Action    string `json:"action"` // read/write/delete/create/rename/connect/disconnect/permission_change
	Success   bool   `json:"success"`
	Detail    string `json:"detail"`
	SessionID string `json:"session_id"`
}

// AuditConfig 审计模块配置
type AuditConfig struct {
	MaxEvents       int      `json:"max_events"`        // 最大事件数，默认 10000
	RetentionDays   int      `json:"retention_days"`    // 保留天数，默认 90
	EnableFileLogging bool   `json:"enable_file_logging"`
	LogPath         string   `json:"log_path"`          // 默认 /var/log/nas-os/smb-audit.log
	FilteredUsers   []string `json:"filtered_users"`    // 不审计的用户
	FilteredShares  []string `json:"filtered_shares"`   // 不审计的共享
}

// Auditor SMB 访问审计器
type Auditor struct {
	mu           sync.RWMutex
	config       AuditConfig
	events       []AuditEvent
	sessionCount int64
	logFile      *os.File
}

// defaultConfig 返回默认配置
func defaultConfig() AuditConfig {
	return AuditConfig{
		MaxEvents:     10000,
		RetentionDays: 90,
		LogPath:       "/var/log/nas-os/smb-audit.log",
	}
}

// NewAuditor 创建一个新的审计器
func NewAuditor(cfg *AuditConfig) *Auditor {
	config := defaultConfig()
	if cfg != nil {
		if cfg.MaxEvents > 0 {
			config.MaxEvents = cfg.MaxEvents
		}
		if cfg.RetentionDays > 0 {
			config.RetentionDays = cfg.RetentionDays
		}
		if cfg.LogPath != "" {
			config.LogPath = cfg.LogPath
		}
		config.EnableFileLogging = cfg.EnableFileLogging
		config.FilteredUsers = cfg.FilteredUsers
		config.FilteredShares = cfg.FilteredShares
	}

	a := &Auditor{
		config: config,
		events: make([]AuditEvent, 0, 1024),
	}

	if config.EnableFileLogging {
		if err := a.openLogFile(); err != nil {
			log.Printf("⚠️ SMB审计日志文件打开失败: %v", err)
		}
	}

	return a
}

// openLogFile 打开日志文件
func (a *Auditor) openLogFile() error {
	dir := filepath.Dir(a.config.LogPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	f, err := os.OpenFile(a.config.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	a.logFile = f
	return nil
}

// isFiltered 检查用户或共享是否被过滤
func (a *Auditor) isFiltered(username, shareName string) bool {
	for _, u := range a.config.FilteredUsers {
		if strings.EqualFold(u, username) {
			return true
		}
	}
	for _, s := range a.config.FilteredShares {
		if strings.EqualFold(s, shareName) {
			return true
		}
	}
	return false
}

// LogEvent 记录一条审计事件
func (a *Auditor) LogEvent(username, clientIP, shareName, filePath, action string, success bool, detail string) error {
	if a.isFiltered(username, shareName) {
		return nil
	}

	event := AuditEvent{
		EventID:   uuid.New().String(),
		Timestamp: time.Now(),
		Username:  username,
		ClientIP:  clientIP,
		ShareName: shareName,
		FilePath:  filePath,
		Action:    action,
		Success:   success,
		Detail:    detail,
	}

	a.mu.Lock()
	// 超出最大事件数时移除最早的事件
	if len(a.events) >= a.config.MaxEvents {
		a.events = a.events[1:]
	}
	a.events = append(a.events, event)
	a.sessionCount++
	a.mu.Unlock()

	// 文件日志写入
	if a.config.EnableFileLogging && a.logFile != nil {
		a.writeToFile(event)
	}

	return nil
}

// writeToFile 将事件写入日志文件
func (a *Auditor) writeToFile(event AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.logFile == nil {
		return
	}
	line := fmt.Sprintf("[%s] user=%s ip=%s share=%s file=%s action=%s success=%v detail=%s\n",
		event.Timestamp.Format(time.RFC3339),
		event.Username, event.ClientIP, event.ShareName,
		event.FilePath, event.Action, event.Success, event.Detail,
	)
	if _, err := a.logFile.WriteString(line); err != nil {
		log.Printf("⚠️ SMB审计日志写入失败: %v", err)
	}
}

// GetEvents 分页获取审计事件，返回事件列表和总数
func (a *Auditor) GetEvents(limit, offset int) ([]AuditEvent, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	total := len(a.events)
	if offset >= total {
		return []AuditEvent{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// 返回按时间倒序排列
	result := make([]AuditEvent, end-offset)
	for i, j := end-1, 0; i >= offset; i, j = i-1, j+1 {
		result[j] = a.events[i]
	}
	return result, total
}

// GetEventsByUser 按用户名查询事件
func (a *Auditor) GetEventsByUser(username string, limit int) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEvent
	for i := len(a.events) - 1; i >= 0 && len(result) < limit; i-- {
		if strings.EqualFold(a.events[i].Username, username) {
			result = append(result, a.events[i])
		}
	}
	return result
}

// GetEventsByShare 按共享名查询事件
func (a *Auditor) GetEventsByShare(shareName string, limit int) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEvent
	for i := len(a.events) - 1; i >= 0 && len(result) < limit; i-- {
		if strings.EqualFold(a.events[i].ShareName, shareName) {
			result = append(result, a.events[i])
		}
	}
	return result
}

// GetEventsByTimeRange 按时间范围查询事件
func (a *Auditor) GetEventsByTimeRange(start, end time.Time) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEvent
	for _, e := range a.events {
		if (e.Timestamp.Equal(start) || e.Timestamp.After(start)) &&
			(e.Timestamp.Equal(end) || e.Timestamp.Before(end)) {
			result = append(result, e)
		}
	}
	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}

// GetEventsByAction 按操作类型查询事件
func (a *Auditor) GetEventsByAction(action string, limit int) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEvent
	for i := len(a.events) - 1; i >= 0 && len(result) < limit; i-- {
		if strings.EqualFold(a.events[i].Action, action) {
			result = append(result, a.events[i])
		}
	}
	return result
}

// GetFailedEvents 获取所有失败的事件
func (a *Auditor) GetFailedEvents(limit int) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEvent
	for i := len(a.events) - 1; i >= 0 && len(result) < limit; i-- {
		if !a.events[i].Success {
			result = append(result, a.events[i])
		}
	}
	return result
}

// GetAuditStats 获取审计统计信息
func (a *Auditor) GetAuditStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	total := len(a.events)
	byUser := make(map[string]int)
	byShare := make(map[string]int)
	byAction := make(map[string]int)
	failedCount := 0

	for _, e := range a.events {
		byUser[e.Username]++
		byShare[e.ShareName]++
		byAction[e.Action]++
		if !e.Success {
			failedCount++
		}
	}

	var oldest, newest *time.Time
	if total > 0 {
		oldest = &a.events[0].Timestamp
		newest = &a.events[total-1].Timestamp
	}

	stats := map[string]interface{}{
		"total_events":  total,
		"failed_events": failedCount,
		"by_user":       byUser,
		"by_share":      byShare,
		"by_action":     byAction,
		"session_count": a.sessionCount,
	}
	if oldest != nil {
		stats["oldest_event"] = oldest
	}
	if newest != nil {
		stats["newest_event"] = newest
	}
	return stats
}

// ClearEvents 清理指定时间之前的事件，返回清理数量
func (a *Auditor) ClearEvents(before time.Time) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	kept := make([]AuditEvent, 0, len(a.events))
	removed := 0
	for _, e := range a.events {
		if e.Timestamp.Before(before) {
			removed++
		} else {
			kept = append(kept, e)
		}
	}
	a.events = kept
	return removed
}

// ExportEvents 导出指定时间范围的事件，支持 JSON 和 CSV 格式
func (a *Auditor) ExportEvents(start, end time.Time, format string) ([]byte, error) {
	events := a.GetEventsByTimeRange(start, end)

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(events, "", "  ")
	case "csv":
		var buf strings.Builder
		w := csv.NewWriter(&buf)
		// 写入表头
		_ = w.Write([]string{
			"event_id", "timestamp", "username", "client_ip",
			"share_name", "file_path", "action", "success", "detail", "session_id",
		})
		for _, e := range events {
			_ = w.Write([]string{
				e.EventID,
				e.Timestamp.Format(time.RFC3339),
				e.Username,
				e.ClientIP,
				e.ShareName,
				e.FilePath,
				e.Action,
				fmt.Sprintf("%v", e.Success),
				e.Detail,
				e.SessionID,
			})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, fmt.Errorf("CSV编码失败: %w", err)
		}
		return []byte(buf.String()), nil
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s (仅支持 json/csv)", format)
	}
}

// GetConfig 获取当前配置
func (a *Auditor) GetConfig() AuditConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// UpdateConfig 更新配置
func (a *Auditor) UpdateConfig(cfg AuditConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if cfg.MaxEvents > 0 {
		a.config.MaxEvents = cfg.MaxEvents
	}
	if cfg.RetentionDays > 0 {
		a.config.RetentionDays = cfg.RetentionDays
	}
	if cfg.LogPath != "" {
		a.config.LogPath = cfg.LogPath
	}
	a.config.EnableFileLogging = cfg.EnableFileLogging
	a.config.FilteredUsers = cfg.FilteredUsers
	a.config.FilteredShares = cfg.FilteredShares

	// 如果启用文件日志但尚未打开，尝试打开
	if a.config.EnableFileLogging && a.logFile == nil {
		if err := a.openLogFile(); err != nil {
			log.Printf("⚠️ SMB审计日志文件打开失败: %v", err)
		}
	}
}

// Close 关闭审计器，释放资源
func (a *Auditor) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.logFile != nil {
		a.logFile.Close()
		a.logFile = nil
	}
}
