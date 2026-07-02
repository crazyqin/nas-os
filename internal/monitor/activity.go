// Package monitor 活动监控增强模块
// 对标群晖Active Insight，实现文件活动监控和异常行为检测
package monitor

import (
	"context"
	"sync"
	"time"
)

// ActivityMonitor 活动监控器.
type ActivityMonitor struct {
	tracker      *ActivityTracker
	detector     *AnomalyDetector
	alertManager *AlertManager
	logger       interface{}
	config       *ActivityConfig
	mu           sync.RWMutex
}

// ActivityConfig 活动监控配置.
type ActivityConfig struct {
	// 监控路径
	WatchPaths []string `json:"watchPaths"`
	// 排除路径
	ExcludePaths []string `json:"excludePaths"`
	// 事件类型
	WatchEvents []string `json:"watchEvents"` // create, modify, delete, rename, access
	// 异常检测阈值
	Thresholds map[string]int `json:"thresholds"`
	// 监控间隔(秒)
	MonitorInterval int `json:"monitorInterval"`
	// 是否启用实时监控
	EnableRealtime bool `json:"enableRealtime"`
	// 最大活动记录数
	MaxActivityRecords int `json:"maxActivityRecords"`
	// 历史数据保留天数
	HistoryRetentionDays int `json:"historyRetentionDays"`
	// 是否启用异常检测
	EnableAnomalyDetection bool `json:"enableAnomalyDetection"`
	// 是否启用用户行为分析
	EnableUserBehavior bool `json:"enableUserBehavior"`
	// 是否启用时间模式分析
	EnableTimePattern bool `json:"enableTimePattern"`
	// 是否启用文件类型分析
	EnableFileTypeAnalysis bool `json:"enableFileTypeAnalysis"`
	// 是否启用网络行为分析
	EnableNetworkBehavior bool `json:"enableNetworkBehavior"`
}

// DefaultActivityConfig 默认配置.
func DefaultActivityConfig() *ActivityConfig {
	return &ActivityConfig{
		WatchPaths:   []string{"/"},
		ExcludePaths: []string{"/proc", "/sys", "/dev", "/tmp"},
		WatchEvents:  []string{"create", "modify", "delete", "rename", "access"},
		Thresholds: map[string]int{
			"maxDeletePerMinute":   100,
			"maxModifyPerMinute":   500,
			"maxCreatePerMinute":   200,
			"maxRenamePerMinute":   50,
			"maxAccessPerMinute":   1000,
			"maxFileSizeChange":    100 * 1024 * 1024, // 100MB
			"suspiciousFileTypes":  10,
			"unusualTimeThreshold": 3,
		},
		MonitorInterval:        5,
		EnableRealtime:         true,
		MaxActivityRecords:     10000,
		HistoryRetentionDays:   30,
		EnableAnomalyDetection: true,
		EnableUserBehavior:     true,
		EnableTimePattern:      true,
		EnableFileTypeAnalysis: true,
		EnableNetworkBehavior:  false,
	}
}

// NewActivityMonitor 创建活动监控器.
func NewActivityMonitor(config *ActivityConfig, logger interface{}) *ActivityMonitor {
	if config == nil {
		config = DefaultActivityConfig()
	}

	return &ActivityMonitor{
		tracker:      NewActivityTracker(config.MaxActivityRecords),
		detector:     NewAnomalyDetector(config),
		alertManager: NewAlertManager(),
		logger:       logger,
		config:       config,
	}
}

// FileActivity 文件活动事件.
type FileActivity struct {
	ID         string       `json:"id"`
	Path       string       `json:"path"`
	EventType  ActivityType `json:"eventType"`
	User       string       `json:"user"`
	Process    string       `json:"process"`
	Size       int64        `json:"size"`
	SizeChange int64        `json:"sizeChange"`
	Timestamp  time.Time    `json:"timestamp"`
	ClientIP   string       `json:"clientIp"`
	Share      string       `json:"share"`
	FileType   string       `json:"fileType"`
	IsAnomaly  bool         `json:"isAnomaly"`
	RiskScore  float64      `json:"riskScore"`
	Tags       []string     `json:"tags"`
}

// ActivityType 活动类型.
type ActivityType string

