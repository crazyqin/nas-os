package smb

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SecurityEvent SMB 安全事件类型
type SecurityEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`    // access, auth, anomaly, ransomware
	Severity     string                 `json:"severity"`      // low, medium, high, critical
	ShareName    string                 `json:"share_name"`
	Username     string                 `json:"username"`
	ClientIP     string                 `json:"client_ip"`
 FilePath     string                 `json:"file_path"`
	Action       string                 `json:"action"`        // read, write, delete, rename
	Status       string                 `json:"status"`        // success, denied, error
	Details      map[string]interface{} `json:"details"`
	DetectedBy   string                 `json:"detected_by"`   // audit, anomaly_detector, ransomware_detector
}

// FileAccessRecord 文件访问记录
type FileAccessRecord struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	ShareName   string    `json:"share_name"`
	FilePath    string    `json:"file_path"`
	Username    string    `json:"username"`
	ClientIP    string    `json:"client_ip"`
	Action      string    `json:"action"` // read, write, delete, rename, create
	Status      string    `json:"status"`
	FileSize    int64     `json:"file_size,omitempty"`
	FileType    string    `json:"file_type,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
}

// AnomalyRule 异常检测规则
type AnomalyRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Severity    string                 `json:"severity"`
	Category    string                 `json:"category"` // access_pattern, file_operation, time_based
	Conditions  []AnomalyCondition     `json:"conditions"`
	Threshold   int                    `json:"threshold"`
	TimeWindow  time.Duration          `json:"time_window"`
	Actions     []string               `json:"actions"` // alert, block, notify
}

// AnomalyCondition 异常检测条件
type AnomalyCondition struct {
	Field    string      `json:"field"`    // file_count, delete_count, access_frequency, unusual_time
	Operator string      `json:"operator"` // gt, lt, eq, ne, contains
	Value    interface{} `json:"value"`
}

// RansomwareIndicator 勒索软件检测指标
type RansomwareIndicator struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Enabled                bool     `json:"enabled"`
	FileExtensions         []string `json:"file_extensions"`         // 可疑扩展名
	SuspiciousPatterns     []string `json:"suspicious_patterns"`     // 可疑文件名模式
	RapidDeleteThreshold   int      `json:"rapid_delete_threshold"`  // 快速删除阈值
	RapidModifyThreshold   int      `json:"rapid_modify_threshold"`  // 快速修改阈值
	TimeWindowSeconds      int      `json:"time_window_seconds"`     // 时间窗口（秒）
}

// SecurityAuditConfig 安全审计配置
type SecurityAuditConfig struct {
	Enabled                bool                   `json:"enabled"`
	FileAccessAudit        bool                   `json:"file_access_audit"`
	AuthAudit              bool                   `json:"auth_audit"`
	AnomalyDetection       bool                   `json:"anomaly_detection"`
	RansomwareDetection    bool                   `json:"ransomware_detection"`
	MaxRecords             int                    `json:"max_records"`
	RetentionDays          int                    `json:"retention_days"`
	AlertThreshold         int                    `json:"alert_threshold"`
	AlertChannels          []string               `json:"alert_channels"`
	CustomRules            []AnomalyRule          `json:"custom_rules"`
	WhitelistedIPs         []string               `json:"whitelisted_ips"`
	WhitelistedUsers       []string               `json:"whitelisted_users"`
}

// SecurityAuditManager SMB 安全审计管理器
type SecurityAuditManager struct {
	config            SecurityAuditConfig
	events            []*SecurityEvent
	fileAccessRecords []*FileAccessRecord
	anomalyRules      []AnomalyRule
	ransomwareIndicators []RansomwareIndicator
	userAccessStats   map[string]*UserAccessStats // 用户访问统计
	ipAccessStats     map[string]*IPAccessStats   // IP 访问统计
	alertCallback     func(event SecurityEvent)   // 告警回调
	mu                sync.RWMutex
}

// UserAccessStats 用户访问统计
type UserAccessStats struct {
	Username          string    `json:"username"`
	TotalAccess       int       `json:"total_access"`
	ReadCount         int       `json:"read_count"`
	WriteCount        int       `json:"write_count"`
	DeleteCount       int       `json:"delete_count"`
	LastAccess        time.Time `json:"last_access"`
	AccessFrequency   float64   `json:"access_frequency"` // 每分钟访问次数
	RecentFiles       []string  `json:"recent_files"`
	RecentDeleteCount int       `json:"recent_delete_count"`
}

