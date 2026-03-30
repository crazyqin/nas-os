package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditManager 审计日志管理器.
type AuditManager struct {
	config    AuditConfig
	logs      []*AuditLogEntry
	loginLogs []*LoginLogEntry
	alerts    []*Alert
	mu        sync.RWMutex
	// logPath     string - 保留用于未来自定义日志路径
	// maxLogs     int // 最大保留日志数 - 保留用于未来配置化
}

// AuditConfig 审计配置.
type AuditConfig struct {
	Enabled        bool          `json:"enabled"`
	LogPath        string        `json:"log_path"`
	MaxLogs        int           `json:"max_logs"`      // 最大保留日志数
	MaxAgeDays     int           `json:"max_age_days"`  // 最大保留天数
	AutoSave       bool          `json:"auto_save"`     // 自动保存到文件
	SaveInterval   time.Duration `json:"save_interval"` // 保存间隔
	AlertEnabled   bool          `json:"alert_enabled"`
	AlertThreshold int           `json:"alert_threshold"` // 告警阈值
}

// NewAuditManager 创建审计日志管理器.
func NewAuditManager() *AuditManager {
	return &AuditManager{
		config: AuditConfig{
			Enabled:        true,
			LogPath:        "/var/log/nas-os/audit",
			MaxLogs:        10000,
			MaxAgeDays:     90,
			AutoSave:       true,
			SaveInterval:   time.Minute * 5,
			AlertEnabled:   true,
			AlertThreshold: 10,
		},
		logs:      make([]*AuditLogEntry, 0),
		loginLogs: make([]*LoginLogEntry, 0),
		alerts:    make([]*Alert, 0),
	}
}

// SetConfig 设置审计配置.
func (am *AuditManager) SetConfig(config AuditConfig) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.config = config
}

// GetConfig 获取审计配置.
func (am *AuditManager) GetConfig() AuditConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.config
}

// Log 记录审计日志.
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

// checkAlertCondition 检查是否需要生成告警.
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
		alert := Alert{
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

// Message 生成日志消息.
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

// LogLogin 记录登录日志.
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

// LogAction 记录操作日志.
func (am *AuditManager) LogAction(userID, username, ip, resource, action string, details map[string]interface{}, status string) {
	level := "info"
	if status == "failure" {
		level = "warning"
	}

	entry := AuditLogEntry{
		Level:    level,
		Category: "action",
		Event:    action,
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Details:  details,
		Status:   status,
	}

	am.Log(entry)
}

// GetAuditLogs 获取审计日志.
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

// GetLoginLogs 获取登录日志.
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

// matchesFilters 检查日志是否匹配筛选条件.
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

// matchesLoginFilters 检查登录日志是否匹配筛选条件.
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

// GetAlerts 获取安全告警.
func (am *AuditManager) GetAlerts(limit, offset int, acknowledged *bool) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, 0)

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

// AcknowledgeAlert 确认告警.
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

// GetAlertStats 获取告警统计.
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
		"total":          total,
		"acknowledged":   acknowledged,
		"unacknowledged": unacknowledged,
		"by_severity":    bySeverity,
	}
}

// saveLogs 保存日志到文件.
func (am *AuditManager) saveLogs() {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if !am.config.AutoSave {
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(am.config.LogPath, 0750); err != nil {
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

// writeLogsToFile 写入日志到文件.
func (am *AuditManager) writeLogsToFile(filename string, logs interface{}) {
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return
	}

	// 追加写入
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	_, _ = f.Write(data)
	_, _ = f.Write([]byte("\n"))
}

// LoadLogs 从文件加载日志.
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

// CleanupOldLogs 清理旧日志.
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
	cleanedAlerts := make([]*Alert, 0)
	for _, alert := range am.alerts {
		if alert.Timestamp.After(cutoff) {
			cleanedAlerts = append(cleanedAlerts, alert)
		}
	}
	am.alerts = cleanedAlerts
}

// StartCleanupRoutine 启动定期清理例程.
func (am *AuditManager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			am.CleanupOldLogs()
		}
	}()
}

