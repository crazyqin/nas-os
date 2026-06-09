// Package ransomware 勒索软件检测与防护模块
// 实时监控文件系统异常行为，检测并阻止勒索软件攻击
// 参考: TrueNAS 勒索软件检测功能
package ransomware

import (
	"fmt"
	"sync"
	"time"
)

// ActionType 操作类型
type ActionType string

const (
	ActionTypeFileModified    ActionType = "file_modified"
	ActionTypeFileDeleted     ActionType = "file_deleted"
	ActionTypeFileRenamed     ActionType = "file_renamed"
	ActionTypeBulkOperation   ActionType = "bulk_operation"
	ActionTypeExtensionChange ActionType = "extension_change"
	ActionTypeEncryption      ActionType = "encryption"
)

// DetectionRule 检测规则
type DetectionRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled"`
	Threshold   int         `json:"threshold"`    // 触发阈值
	TimeWindow  int         `json:"time_window"`  // 时间窗口（秒）
	Action      ActionType  `json:"action"`
	ThreatLevel ThreatLevel `json:"threat_level"`
}

// SuspiciousActivity 可疑活动
type SuspiciousActivity struct {
	ID          string      `json:"id"`
	RuleID      string      `json:"rule_id"`
	RuleName    string      `json:"rule_name"`
	Action      ActionType  `json:"action"`
	FilePath    string      `json:"file_path"`
	UserID      string      `json:"user_id"`
	Timestamp   time.Time   `json:"timestamp"`
	ThreatLevel ThreatLevel `json:"threat_level"`
	Blocked     bool        `json:"blocked"`
	Details     string      `json:"details"`
}

// FileSnapshot 文件快照
type FileSnapshot struct {
	FilePath  string    `json:"file_path"`
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	Extension string    `json:"extension"`
}

// RansomwareDetector 勒索软件检测器
type RansomwareDetector struct {
	mu              sync.RWMutex
	rules           map[string]*DetectionRule
	activities      []SuspiciousActivity
	fileSnapshots   map[string]*FileSnapshot
	actionCounts    map[string]int
	lastReset       time.Time
	isMonitoring    bool
	alertCallback   func(SuspiciousActivity)
	blockCallback   func(string) bool
	config          *DetectorConfig
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	MaxActivities      int  `json:"max_activities"`
	MonitorInterval    int  `json:"monitor_interval"` // 秒
	AutoBlock          bool `json:"auto_block"`
	AlertThreshold     int  `json:"alert_threshold"`
	SnapshotRetention  int  `json:"snapshot_retention"` // 天
	EnableRealTime     bool `json:"enable_real_time"`
}

// NewRansomwareDetector 创建勒索软件检测器
func NewRansomwareDetector(config *DetectorConfig) *RansomwareDetector {
	if config == nil {
		config = &DetectorConfig{
			MaxActivities:     10000,
			MonitorInterval:   5,
			AutoBlock:         true,
			AlertThreshold:    10,
			SnapshotRetention: 30,
			EnableRealTime:    true,
		}
	}

	detector := &RansomwareDetector{
		rules:         make(map[string]*DetectionRule),
		activities:    make([]SuspiciousActivity, 0),
		fileSnapshots: make(map[string]*FileSnapshot),
		actionCounts:  make(map[string]int),
		lastReset:     time.Now(),
		config:        config,
	}

	// 初始化默认规则
	detector.initDefaultRules()

	return detector
}

// initDefaultRules 初始化默认检测规则
func (d *RansomwareDetector) initDefaultRules() {
	defaultRules := []*DetectionRule{
		{
			ID:          "bulk_modify",
			Name:        "批量文件修改检测",
			Description: "检测短时间内大量文件被修改的情况",
			Enabled:     true,
			Threshold:   50,
			TimeWindow:  60,
			Action:      ActionTypeBulkOperation,
			ThreatLevel: ThreatLevelHigh,
		},
		{
			ID:          "extension_change",
			Name:        "文件扩展名篡改检测",
			Description: "检测文件扩展名被批量修改的情况",
			Enabled:     true,
			Threshold:   10,
			TimeWindow:  30,
			Action:      ActionTypeExtensionChange,
			ThreatLevel: ThreatLevelCritical,
		},
		{
			ID:          "encryption_pattern",
			Name:        "加密行为模式检测",
			Description: "检测疑似加密操作的行为模式",
			Enabled:     true,
			Threshold:   20,
			TimeWindow:  120,
			Action:      ActionTypeEncryption,
			ThreatLevel: ThreatLevelCritical,
		},
		{
			ID:          "mass_delete",
			Name:        "批量删除检测",
			Description: "检测短时间内大量文件被删除的情况",
			Enabled:     true,
			Threshold:   100,
			TimeWindow:  60,
			Action:      ActionTypeFileDeleted,
			ThreatLevel: ThreatLevelHigh,
		},
		{
			ID:          "suspicious_rename",
			Name:        "可疑重命名检测",
			Description: "检测文件被重命名为可疑扩展名（如 .encrypted, .locked）",
			Enabled:     true,
			Threshold:   5,
			TimeWindow:  30,
			Action:      ActionTypeFileRenamed,
			ThreatLevel: ThreatLevelHigh,
		},
	}

	for _, rule := range defaultRules {
		d.rules[rule.ID] = rule
	}
}