// IPAccessStats IP 访问统计
type IPAccessStats struct {
	ClientIP          string    `json:"client_ip"`
	TotalAccess       int       `json:"total_access"`
	FailedAuthCount   int       `json:"failed_auth_count"`
	LastAccess        time.Time `json:"last_access"`
	ConnectedUsers    []string  `json:"connected_users"`
	AccessFrequency   float64   `json:"access_frequency"`
	RecentDeleteCount int       `json:"recent_delete_count"`
}

// NewSecurityAuditManager 创建 SMB 安全审计管理器
func NewSecurityAuditManager() *SecurityAuditManager {
	return &SecurityAuditManager{
		config: SecurityAuditConfig{
			Enabled:                true,
			FileAccessAudit:        true,
			AuthAudit:              true,
			AnomalyDetection:       true,
			RansomwareDetection:    true,
			MaxRecords:             50000,
			RetentionDays:          90,
			AlertThreshold:         10,
			AlertChannels:          []string{"webhook", "email"},
			CustomRules:            []AnomalyRule{},
			WhitelistedIPs:         []string{},
			WhitelistedUsers:       []string{},
		},
		events:            make([]*SecurityEvent, 0),
		fileAccessRecords: make([]*FileAccessRecord, 0),
		anomalyRules:      getDefaultAnomalyRules(),
		ransomwareIndicators: getDefaultRansomwareIndicators(),
		userAccessStats:   make(map[string]*UserAccessStats),
		ipAccessStats:     make(map[string]*IPAccessStats),
	}
}

// getDefaultAnomalyRules 获取默认异常检测规则
func getDefaultAnomalyRules() []AnomalyRule {
	return []AnomalyRule{
		{
			ID:          "rapid_deletion",
			Name:        "快速删除检测",
			Description: "短时间内大量文件删除行为",
			Enabled:     true,
			Severity:    "high",
			Category:    "file_operation",
			Conditions: []AnomalyCondition{
				{Field: "delete_count", Operator: "gt", Value: 10},
			},
			Threshold:  10,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "notify"},
		},
		{
			ID:          "unusual_access_time",
			Name:        "异常访问时间",
			Description: "非工作时间的大量访问",
			Enabled:     true,
			Severity:    "medium",
			Category:    "time_based",
			Conditions: []AnomalyCondition{
				{Field: "hour", Operator: "lt", Value: 6},
				{Field: "hour", Operator: "gt", Value: 23},
				{Field: "access_count", Operator: "gt", Value: 5},
			},
			Threshold:  5,
			TimeWindow: time.Hour,
			Actions:    []string{"alert"},
		},
		{
			ID:          "mass_file_operation",
			Name:        "批量文件操作",
			Description: "短时间内大量文件操作",
			Enabled:     true,
			Severity:    "medium",
			Category:    "access_pattern",
			Conditions: []AnomalyCondition{
				{Field: "file_count", Operator: "gt", Value: 50},
			},
			Threshold:  50,
			TimeWindow: time.Minute * 5,
			Actions:    []string{"alert", "notify"},
		},
		{
			ID:          "suspicious_extension",
			Name:        "可疑扩展名检测",
			Description: "勒索软件相关扩展名修改",
			Enabled:     true,
			Severity:    "critical",
			Category:    "ransomware",
			Conditions: []AnomalyCondition{
				{Field: "extension", Operator: "contains", Value: []string{".encrypted", ".locked", ".crypto"}},
			},
			Threshold:  1,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "block", "notify"},
		},
		{
			ID:          "auth_failure_burst",
			Name:        "认证失败爆发",
			Description: "短时间内大量认证失败",
			Enabled:     true,
			Severity:    "high",
			Category:    "auth",
			Conditions: []AnomalyCondition{
				{Field: "failed_auth_count", Operator: "gt", Value: 5},
			},
			Threshold:  5,
			TimeWindow: time.Minute,
			Actions:    []string{"alert", "block"},
		},
	}
}