// ExportLogs 导出日志.
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

// exportToCSV 导出为 CSV 格式.
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

// GetLoginStats 获取登录统计.
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

// ========== 文件访问审计追踪 ==========

// FileAccessAudit 文件访问审计记录
type FileAccessAudit struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	FilePath  string                 `json:"file_path"`
	ShareName string                 `json:"share_name"`
	Username  string                 `json:"username"`
	ClientIP  string                 `json:"client_ip"`
	Action    string                 `json:"action"` // read, write, delete, rename, create
	Status    string                 `json:"status"` // success, denied, error
	FileSize  int64                  `json:"file_size,omitempty"`
	FileType  string                 `json:"file_type,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// FileAccessStats 文件访问统计
type FileAccessStats struct {
	TotalAccess     int                `json:"total_access"`
	ByUser          map[string]int     `json:"by_user"`
	ByIP            map[string]int     `json:"by_ip"`
	ByShare         map[string]int     `json:"by_share"`
	ByAction        map[string]int     `json:"by_action"`
	SensitiveFiles  []string           `json:"sensitive_files"`
	AccessFrequency map[string]float64 `json:"access_frequency"` // 用户访问频率
}

// LogFileAccess 记录文件访问审计
func (am *AuditManager) LogFileAccess(access FileAccessAudit) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.config.Enabled {
		return
	}

	access.ID = uuid.New().String()
	access.Timestamp = time.Now()

	// 记录到审计日志
	entry := AuditLogEntry{
		ID:        uuid.New().String(),
		Timestamp: access.Timestamp,
		Level:     "info",
		Category:  "file",
		Event:     "file_access",
		UserID:    "",
		Username:  access.Username,
		IP:        access.ClientIP,
		Resource:  access.FilePath,
		Action:    access.Action,
		Details: map[string]interface{}{
			"share_name": access.ShareName,
			"file_size":  access.FileSize,
			"file_type":  access.FileType,
			"status":     access.Status,
		},
		Status: access.Status,
	}

	am.logs = append(am.logs, &entry)

	// 检查敏感文件访问
	if am.isSensitiveFile(access.FilePath) {
		entry.Level = "warning"
		entry.Event = "sensitive_file_access"
		am.checkAlertCondition(&entry)
	}

	// 自动保存
	if am.config.AutoSave {
		go am.saveLogs()
	}
}

// isSensitiveFile 检查是否为敏感文件
func (am *AuditManager) isSensitiveFile(path string) bool {
	sensitivePatterns := []string{
		"password", "secret", "key", "credential", "config",
		".pem", ".key", ".env", ".ssh", ".git",
	}

	for _, pattern := range sensitivePatterns {
		if containsIgnoreCase(path, pattern) {
			return true
		}
	}
	return false
}

// GetFileAccessAudits 获取文件访问审计记录
func (am *AuditManager) GetFileAccessAudits(startTime, endTime time.Time, filters map[string]string) []*AuditLogEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*AuditLogEntry, 0)

	for _, entry := range am.logs {
		if entry.Category != "file" {
			continue
		}

		if entry.Timestamp.Before(startTime) || entry.Timestamp.After(endTime) {
			continue
		}

		if !am.matchesFilters(entry, filters) {
			continue
		}

		result = append(result, entry)
	}

	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// GetFileAccessStats 获取文件访问统计
func (am *AuditManager) GetFileAccessStats(startTime, endTime time.Time) *FileAccessStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := &FileAccessStats{
		ByUser:          make(map[string]int),
		ByIP:            make(map[string]int),
		ByShare:         make(map[string]int),
		ByAction:        make(map[string]int),
		SensitiveFiles:  make([]string, 0),
		AccessFrequency: make(map[string]float64),
	}

	userAccessTimes := make(map[string][]time.Time)

	for _, entry := range am.logs {
		if entry.Category != "file" {
			continue
		}

		if entry.Timestamp.Before(startTime) || entry.Timestamp.After(endTime) {
			continue
		}

		stats.TotalAccess++

		if entry.Username != "" {
			stats.ByUser[entry.Username]++
			userAccessTimes[entry.Username] = append(userAccessTimes[entry.Username], entry.Timestamp)
		}

		if entry.IP != "" {
			stats.ByIP[entry.IP]++
		}

		if shareName, ok := entry.Details["share_name"].(string); ok && shareName != "" {
			stats.ByShare[shareName]++
		}

		if entry.Action != "" {
			stats.ByAction[entry.Action]++
		}

		// 记录敏感文件
		if entry.Event == "sensitive_file_access" && entry.Resource != "" {
			stats.SensitiveFiles = append(stats.SensitiveFiles, entry.Resource)
		}
	}

	// 计算访问频率（每分钟）
	for user, times := range userAccessTimes {
		if len(times) >= 2 {
			elapsed := times[len(times)-1].Sub(times[0]).Minutes()
			if elapsed > 0 {
				stats.AccessFrequency[user] = float64(len(times)) / elapsed
			}
		}
	}

	return stats
}

// ========== 异常访问检测规则 ==========

// AnomalyDetectionRule 异常检测规则
type AnomalyDetectionRule struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Enabled     bool               `json:"enabled"`
	Severity    string             `json:"severity"` // low, medium, high, critical
	Category    string             `json:"category"` // access_pattern, time_based, location, behavior
	Conditions  []AnomalyCondition `json:"conditions"`
	Threshold   int                `json:"threshold"`
	TimeWindow  time.Duration      `json:"time_window"`
	Actions     []string           `json:"actions"` // alert, block, notify, log
	Whitelist   []string           `json:"whitelist"`
}

// AnomalyCondition 异常检测条件
type AnomalyCondition struct {
	Field    string      `json:"field"`    // access_count, file_count, failed_auth, unusual_hour
	Operator string      `json:"operator"` // gt, lt, eq, ne, in, not_in
	Value    interface{} `json:"value"`
}

// AnomalyResult 异常检测结果
type AnomalyResult struct {
	RuleID       string                 `json:"rule_id"`
	RuleName     string                 `json:"rule_name"`
	Severity     string                 `json:"severity"`
	DetectedAt   time.Time              `json:"detected_at"`
	TriggeredBy  string                 `json:"triggered_by"` // user, ip, share
	TriggerValue interface{}            `json:"trigger_value"`
	Details      map[string]interface{} `json:"details"`
}

// 默认异常检测规则
func getDefaultAnomalyRules() []AnomalyDetectionRule {
	return []AnomalyDetectionRule{
		{
			ID:          "excessive_file_deletion",
			Name:        "大量文件删除",
			Description: "短时间内删除大量文件",
			Enabled:     true,
			Severity:    "high",
			Category:    "access_pattern",
			Conditions: []AnomalyCondition{
				{Field: "delete_count", Operator: "gt", Value: 10},
			},
			Threshold:  10,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "log"},
			Whitelist:  []string{},
		},
		{
			ID:          "unusual_access_time",
			Name:        "异常访问时间",
			Description: "非工作时间的大量访问",
			Enabled:     true,
			Severity:    "medium",
			Category:    "time_based",
			Conditions: []AnomalyCondition{
				{Field: "hour", Operator: "in", Value: []int{0, 1, 2, 3, 4, 5, 23}},
				{Field: "access_count", Operator: "gt", Value: 5},
			},
			Threshold:  5,
			TimeWindow: time.Hour,
			Actions:    []string{"alert", "log"},
			Whitelist:  []string{},
		},
		{
			ID:          "rapid_access_burst",
			Name:        "访问频率爆发",
			Description: "短时间内大量文件访问",
			Enabled:     true,
			Severity:    "medium",
			Category:    "access_pattern",
			Conditions: []AnomalyCondition{
				{Field: "access_count", Operator: "gt", Value: 50},
			},
			Threshold:  50,
			TimeWindow: time.Minute * 5,
			Actions:    []string{"alert", "log"},
			Whitelist:  []string{},
		},
		{
			ID:          "sensitive_file_access_burst",
			Name:        "敏感文件批量访问",
			Description: "短时间内大量敏感文件访问",
			Enabled:     true,
			Severity:    "high",
			Category:    "access_pattern",
			Conditions: []AnomalyCondition{
				{Field: "sensitive_file_count", Operator: "gt", Value: 3},
			},
			Threshold:  3,
			TimeWindow: time.Minute * 10,
			Actions:    []string{"alert", "block", "notify"},
			Whitelist:  []string{},
		},
		{
			ID:          "login_failure_burst",
			Name:        "登录失败爆发",
			Description: "短时间内大量登录失败",
			Enabled:     true,
			Severity:    "high",
			Category:    "behavior",
			Conditions: []AnomalyCondition{
				{Field: "failed_auth_count", Operator: "gt", Value: 5},
			},
			Threshold:  5,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "block"},
			Whitelist:  []string{},
		},
		{
			ID:          "ransomware_pattern",
			Name:        "勒索软件模式",
			Description: "检测勒索软件行为特征",
			Enabled:     true,
			Severity:    "critical",
			Category:    "behavior",
			Conditions: []AnomalyCondition{
				{Field: "extension_change", Operator: "in", Value: []string{".encrypted", ".locked", ".crypto"}},
			},
			Threshold:  1,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "block", "notify"},
			Whitelist:  []string{},
		},
		{
			ID:          "cross_share_access",
			Name:        "跨共享访问",
			Description: "短时间内跨多个共享访问",
			Enabled:     true,
			Severity:    "medium",
			Category:    "access_pattern",
			Conditions: []AnomalyCondition{
				{Field: "share_count", Operator: "gt", Value: 3},
			},
			Threshold:  3,
			TimeWindow: time.Minute * 10,
			Actions:    []string{"alert", "log"},
			Whitelist:  []string{},
		},
	}
}

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	rules         []AnomalyDetectionRule
	results       []AnomalyResult
	userStats     map[string]*UserAnomalyStats
	ipStats       map[string]*IPAnomalyStats
	mu            sync.RWMutex
	alertCallback func(result AnomalyResult)
}

// UserAnomalyStats 用户异常统计
type UserAnomalyStats struct {
	Username       string    `json:"username"`
	AccessCount    int       `json:"access_count"`
	DeleteCount    int       `json:"delete_count"`
	SensitiveCount int       `json:"sensitive_count"`
	ShareCount     int       `json:"share_count"`
	LastAccess     time.Time `json:"last_access"`
	RecentShares   []string  `json:"recent_shares"`
	RecentFiles    []string  `json:"recent_files"`
}

// IPAnomalyStats IP 异常统计
type IPAnomalyStats struct {
	ClientIP        string    `json:"client_ip"`
	AccessCount     int       `json:"access_count"`
	FailedAuthCount int       `json:"failed_auth_count"`
	LastAccess      time.Time `json:"last_access"`
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		rules:     getDefaultAnomalyRules(),
		results:   make([]AnomalyResult, 0),
		userStats: make(map[string]*UserAnomalyStats),
		ipStats:   make(map[string]*IPAnomalyStats),
	}
}

// SetAlertCallback 设置告警回调
func (ad *AnomalyDetector) SetAlertCallback(callback func(result AnomalyResult)) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.alertCallback = callback
}

// GetRules 获取所有规则
func (ad *AnomalyDetector) GetRules() []AnomalyDetectionRule {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	return ad.rules
}

// AddRule 添加规则
func (ad *AnomalyDetector) AddRule(rule AnomalyDetectionRule) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.rules = append(ad.rules, rule)
}

// UpdateRule 更新规则
func (ad *AnomalyDetector) UpdateRule(ruleID string, rule AnomalyDetectionRule) error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	for i, existing := range ad.rules {
		if existing.ID == ruleID {
			ad.rules[i] = rule
			return nil
		}
	}
	return fmt.Errorf("规则不存在: %s", ruleID)
}

// DeleteRule 删除规则
func (ad *AnomalyDetector) DeleteRule(ruleID string) error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	for i, existing := range ad.rules {
		if existing.ID == ruleID {
			ad.rules = append(ad.rules[:i], ad.rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("规则不存在: %s", ruleID)
}

// DetectFromLogEntry 从审计日志条目检测异常
func (ad *AnomalyDetector) DetectFromLogEntry(entry *AuditLogEntry) []AnomalyResult {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	results := make([]AnomalyResult, 0)

	// 更新统计
	ad.updateStats(entry)

	// 检查每个规则
	for _, rule := range ad.rules {
		if !rule.Enabled {
			continue
		}

		// 检查白名单
		if isWhitelisted(entry.Username, rule.Whitelist) || isWhitelisted(entry.IP, rule.Whitelist) {
			continue
		}

		// 检查规则条件
		if ad.checkRule(rule, entry) {
			result := AnomalyResult{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				DetectedAt:  time.Now(),
				TriggeredBy: entry.Username,
				Details: map[string]interface{}{
					"entry_id":  entry.ID,
					"event":     entry.Event,
					"resource":  entry.Resource,
					"ip":        entry.IP,
					"threshold": rule.Threshold,
				},
			}

			ad.results = append(ad.results, result)
			results = append(results, result)

			// 触发告警回调
			if ad.alertCallback != nil && containsString(rule.Actions, "alert") {
				go ad.alertCallback(result)
			}
		}
	}

	return results
}

// updateStats 更新统计
func (ad *AnomalyDetector) updateStats(entry *AuditLogEntry) {
	// 更新用户统计
	if entry.Username != "" {
		stats, exists := ad.userStats[entry.Username]
		if !exists {
			stats = &UserAnomalyStats{
				Username:     entry.Username,
				RecentShares: make([]string, 0),
				RecentFiles:  make([]string, 0),
			}
			ad.userStats[entry.Username] = stats
		}

		stats.AccessCount++
		stats.LastAccess = entry.Timestamp

		if entry.Action == "delete" {
			stats.DeleteCount++
		}

		if entry.Event == "sensitive_file_access" {
			stats.SensitiveCount++
		}

		if entry.Resource != "" {
			stats.RecentFiles = append(stats.RecentFiles, entry.Resource)
			if len(stats.RecentFiles) > 100 {
				stats.RecentFiles = stats.RecentFiles[len(stats.RecentFiles)-100:]
			}
		}

		if shareName, ok := entry.Details["share_name"].(string); ok && shareName != "" {
			if !containsString(stats.RecentShares, shareName) {
				stats.RecentShares = append(stats.RecentShares, shareName)
				stats.ShareCount++
			}
		}
	}

	// 更新 IP 统计
	if entry.IP != "" {
		stats, exists := ad.ipStats[entry.IP]
		if !exists {
			stats = &IPAnomalyStats{ClientIP: entry.IP}
			ad.ipStats[entry.IP] = stats
		}

		stats.AccessCount++
		stats.LastAccess = entry.Timestamp

		if entry.Event == "login_failure" {
			stats.FailedAuthCount++
		}
	}
}

// checkRule 检查规则条件
func (ad *AnomalyDetector) checkRule(rule AnomalyDetectionRule, entry *AuditLogEntry) bool {
	windowStart := entry.Timestamp.Add(-rule.TimeWindow)

	for _, condition := range rule.Conditions {
		value := ad.getFieldValue(condition.Field, entry.Username, entry.IP, windowStart)

		if !checkCondition(value, condition.Operator, condition.Value) {
			return false
		}
	}

	return true
}

// getFieldValue 获取字段值
func (ad *AnomalyDetector) getFieldValue(field, username, clientIP string, windowStart time.Time) interface{} {
	switch field {
	case "access_count":
		if username != "" && ad.userStats[username] != nil {
			return ad.userStats[username].AccessCount
		}
		return 0
	case "delete_count":
		if username != "" && ad.userStats[username] != nil {
			return ad.userStats[username].DeleteCount
		}
		return 0
	case "sensitive_file_count":
		if username != "" && ad.userStats[username] != nil {
			return ad.userStats[username].SensitiveCount
		}
		return 0
	case "share_count":
		if username != "" && ad.userStats[username] != nil {
			return ad.userStats[username].ShareCount
		}
		return 0
	case "failed_auth_count":
		if clientIP != "" && ad.ipStats[clientIP] != nil {
			return ad.ipStats[clientIP].FailedAuthCount
		}
		return 0
	case "hour":
		return time.Now().Hour()
	default:
		return 0
	}
}

// checkCondition 检查条件
func checkCondition(value interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "gt":
		return compareNumbers(value, expected) > 0
	case "lt":
		return compareNumbers(value, expected) < 0
	case "eq":
		return compareNumbers(value, expected) == 0
	case "ne":
		return compareNumbers(value, expected) != 0
	case "in":
		return isInList(value, expected)
	case "not_in":
		return !isInList(value, expected)
	default:
		return false
	}
}

// compareNumbers 比较数值
func compareNumbers(a, b interface{}) int {
	aNum := toFloat64(a)
	bNum := toFloat64(b)
	if aNum > bNum {
		return 1
	} else if aNum < bNum {
		return -1
	}
	return 0
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

// isInList 检查是否在列表中
func isInList(value, list interface{}) bool {
	switch l := list.(type) {
	case []string:
		strVal, ok := value.(string)
		if !ok {
			return false
		}
		for _, s := range l {
			if s == strVal {
				return true
			}
		}
	case []int:
		intVal, ok := value.(int)
		if !ok {
			return false
		}
		for _, i := range l {
			if i == intVal {
				return true
			}
		}
	}
	return false
}

// GetResults 获取检测结果
func (ad *AnomalyDetector) GetResults(limit, offset int, filters map[string]string) []AnomalyResult {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	result := make([]AnomalyResult, 0)

	for _, r := range ad.results {
		if !ad.matchesResultFilters(r, filters) {
			continue
		}
		result = append(result, r)
	}

	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].DetectedAt.After(result[j].DetectedAt)
	})

	// 分页
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

// matchesResultFilters 检查结果是否匹配筛选条件
func (ad *AnomalyDetector) matchesResultFilters(result AnomalyResult, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "rule_id":
			if result.RuleID != value {
				return false
			}
		case "severity":
			if result.Severity != value {
				return false
			}
		case "triggered_by":
			if result.TriggeredBy != value {
				return false
			}
		}
	}
	return true
}

// GetUserAnomalyStats 获取用户异常统计
func (ad *AnomalyDetector) GetUserAnomalyStats(username string) (*UserAnomalyStats, bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	stats, exists := ad.userStats[username]
	if !exists {
		return nil, false
	}

	// 返回副本
	copy := *stats
	copy.RecentShares = make([]string, len(stats.RecentShares))
	copySlice(stats.RecentShares, copy.RecentShares)
	copy.RecentFiles = make([]string, len(stats.RecentFiles))
	copySlice(stats.RecentFiles, copy.RecentFiles)

	return &copy, true
}

// GetIPAnomalyStats 获取 IP 异常统计
func (ad *AnomalyDetector) GetIPAnomalyStats(clientIP string) (*IPAnomalyStats, bool) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	stats, exists := ad.ipStats[clientIP]
	if !exists {
		return nil, false
	}

	copy := *stats
	return &copy, true
}

// ClearStats 清除统计（用于测试和调试）
func (ad *AnomalyDetector) ClearStats() {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	ad.userStats = make(map[string]*UserAnomalyStats)
	ad.ipStats = make(map[string]*IPAnomalyStats)
}

// ========== 安全事件告警推送 ==========

// AlertNotifier 告警通知器
type AlertNotifier struct {
	config     AlertNotifierConfig
	notified   []NotificationRecord
	alertQueue chan Alert
	mu         sync.RWMutex
}

// AlertNotifierConfig 告警通知配置
type AlertNotifierConfig struct {
	Enabled        bool             `json:"enabled"`
	Channels       []string         `json:"channels"`     // email, webhook, wecom, telegram
	MinSeverity    string           `json:"min_severity"` // 最低告警级别
	RateLimit      int              `json:"rate_limit"`   // 每分钟最大通知数
	QuietHours     QuietHoursConfig `json:"quiet_hours"`  // 免打扰时段
	EmailConfig    EmailAlertConfig `json:"email_config"`
	WebhookConfig  WebhookConfig    `json:"webhook_config"`
	WeComConfig    WeComConfig      `json:"wecom_config"`
	TelegramConfig TelegramConfig   `json:"telegram_config"`
}

// QuietHoursConfig 免打扰时段配置
type QuietHoursConfig struct {
	Enabled   bool   `json:"enabled"`
	StartTime string `json:"start_time"` // HH:MM
	EndTime   string `json:"end_time"`   // HH:MM
}

// EmailAlertConfig 邮件告警配置
type EmailAlertConfig struct {
	Enabled    bool     `json:"enabled"`
	Recipients []string `json:"recipients"`
	SMTPServer string   `json:"smtp_server"`
	SMTPPort   int      `json:"smtp_port"`
	From       string   `json:"from"`
	Subject    string   `json:"subject_prefix"`
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	Enabled bool              `json:"enabled"`
	URLs    []string          `json:"urls"`
	Headers map[string]string `json:"headers"`
}

// WeComConfig 企业微信配置
type WeComConfig struct {
	Enabled bool   `json:"enabled"`
	Webhook string `json:"webhook"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// NotificationRecord 通知记录
