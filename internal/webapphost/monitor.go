package webapphost

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Monitor 资源监控器.
type Monitor struct {
	mu           sync.RWMutex
	metrics      map[string]*AppMetrics
	alerts       map[string]*AlertRule
	alertHistory []AlertEvent
	manager      *WebAppManager
	config       *MonitorConfig
	stopCh       chan struct{}
}

// MonitorConfig 监控配置.
type MonitorConfig struct {
	Enabled          bool          `json:"enabled"`
	Interval         time.Duration `json:"interval"`
	MetricsRetention time.Duration `json:"metrics_retention"`
	AlertRetention   time.Duration `json:"alert_retention"`
}

// AlertEvent 告警事件.
type AlertEvent struct {
	ID        string    `json:"id"`
	AlertID   string    `json:"alert_id"`
	AppID     string    `json:"app_id"`
	Type      string    `json:"type"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMonitor 创建监控器.
func NewMonitor(manager *WebAppManager, config *MonitorConfig) *Monitor {
	if config == nil {
		config = &MonitorConfig{
			Enabled:          true,
			Interval:         30 * time.Second,
			MetricsRetention: 24 * time.Hour,
			AlertRetention:   7 * 24 * time.Hour,
		}
	}

	return &Monitor{
		metrics:      make(map[string]*AppMetrics),
		alerts:       make(map[string]*AlertRule),
		alertHistory: make([]AlertEvent, 0),
		manager:      manager,
		config:       config,
		stopCh:       make(chan struct{}),
	}
}

// Start 启动监控.
func (m *Monitor) Start() {
	if !m.config.Enabled {
		log.Printf("Monitor is disabled")
		return
	}

	log.Printf("Starting monitor with interval: %s", m.config.Interval)

	go m.monitorLoop()
}

// Stop 停止监控.
func (m *Monitor) Stop() {
	log.Printf("Stopping monitor")
	close(m.stopCh)
}

// monitorLoop 监控循环.
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectMetrics()
			m.checkAlerts()
		}
	}
}

// collectMetrics 采集指标.
func (m *Monitor) collectMetrics() {
	apps := m.manager.ListApps(nil)

	for _, app := range apps {
		if app.Status != "running" {
			continue
		}

		metrics := m.collectAppMetrics(app)
		m.mu.Lock()
		m.metrics[app.ID] = metrics
		m.mu.Unlock()
	}
}

// collectAppMetrics 采集单个应用的指标.
func (m *Monitor) collectAppMetrics(app *WebApp) *AppMetrics {
	// 模拟指标采集
	// 实际实现需要调用 Docker API 或系统 API

	var uptime int64
	if app.StartedAt != nil {
		uptime = int64(time.Since(*app.StartedAt).Seconds())
	}

	return &AppMetrics{
		AppID:        app.ID,
		CPUUsage:     15.5,              // 模拟 CPU 使用率
		MemoryUsage:  128 * 1024 * 1024, // 128MB
		MemoryLimit:  int64(app.Resources.MemoryMB) * 1024 * 1024,
		DiskUsage:    512 * 1024 * 1024, // 512MB
		NetworkRx:    1024 * 1024,       // 1MB
		NetworkTx:    512 * 1024,        // 512KB
		Uptime:       uptime,
		RequestCount: 1000,
		ErrorCount:   5,
		AvgResponse:  50.5,
		Timestamp:    time.Now(),
	}
}

// GetMetrics 获取应用指标.
func (m *Monitor) GetMetrics(appID string) (*AppMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[appID]
	if !exists {
		return nil, fmt.Errorf("metrics not found for app: %s", appID)
	}
	return metrics, nil
}

// GetAllMetrics 获取所有应用指标.
func (m *Monitor) GetAllMetrics() map[string]*AppMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*AppMetrics, len(m.metrics))
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// AddAlertRule 添加告警规则.
func (m *Monitor) AddAlertRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = GenerateID("alert")
	}

	if rule.AppID == "" {
		return fmt.Errorf("app ID is required")
	}

	if rule.Type == "" {
		return fmt.Errorf("alert type is required")
	}

	if rule.Threshold <= 0 {
		return fmt.Errorf("threshold must be positive")
	}

	rule.CreatedAt = time.Now()
	rule.Enabled = true
	m.alerts[rule.ID] = rule

	log.Printf("Alert rule added: %s for app %s", rule.ID, rule.AppID)
	return nil
}

// RemoveAlertRule 移除告警规则.
func (m *Monitor) RemoveAlertRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alerts[id]; !exists {
		return fmt.Errorf("alert rule not found: %s", id)
	}

	delete(m.alerts, id)
	log.Printf("Alert rule removed: %s", id)
	return nil
}

// UpdateAlertRule 更新告警规则.
func (m *Monitor) UpdateAlertRule(id string, updates *AlertRule) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.alerts[id]
	if !exists {
		return nil, fmt.Errorf("alert rule not found: %s", id)
	}

	if updates.Name != "" {
		rule.Name = updates.Name
	}
	if updates.Threshold > 0 {
		rule.Threshold = updates.Threshold
	}
	if updates.Duration > 0 {
		rule.Duration = updates.Duration
	}
	if updates.Enabled != rule.Enabled {
		rule.Enabled = updates.Enabled
	}
	if updates.Notify != nil {
		rule.Notify = updates.Notify
	}

	return rule, nil
}

// GetAlertRule 获取告警规则.
func (m *Monitor) GetAlertRule(id string) (*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.alerts[id]
	if !exists {
		return nil, fmt.Errorf("alert rule not found: %s", id)
	}
	return rule, nil
}

// ListAlertRules 列出告警规则.
func (m *Monitor) ListAlertRules(appID string) []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AlertRule, 0)
	for _, rule := range m.alerts {
		if appID == "" || rule.AppID == appID {
			rules = append(rules, rule)
		}
	}
	return rules
}

// checkAlerts 检查告警.
func (m *Monitor) checkAlerts() {
	m.mu.RLock()
	alerts := make([]*AlertRule, 0, len(m.alerts))
	for _, rule := range m.alerts {
		if rule.Enabled {
			alerts = append(alerts, rule)
		}
	}
	m.mu.RUnlock()

	for _, rule := range alerts {
		metrics, err := m.GetMetrics(rule.AppID)
		if err != nil {
			continue
		}

		var value float64
		switch rule.Type {
		case "cpu":
			value = metrics.CPUUsage
		case "memory":
			if metrics.MemoryLimit > 0 {
				value = float64(metrics.MemoryUsage) / float64(metrics.MemoryLimit) * 100
			}
		case "disk":
			value = float64(metrics.DiskUsage) / (1024 * 1024 * 1024) // GB
		case "error_rate":
			if metrics.RequestCount > 0 {
				value = float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
			}
		case "response_time":
			value = metrics.AvgResponse
		default:
			continue
		}

		if value > rule.Threshold {
			m.triggerAlert(rule, value)
		}
	}
}

// triggerAlert 触发告警.
func (m *Monitor) triggerAlert(rule *AlertRule, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已触发过（避免重复告警）
	if rule.LastTrigger != nil && time.Since(*rule.LastTrigger) < rule.Duration {
		return
	}

	now := time.Now()
	rule.LastTrigger = &now

	event := AlertEvent{
		ID:        GenerateID("event"),
		AlertID:   rule.ID,
		AppID:     rule.AppID,
		Type:      rule.Type,
		Value:     value,
		Threshold: rule.Threshold,
		Message:   fmt.Sprintf("Alert: %s for app %s (%s: %.2f > %.2f)", rule.Name, rule.AppID, rule.Type, value, rule.Threshold),
		Timestamp: now,
	}

	m.alertHistory = append(m.alertHistory, event)

	log.Printf("Alert triggered: %s", event.Message)

	// 发送通知
	go m.sendNotifications(rule, event)
}

// sendNotifications 发送告警通知.
func (m *Monitor) sendNotifications(rule *AlertRule, event AlertEvent) {
	for _, channel := range rule.Notify {
		switch channel {
		case "email":
			log.Printf("Sending email alert: %s", event.Message)
		case "webhook":
			log.Printf("Sending webhook alert: %s", event.Message)
		default:
			log.Printf("Unknown notification channel: %s", channel)
		}
	}
}

// GetAlertHistory 获取告警历史.
func (m *Monitor) GetAlertHistory(appID string, limit int) []AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]AlertEvent, 0)
	for i := len(m.alertHistory) - 1; i >= 0; i-- {
		event := m.alertHistory[i]
		if appID == "" || event.AppID == appID {
			events = append(events, event)
			if limit > 0 && len(events) >= limit {
				break
			}
		}
	}
	return events
}

// ClearAlertHistory 清除告警历史.
func (m *Monitor) ClearAlertHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHistory = make([]AlertEvent, 0)
}

// GetMonitorStats 获取监控统计.
func (m *Monitor) GetMonitorStats() *MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MonitorStats{
		TotalApps:    m.manager.GetAppCount(),
		RunningApps:  m.manager.GetRunningAppCount(),
		AlertRules:   len(m.alerts),
		AlertHistory: len(m.alertHistory),
	}

	// 计算资源使用汇总
	for _, metrics := range m.metrics {
		stats.TotalCPUUsage += metrics.CPUUsage
		stats.TotalMemoryUsage += metrics.MemoryUsage
		stats.TotalDiskUsage += metrics.DiskUsage
		stats.TotalRequests += metrics.RequestCount
		stats.TotalErrors += metrics.ErrorCount
	}

	if stats.TotalRequests > 0 {
		stats.ErrorRate = float64(stats.TotalErrors) / float64(stats.TotalRequests) * 100
	}

	return stats
}

// MonitorStats 监控统计.
type MonitorStats struct {
	TotalApps        int     `json:"total_apps"`
	RunningApps      int     `json:"running_apps"`
	AlertRules       int     `json:"alert_rules"`
	AlertHistory     int     `json:"alert_history"`
	TotalCPUUsage    float64 `json:"total_cpu_usage"`
	TotalMemoryUsage int64   `json:"total_memory_usage"`
	TotalDiskUsage   int64   `json:"total_disk_usage"`
	TotalRequests    int64   `json:"total_requests"`
	TotalErrors      int64   `json:"total_errors"`
	ErrorRate        float64 `json:"error_rate"`
}