// getDefaultRansomwareIndicators 获取默认勒索软件检测指标
func getDefaultRansomwareIndicators() []RansomwareIndicator {
	return []RansomwareIndicator{
		{
			ID:                   "ransomware_extensions",
			Name:                 "勒索软件扩展名检测",
			Enabled:              true,
			FileExtensions:       []string{".encrypted", ".locked", ".crypto", ".enc", ".zzz", ".ransom", ".pay", ".decrypt"},
			SuspiciousPatterns:   []string{"README_FOR_DECRYPT", "DECRYPT_INSTRUCTIONS", "_DECRYPT_", "_READ_ME_", "RESTORE_FILES"},
			RapidDeleteThreshold: 20,
			RapidModifyThreshold: 30,
			TimeWindowSeconds:    60,
		},
		{
			ID:                   "mass_rename",
			Name:                 "批量重命名检测",
			Enabled:              true,
			FileExtensions:       []string{},
			SuspiciousPatterns:   []string{},
			RapidDeleteThreshold: 50,
			RapidModifyThreshold: 100,
			TimeWindowSeconds:    30,
		},
	}
}

// SetConfig 设置安全审计配置
func (sam *SecurityAuditManager) SetConfig(config SecurityAuditConfig) {
	sam.mu.Lock()
	defer sam.mu.Unlock()
	sam.config = config
}

// GetConfig 获取安全审计配置
func (sam *SecurityAuditManager) GetConfig() SecurityAuditConfig {
	sam.mu.RLock()
	defer sam.mu.RUnlock()
	return sam.config
}

// SetAlertCallback 设置告警回调函数
func (sam *SecurityAuditManager) SetAlertCallback(callback func(event SecurityEvent)) {
	sam.mu.Lock()
	defer sam.mu.Unlock()
	sam.alertCallback = callback
}

