package performance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert 告警
type Alert struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Level          AlertLevel             `json:"level"`
	Type           string                 `json:"type"`
	Message        string                 `json:"message"`
	Source         string                 `json:"source,omitempty"`
	CurrentValue   interface{}            `json:"current_value"`
	Threshold      interface{}            `json:"threshold"`
	Labels         map[string]string      `json:"labels,omitempty"`
	Annotations    map[string]string      `json:"annotations,omitempty"`
	Status         string                 `json:"status"` // firing, resolved
	StartsAt       time.Time              `json:"starts_at"`
	EndsAt         *time.Time             `json:"ends_at,omitempty"`
	Acknowledged   bool                   `json:"acknowledged"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	Silenced       bool                   `json:"silenced"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`      // cpu, memory, disk, disk_io, network, service, performance
	Metric      string            `json:"metric"`    // 指标名称
	Operator    string            `json:"operator"`  // >, <, >=, <=, ==, !=
	Threshold   float64           `json:"threshold"` // 阈值
	Level       AlertLevel        `json:"level"`     // warning, critical
	Duration    time.Duration     `json:"duration"`  // 持续时间
	Enabled     bool              `json:"enabled"`   // 是否启用
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// 告警抑制
	InhibitRules []string `json:"inhibit_rules,omitempty"`

	// 内部状态
	pendingSince time.Time
	fireCount    int
}

// AlertManager 告警管理器
type AlertManager struct {
	logger *zap.Logger
	mu     sync.RWMutex

	rules   []*AlertRule
	alerts  []*Alert
	history []*Alert

	// 配置
	maxAlerts  int
	maxHistory int

	// 回调
	onAlert   func(alert *Alert)
	onResolve func(alert *Alert)

	// 收集器引用
	collector *SystemCollector
	storage   *StorageCollector
	health    *HealthChecker
}

// NewAlertManager 创建告警管理器
func NewAlertManager(
	logger *zap.Logger,
	collector *SystemCollector,
	storage *StorageCollector,
	health *HealthChecker,
) *AlertManager {
	am := &AlertManager{
		logger:     logger,
		rules:      make([]*AlertRule, 0),
		alerts:     make([]*Alert, 0),
		history:    make([]*Alert, 0),
		maxAlerts:  100,
		maxHistory: 1000,
		collector:  collector,
		storage:    storage,
		health:     health,
	}

	// 添加默认规则
	am.addDefaultRules()

	return am
}

// addDefaultRules 添加默认告警规则
func (am *AlertManager) addDefaultRules() {
	am.rules = []*AlertRule{
		// CPU 告警
		{
			ID:        "cpu-warning",
			Name:      "CPU使用率警告",
			Type:      "cpu",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 80,
			Level:     AlertLevelWarning,
			Duration:  5 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "cpu-critical",
			Name:      "CPU使用率严重",
			Type:      "cpu",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 95,
			Level:     AlertLevelCritical,
			Duration:  1 * time.Minute,
			Enabled:   true,
		},

		// 内存告警
		{
			ID:        "memory-warning",
			Name:      "内存使用率警告",
			Type:      "memory",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 85,
			Level:     AlertLevelWarning,
			Duration:  5 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "memory-critical",
			Name:      "内存使用率严重",
			Type:      "memory",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 95,
			Level:     AlertLevelCritical,
			Duration:  1 * time.Minute,
			Enabled:   true,
		},

		// 磁盘告警
		{
			ID:        "disk-warning",
			Name:      "磁盘使用率警告",
			Type:      "disk",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 85,
			Level:     AlertLevelWarning,
			Duration:  0, // 立即告警
			Enabled:   true,
		},
		{
			ID:        "disk-critical",
			Name:      "磁盘使用率严重",
			Type:      "disk",
			Metric:    "usage_percent",
			Operator:  ">",
			Threshold: 95,
			Level:     AlertLevelCritical,
			Duration:  0,
			Enabled:   true,
		},

		// 磁盘 I/O 延迟告警
		{
			ID:        "disk-latency-warning",
			Name:      "磁盘延迟警告",
			Type:      "disk_io",
			Metric:    "latency_ms",
			Operator:  ">",
			Threshold: 50,
			Level:     AlertLevelWarning,
			Duration:  3 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "disk-latency-critical",
			Name:      "磁盘延迟严重",
			Type:      "disk_io",
			Metric:    "latency_ms",
			Operator:  ">",
			Threshold: 100,
			Level:     AlertLevelCritical,
			Duration:  1 * time.Minute,
			Enabled:   true,
		},

		// 服务告警
		{
			ID:        "service-down",
			Name:      "服务异常",
			Type:      "service",
			Metric:    "status",
			Operator:  "==",
			Threshold: 0, // 0 = not running
			Level:     AlertLevelCritical,
			Duration:  0,
			Enabled:   true,
		},

		// 性能告警
		{
			ID:        "performance-slow",
			Name:      "性能下降",
			Type:      "performance",
			Metric:    "response_time_ms",
			Operator:  ">",
			Threshold: 500,
			Level:     AlertLevelWarning,
			Duration:  5 * time.Minute,
			Enabled:   true,
		},
	}
}

// SetCallbacks 设置回调
func (am *AlertManager) SetCallbacks(onAlert, onResolve func(alert *Alert)) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.onAlert = onAlert
	am.onResolve = onResolve
}

// Start 启动告警检查
func (am *AlertManager) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				am.Check()
			}
		}
	}()
}

// Check 执行告警检查
func (am *AlertManager) Check() {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 收集当前指标
	cpuMetrics := am.collector.collectCPU()
	memMetrics := am.collector.collectMemory()
	diskMetrics := am.collector.collectDisks()
	diskIOMetrics := am.collector.collectDiskIO()

	// 检查每条规则
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		var value float64
		var source string

		switch rule.Type {
		case "cpu":
			value = cpuMetrics.UsagePercent
			source = "system"
		case "memory":
			value = memMetrics.UsagePercent
			source = "system"
		case "disk":
			// 检查所有磁盘
			for _, d := range diskMetrics {
				if am.evaluateRule(rule, d.UsagePercent) {
					am.processAlert(rule, d.UsagePercent, d.Device)
				}
			}
			continue
		case "disk_io":
			for _, d := range diskIOMetrics {
				avgLatency := (d.ReadLatency + d.WriteLatency) / 2
				if am.evaluateRule(rule, avgLatency) {
					am.processAlert(rule, avgLatency, d.Device)
				}
			}
			continue
		case "service":
			// 服务检查
			health := am.health.GetHealth()
			for _, check := range health.Checks {
				if check.Name == "services" && check.Status != "healthy" {
					am.processAlert(rule, 0, "services")
				}
			}
			continue
		default:
			continue
		}

		// 评估规则
		if am.evaluateRule(rule, value) {
			am.processAlert(rule, value, source)
		} else {
			// 检查是否需要恢复告警
			am.checkResolve(rule, source)
		}
	}
}

// evaluateRule 评估规则
func (am *AlertManager) evaluateRule(rule *AlertRule, value float64) bool {
	switch rule.Operator {
	case ">":
		return value > rule.Threshold
	case ">=":
		return value >= rule.Threshold
	case "<":
		return value < rule.Threshold
	case "<=":
		return value <= rule.Threshold
	case "==":
		return value == rule.Threshold
	case "!=":
		return value != rule.Threshold
	}
	return false
}

// processAlert 处理告警
func (am *AlertManager) processAlert(rule *AlertRule, value float64, source string) {
	// 查找是否已存在相同的告警
	for _, alert := range am.alerts {
		if alert.Name == rule.Name && alert.Source == source && alert.Status == "firing" {
			// 更新现有告警
			alert.CurrentValue = value
			return
		}
	}

	// 检查持续时间
	if rule.Duration > 0 {
		if rule.pendingSince.IsZero() {
			rule.pendingSince = time.Now()
			rule.fireCount++
			return
		}

		if time.Since(rule.pendingSince) < rule.Duration {
			rule.fireCount++
			return
		}
	}

	// 创建新告警
	alert := &Alert{
		ID:           generateAlertID(),
		Name:         rule.Name,
		Level:        rule.Level,
		Type:         rule.Type,
		Message:      fmt.Sprintf("%s: %.2f (阈值: %.2f)", rule.Name, value, rule.Threshold),
		Source:       source,
		CurrentValue: value,
		Threshold:    rule.Threshold,
		Labels:       rule.Labels,
		Annotations:  rule.Annotations,
		Status:       "firing",
		StartsAt:     time.Now(),
	}

	am.alerts = append(am.alerts, alert)

	// 限制告警数量
	if len(am.alerts) > am.maxAlerts {
		am.alerts = am.alerts[1:]
	}

	// 调用回调
	if am.onAlert != nil {
		go am.onAlert(alert)
	}

	am.logger.Warn("告警触发",
		zap.String("name", rule.Name),
		zap.String("level", string(rule.Level)),
		zap.Float64("value", value),
		zap.Float64("threshold", rule.Threshold),
	)

	// 重置规则状态
	rule.pendingSince = time.Time{}
	rule.fireCount = 0
}

// checkResolve 检查告警是否恢复
func (am *AlertManager) checkResolve(rule *AlertRule, source string) {
	for i, alert := range am.alerts {
		if alert.Name == rule.Name && alert.Source == source && alert.Status == "firing" {
			// 标记为已恢复
			now := time.Now()
			alert.Status = "resolved"
			alert.EndsAt = &now

			// 移动到历史
			am.history = append(am.history, alert)
			am.alerts = append(am.alerts[:i], am.alerts[i+1:]...)

			// 调用回调
			if am.onResolve != nil {
				go am.onResolve(alert)
			}

			am.logger.Info("告警恢复",
				zap.String("name", rule.Name),
				zap.String("source", source),
			)
			break
		}
	}
}

// GetAlerts 获取活动告警
func (am *AlertManager) GetAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, len(am.alerts))
	copy(result, am.alerts)
	return result
}

// GetAlertHistory 获取告警历史
func (am *AlertManager) GetAlertHistory(limit int) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.history) {
		limit = len(am.history)
	}

	start := len(am.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Alert, limit)
	copy(result, am.history[start:])
	return result
}

// AcknowledgeAlert 确认告警
func (am *AlertManager) AcknowledgeAlert(id, by string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			alert.AcknowledgedBy = by
			return nil
		}
	}

	return fmt.Errorf("告警不存在: %s", id)
}

// SilenceAlert 静默告警
func (am *AlertManager) SilenceAlert(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.Silenced = true
			return nil
		}
	}

	return fmt.Errorf("告警不存在: %s", id)
}

// ClearAlert 清除告警
func (am *AlertManager) ClearAlert(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, alert := range am.alerts {
		if alert.ID == id {
			am.alerts = append(am.alerts[:i], am.alerts[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("告警不存在: %s", id)
}

// AddRule 添加规则
func (am *AlertManager) AddRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// UpdateRule 更新规则
func (am *AlertManager) UpdateRule(id string, updates map[string]interface{}) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, rule := range am.rules {
		if rule.ID == id {
			if v, ok := updates["threshold"].(float64); ok {
				rule.Threshold = v
			}
			if v, ok := updates["level"].(AlertLevel); ok {
				rule.Level = v
			}
			if v, ok := updates["enabled"].(bool); ok {
				rule.Enabled = v
			}
			if v, ok := updates["duration"].(time.Duration); ok {
				rule.Duration = v
			}
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", id)
}

// GetRules 获取所有规则
func (am *AlertManager) GetRules() []*AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*AlertRule, len(am.rules))
	copy(result, am.rules)
	return result
}

// GetAlertStats 获取告警统计
func (am *AlertManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := map[string]interface{}{
		"total_active":  len(am.alerts),
		"total_history": len(am.history),
		"by_level":      make(map[AlertLevel]int),
		"by_type":       make(map[string]int),
	}

	levelStats, ok := stats["by_level"].(map[AlertLevel]int)
	if !ok {
		levelStats = make(map[AlertLevel]int)
		stats["by_level"] = levelStats
	}
	typeStats, ok := stats["by_type"].(map[string]int)
	if !ok {
		typeStats = make(map[string]int)
		stats["by_type"] = typeStats
	}

	for _, alert := range am.alerts {
		levelStats[alert.Level]++
		typeStats[alert.Type]++
	}

	return stats
}

func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}
