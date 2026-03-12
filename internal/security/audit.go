package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditManager 审计日志管理器
type AuditManager struct {
	config      AuditConfig
	logs        []*AuditLogEntry
	loginLogs   []*LoginLogEntry
	alerts      []*SecurityAlert
	mu          sync.RWMutex
	logPath     string
	maxLogs     int // 最大保留日志数
	autoSave    bool
}

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled        bool          `json:"enabled"`
	LogPath        string        `json:"log_path"`
	MaxLogs        int           `json:"max_logs"` // 最大保留日志数
	MaxAgeDays     int           `json:"max_age_days"` // 最大保留天数
	AutoSave       bool          `json:"auto_save"` // 自动保存到文件
	SaveInterval   time.Duration `json:"save_interval"` // 保存间隔
	AlertEnabled   bool          `json:"alert_enabled"`
	AlertThreshold int           `json:"alert_threshold"` // 告警阈值
}

// NewAuditManager 创建审计日志管理器
func NewAuditManager() *AuditManager {
	return &AuditManager{
		config: AuditConfig{
			Enabled:      true,
			LogPath:      "/var/log/nas-os/audit",
			MaxLogs:      10000,
			MaxAgeDays:   90,
			AutoSave:     true,
			SaveInterval: time.Minute * 5,
			AlertEnabled: true,
			AlertThreshold: 10,
		},
		logs:      make([]*AuditLogEntry, 0),
		loginLogs: make([]*LoginLogEntry, 0),
		alerts:    make([]*SecurityAlert, 0),
	}
}

// SetConfig 设置审计配置
func (am *AuditManager) SetConfig(config AuditConfig) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.config = config
}

// GetConfig 获取审计配置
func (am *AuditManager) GetConfig() AuditConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.config
}

// Log 记录审计日志
func (am *AuditManager) Log(entry AuditLogEntry) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.config.Enabled {
		return
	}

	entry.ID = uuid.New().String()
	entry.Timestamp = time.Now()

	am.logs = append(am.logs, &entry)

	// 限制日志数量
	if len(am.logs) > am.config.MaxLogs {
		am.logs = am.logs[len(am.logs)-am.config.MaxLogs:]
	}

	// 检查是否需要生成告警
	if am.config.AlertEnabled {
		am.checkAlertCondition(&entry)
	}

	// 自动保存
	if am.config.AutoSave {
		go am.saveLogs()
	}
}

// checkAlertCondition 检查是否需要生成告警
func (am *AuditManager) checkAlertCondition(entry *AuditLogEntry) {
	// 根据日志级别和类型判断是否需要告警
	shouldAlert := false
	severity := "low"

	switch entry.Level {
	case "critical":
		shouldAlert = true
		severity = "critical"
	case "error":
		shouldAlert = true
		severity = "high"
	case "warning":
		shouldAlert = true
		severity = "medium"
	}

	// 特定事件类型总是告警
	switch entry.Event {
	case "login_failure_multiple", "firewall_rule_violation", "unauthorized_access":
		shouldAlert = true
		severity = "high"
	}

	if shouldAlert {
		alert := SecurityAlert{
			ID:          generateAlertID(),
			Timestamp:   entry.Timestamp,
			Severity:    severity,
			Type:        entry.Event,
			Title:       fmt.Sprintf("安全事件：%s", entry.Event),
			Description: entry.Message(),
			SourceIP:    entry.IP,
			Username:    entry.Username,
			Details:     entry.Details,
		}
		am.alerts = append(am.alerts, &alert)
	}
}

// Message 生成日志消息
func (e *AuditLogEntry) Message() string {
	msg := fmt.Sprintf("[%s] %s", e.Category, e.Event)
	if e.Username != "" {
		msg += fmt.Sprintf(" by %s", e.Username)
	}
	if e.IP != "" {
		msg += fmt.Sprintf(" from %s", e.IP)
	}
	if e.Status != "" {
		msg += fmt.Sprintf(" - %s", e.Status)
	}
	return msg
}