// LogFileAccess 记录文件访问
func (sam *SecurityAuditManager) LogFileAccess(record FileAccessRecord) {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	if !sam.config.Enabled || !sam.config.FileAccessAudit {
		return
	}

	// 设置 ID 和时间戳
	if record.ID == "" {
		record.ID = generateEventID()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// 添加记录
	sam.fileAccessRecords = append(sam.fileAccessRecords, &record)

	// 限制记录数量
	if len(sam.fileAccessRecords) > sam.config.MaxRecords {
		sam.fileAccessRecords = sam.fileAccessRecords[len(sam.fileAccessRecords)-sam.config.MaxRecords:]
	}

	// 更新统计
	sam.updateUserStats(&record)
	sam.updateIPStats(&record)

	// 检查异常行为
	if sam.config.AnomalyDetection {
		sam.checkAnomalies(&record)
	}

	// 检查勒索软件指标
	if sam.config.RansomwareDetection {
		sam.checkRansomwareIndicators(&record)
	}
}

// LogSecurityEvent 记录安全事件
func (sam *SecurityAuditManager) LogSecurityEvent(event SecurityEvent) {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	if !sam.config.Enabled {
		return
	}

	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	sam.events = append(sam.events, &event)

	// 触发告警回调
	if sam.alertCallback != nil && event.Severity != "low" {
		go sam.alertCallback(event)
	}
}

// updateUserStats 更新用户访问统计
func (sam *SecurityAuditManager) updateUserStats(record *FileAccessRecord) {
	stats, exists := sam.userAccessStats[record.Username]
	if !exists {
		stats = &UserAccessStats{
			Username:    record.Username,
			RecentFiles: make([]string, 0),
		}
		sam.userAccessStats[record.Username] = stats
	}

	stats.TotalAccess++
	stats.LastAccess = record.Timestamp

	switch record.Action {
	case "read":
		stats.ReadCount++
	case "write":
		stats.WriteCount++
	case "delete":
		stats.DeleteCount++
		stats.RecentDeleteCount++
	}

	// 计算访问频率（每分钟）
	if stats.TotalAccess > 0 {
		elapsed := time.Since(stats.LastAccess).Minutes()
		if elapsed > 0 {
			stats.AccessFrequency = float64(stats.TotalAccess) / elapsed
		}
	}

	// 记录最近访问的文件
	stats.RecentFiles = append(stats.RecentFiles, record.FilePath)
	if len(stats.RecentFiles) > 100 {
		stats.RecentFiles = stats.RecentFiles[len(stats.RecentFiles)-100:]
	}
}

// updateIPStats 更新 IP 访问统计
func (sam *SecurityAuditManager) updateIPStats(record *FileAccessRecord) {
	stats, exists := sam.ipAccessStats[record.ClientIP]
	if !exists {
		stats = &IPAccessStats{
			ClientIP:       record.ClientIP,
			ConnectedUsers: make([]string, 0),
		}
		sam.ipAccessStats[record.ClientIP] = stats
	}

	stats.TotalAccess++
	stats.LastAccess = record.Timestamp

	if record.Action == "delete" {
		stats.RecentDeleteCount++
	}

	// 记录连接的用户
	found := false
	for _, u := range stats.ConnectedUsers {
		if u == record.Username {
			found = true
			break
		}
	}
	if !found {
		stats.ConnectedUsers = append(stats.ConnectedUsers, record.Username)
	}
}

// checkAnomalies 检查异常行为
func (sam *SecurityAuditManager) checkAnomalies(record *FileAccessRecord) {
	now := time.Now()

	for _, rule := range sam.anomalyRules {
		if !rule.Enabled {
			continue
		}

		// 检查白名单
		if sam.isWhitelisted(record) {
			continue
		}

		// 检查时间窗口内的统计
		windowStart := now.Add(-rule.TimeWindow)

		// 根据规则类型检查
		switch rule.Category {
		case "file_operation":
			if rule.ID == "rapid_deletion" {
				deleteCount := sam.countRecentDeletes(record.Username, windowStart)
				if deleteCount > rule.Threshold {
					sam.triggerAnomalyAlert(record, rule, deleteCount)
				}
			}
		case "time_based":
			if rule.ID == "unusual_access_time" {
				hour := now.Hour()
				accessCount := sam.countRecentAccess(record.Username, windowStart)
				if (hour < 6 || hour > 23) && accessCount > rule.Threshold {
					sam.triggerAnomalyAlert(record, rule, accessCount)
				}
			}
		case "access_pattern":
			if rule.ID == "mass_file_operation" {
				fileCount := sam.countRecentAccess(record.Username, windowStart)
				if fileCount > rule.Threshold {
					sam.triggerAnomalyAlert(record, rule, fileCount)
				}
			}
		case "ransomware":
			if rule.ID == "suspicious_extension" {
				ext := getFileExtension(record.FilePath)
				for _, suspiciousExt := range rule.Conditions[0].Value.([]string) {
					if strings.HasSuffix(ext, suspiciousExt) {
						sam.triggerAnomalyAlert(record, rule, 1)
						break
					}
				}
			}
		}
	}
}

// checkRansomwareIndicators 检查勒索软件指标
func (sam *SecurityAuditManager) checkRansomwareIndicators(record *FileAccessRecord) {
	for _, indicator := range sam.ransomwareIndicators {
		if !indicator.Enabled {
			continue
		}

		// 检查可疑扩展名
		ext := getFileExtension(record.FilePath)
		for _, suspiciousExt := range indicator.FileExtensions {
			if strings.HasSuffix(ext, suspiciousExt) {
				sam.triggerRansomwareAlert(record, indicator, "可疑扩展名")
				return
			}
		}

		// 检查可疑文件名模式
		for _, pattern := range indicator.SuspiciousPatterns {
			if strings.Contains(record.FilePath, pattern) {
				sam.triggerRansomwareAlert(record, indicator, "可疑文件名模式")
				return
			}
		}

		// 检查快速删除/修改
		if record.Action == "delete" || record.Action == "rename" {
			windowStart := time.Now().Add(-time.Duration(indicator.TimeWindowSeconds) * time.Second)
			deleteCount := sam.countRecentDeletes(record.Username, windowStart)
			if deleteCount > indicator.RapidDeleteThreshold {
				sam.triggerRansomwareAlert(record, indicator, "快速删除行为")
				return
			}
		}
	}
}

// triggerAnomalyAlert 触发异常告警
func (sam *SecurityAuditManager) triggerAnomalyAlert(record *FileAccessRecord, rule AnomalyRule, count int) {
	event := SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: "anomaly",
		Severity:  rule.Severity,
		ShareName: record.ShareName,
		Username:  record.Username,
		ClientIP:  record.ClientIP,
		FilePath:  record.FilePath,
		Action:    record.Action,
		Status:    "detected",
		Details: map[string]interface{}{
			"rule_id":    rule.ID,
			"rule_name":  rule.Name,
			"count":      count,
			"threshold":  rule.Threshold,
			"time_window": rule.TimeWindow.String(),
		},
		DetectedBy: "anomaly_detector",
	}

	sam.events = append(sam.events, &event)

	// 触发告警回调
	if sam.alertCallback != nil {
		go sam.alertCallback(event)
	}
}

