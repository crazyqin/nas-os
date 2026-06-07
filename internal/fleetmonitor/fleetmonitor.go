// Package fleetmonitor 提供集群监控
// 对标群晖 Active Insight，统一监控多台设备
package fleetmonitor

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ========== 集群设备管理 ==========

// ClusterDevice 集群设备
type ClusterDevice struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Hostname        string            `json:"hostname"`
	IPAddress       string            `json:"ip_address"`
	Type            DeviceType        `json:"type"`
	Model           string            `json:"model"`
	SerialNumber    string            `json:"serial_number"`
	OSVersion       string            `json:"os_version"`
	FirmwareVersion string            `json:"firmware_version"`
	Location        DeviceLocation    `json:"location"`
	Resources       DeviceResources   `json:"resources"`
	Status          DeviceStatus      `json:"status"`
	Health          HealthStatus      `json:"health"`
	Tags            []string          `json:"tags"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	LastSeen        time.Time         `json:"last_seen"`
	RegisteredAt    time.Time         `json:"registered_at"`
}

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeNAS     DeviceType = "nas"
	DeviceTypeServer  DeviceType = "server"
	DeviceTypeSwitch  DeviceType = "switch"
	DeviceTypeUPS     DeviceType = "ups"
	DeviceTypeRouter  DeviceType = "router"
	DeviceTypeStorage DeviceType = "storage"
)

// DeviceLocation 设备位置
type DeviceLocation struct {
	Rack    string `json:"rack"`
	Row     string `json:"row"`
	Unit    int    `json:"unit"`
	Site    string `json:"site"`
	City    string `json:"city"`
	Country string `json:"country"`
}

// DeviceResources 设备资源
type DeviceResources struct {
	CPU         CPUInfo         `json:"cpu"`
	Memory      MemoryInfo      `json:"memory"`
	Storage     []StorageInfo   `json:"storage"`
	Network     []NetworkInfo   `json:"network"`
	Temperature TemperatureInfo `json:"temperature"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model        string  `json:"model"`
	Cores        int     `json:"cores"`
	Threads      int     `json:"threads"`
	FrequencyGHz float64 `json:"frequency_ghz"`
	UsagePercent float64 `json:"usage_percent"`
	LoadAvg1     float64 `json:"load_avg_1"`
	LoadAvg5     float64 `json:"load_avg_5"`
	LoadAvg15    float64 `json:"load_avg_15"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
	SwapTotalGB  float64 `json:"swap_total_gb"`
	SwapUsedGB   float64 `json:"swap_used_gb"`
}

// StorageInfo 存储信息
type StorageInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
	Health       string  `json:"health"`
	Temperature  float64 `json:"temperature"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	Name      string `json:"name"`
	SpeedMbps int    `json:"speed_mbps"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	BytesSent int64  `json:"bytes_sent"`
	BytesRecv int64  `json:"bytes_recv"`
	Errors    int64  `json:"errors"`
	Up        bool   `json:"up"`
}

// TemperatureInfo 温度信息
type TemperatureInfo struct {
	CPU    float64 `json:"cpu"`
	System float64 `json:"system"`
	Disk   float64 `json:"disk"`
	Max    float64 `json:"max"`
}

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline      DeviceStatus = "online"
	DeviceStatusOffline     DeviceStatus = "offline"
	DeviceStatusDegraded    DeviceStatus = "degraded"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusWarning  HealthStatus = "warning"
	HealthStatusCritical HealthStatus = "critical"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// ========== 监控指标 ==========

// MetricPoint 指标点
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	DeviceID  string            `json:"device_id"`
	Metric    string            `json:"metric"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// MetricSeries 指标序列
type MetricSeries struct {
	DeviceID string        `json:"device_id"`
	Metric   string        `json:"metric"`
	Unit     string        `json:"unit"`
	Points   []MetricPoint `json:"points"`
}

// ========== 告警管理 ==========