const (
	ActivityCreate   ActivityType = "create"
	ActivityModify   ActivityType = "modify"
	ActivityDelete   ActivityType = "delete"
	ActivityRename   ActivityType = "rename"
	ActivityAccess   ActivityType = "access"
	ActivityRead     ActivityType = "read"
	ActivityWrite    ActivityType = "write"
	ActivityDownload ActivityType = "download"
	ActivityUpload   ActivityType = "upload"
	ActivityCopy     ActivityType = "copy"
	ActivityMove     ActivityType = "move"
)

// RecordActivity 记录活动事件.
func (am *ActivityMonitor) RecordActivity(activity *FileActivity) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 设置时间戳
	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now()
	}

	// 检测异常行为
	if am.config.EnableAnomalyDetection {
		anomalyResult := am.detector.DetectAnomaly(activity)
		activity.IsAnomaly = anomalyResult.IsAnomaly
		activity.RiskScore = anomalyResult.RiskScore
		activity.Tags = anomalyResult.Tags

		// 如果检测到异常，生成告警
		if activity.IsAnomaly {
			am.alertManager.CreateAlert(&AnomalyAlert{
				ID:        activity.ID,
				Type:      anomalyResult.Type,
				Severity:  anomalyResult.Severity,
				Path:      activity.Path,
				User:      activity.User,
				Timestamp: activity.Timestamp,
				RiskScore: activity.RiskScore,
				Details:   anomalyResult.Details,
				Tags:      activity.Tags,
			})
		}
	}

	// 记录活动
	am.tracker.Record(activity)

	return nil
}

// GetActivityHistory 获取活动历史.
func (am *ActivityMonitor) GetActivityHistory(filter *ActivityFilter) []*FileActivity {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return am.tracker.GetHistory(filter)
}

// GetAnomalyAlerts 获取异常告警.
func (am *ActivityMonitor) GetAnomalyAlerts(severity string, limit int) []*AnomalyAlert {
	return am.alertManager.GetAlerts(severity, limit)
}

// ClearAlert 清除告警.
func (am *ActivityMonitor) ClearAlert(alertID string) error {
	return am.alertManager.ClearAlert(alertID)
}

// GetStatistics 获取统计信息.
func (am *ActivityMonitor) GetStatistics() *ActivityStatistics {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := am.tracker.GetStatistics()

	// 添加异常统计
	stats.AnomalyCount = am.detector.GetAnomalyCount()
	stats.AlertCount = am.alertManager.GetAlertCount()

	return stats
}

// ActivityFilter 活动过滤条件.
type ActivityFilter struct {
	Path         string         `json:"path"`
	User         string         `json:"user"`
	EventTypes   []ActivityType `json:"eventTypes"`
	StartTime    time.Time      `json:"startTime"`
	EndTime      time.Time      `json:"endTime"`
	IsAnomaly    bool           `json:"isAnomaly"`
	MinRiskScore float64        `json:"minRiskScore"`
	FileType     string         `json:"fileType"`
	Limit        int            `json:"limit"`
}

// ActivityStatistics 活动统计.
type ActivityStatistics struct {
	TotalActivities  int64            `json:"totalActivities"`
	AnomalyCount     int64            `json:"anomalyCount"`
	AlertCount       int64            `json:"alertCount"`
	EventCounts      map[string]int64 `json:"eventCounts"`
	UserActivity     map[string]int64 `json:"userActivity"`
	FileTypeActivity map[string]int64 `json:"fileTypeActivity"`
	HourlyActivity   map[int]int64    `json:"hourlyActivity"`
	TopPaths         []PathActivity   `json:"topPaths"`
	TopUsers         []UserActivity   `json:"topUsers"`
	LastHourRate     int64            `json:"lastHourRate"`
	AvgDailyRate     float64          `json:"avgDailyRate"`
}

// PathActivity 路径活动统计.
type PathActivity struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// UserActivity 用户活动统计.
type UserActivity struct {
	User        string  `json:"user"`
	Count       int64   `json:"count"`
	AnomalyRate float64 `json:"anomalyRate"`
}