type NotificationRecord struct {
	ID        string    `json:"id"`
	AlertID   string    `json:"alert_id"`
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // success, failed
	Error     string    `json:"error,omitempty"`
}

// NewAlertNotifier 创建告警通知器
func NewAlertNotifier() *AlertNotifier {
	return &AlertNotifier{
		config: AlertNotifierConfig{
			Enabled:        true,
			Channels:       []string{"webhook"},
			MinSeverity:    "medium",
			RateLimit:      10,
			QuietHours:     QuietHoursConfig{Enabled: false, StartTime: "23:00", EndTime: "07:00"},
			EmailConfig:    EmailAlertConfig{Enabled: false},
			WebhookConfig:  WebhookConfig{Enabled: false},
			WeComConfig:    WeComConfig{Enabled: false},
			TelegramConfig: TelegramConfig{Enabled: false},
		},
		notified:   make([]NotificationRecord, 0),
		alertQueue: make(chan Alert, 100),
	}
}

// SetConfig 设置配置
func (an *AlertNotifier) SetConfig(config AlertNotifierConfig) {
	an.mu.Lock()
	defer an.mu.Unlock()
	an.config = config
}

// GetConfig 获取配置
func (an *AlertNotifier) GetConfig() AlertNotifierConfig {
	an.mu.RLock()
	defer an.mu.RUnlock()
	return an.config
}