// triggerRansomwareAlert 触发勒索软件告警
func (sam *SecurityAuditManager) triggerRansomwareAlert(record *FileAccessRecord, indicator RansomwareIndicator, reason string) {
	event := SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: "ransomware",
		Severity:  "critical",
		ShareName: record.ShareName,
		Username:  record.Username,
		ClientIP:  record.ClientIP,
		FilePath:  record.FilePath,
		Action:    record.Action,
		Status:    "detected",
		Details: map[string]interface{}{
			"indicator_id":   indicator.ID,
			"indicator_name": indicator.Name,
			"reason":         reason,
		},
		DetectedBy: "ransomware_detector",
	}

	sam.events = append(sam.events, &event)

	// 触发告警回调
	if sam.alertCallback != nil {
		go sam.alertCallback(event)
	}
}

// isWhitelisted 检查是否在白名单中
func (sam *SecurityAuditManager) isWhitelisted(record *FileAccessRecord) bool {
	for _, ip := range sam.config.WhitelistedIPs {
		if record.ClientIP == ip {
			return true
		}
	}
	for _, user := range sam.config.WhitelistedUsers {
		if record.Username == user {
			return true
		}
	}
	return false
}

// countRecentDeletes 计算最近删除次数
func (sam *SecurityAuditManager) countRecentDeletes(username string, windowStart time.Time) int {
	count := 0
	for _, record := range sam.fileAccessRecords {
		if record.Username == username && record.Action == "delete" && record.Timestamp.After(windowStart) {
			count++
		}
	}

	// 也检查用户统计
	if stats, exists := sam.userAccessStats[username]; exists {
		// 重置超过时间窗口的计数
		if stats.LastAccess.After(windowStart) {
			count += stats.RecentDeleteCount
		}
	}

	return count
}

// countRecentAccess 计算最近访问次数
func (sam *SecurityAuditManager) countRecentAccess(username string, windowStart time.Time) int {
	count := 0
	for _, record := range sam.fileAccessRecords {
		if record.Username == username && record.Timestamp.After(windowStart) {
			count++
		}
	}
	return count
}

// GetFileAccessRecords 获取文件访问记录
func (sam *SecurityAuditManager) GetFileAccessRecords(limit, offset int, filters map[string]string) []*FileAccessRecord {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	result := make([]*FileAccessRecord, 0)

	for _, record := range sam.fileAccessRecords {
		if !sam.matchesFileFilters(record, filters) {
			continue
		}
		result = append(result, record)
	}

	// 按时间倒序
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

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

// GetSecurityEvents 获取安全事件
func (sam *SecurityAuditManager) GetSecurityEvents(limit, offset int, filters map[string]string) []*SecurityEvent {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	result := make([]*SecurityEvent, 0)

	for _, event := range sam.events {
		if !sam.matchesEventFilters(event, filters) {
			continue
		}
		result = append(result, event)
	}

	// 按时间倒序
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

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

// matchesFileFilters 检查文件访问记录是否匹配筛选条件
func (sam *SecurityAuditManager) matchesFileFilters(record *FileAccessRecord, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "username":
			if record.Username != value {
				return false
			}
		case "share_name":
			if record.ShareName != value {
				return false
			}
		case "client_ip":
			if record.ClientIP != value {
				return false
			}
		case "action":
			if record.Action != value {
				return false
			}
		case "status":
			if record.Status != value {
				return false
			}
		}
	}
	return true
}

// matchesEventFilters 检查安全事件是否匹配筛选条件
func (sam *SecurityAuditManager) matchesEventFilters(event *SecurityEvent, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "event_type":
			if event.EventType != value {
				return false
			}
		case "severity":
			if event.Severity != value {
				return false
			}
		case "username":
			if event.Username != value {
				return false
			}
		case "share_name":
			if event.ShareName != value {
				return false
			}
		case "client_ip":
			if event.ClientIP != value {
				return false
			}
		}
	}
	return true
}

// GetUserAccessStats 获取用户访问统计
func (sam *SecurityAuditManager) GetUserAccessStats(username string) (*UserAccessStats, bool) {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	stats, exists := sam.userAccessStats[username]
	if !exists {
		return nil, false
	}

	// 返回副本
	copy := *stats
	copy.RecentFiles = make([]string, len(stats.RecentFiles))
	copySlice(stats.RecentFiles, copy.RecentFiles)

	return &copy, true
}

// GetIPAccessStats 获取 IP 访问统计
func (sam *SecurityAuditManager) GetIPAccessStats(clientIP string) (*IPAccessStats, bool) {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	stats, exists := sam.ipAccessStats[clientIP]
	if !exists {
		return nil, false
	}

	// 返回副本
	copy := *stats
	copy.ConnectedUsers = make([]string, len(stats.ConnectedUsers))
	copySlice(stats.ConnectedUsers, copy.ConnectedUsers)

	return &copy, true
}