// StartMonitoring 开始监控
func (d *RansomwareDetector) StartMonitoring() {
	d.mu.Lock()
	d.isMonitoring = true
	d.mu.Unlock()

	go d.monitorLoop()
}

// StopMonitoring 停止监控
func (d *RansomwareDetector) StopMonitoring() {
	d.mu.Lock()
	d.isMonitoring = false
	d.mu.Unlock()
}

// monitorLoop 监控循环
func (d *RansomwareDetector) monitorLoop() {
	ticker := time.NewTicker(time.Duration(d.config.MonitorInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.RLock()
		monitoring := d.isMonitoring
		d.mu.RUnlock()

		if !monitoring {
			return
		}

		d.checkThresholds()
	}
}

// checkThresholds 检查阈值
func (d *RansomwareDetector) checkThresholds() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 重置计数器（每分钟）
	if time.Since(d.lastReset) > time.Minute {
		d.actionCounts = make(map[string]int)
		d.lastReset = time.Now()
	}

	// 检查是否有规则被触发
	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}

		count := d.actionCounts[string(rule.Action)]
		if count >= rule.Threshold {
			d.triggerAlert(rule, count)
		}
	}
}

// triggerAlert 触发告警
func (d *RansomwareDetector) triggerAlert(rule *DetectionRule, count int) {
	activity := SuspiciousActivity{
		ID:          fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Action:      rule.Action,
		ThreatLevel: rule.ThreatLevel,
		Timestamp:   time.Now(),
		Details:     fmt.Sprintf("检测到 %d 次 %s 操作，超过阈值 %d", count, rule.Action, rule.Threshold),
		Blocked:     d.config.AutoBlock,
	}

	d.activities = append(d.activities, activity)

	// 限制活动记录数量
	if len(d.activities) > d.config.MaxActivities {
		d.activities = d.activities[100:]
	}

	// 回调通知
	if d.alertCallback != nil {
		d.alertCallback(activity)
	}
}

// ReportActivity 报告文件活动
func (d *RansomwareDetector) ReportActivity(action ActionType, filePath, userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 更新计数
	d.actionCounts[string(action)]++

	// 检查是否匹配任何规则
	for _, rule := range d.rules {
		if !rule.Enabled || rule.Action != action {
			continue
		}

		count := d.actionCounts[string(action)]
		if count >= rule.Threshold {
			d.triggerAlert(rule, count)
		}
	}
}

// UpdateSnapshot 更新文件快照
func (d *RansomwareDetector) UpdateSnapshot(filePath, sha256 string, size int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.fileSnapshots[filePath] = &FileSnapshot{
		FilePath: filePath,
		SHA256:   sha256,
		Size:     size,
		ModTime:  time.Now(),
	}
}

// GetSnapshot 获取文件快照
func (d *RansomwareDetector) GetSnapshot(filePath string) *FileSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.fileSnapshots[filePath]
}

// AddRule 添加检测规则
func (d *RansomwareDetector) AddRule(rule *DetectionRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}

	d.rules[rule.ID] = rule
	return nil
}

// RemoveRule 移除检测规则
func (d *RansomwareDetector) RemoveRule(ruleID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.rules[ruleID]; !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	delete(d.rules, ruleID)
	return nil
}

// ListRules 列出所有规则
func (d *RansomwareDetector) ListRules() []*DetectionRule {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rules := make([]*DetectionRule, 0, len(d.rules))
	for _, r := range d.rules {
		rules = append(rules, r)
	}
	return rules
}

// GetActivities 获取活动记录
func (d *RansomwareDetector) GetActivities(limit int) []SuspiciousActivity {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.activities) {
		limit = len(d.activities)
	}

	start := len(d.activities) - limit
	if start < 0 {
		start = 0
	}
	return d.activities[start:]
}

// SetAlertCallback 设置告警回调
func (d *RansomwareDetector) SetAlertCallback(callback func(SuspiciousActivity)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.alertCallback = callback
}

// SetBlockCallback 设置阻止回调
func (d *RansomwareDetector) SetBlockCallback(callback func(string) bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blockCallback = callback
}

// GetStats 获取统计信息
func (d *RansomwareDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	threatCounts := make(map[ThreatLevel]int)
	for _, a := range d.activities {
		threatCounts[a.ThreatLevel]++
	}

	blockedCount := 0
	for _, a := range d.activities {
		if a.Blocked {
			blockedCount++
		}
	}

	return map[string]interface{}{
		"is_monitoring":     d.isMonitoring,
		"total_rules":       len(d.rules),
		"enabled_rules":     d.countEnabledRules(),
		"total_activities":  len(d.activities),
		"blocked_count":     blockedCount,
		"threat_breakdown":  threatCounts,
		"snapshot_count":    len(d.fileSnapshots),
		"last_reset":        d.lastReset,
	}
}

// countEnabledRules 统计启用的规则数
func (d *RansomwareDetector) countEnabledRules() int {
	count := 0
	for _, r := range d.rules {
		if r.Enabled {
			count++
		}
	}
	return count
}