// Notify 发送告警通知
func (an *AlertNotifier) Notify(alert Alert) error {
	an.mu.Lock()
	defer an.mu.Unlock()

	if !an.config.Enabled {
		return nil
	}

	// 检查严重级别
	if !an.shouldNotify(alert.Severity) {
		return nil
	}

	// 检查免打扰时段
	if an.config.QuietHours.Enabled && an.isQuietHours() {
		return nil
	}

	// 检查速率限制
	if !an.checkRateLimit() {
		return fmt.Errorf("达到告警速率限制")
	}

	// 发送到各渠道
	for _, channel := range an.config.Channels {
		record := NotificationRecord{
			ID:        uuid.New().String(),
			AlertID:   alert.ID,
			Channel:   channel,
			Timestamp: time.Now(),
		}

		err := an.sendToChannel(channel, alert)
		if err != nil {
			record.Status = "failed"
			record.Error = err.Error()
		} else {
			record.Status = "success"
		}

		an.notified = append(an.notified, record)
	}

	return nil
}

// shouldNotify 检查是否应该通知
func (an *AlertNotifier) shouldNotify(severity string) bool {
	severityLevels := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	alertLevel, exists := severityLevels[severity]
	if !exists {
		return false
	}

	minLevel, exists := severityLevels[an.config.MinSeverity]
	if !exists {
		return true
	}

	return alertLevel >= minLevel
}

