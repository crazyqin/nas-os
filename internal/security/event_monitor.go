// Package security provides security event monitoring and analysis
// Version: 2.40.0 - Enhanced Security Audit Module
package security

import (
	"context"
	"sync"
	"time"
)

// ========== 安全事件监控器 ==========

// EventMonitor 安全事件监控器
type EventMonitor struct {
	config          MonitorConfig
	eventBuffer     []*AuditLogEntry
	anomalyDetector *AnomalyDetector
	alertChan       chan *SecurityAlert
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// MonitorConfig 监控器配置
type MonitorConfig struct {
	Enabled              bool          `json:"enabled"`
	BufferSize           int           `json:"buffer_size"`            // 事件缓冲区大小
	AnalysisInterval     time.Duration `json:"analysis_interval"`      // 分析间隔
	AlertCooldown        time.Duration `json:"alert_cooldown"`         // 相同告警冷却时间
	MaxEventsPerMinute   int           `json:"max_events_per_minute"`  // 每分钟最大事件数阈值
	FailedLoginThreshold int           `json:"failed_login_threshold"` // 失败登录阈值
	BruteForceWindow     time.Duration `json:"brute_force_window"`     // 暴力破解检测窗口
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Enabled:              true,
		BufferSize:           1000,
		AnalysisInterval:     time.Minute,
		AlertCooldown:        time.Hour,
		MaxEventsPerMinute:   100,
		FailedLoginThreshold: 5,
		BruteForceWindow:     5 * time.Minute,
	}
}

// NewEventMonitor 创建安全事件监控器
func NewEventMonitor(config MonitorConfig) *EventMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventMonitor{
		config:          config,
		eventBuffer:     make([]*AuditLogEntry, 0, config.BufferSize),
		anomalyDetector: NewAnomalyDetector(),
		alertChan:       make(chan *SecurityAlert, 100),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// ProcessEvent 处理安全事件
func (m *EventMonitor) ProcessEvent(entry *AuditLogEntry) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加到缓冲区
	m.eventBuffer = append(m.eventBuffer, entry)

	// 限制缓冲区大小
	if len(m.eventBuffer) > m.config.BufferSize {
		m.eventBuffer = m.eventBuffer[len(m.eventBuffer)-m.config.BufferSize:]
	}

	// 实时检测异常
	m.detectRealTimeAnomaly(entry)
}

// detectRealTimeAnomaly 实时异常检测
func (m *EventMonitor) detectRealTimeAnomaly(entry *AuditLogEntry) {
	// 检测暴力破解
	if entry.Event == "login_failure" {
		m.detectBruteForce(entry)
	}

	// 检测权限提升
	if entry.Event == "permission_change" || entry.Event == "role_change" {
		m.detectPrivilegeEscalation(entry)
	}

	// 检测异常访问
	if entry.Event == "unauthorized_access" {
		m.generateAlert("high", "unauthorized_access", "检测到未授权访问", entry)
	}

	// 检测配置变更
	if entry.Category == "config" && entry.Status == "success" {
		m.detectSuspiciousConfigChange(entry)
	}
}

// detectBruteForce 检测暴力破解
func (m *EventMonitor) detectBruteForce(entry *AuditLogEntry) {
	now := time.Now()
	window := m.config.BruteForceWindow

	var failures int
	for _, e := range m.eventBuffer {
		if e.Event == "login_failure" &&
			e.IP == entry.IP &&
			e.Timestamp.After(now.Add(-window)) {
			failures++
		}
	}

	if failures >= m.config.FailedLoginThreshold {
		m.generateAlert("high", "brute_force_attempt",
			"检测到暴力破解尝试", entry)
	}
}

// detectPrivilegeEscalation 检测权限提升
func (m *EventMonitor) detectPrivilegeEscalation(entry *AuditLogEntry) {
	// 检查是否在非工作时间
	hour := entry.Timestamp.Hour()
	if hour < 6 || hour > 22 {
		m.generateAlert("medium", "suspicious_privilege_change",
			"非工作时间的权限变更", entry)
	}
}

// detectSuspiciousConfigChange 检测可疑配置变更
func (m *EventMonitor) detectSuspiciousConfigChange(entry *AuditLogEntry) {
	// 检测敏感配置变更
	sensitiveResources := map[string]bool{
		"firewall":       true,
		"authentication": true,
		"ssl":            true,
		"backup":         true,
	}

	if sensitiveResources[entry.Resource] {
		m.generateAlert("medium", "sensitive_config_change",
			"敏感配置已变更", entry)
	}
}

// generateAlert 生成告警
func (m *EventMonitor) generateAlert(severity, alertType, message string, entry *AuditLogEntry) {
	alert := &SecurityAlert{
		ID:          generateAlertID(),
		Timestamp:   time.Now(),
		Severity:    severity,
		Type:        alertType,
		Title:       message,
		Description: entry.Message(),
		SourceIP:    entry.IP,
		Username:    entry.Username,
		Details: map[string]interface{}{
			"event_id":   entry.ID,
			"event_type": entry.Event,
			"category":   entry.Category,
		},
		Acknowledged: false,
	}

	select {
	case m.alertChan <- alert:
	default:
		// 告警通道已满，丢弃
	}
}

// GetAlertChannel 获取告警通道
func (m *EventMonitor) GetAlertChannel() <-chan *SecurityAlert {
	return m.alertChan
}

// Start 启动监控器
func (m *EventMonitor) Start() {
	go m.runAnalysisLoop()
}