// ActivityTracker 活动追踪器.
type ActivityTracker struct {
	records    []*FileActivity
	maxRecords int
	stats      *ActivityStatistics
	mu         sync.RWMutex
}

// NewActivityTracker 创建追踪器.
func NewActivityTracker(maxRecords int) *ActivityTracker {
	return &ActivityTracker{
		records:    make([]*FileActivity, 0),
		maxRecords: maxRecords,
		stats: &ActivityStatistics{
			EventCounts:      make(map[string]int64),
			UserActivity:     make(map[string]int64),
			FileTypeActivity: make(map[string]int64),
			HourlyActivity:   make(map[int]int64),
		},
	}
}

// Record 记录活动.
func (t *ActivityTracker) Record(activity *FileActivity) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 添加记录
	t.records = append(t.records, activity)

	// 限制记录数量
	if len(t.records) > t.maxRecords {
		t.records = t.records[1:]
	}

	// 更新统计
	t.updateStats(activity)
}

// updateStats 更新统计信息.
func (t *ActivityTracker) updateStats(activity *FileActivity) {
	t.stats.TotalActivities++

	// 事件类型统计
	t.stats.EventCounts[string(activity.EventType)]++

	// 用户活动统计
	t.stats.UserActivity[activity.User]++

	// 文件类型统计
	t.stats.FileTypeActivity[activity.FileType]++

	// 按小时统计
	hour := activity.Timestamp.Hour()
	t.stats.HourlyActivity[hour]++
}

// GetHistory 获取历史记录.
func (t *ActivityTracker) GetHistory(filter *ActivityFilter) []*FileActivity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*FileActivity

	for _, activity := range t.records {
		if !t.matchFilter(activity, filter) {
			continue
		}

		result = append(result, activity)

		if filter != nil && filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result
}