// isQuietHours 检查是否在免打扰时段
func (an *AlertNotifier) isQuietHours() bool {
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	startParts := parseTimeString(an.config.QuietHours.StartTime)
	endParts := parseTimeString(an.config.QuietHours.EndTime)

	startMinutes := startParts[0]*60 + startParts[1]
	endMinutes := endParts[0]*60 + endParts[1]

	if startMinutes <= endMinutes {
		return currentMinutes >= startMinutes && currentMinutes <= endMinutes
	} else {
		return currentMinutes >= startMinutes || currentMinutes <= endMinutes
	}
}

// parseTimeString 解析时间字符串
func parseTimeString(timeStr string) [2]int {
	var h, m int
	_, _ = fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	return [2]int{h, m}
}

// checkRateLimit 检查速率限制
func (an *AlertNotifier) checkRateLimit() bool {
	now := time.Now()
	windowStart := now.Add(-time.Minute)

	count := 0
	for _, record := range an.notified {
		if record.Timestamp.After(windowStart) {
			count++
		}
	}

	return count < an.config.RateLimit
}

// sendToChannel 发送到指定渠道
func (an *AlertNotifier) sendToChannel(channel string, alert Alert) error {
	switch channel {
	case "email":
		return an.sendEmail(alert)
	case "webhook":
		return an.sendWebhook(alert)
	case "wecom":
		return an.sendWeCom(alert)
	case "telegram":
		return an.sendTelegram(alert)
	default:
		return fmt.Errorf("未知渠道: %s", channel)
	}
}