// Stop 停止监控器
func (m *EventMonitor) Stop() {
	m.cancel()
}

// runAnalysisLoop 运行分析循环
func (m *EventMonitor) runAnalysisLoop() {
	ticker := time.NewTicker(m.config.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performPeriodicAnalysis()
		case <-m.ctx.Done():
			return
		}
	}
}

// performPeriodicAnalysis 执行周期性分析
func (m *EventMonitor) performPeriodicAnalysis() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.eventBuffer) == 0 {
		return
	}

	// 使用异常检测器分析
	anomalies := m.anomalyDetector.Analyze(m.eventBuffer)
	for _, anomaly := range anomalies {
		m.generateAlert(anomaly.Severity, anomaly.Type, anomaly.Message, nil)
	}
}

// GetEventStats 获取事件统计
func (m *EventMonitor) GetEventStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	eventsByType := make(map[string]int)
	eventsByIP := make(map[string]int)

	stats := map[string]interface{}{
		"buffer_size":    len(m.eventBuffer),
		"events_by_type": eventsByType,
		"events_by_ip":   eventsByIP,
	}

	for _, entry := range m.eventBuffer {
		eventsByType[entry.Event]++
		if entry.IP != "" {
			eventsByIP[entry.IP]++
		}
	}

	return stats
}

// ========== 异常检测器 ==========

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	thresholds AnomalyThresholds
}

// AnomalyThresholds 异常阈值
type AnomalyThresholds struct {
	HighEventRate      float64 `json:"high_event_rate"`      // 高事件率阈值 (事件/秒)
	UniqueIPThreshold  int     `json:"unique_ip_threshold"`  // 唯一IP阈值
	ErrorRateThreshold float64 `json:"error_rate_threshold"` // 错误率阈值
}

// AnomalyResult 异常检测结果
type AnomalyResult struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		thresholds: AnomalyThresholds{
			HighEventRate:      10.0,
			UniqueIPThreshold:  50,
			ErrorRateThreshold: 0.3,
		},
	}
}

// Analyze 分析事件
func (d *AnomalyDetector) Analyze(events []*AuditLogEntry) []*AnomalyResult {
	var results []*AnomalyResult

	if len(events) == 0 {
		return results
	}

	// 检测高频事件
	if anomaly := d.detectHighFrequency(events); anomaly != nil {
		results = append(results, anomaly)
	}

	// 检测多IP访问
	if anomaly := d.detectMultipleIPs(events); anomaly != nil {
		results = append(results, anomaly)
	}

	// 检测高错误率
	if anomaly := d.detectHighErrorRate(events); anomaly != nil {
		results = append(results, anomaly)
	}

	// 检测异常时间模式
	if anomaly := d.detectAbnormalTimePattern(events); anomaly != nil {
		results = append(results, anomaly)
	}

	return results
}

// detectHighFrequency 检测高频事件
func (d *AnomalyDetector) detectHighFrequency(events []*AuditLogEntry) *AnomalyResult {
	if len(events) < 10 {
		return nil
	}

	// 计算事件率
	duration := events[len(events)-1].Timestamp.Sub(events[0].Timestamp).Seconds()
	if duration <= 0 {
		return nil
	}

	rate := float64(len(events)) / duration
	if rate > d.thresholds.HighEventRate {
		return &AnomalyResult{
			Type:     "high_event_rate",
			Severity: "medium",
			Message:  "检测到异常高频事件",
		}
	}

	return nil
}

// detectMultipleIPs 检测多IP访问
func (d *AnomalyDetector) detectMultipleIPs(events []*AuditLogEntry) *AnomalyResult {
	uniqueIPs := make(map[string]bool)
	for _, entry := range events {
		if entry.IP != "" {
			uniqueIPs[entry.IP] = true
		}
	}

	if len(uniqueIPs) > d.thresholds.UniqueIPThreshold {
		return &AnomalyResult{
			Type:     "multiple_ip_access",
			Severity: "medium",
			Message:  "检测到大量不同IP访问",
		}
	}

	return nil
}

// detectHighErrorRate 检测高错误率
func (d *AnomalyDetector) detectHighErrorRate(events []*AuditLogEntry) *AnomalyResult {
	if len(events) < 5 {
		return nil
	}

	var errors int
	for _, entry := range events {
		if entry.Status == "failure" || entry.Level == "error" {
			errors++
		}
	}

	errorRate := float64(errors) / float64(len(events))
	if errorRate > d.thresholds.ErrorRateThreshold {
		return &AnomalyResult{
			Type:     "high_error_rate",
			Severity: "high",
			Message:  "检测到异常高错误率",
		}
	}

	return nil
}

// detectAbnormalTimePattern 检测异常时间模式
func (d *AnomalyDetector) detectAbnormalTimePattern(events []*AuditLogEntry) *AnomalyResult {
	// 检测深夜活动 (0:00-5:00)
	var lateNightEvents int
	for _, entry := range events {
		hour := entry.Timestamp.Hour()
		if hour >= 0 && hour < 5 {
			lateNightEvents++
		}
	}

	// 如果深夜事件超过总事件的30%
	if len(events) > 10 && float64(lateNightEvents)/float64(len(events)) > 0.3 {
		return &AnomalyResult{
			Type:     "abnormal_time_pattern",
			Severity: "low",
			Message:  "检测到异常时间模式的活跃",
		}
	}

	return nil
}