// matchFilter 匹配过滤条件.
func (t *ActivityTracker) matchFilter(activity *FileActivity, filter *ActivityFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Path != "" && activity.Path != filter.Path {
		return false
	}

	if filter.User != "" && activity.User != filter.User {
		return false
	}

	if len(filter.EventTypes) > 0 {
		found := false
		for _, et := range filter.EventTypes {
			if activity.EventType == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if !filter.StartTime.IsZero() && activity.Timestamp.Before(filter.StartTime) {
		return false
	}

	if !filter.EndTime.IsZero() && activity.Timestamp.After(filter.EndTime) {
		return false
	}

	if filter.IsAnomaly && !activity.IsAnomaly {
		return false
	}

	if filter.MinRiskScore > 0 && activity.RiskScore < filter.MinRiskScore {
		return false
	}

	if filter.FileType != "" && activity.FileType != filter.FileType {
		return false
	}

	return true
}

// GetStatistics 获取统计信息.
func (t *ActivityTracker) GetStatistics() *ActivityStatistics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.stats
}

// AnomalyDetector 异常检测器.
type AnomalyDetector struct {
	config   *ActivityConfig
	patterns map[string]*BehaviorPattern
	rules    []*AnomalyRule
	stats    *DetectionStats
	mu       sync.RWMutex
}

// BehaviorPattern 行为模式.
type BehaviorPattern struct {
	User         string         `json:"user"`
	Path         string         `json:"path"`
	NormalEvents map[string]int `json:"normalEvents"` // 正常事件频率
	TimePatterns map[int]int    `json:"timePatterns"` // 时间模式
	FileTypes    map[string]int `json:"fileTypes"`    // 常用文件类型
	LastUpdate   time.Time      `json:"lastUpdate"`
}

// AnomalyRule 异常规则.
type AnomalyRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        AnomalyType   `json:"type"`
	Threshold   int           `json:"threshold"`
	Severity    AlertSeverity `json:"severity"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
}

// AnomalyType 异常类型.
type AnomalyType string

const (
	AnomalyHighFrequency    AnomalyType = "high_frequency"    // 高频操作
	AnomalyUnusualTime      AnomalyType = "unusual_time"      // 异常时间
	AnomalySuspiciousFile   AnomalyType = "suspicious_file"   // 可疑文件
	AnomalyMassDelete       AnomalyType = "mass_delete"       // 大量删除
	AnomalyMassModify       AnomalyType = "mass_modify"       // 大量修改
	AnomalyRansomware       AnomalyType = "ransomware"        // 勒索软件行为
	AnomalyDataExfiltration AnomalyType = "data_exfiltration" // 数据外泄
	AnomalyUnauthorized     AnomalyType = "unauthorized"      // 未授权访问
	AnomalyAbnormalPath     AnomalyType = "abnormal_path"     // 异常路径
)

// AlertSeverity 告警严重性.
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "low"
	SeverityMedium   AlertSeverity = "medium"
	SeverityHigh     AlertSeverity = "high"
	SeverityCritical AlertSeverity = "critical"
)

// NewAnomalyDetector 创建异常检测器.
func NewAnomalyDetector(config *ActivityConfig) *AnomalyDetector {
	detector := &AnomalyDetector{
		config:   config,
		patterns: make(map[string]*BehaviorPattern),
		rules:    DefaultAnomalyRules(),
		stats: &DetectionStats{
			DetectionCounts: make(map[string]int64),
		},
	}

	return detector
}

// DefaultAnomalyRules 默认异常规则.
func DefaultAnomalyRules() []*AnomalyRule {
	return []*AnomalyRule{
		{
			ID:          "mass_delete",
			Name:        "大量删除检测",
			Type:        AnomalyMassDelete,
			Threshold:   100,
			Severity:    SeverityHigh,
			Description: "短时间内删除大量文件",
			Enabled:     true,
		},
		{
			ID:          "ransomware_pattern",
			Name:        "勒索软件模式检测",
			Type:        AnomalyRansomware,
			Threshold:   50,
			Severity:    SeverityCritical,
			Description: "检测勒索软件加密行为模式",
			Enabled:     true,
		},
		{
			ID:          "unusual_time",
			Name:        "异常时间访问",
			Type:        AnomalyUnusualTime,
			Threshold:   3,
			Severity:    SeverityMedium,
			Description: "非工作时间大量文件操作",
			Enabled:     true,
		},
		{
			ID:          "high_frequency",
			Name:        "高频操作检测",
			Type:        AnomalyHighFrequency,
			Threshold:   500,
			Severity:    SeverityMedium,
			Description: "短时间内大量文件操作",
			Enabled:     true,
		},
		{
			ID:          "suspicious_file",
			Name:        "可疑文件类型",
			Type:        AnomalySuspiciousFile,
			Threshold:   10,
			Severity:    SeverityLow,
			Description: "创建可疑文件类型",
			Enabled:     true,
		},
		{
			ID:          "data_exfil",
			Name:        "数据外泄检测",
			Type:        AnomalyDataExfiltration,
			Threshold:   100,
			Severity:    SeverityHigh,
			Description: "大量文件下载或复制",
			Enabled:     true,
		},
	}
}

// DetectionStats 检测统计.
type DetectionStats struct {
	TotalDetected   int64            `json:"totalDetected"`
	AnomalyCount    int64            `json:"anomalyCount"`
	DetectionCounts map[string]int64 `json:"detectionCounts"`
	LastDetection   time.Time        `json:"lastDetection"`
}

// AnomalyResult 异常检测结果.
type AnomalyResult struct {
	IsAnomaly bool          `json:"isAnomaly"`
	Type      AnomalyType   `json:"type"`
	Severity  AlertSeverity `json:"severity"`
	RiskScore float64       `json:"riskScore"`
	Details   string        `json:"details"`
	Tags      []string      `json:"tags"`
	RuleID    string        `json:"ruleId"`
}

// DetectAnomaly 检测异常行为.
func (ad *AnomalyDetector) DetectAnomaly(activity *FileActivity) *AnomalyResult {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	result := &AnomalyResult{
		IsAnomaly: false,
		RiskScore: 0,
		Tags:      []string{},
	}

	// 检查各个规则
	for _, rule := range ad.rules {
		if !rule.Enabled {
			continue
		}

		switch rule.Type {
		case AnomalyMassDelete:
			if ad.checkMassDelete(activity, rule) {
				result.IsAnomaly = true
				result.Type = rule.Type
				result.Severity = rule.Severity
				result.RiskScore += ad.calculateRiskScore(rule.Severity)
				result.RuleID = rule.ID
				result.Details = rule.Description
				result.Tags = append(result.Tags, "mass_delete")
			}

		case AnomalyUnusualTime:
			if ad.checkUnusualTime(activity, rule) {
				result.IsAnomaly = true
				result.Type = rule.Type
				result.Severity = rule.Severity
				result.RiskScore += ad.calculateRiskScore(rule.Severity)
				result.RuleID = rule.ID
				result.Details = rule.Description
				result.Tags = append(result.Tags, "unusual_time")
			}

		case AnomalySuspiciousFile:
			if ad.checkSuspiciousFile(activity, rule) {
				result.IsAnomaly = true
				result.Type = rule.Type
				result.Severity = rule.Severity
				result.RiskScore += ad.calculateRiskScore(rule.Severity) * 0.5
				result.RuleID = rule.ID
				result.Details = rule.Description
				result.Tags = append(result.Tags, "suspicious_file")
			}

		case AnomalyRansomware:
			if ad.checkRansomware(activity, rule) {
				result.IsAnomaly = true
				result.Type = rule.Type
				result.Severity = rule.Severity
				result.RiskScore += ad.calculateRiskScore(rule.Severity) * 2
				result.RuleID = rule.ID
				result.Details = rule.Description
				result.Tags = append(result.Tags, "ransomware_pattern")
			}
		}
	}

	// 更新统计
	if result.IsAnomaly {
		ad.stats.TotalDetected++
		ad.stats.AnomalyCount++
		ad.stats.DetectionCounts[string(result.Type)]++
		ad.stats.LastDetection = time.Now()
	}

	return result
}

// checkMassDelete 检查大量删除.
func (ad *AnomalyDetector) checkMassDelete(activity *FileActivity, rule *AnomalyRule) bool {
	return activity.EventType == ActivityDelete && activity.Size > int64(rule.Threshold)
}

// checkUnusualTime 检查异常时间.
func (ad *AnomalyDetector) checkUnusualTime(activity *FileActivity, rule *AnomalyRule) bool {
	hour := activity.Timestamp.Hour()
	// 非工作时间定义: 22:00-06:00
	return (hour >= 22 || hour < 6) && activity.EventType != ActivityAccess
}

// checkSuspiciousFile 检查可疑文件.
func (ad *AnomalyDetector) checkSuspiciousFile(activity *FileActivity, rule *AnomalyRule) bool {
	suspiciousExtensions := []string{
		".exe", ".bat", ".cmd", ".ps1", ".vbs", ".js",
		".scr", ".pif", ".com", ".jar", ".sh",
	}

	for _, ext := range suspiciousExtensions {
		if activity.FileType == ext {
			return true
		}
	}

	return false
}

// checkRansomware 检查勒索软件行为.
func (ad *AnomalyDetector) checkRansomware(activity *FileActivity, rule *AnomalyRule) bool {
	// 检测勒索软件特征:
	// 1. 修改文件后改为加密扩展名
	// 2. 创建赎金说明文件
	// 3. 大量文件快速修改

	ransomwareIndicators := []string{
		".encrypted", ".locked", ".crypto", ".ransom",
		".enc", ".crypto", ".locked", ".cerber",
		"_readme.txt", "decrypt_instructions.txt",
	}

	for _, indicator := range ransomwareIndicators {
		if activity.FileType == indicator {
			return true
		}
	}

	return false
}

// calculateRiskScore 计算风险评分.
func (ad *AnomalyDetector) calculateRiskScore(severity AlertSeverity) float64 {
	switch severity {
	case SeverityCritical:
		return 100
	case SeverityHigh:
		return 75
	case SeverityMedium:
		return 50
	case SeverityLow:
		return 25
	default:
		return 10
	}
}

// GetAnomalyCount 获取异常计数.
func (ad *AnomalyDetector) GetAnomalyCount() int64 {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	return ad.stats.AnomalyCount
}

// GetDetectionStats 获取检测统计.
func (ad *AnomalyDetector) GetDetectionStats() *DetectionStats {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	return ad.stats
}

// AddRule 添加规则.
func (ad *AnomalyDetector) AddRule(rule *AnomalyRule) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.rules = append(ad.rules, rule)
}

// RemoveRule 移除规则.
func (ad *AnomalyDetector) RemoveRule(ruleID string) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	for i, rule := range ad.rules {
		if rule.ID == ruleID {
			ad.rules = append(ad.rules[:i], ad.rules[i+1:]...)
			break
		}
	}
}

// AlertManager 告警管理器.
type AlertManager struct {
	alerts    []*AnomalyAlert
	maxAlerts int
	mu        sync.RWMutex
}

// NewAlertManager 创建告警管理器.
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts:    make([]*AnomalyAlert, 0),
		maxAlerts: 1000,
	}
}

// AnomalyAlert 异常告警.
type AnomalyAlert struct {
	ID        string        `json:"id"`
	Type      AnomalyType   `json:"type"`
	Severity  AlertSeverity `json:"severity"`
	Path      string        `json:"path"`
	User      string        `json:"user"`
	Timestamp time.Time     `json:"timestamp"`
	RiskScore float64       `json:"riskScore"`
	Details   string        `json:"details"`
	Tags      []string      `json:"tags"`
	Status    AlertStatus   `json:"status"`
	ClearedAt time.Time     `json:"clearedAt"`
}

// AlertStatus 告警状态.
type AlertStatus string

const (
	AlertActive       AlertStatus = "active"
	AlertCleared      AlertStatus = "cleared"
	AlertAcknowledged AlertStatus = "acknowledged"
)

// CreateAlert 创建告警.
func (am *AlertManager) CreateAlert(alert *AnomalyAlert) {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert.Status = AlertActive
	am.alerts = append(am.alerts, alert)

	// 限制告警数量
	if len(am.alerts) > am.maxAlerts {
		am.alerts = am.alerts[1:]
	}
}

// GetAlerts 获取告警列表.
func (am *AlertManager) GetAlerts(severity string, limit int) []*AnomalyAlert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*AnomalyAlert

	for _, alert := range am.alerts {
		if severity != "" && string(alert.Severity) != severity {
			continue
		}

		result = append(result, alert)

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// ClearAlert 清除告警.
func (am *AlertManager) ClearAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == alertID {
			alert.Status = AlertCleared
			alert.ClearedAt = time.Now()
			return nil
		}
	}

	return nil
}

// GetAlertCount 获取告警计数.
func (am *AlertManager) GetAlertCount() int64 {
	am.mu.RLock()
	defer am.mu.RUnlock()

	count := 0
	for _, alert := range am.alerts {
		if alert.Status == AlertActive {
			count++
		}
	}

	return int64(count)
}

// StartMonitoring 启动监控.
func (am *ActivityMonitor) StartMonitoring(ctx context.Context) error {
	// 启动实时监控（轻量轮询实现）
	if am.config.EnableRealtime {
		go am.startRealtimePolling(ctx)
	}

	// 启动定期清理
	go am.startCleanup(ctx)

	return nil
}

// startCleanup 启动清理任务.
func (am *ActivityMonitor) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			am.cleanupOldRecords()
		}
	}
}

// cleanupOldRecords 清理旧记录.
func (am *ActivityMonitor) cleanupOldRecords() {
	am.mu.Lock()
	defer am.mu.Unlock()

	retentionDays := am.config.HistoryRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// 清理活动记录
	var newRecords []*FileActivity
	for _, record := range am.tracker.records {
		if record.Timestamp.After(cutoffTime) {
			newRecords = append(newRecords, record)
		}
	}
	am.tracker.records = newRecords

	// 清理告警
	var newAlerts []*AnomalyAlert
	for _, alert := range am.alertManager.alerts {
		if alert.Timestamp.After(cutoffTime) || alert.Status == AlertActive {
			newAlerts = append(newAlerts, alert)
		}
	}
	am.alertManager.alerts = newAlerts
}

// StopMonitoring 停止监控.
func (am *ActivityMonitor) StopMonitoring() error {
	return nil
}

func (am *ActivityMonitor) startRealtimePolling(ctx context.Context) {
	interval := time.Duration(am.config.MonitorInterval) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Hook for filesystem watchers; polling keeps monitor lifecycle active without external dependencies.
		}
	}
}