// LogLogin 记录登录日志
func (am *AuditManager) LogLogin(entry LoginLogEntry) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.config.Enabled {
		return
	}

	entry.ID = uuid.New().String()
	entry.Timestamp = time.Now()

	am.loginLogs = append(am.loginLogs, &entry)

	// 限制日志数量
	if len(am.loginLogs) > am.config.MaxLogs {
		am.loginLogs = am.loginLogs[len(am.loginLogs)-am.config.MaxLogs:]
	}

	// 记录失败登录到审计日志
	if entry.Status == "failure" {
		am.logs = append(am.logs, &AuditLogEntry{
			ID:        uuid.New().String(),
			Timestamp: entry.Timestamp,
			Level:     "warning",
			Category:  "auth",
			Event:     "login_failure",
			Username:  entry.Username,
			IP:        entry.IP,
			UserAgent: entry.UserAgent,
			Details: map[string]interface{}{
				"reason": entry.Reason,
			},
			Status: "failure",
		})
	} else {
		am.logs = append(am.logs, &AuditLogEntry{
			ID:        uuid.New().String(),
			Timestamp: entry.Timestamp,
			Level:     "info",
			Category:  "auth",
			Event:     "login_success",
			Username:  entry.Username,
			IP:        entry.IP,
			UserAgent: entry.UserAgent,
			Details: map[string]interface{}{
				"mfa_method": entry.MFAMethod,
			},
			Status: "success",
		})
	}
}

// LogAction 记录操作日志
func (am *AuditManager) LogAction(userID, username, ip, resource, action string, details map[string]interface{}, status string) {
	level := "info"
	if status == "failure" {
		level = "warning"
	}

	entry := AuditLogEntry{
		Level:     level,
		Category:  "action",
		Event:     action,
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Resource:  resource,
		Action:    action,
		Details:   details,
		Status:    status,
	}

	am.Log(entry)
}

// GetAuditLogs 获取审计日志
func (am *AuditManager) GetAuditLogs(limit, offset int, filters map[string]string) []*AuditLogEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*AuditLogEntry, 0)

	for _, entry := range am.logs {
		// 应用筛选
		if !am.matchesFilters(entry, filters) {
			continue
		}
		result = append(result, entry)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 应用分页
	start := offset
	if start > len(result) {
		start = len(result)
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// GetLoginLogs 获取登录日志
func (am *AuditManager) GetLoginLogs(limit, offset int, filters map[string]string) []*LoginLogEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*LoginLogEntry, 0)

	for _, entry := range am.loginLogs {
		if !am.matchesLoginFilters(entry, filters) {
			continue
		}
		result = append(result, entry)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 应用分页
	start := offset
	if start > len(result) {
		start = len(result)
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// matchesFilters 检查日志是否匹配筛选条件
func (am *AuditManager) matchesFilters(entry *AuditLogEntry, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "category":
			if entry.Category != value {
				return false
			}
		case "level":
			if entry.Level != value {
				return false
			}
		case "username":
			if entry.Username != value {
				return false
			}
		case "status":
			if entry.Status != value {
				return false
			}
		case "event":
			if entry.Event != value {
				return false
			}
		case "ip":
			if entry.IP != value {
				return false
			}
		}
	}
	return true
}

// matchesLoginFilters 检查登录日志是否匹配筛选条件
func (am *AuditManager) matchesLoginFilters(entry *LoginLogEntry, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "username":
			if entry.Username != value {
				return false
			}
		case "status":
			if entry.Status != value {
				return false
			}
		case "ip":
			if entry.IP != value {
				return false
			}
		}
	}
	return true
}

// GetAlerts 获取安全告警
func (am *AuditManager) GetAlerts(limit, offset int, acknowledged *bool) []*SecurityAlert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*SecurityAlert, 0)

	for _, alert := range am.alerts {
		// 应用筛选
		if acknowledged != nil {
			if alert.Acknowledged != *acknowledged {
				continue
			}
		}
		result = append(result, alert)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 应用分页
	start := offset
	if start > len(result) {
		start = len(result)
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// AcknowledgeAlert 确认告警
func (am *AuditManager) AcknowledgeAlert(alertID, ackedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for _, alert := range am.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			alert.AckedBy = ackedBy
			alert.AckedAt = &now
			return nil
		}
	}

	return fmt.Errorf("告警不存在")
}

// GetAlertStats 获取告警统计
func (am *AuditManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	total := len(am.alerts)
	acknowledged := 0
	unacknowledged := 0
	bySeverity := make(map[string]int)

	for _, alert := range am.alerts {
		if alert.Acknowledged {
			acknowledged++
		} else {
			unacknowledged++
		}
		bySeverity[alert.Severity]++
	}

	return map[string]interface{}{
		"total":         total,
		"acknowledged":  acknowledged,
		"unacknowledged": unacknowledged,
		"by_severity":   bySeverity,
	}
}

