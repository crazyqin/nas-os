// Package ransomware 勒索软件检测器核心实现
package ransomware

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DetectionRule 检测规则
type DetectionRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// 检测类型: entropy/bulk_write/rapid_delete/extension
	Type string `json:"type"`
	// 阈值
	Threshold float64 `json:"threshold"`
	// 时间窗口（秒）
	WindowSec int `json:"window_sec"`
	// 动作: alert/block/quarantine/snapshot
	Action string `json:"action"`
	// 是否启用
	Enabled bool `json:"enabled"`
	// 严重级别: low/medium/high/critical
	Severity string `json:"severity"`
}

// Activity 检测活动记录
type Activity struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	Details   string    `json:"details"`
	Severity  string    `json:"severity"`
	Action    string    `json:"action"`
}

// RansomwareDetector 勒索软件检测器
type RansomwareDetector struct {
	mu             sync.RWMutex
	monitoring     bool
	rules          []*DetectionRule
	activities     []*Activity
	stats          map[string]interface{}
	logger         *slog.Logger
	stopCh         chan struct{}
	watchPaths     []string
	excludePaths   []string
	entropyThreshold float64
	bulkThreshold    int
	deleteThreshold  int
}

// NewRansomwareDetector 创建勒索软件检测器
func NewRansomwareDetector(logger *slog.Logger) *RansomwareDetector {
	if logger == nil {
		logger = slog.Default()
	}

	d := &RansomwareDetector{
		rules:            make([]*DetectionRule, 0),
		activities:       make([]*Activity, 0),
		stats:            make(map[string]interface{}),
		logger:           logger,
		stopCh:           make(chan struct{}),
		entropyThreshold: 7.5,
		bulkThreshold:    50,
		deleteThreshold:  30,
	}

	d.initDefaultRules()
	d.resetStats()
	return d
}

func (d *RansomwareDetector) initDefaultRules() {
	d.rules = []*DetectionRule{
		{
			ID:          "entropy-spike",
			Name:        "高熵值检测",
			Description: "检测文件熵值异常升高，可能表明加密行为",
			Type:        "entropy",
			Threshold:   7.5,
			WindowSec:   60,
			Action:      "alert",
			Enabled:     true,
			Severity:    "high",
		},
		{
			ID:          "bulk-write",
			Name:        "批量写入检测",
			Description: "短时间内大量文件写入，可能是勒索软件加密",
			Type:        "bulk_write",
			Threshold:   50,
			WindowSec:   30,
			Action:      "block",
			Enabled:     true,
			Severity:    "critical",
		},
		{
			ID:          "rapid-delete",
			Name:        "快速删除检测",
			Description: "短时间内大量文件删除",
			Type:        "rapid_delete",
			Threshold:   30,
			WindowSec:   30,
			Action:      "snapshot",
			Enabled:     true,
			Severity:    "high",
		},
		{
			ID:          "suspicious-ext",
			Name:        "可疑扩展名检测",
			Description: "检测已知勒索软件扩展名",
			Type:        "extension",
			Threshold:   1,
			WindowSec:   0,
			Action:      "block",
			Enabled:     true,
			Severity:    "critical",
		},
	}
}

func (d *RansomwareDetector) resetStats() {
	d.stats = map[string]interface{}{
		"is_monitoring":     false,
		"total_events":      int64(0),
		"threats_detected":  int64(0),
		"protections_triggered": int64(0),
		"snapshots_created": int64(0),
		"rules_count":       len(d.rules),
		"activities_count":  0,
		"start_time":        nil,
		"last_event":        nil,
	}
}

// StartMonitoring 开始监控
func (d *RansomwareDetector) StartMonitoring() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.monitoring {
		return
	}

	d.monitoring = true
	d.stats["is_monitoring"] = true
	now := time.Now()
	d.stats["start_time"] = &now
	d.logger.Info("勒索软件检测器已启动")

	go d.monitorLoop()
}

// StopMonitoring 停止监控
func (d *RansomwareDetector) StopMonitoring() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.monitoring {
		return
	}

	d.monitoring = false
	d.stats["is_monitoring"] = false
	close(d.stopCh)
	d.stopCh = make(chan struct{})
	d.logger.Info("勒索软件检测器已停止")
}

// GetStats 获取统计信息
func (d *RansomwareDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := make(map[string]interface{})
	for k, v := range d.stats {
		stats[k] = v
	}
	stats["rules_count"] = len(d.rules)
	stats["activities_count"] = len(d.activities)
	return stats
}

// ListRules 获取所有规则
func (d *RansomwareDetector) ListRules() []*DetectionRule {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rules := make([]*DetectionRule, len(d.rules))
	copy(rules, d.rules)
	return rules
}

// AddRule 添加检测规则
func (d *RansomwareDetector) AddRule(rule *DetectionRule) error {
	if rule == nil {
		return fmt.Errorf("规则不能为空")
	}
	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}
	if rule.Type == "" {
		return fmt.Errorf("规则类型不能为空")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// 检查重复
	for _, r := range d.rules {
		if r.ID == rule.ID {
			return fmt.Errorf("规则 %s 已存在", rule.ID)
		}
	}

	d.rules = append(d.rules, rule)
	d.logger.Info("添加检测规则", "id", rule.ID, "name", rule.Name)
	return nil
}

// GetActivities 获取活动记录
func (d *RansomwareDetector) GetActivities(limit int) []*Activity {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.activities) {
		limit = len(d.activities)
	}

	// 返回最新的记录
	start := len(d.activities) - limit
	if start < 0 {
		start = 0
	}

	activities := make([]*Activity, limit)
	copy(activities, d.activities[start:])
	return activities
}

// addActivity 添加活动记录
func (d *RansomwareDetector) addActivity(actType, path, details, severity, action string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	act := &Activity{
		ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Type:      actType,
		Path:      path,
		Details:   details,
		Severity:  severity,
		Action:    action,
	}
	d.activities = append(d.activities, act)

	// 保留最近1000条
	if len(d.activities) > 1000 {
		d.activities = d.activities[len(d.activities)-1000:]
	}

	// 更新统计
	if v, ok := d.stats["total_events"].(int64); ok {
		d.stats["total_events"] = v + 1
	}
	if severity == "high" || severity == "critical" {
		if v, ok := d.stats["threats_detected"].(int64); ok {
			d.stats["threats_detected"] = v + 1
		}
	}
	now := time.Now()
	d.stats["last_event"] = &now
}

// monitorLoop 监控循环
func (d *RansomwareDetector) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.checkThresholds()
		}
	}
}

// checkThresholds 检查阈值
func (d *RansomwareDetector) checkThresholds() {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 这里会集成实际的文件系统监控
	// 目前作为框架实现，后续可通过 fsnotify 扩展
}

// IsMonitoring 是否正在监控
func (d *RansomwareDetector) IsMonitoring() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.monitoring
}