// sendEmail 发送邮件
func (an *AlertNotifier) sendEmail(alert Alert) error {
	if !an.config.EmailConfig.Enabled {
		return nil
	}

	// 需要外部邮件发送实现
	// 这里只是记录，实际发送由外部模块完成
	return nil
}

// sendWebhook 发送 Webhook
func (an *AlertNotifier) sendWebhook(alert Alert) error {
	if !an.config.WebhookConfig.Enabled || len(an.config.WebhookConfig.URLs) == 0 {
		return nil
	}

	// 需要外部 HTTP 客户端实现
	// 这里只是记录，实际发送由外部模块完成
	return nil
}

// sendWeCom 发送企业微信
func (an *AlertNotifier) sendWeCom(alert Alert) error {
	if !an.config.WeComConfig.Enabled || an.config.WeComConfig.Webhook == "" {
		return nil
	}

	// 需要外部 HTTP 客户端实现
	return nil
}

// sendTelegram 发送 Telegram
func (an *AlertNotifier) sendTelegram(alert Alert) error {
	if !an.config.TelegramConfig.Enabled || an.config.TelegramConfig.BotToken == "" {
		return nil
	}

	// 需要外部 HTTP 客户端实现
	return nil
}

// GetNotificationRecords 获取通知记录
func (an *AlertNotifier) GetNotificationRecords(limit, offset int) []NotificationRecord {
	an.mu.RLock()
	defer an.mu.RUnlock()

	start := offset
	if start > len(an.notified) {
		start = len(an.notified)
	}
	end := start + limit
	if end > len(an.notified) {
		end = len(an.notified)
	}

	return an.notified[start:end]
}

// 辅助函数

// containsIgnoreCase 检查字符串是否包含（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// isWhitelisted 检查是否在白名单中
func isWhitelisted(value string, whitelist []string) bool {
	for _, w := range whitelist {
		if w == value {
			return true
		}
	}
	return false
}

// containsString 检查字符串切片是否包含指定值
func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// copySlice 复制字符串切片
func copySlice(src, dst []string) {
	for i, v := range src {
		if i < len(dst) {
			dst[i] = v
		}
	}
}

// generateAlertID 生成告警 ID
func generateAlertID() string {
	return uuid.New().String()
}