// saveLogs 保存日志到文件
func (am *AuditManager) saveLogs() {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if !am.config.AutoSave {
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(am.config.LogPath, 0755); err != nil {
		return
	}

	// 保存审计日志
	today := time.Now().Format("2006-01-02")
	auditLogFile := filepath.Join(am.config.LogPath, fmt.Sprintf("audit-%s.log", today))
	am.writeLogsToFile(auditLogFile, am.logs)

	// 保存登录日志
	loginLogFile := filepath.Join(am.config.LogPath, fmt.Sprintf("login-%s.log", today))
	am.writeLogsToFile(loginLogFile, am.loginLogs)
}

// writeLogsToFile 写入日志到文件
func (am *AuditManager) writeLogsToFile(filename string, logs interface{}) {
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return
	}

	// 追加写入
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.Write([]byte("\n"))
}

// LoadLogs 从文件加载日志
func (am *AuditManager) LoadLogs(date string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	auditLogFile := filepath.Join(am.config.LogPath, fmt.Sprintf("audit-%s.log", date))
	loginLogFile := filepath.Join(am.config.LogPath, fmt.Sprintf("login-%s.log", date))

	// 加载审计日志
	if data, err := os.ReadFile(auditLogFile); err == nil {
		var logs []*AuditLogEntry
		if err := json.Unmarshal(data, &logs); err == nil {
			am.logs = append(am.logs, logs...)
		}
	}

	// 加载登录日志
	if data, err := os.ReadFile(loginLogFile); err == nil {
		var logs []*LoginLogEntry
		if err := json.Unmarshal(data, &logs); err == nil {
			am.loginLogs = append(am.loginLogs, logs...)
		}
	}

	return nil
}

// CleanupOldLogs 清理旧日志
func (am *AuditManager) CleanupOldLogs() {
	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -am.config.MaxAgeDays)

	// 清理审计日志
	cleanedLogs := make([]*AuditLogEntry, 0)
	for _, entry := range am.logs {
		if entry.Timestamp.After(cutoff) {
			cleanedLogs = append(cleanedLogs, entry)
		}
	}
	am.logs = cleanedLogs

	// 清理登录日志
	cleanedLoginLogs := make([]*LoginLogEntry, 0)
	for _, entry := range am.loginLogs {
		if entry.Timestamp.After(cutoff) {
			cleanedLoginLogs = append(cleanedLoginLogs, entry)
		}
	}
	am.loginLogs = cleanedLoginLogs

	// 清理告警
	cleanedAlerts := make([]*SecurityAlert, 0)
	for _, alert := range am.alerts {
		if alert.Timestamp.After(cutoff) {
			cleanedAlerts = append(cleanedAlerts, alert)
		}
	}
	am.alerts = cleanedAlerts
}

// StartCleanupRoutine 启动定期清理例程
func (am *AuditManager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			am.CleanupOldLogs()
		}
	}()
}

// ExportLogs 导出日志
func (am *AuditManager) ExportLogs(startTime, endTime time.Time, format string) ([]byte, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// 筛选时间范围内的日志
	filteredLogs := make([]*AuditLogEntry, 0)
	for _, entry := range am.logs {
		if entry.Timestamp.After(startTime) && entry.Timestamp.Before(endTime) {
			filteredLogs = append(filteredLogs, entry)
		}
	}

	switch format {
	case "json":
		return json.MarshalIndent(filteredLogs, "", "  ")
	case "csv":
		return am.exportToCSV(filteredLogs)
	default:
		return json.MarshalIndent(filteredLogs, "", "  ")
	}
}

// exportToCSV 导出为 CSV 格式
func (am *AuditManager) exportToCSV(logs []*AuditLogEntry) ([]byte, error) {
	var csv string
	csv += "Timestamp,Level,Category,Event,Username,IP,Status\n"

	for _, entry := range logs {
		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			entry.Timestamp.Format(time.RFC3339),
			entry.Level,
			entry.Category,
			entry.Event,
			entry.Username,
			entry.IP,
			entry.Status,
		)
	}

	return []byte(csv), nil
}

// GetLoginStats 获取登录统计
func (am *AuditManager) GetLoginStats(startTime, endTime time.Time) map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	total := 0
	success := 0
	failure := 0
	byUser := make(map[string]int)
	byIP := make(map[string]int)

	for _, entry := range am.loginLogs {
		if entry.Timestamp.Before(startTime) || entry.Timestamp.After(endTime) {
			continue
		}

		total++
		if entry.Status == "success" {
			success++
		} else {
			failure++
		}

		byUser[entry.Username]++
		byIP[entry.IP]++
	}

	return map[string]interface{}{
		"total":   total,
		"success": success,
		"failure": failure,
		"by_user": byUser,
		"by_ip":   byIP,
	}
}