// GetAnomalyRules 获取异常检测规则
func (sam *SecurityAuditManager) GetAnomalyRules() []AnomalyRule {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	result := make([]AnomalyRule, len(sam.anomalyRules))
	copy(result, sam.anomalyRules)
	return result
}

// AddAnomalyRule 添加异常检测规则
func (sam *SecurityAuditManager) AddAnomalyRule(rule AnomalyRule) {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	rule.Enabled = true
	sam.anomalyRules = append(sam.anomalyRules, rule)
}

// UpdateAnomalyRule 更新异常检测规则
func (sam *SecurityAuditManager) UpdateAnomalyRule(ruleID string, rule AnomalyRule) error {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	for i, existing := range sam.anomalyRules {
		if existing.ID == ruleID {
			sam.anomalyRules[i] = rule
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", ruleID)
}

// DeleteAnomalyRule 删除异常检测规则
func (sam *SecurityAuditManager) DeleteAnomalyRule(ruleID string) error {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	for i, existing := range sam.anomalyRules {
		if existing.ID == ruleID {
			sam.anomalyRules = append(sam.anomalyRules[:i], sam.anomalyRules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", ruleID)
}

// GetRansomwareIndicators 获取勒索软件检测指标
func (sam *SecurityAuditManager) GetRansomwareIndicators() []RansomwareIndicator {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	result := make([]RansomwareIndicator, len(sam.ransomwareIndicators))
	copy(result, sam.ransomwareIndicators)
	return result
}

// GetSecurityStats 获取安全统计信息
func (sam *SecurityAuditManager) GetSecurityStats() map[string]interface{} {
	sam.mu.RLock()
	defer sam.mu.RUnlock()

	// 统计事件
	eventByType := make(map[string]int)
	eventBySeverity := make(map[string]int)
	for _, event := range sam.events {
		eventByType[event.EventType]++
		eventBySeverity[event.Severity]++
	}

	// 统计文件访问
	actionStats := make(map[string]int)
	for _, record := range sam.fileAccessRecords {
		actionStats[record.Action]++
	}

	return map[string]interface{}{
		"total_events":        len(sam.events),
		"total_file_access":   len(sam.fileAccessRecords),
		"events_by_type":      eventByType,
		"events_by_severity":  eventBySeverity,
		"actions_stats":       actionStats,
		"unique_users":        len(sam.userAccessStats),
		"unique_ips":          len(sam.ipAccessStats),
		"anomaly_rules":       len(sam.anomalyRules),
		"ransomware_indicators": len(sam.ransomwareIndicators),
	}
}

// CleanupOldRecords 清理旧记录
func (sam *SecurityAuditManager) CleanupOldRecords() {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -sam.config.RetentionDays)

	// 清理文件访问记录
	cleanedRecords := make([]*FileAccessRecord, 0)
	for _, record := range sam.fileAccessRecords {
		if record.Timestamp.After(cutoff) {
			cleanedRecords = append(cleanedRecords, record)
		}
	}
	sam.fileAccessRecords = cleanedRecords

	// 清理安全事件
	cleanedEvents := make([]*SecurityEvent, 0)
	for _, event := range sam.events {
		if event.Timestamp.After(cutoff) {
			cleanedEvents = append(cleanedEvents, event)
		}
	}
	sam.events = cleanedEvents
}

// StartCleanupRoutine 启动定期清理例程
func (sam *SecurityAuditManager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			sam.CleanupOldRecords()
		}
	}()
}

// ResetRecentStats 重置近期统计（用于测试和调试）
func (sam *SecurityAuditManager) ResetRecentStats() {
	sam.mu.Lock()
	defer sam.mu.Unlock()

	for _, stats := range sam.userAccessStats {
		stats.RecentDeleteCount = 0
	}
	for _, stats := range sam.ipAccessStats {
		stats.RecentDeleteCount = 0
	}
}

// 辅助函数

// generateEventID 生成事件 ID
func generateEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

// getFileExtension 获取文件扩展名
func getFileExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx == -1 {
		return ""
	}
	return path[idx:]
}

// copySlice 复制切片
func copySlice(src, dst []string) {
	for i, v := range src {
		if i < len(dst) {
			dst[i] = v
		}
	}
}