// Alert 告警
type Alert struct {
	ID             string            `json:"id"`
	DeviceID       string            `json:"device_id"`
	DeviceName     string            `json:"device_name"`
	RuleID         string            `json:"rule_id"`
	RuleName       string            `json:"rule_name"`
	Level          AlertLevel        `json:"level"`
	Category       AlertCategory     `json:"category"`
	Title          string            `json:"title"`
	Message        string            `json:"message"`
	Value          float64           `json:"value"`
	Threshold      float64           `json:"threshold"`
	Status         AlertStatus       `json:"status"`
	AcknowledgedBy string            `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo      AlertLevel = "info"
	AlertLevelWarning   AlertLevel = "warning"
	AlertLevelCritical  AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// AlertCategory 告警分类
type AlertCategory string

const (
	AlertCategoryCPU         AlertCategory = "cpu"
	AlertCategoryMemory      AlertCategory = "memory"
	AlertCategoryStorage     AlertCategory = "storage"
	AlertCategoryNetwork     AlertCategory = "network"
	AlertCategoryTemperature AlertCategory = "temperature"
	AlertCategoryService     AlertCategory = "service"
	AlertCategorySecurity    AlertCategory = "security"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusSuppressed   AlertStatus = "suppressed"
)

// AlertRule 告警规则
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    AlertCategory `json:"category"`
	Metric      string        `json:"metric"`
	Condition   string        `json:"condition"` // gt, lt, eq, gte, lte
	Threshold   float64       `json:"threshold"`
	Duration    int           `json:"duration"` // 秒
	Level       AlertLevel    `json:"level"`
	Enabled     bool          `json:"enabled"`
	Actions     []AlertAction `json:"actions"`
	CreatedAt   time.Time     `json:"created_at"`
}

// AlertAction 告警动作
type AlertAction struct {
	Type    string `json:"type"` // email, webhook, sms, script
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

// ========== 集群监控管理器 ==========

// FleetMonitor 集群监控管理器
type FleetMonitor struct {
	mu      sync.RWMutex
	devices map[string]*ClusterDevice
	metrics map[string][]MetricPoint
	alerts  map[string]*Alert
	rules   map[string]*AlertRule
	config  MonitorConfig
	stats   MonitorStats
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	MetricsRetentionDays int    `json:"metrics_retention_days"`
	CollectionInterval   int    `json:"collection_interval"`  // 秒
	AlertCheckInterval   int    `json:"alert_check_interval"` // 秒
	MaxDevices           int    `json:"max_devices"`
	MaxAlerts            int    `json:"max_alerts"`
	MaxMetricsPerDevice  int    `json:"max_metrics_per_device"`
	EnableAutoDiscovery  bool   `json:"enable_auto_discovery"`
	DiscoverySubnet      string `json:"discovery_subnet"`
	EnableNotifications  bool   `json:"enable_notifications"`
	WebhookURL           string `json:"webhook_url"`
	SMTPServer           string `json:"smtp_server"`
	SMTPPort             int    `json:"smtp_port"`
	AlertEmail           string `json:"alert_email"`
}

// MonitorStats 监控统计
type MonitorStats struct {
	TotalDevices   int       `json:"total_devices"`
	OnlineDevices  int       `json:"online_devices"`
	OfflineDevices int       `json:"offline_devices"`
	TotalAlerts    int       `json:"total_alerts"`
	ActiveAlerts   int       `json:"active_alerts"`
	CriticalAlerts int       `json:"critical_alerts"`
	WarningAlerts  int       `json:"warning_alerts"`
	TotalMetrics   int       `json:"total_metrics"`
	HealthScore    float64   `json:"health_score"`
	LastCollection time.Time `json:"last_collection"`
	LastAlertCheck time.Time `json:"last_alert_check"`
}

// NewFleetMonitor 创建集群监控管理器
func NewFleetMonitor(config MonitorConfig) *FleetMonitor {
	// 设置默认值
	if config.MetricsRetentionDays == 0 {
		config.MetricsRetentionDays = 30
	}
	if config.CollectionInterval == 0 {
		config.CollectionInterval = 60
	}
	if config.AlertCheckInterval == 0 {
		config.AlertCheckInterval = 30
	}
	if config.MaxDevices == 0 {
		config.MaxDevices = 100
	}
	if config.MaxAlerts == 0 {
		config.MaxAlerts = 10000
	}
	if config.MaxMetricsPerDevice == 0 {
		config.MaxMetricsPerDevice = 100000
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}

	return &FleetMonitor{
		devices: make(map[string]*ClusterDevice),
		metrics: make(map[string][]MetricPoint),
		alerts:  make(map[string]*Alert),
		rules:   make(map[string]*AlertRule),
		config:  config,
	}
}

// ========== 设备管理 ==========

// RegisterDevice 注册设备
func (m *FleetMonitor) RegisterDevice(device ClusterDevice) (*ClusterDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.devices) >= m.config.MaxDevices {
		return nil, fmt.Errorf("已达到最大设备数: %d", m.config.MaxDevices)
	}

	if device.ID == "" {
		device.ID = fmt.Sprintf("device-%s-%d", device.Hostname, time.Now().UnixNano())
	}

	if _, exists := m.devices[device.ID]; exists {
		return nil, fmt.Errorf("设备已存在: %s", device.ID)
	}

	device.Status = DeviceStatusOnline
	device.Health = HealthStatusHealthy
	device.LastSeen = time.Now()
	device.RegisteredAt = time.Now()

	m.devices[device.ID] = &device
	m.updateStats()

	return &device, nil
}

// UnregisterDevice 注销设备
func (m *FleetMonitor) UnregisterDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	// 删除设备的指标数据
	delete(m.metrics, id)

	// 删除设备相关的告警
	for alertID, alert := range m.alerts {
		if alert.DeviceID == id {
			delete(m.alerts, alertID)
		}
	}

	delete(m.devices, id)
	m.updateStats()

	return nil
}

// GetDevice 获取设备
func (m *FleetMonitor) GetDevice(id string) (*ClusterDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", id)
	}

	return device, nil
}

// ListDevices 列出所有设备
func (m *FleetMonitor) ListDevices() []*ClusterDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ClusterDevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}

	return result
}

// UpdateDeviceStatus 更新设备状态
func (m *FleetMonitor) UpdateDeviceStatus(id string, status DeviceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[id]
	if !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	device.Status = status
	device.LastSeen = time.Now()
	m.updateStats()

	return nil
}

// UpdateDeviceResources 更新设备资源
func (m *FleetMonitor) UpdateDeviceResources(id string, resources DeviceResources) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[id]
	if !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	device.Resources = resources
	device.LastSeen = time.Now()

	// 评估健康状态
	device.Health = m.evaluateHealth(resources)

	return nil
}

// evaluateHealth 评估健康状态
func (m *FleetMonitor) evaluateHealth(resources DeviceResources) HealthStatus {
	// CPU 使用率过高
	if resources.CPU.UsagePercent > 90 {
		return HealthStatusCritical
	}
	if resources.CPU.UsagePercent > 80 {
		return HealthStatusWarning
	}

	// 内存使用率过高
	if resources.Memory.UsagePercent > 95 {
		return HealthStatusCritical
	}
	if resources.Memory.UsagePercent > 85 {
		return HealthStatusWarning
	}

	// 存储使用率过高
	for _, storage := range resources.Storage {
		if storage.UsagePercent > 95 {
			return HealthStatusCritical
		}
		if storage.UsagePercent > 90 {
			return HealthStatusWarning
		}
	}

	// 温度过高
	if resources.Temperature.CPU > 90 {
		return HealthStatusCritical
	}
	if resources.Temperature.CPU > 80 {
		return HealthStatusWarning
	}

	return HealthStatusHealthy
}

// ========== 指标管理 ==========

// RecordMetric 记录指标
func (m *FleetMonitor) RecordMetric(point MetricPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[point.DeviceID]; !exists {
		return fmt.Errorf("设备不存在: %s", point.DeviceID)
	}

	if point.Timestamp.IsZero() {
		point.Timestamp = time.Now()
	}

	key := point.DeviceID
	m.metrics[key] = append(m.metrics[key], point)

	// 限制指标数量
	if len(m.metrics[key]) > m.config.MaxMetricsPerDevice {
		m.metrics[key] = m.metrics[key][len(m.metrics[key])-m.config.MaxMetricsPerDevice:]
	}

	return nil
}

// GetMetrics 获取指标
func (m *FleetMonitor) GetMetrics(deviceID, metric string, start, end time.Time) []MetricPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MetricPoint, 0)
	points, exists := m.metrics[deviceID]
	if !exists {
		return result
	}

	for _, p := range points {
		if metric != "" && p.Metric != metric {
			continue
		}
		if !start.IsZero() && p.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && p.Timestamp.After(end) {
			continue
		}
		result = append(result, p)
	}

	return result
}

// GetLatestMetrics 获取最新指标
func (m *FleetMonitor) GetLatestMetrics(deviceID string) map[string]MetricPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]MetricPoint)
	points, exists := m.metrics[deviceID]
	if !exists {
		return result
	}

	// 获取每个指标的最新值
	for _, p := range points {
		existing, ok := result[p.Metric]
		if !ok || p.Timestamp.After(existing.Timestamp) {
			result[p.Metric] = p
		}
	}

	return result
}

// ========== 告警管理 ==========

// AddAlertRule 添加告警规则
func (m *FleetMonitor) AddAlertRule(rule AlertRule) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%s-%d", rule.Name, time.Now().UnixNano())
	}

	if _, exists := m.rules[rule.ID]; exists {
		return nil, fmt.Errorf("规则已存在: %s", rule.ID)
	}

	rule.Enabled = true
	rule.CreatedAt = time.Now()

	m.rules[rule.ID] = &rule

	return &rule, nil
}

// RemoveAlertRule 移除告警规则
func (m *FleetMonitor) RemoveAlertRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(m.rules, id)

	return nil
}

// ListAlertRules 列出告警规则
func (m *FleetMonitor) ListAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, r)
	}

	return result
}

// CreateAlert 创建告警
func (m *FleetMonitor) CreateAlert(alert Alert) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.alerts) >= m.config.MaxAlerts {
		return nil, fmt.Errorf("已达到最大告警数: %d", m.config.MaxAlerts)
	}

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%s-%s-%d", alert.DeviceID, alert.RuleID, time.Now().UnixNano())
	}

	alert.Status = AlertStatusActive
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()

	m.alerts[alert.ID] = &alert
	m.updateStats()

	return &alert, nil
}

// AcknowledgeAlert 确认告警
func (m *FleetMonitor) AcknowledgeAlert(id, acknowledgedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[id]
	if !exists {
		return fmt.Errorf("告警不存在: %s", id)
	}

	alert.Status = AlertStatusAcknowledged
	alert.AcknowledgedBy = acknowledgedBy
	alert.UpdatedAt = time.Now()

	m.updateStats()

	return nil
}

// ResolveAlert 解决告警
func (m *FleetMonitor) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[id]
	if !exists {
		return fmt.Errorf("告警不存在: %s", id)
	}

	now := time.Now()
	alert.Status = AlertStatusResolved
	alert.ResolvedAt = &now
	alert.UpdatedAt = now

	m.updateStats()

	return nil
}

// ListAlerts 列出告警
func (m *FleetMonitor) ListAlerts(level AlertLevel, status AlertStatus) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, a := range m.alerts {
		if level != "" && a.Level != level {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		result = append(result, a)
	}

	return result
}

// GetAlert 获取告警
func (m *FleetMonitor) GetAlert(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, exists := m.alerts[id]
	if !exists {
		return nil, fmt.Errorf("告警不存在: %s", id)
	}

	return alert, nil
}

// ========== 健康评估 ==========

// GetFleetHealth 获取集群健康状态
func (m *FleetMonitor) GetFleetHealth() *FleetHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := &FleetHealth{
		Timestamp:    time.Now(),
		TotalDevices: len(m.devices),
		Devices:      make(map[string]DeviceHealth),
	}

	var healthyCount, warningCount, criticalCount int

	for id, device := range m.devices {
		deviceHealth := DeviceHealth{
			DeviceID: id,
			Name:     device.Name,
			Status:   device.Status,
			Health:   device.Health,
		}

		switch device.Health {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusWarning:
			warningCount++
		case HealthStatusCritical:
			criticalCount++
		}

		// 计算设备健康评分
		deviceHealth.Score = m.calculateDeviceScore(device)
		health.Devices[id] = deviceHealth
	}

	health.HealthyDevices = healthyCount
	health.WarningDevices = warningCount
	health.CriticalDevices = criticalCount

	// 计算总体健康评分
	if health.TotalDevices > 0 {
		health.OverallScore = float64(healthyCount*100+warningCount*50) / float64(health.TotalDevices*100) * 100
	} else {
		health.OverallScore = 100
	}

	// 确定总体健康状态
	if criticalCount > 0 {
		health.OverallHealth = HealthStatusCritical
	} else if warningCount > 0 {
		health.OverallHealth = HealthStatusWarning
	} else {
		health.OverallHealth = HealthStatusHealthy
	}

	return health
}

// calculateDeviceScore 计算设备健康评分
func (m *FleetMonitor) calculateDeviceScore(device *ClusterDevice) float64 {
	score := 100.0

	// CPU 评分
	if device.Resources.CPU.UsagePercent > 90 {
		score -= 20
	} else if device.Resources.CPU.UsagePercent > 80 {
		score -= 10
	} else if device.Resources.CPU.UsagePercent > 70 {
		score -= 5
	}

	// 内存评分
	if device.Resources.Memory.UsagePercent > 95 {
		score -= 25
	} else if device.Resources.Memory.UsagePercent > 85 {
		score -= 15
	} else if device.Resources.Memory.UsagePercent > 75 {
		score -= 5
	}

	// 存储评分
	for _, storage := range device.Resources.Storage {
		if storage.UsagePercent > 95 {
			score -= 20
		} else if storage.UsagePercent > 90 {
			score -= 10
		} else if storage.UsagePercent > 80 {
			score -= 5
		}
	}

	// 温度评分
	if device.Resources.Temperature.CPU > 90 {
		score -= 20
	} else if device.Resources.Temperature.CPU > 80 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}

	return score
}

// FleetHealth 集群健康状态
type FleetHealth struct {
	Timestamp       time.Time               `json:"timestamp"`
	TotalDevices    int                     `json:"total_devices"`
	HealthyDevices  int                     `json:"healthy_devices"`
	WarningDevices  int                     `json:"warning_devices"`
	CriticalDevices int                     `json:"critical_devices"`
	OverallScore    float64                 `json:"overall_score"`
	OverallHealth   HealthStatus            `json:"overall_health"`
	Devices         map[string]DeviceHealth `json:"devices"`
}

// DeviceHealth 设备健康状态
type DeviceHealth struct {
	DeviceID string       `json:"device_id"`
	Name     string       `json:"name"`
	Status   DeviceStatus `json:"status"`
	Health   HealthStatus `json:"health"`
	Score    float64      `json:"score"`
}

// ========== 辅助方法 ==========

// updateStats 更新统计
func (m *FleetMonitor) updateStats() {
	m.stats.TotalDevices = len(m.devices)
	m.stats.OnlineDevices = 0
	m.stats.OfflineDevices = 0
	m.stats.TotalAlerts = len(m.alerts)
	m.stats.ActiveAlerts = 0
	m.stats.CriticalAlerts = 0
	m.stats.WarningAlerts = 0

	for _, d := range m.devices {
		switch d.Status {
		case DeviceStatusOnline:
			m.stats.OnlineDevices++
		case DeviceStatusOffline:
			m.stats.OfflineDevices++
		}
	}

	for _, a := range m.alerts {
		if a.Status == AlertStatusActive {
			m.stats.ActiveAlerts++
			switch a.Level {
			case AlertLevelCritical, AlertLevelEmergency:
				m.stats.CriticalAlerts++
			case AlertLevelWarning:
				m.stats.WarningAlerts++
			}
		}
	}
}

// GetStats 获取统计
func (m *FleetMonitor) GetStats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SaveConfig 保存配置
func (m *FleetMonitor) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadConfig 加载配置
func (m *FleetMonitor) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.config)
}
