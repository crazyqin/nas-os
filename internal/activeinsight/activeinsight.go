// Package activeinsight 实现活动洞察模块，对标群晖 Active Insight
package activeinsight

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ActiveInsight 活动洞察.
type ActiveInsight struct {
	mu         sync.RWMutex
	devices    map[string]*Device
	metrics    map[string][]*Metric
	alerts     []*Alert
	config     *Config
	collectors map[string]Collector
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Config 活动洞察配置.
type Config struct {
	CollectInterval time.Duration      `json:"collect_interval"`
	RetentionDays   int                `json:"retention_days"`
	MaxMetrics      int                `json:"max_metrics"`
	AlertThresholds map[string]float64 `json:"alert_thresholds"`
	EnableAlerts    bool               `json:"enable_alerts"`
	WebhookURL      string             `json:"webhook_url"`
	EmailAlerts     []string           `json:"email_alerts"`
}

// Device 设备信息.
type Device struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	IP       string            `json:"ip"`
	Hostname string            `json:"hostname"`
	OS       string            `json:"os"`
	Version  string            `json:"version"`
	Status   DeviceStatus      `json:"status"`
	LastSeen time.Time         `json:"last_seen"`
	Uptime   time.Duration     `json:"uptime"`
	Hardware *HardwareInfo     `json:"hardware"`
	Network  *NetworkInfo      `json:"network"`
	Storage  *StorageInfo      `json:"storage"`
	Tags     map[string]string `json:"tags"`
}

// DeviceStatus 设备状态.
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusWarning DeviceStatus = "warning"
	DeviceStatusError   DeviceStatus = "error"
)

// HardwareInfo 硬件信息.
type HardwareInfo struct {
	CPUModel    string  `json:"cpu_model"`
	CPUCores    int     `json:"cpu_cores"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryTotal int64   `json:"memory_total"`
	MemoryUsed  int64   `json:"memory_used"`
	MemoryFree  int64   `json:"memory_free"`
	GPUModel    string  `json:"gpu_model,omitempty"`
	GPUUsage    float64 `json:"gpu_usage,omitempty"`
	Temperature float64 `json:"temperature"`
}

// NetworkInfo 网络信息.
type NetworkInfo struct {
	Interfaces    []NetworkInterface `json:"interfaces"`
	TotalSent     int64              `json:"total_sent"`
	TotalReceived int64              `json:"total_received"`
	Connections   int                `json:"connections"`
}

// NetworkInterface 网络接口.
type NetworkInterface struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Speed     int    `json:"speed"`
	BytesSent int64  `json:"bytes_sent"`
	BytesRecv int64  `json:"bytes_recv"`
	Status    string `json:"status"`
}

// StorageInfo 存储信息.
type StorageInfo struct {
	Pools     []StoragePool `json:"pools"`
	TotalSize int64         `json:"total_size"`
	UsedSize  int64         `json:"used_size"`
	FreeSize  int64         `json:"free_size"`
	Health    string        `json:"health"`
}

// StoragePool 存储池.
type StoragePool struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Size      int64  `json:"size"`
	Used      int64  `json:"used"`
	Free      int64  `json:"free"`
	Health    string `json:"health"`
	Disks     int    `json:"disks"`
	RAIDLevel string `json:"raid_level"`
}

// Metric 指标.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit"`
	Timestamp time.Time         `json:"timestamp"`
	DeviceID  string            `json:"device_id"`
	Tags      map[string]string `json:"tags"`
}

// Alert 告警.
type Alert struct {
	ID         string        `json:"id"`
	DeviceID   string        `json:"device_id"`
	DeviceName string        `json:"device_name"`
	Type       AlertType     `json:"type"`
	Severity   AlertSeverity `json:"severity"`
	Title      string        `json:"title"`
	Message    string        `json:"message"`
	Value      float64       `json:"value"`
	Threshold  float64       `json:"threshold"`
	CreatedAt  time.Time     `json:"created_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
	Notified   bool          `json:"notified"`
}

// AlertType 告警类型.
type AlertType string

const (
	AlertTypeCPU     AlertType = "cpu"
	AlertTypeMemory  AlertType = "memory"
	AlertTypeDisk    AlertType = "disk"
	AlertTypeNetwork AlertType = "network"
	AlertTypeTemp    AlertType = "temperature"
	AlertTypeHealth  AlertType = "health"
)

// AlertSeverity 告警级别.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Collector 指标收集器接口.
type Collector interface {
	Name() string
	Collect(ctx context.Context, device *Device) ([]*Metric, error)
}

// NewActiveInsight 创建活动洞察.
func NewActiveInsight(config *Config) *ActiveInsight {
	ctx, cancel := context.WithCancel(context.Background())
	return &ActiveInsight{
		devices:    make(map[string]*Device),
		metrics:    make(map[string][]*Metric),
		config:     config,
		collectors: make(map[string]Collector),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动活动洞察.
func (ai *ActiveInsight) Start() error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if ai.running {
		return fmt.Errorf("active insight is already running")
	}

	// 启动指标收集
	go ai.collectMetrics()

	// 启动告警检查
	if ai.config.EnableAlerts {
		go ai.checkAlerts()
	}

	// 启动数据清理
	go ai.cleanupOldData()

	ai.running = true
	return nil
}

// Stop 停止活动洞察.
func (ai *ActiveInsight) Stop() error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if !ai.running {
		return fmt.Errorf("active insight is not running")
	}

	ai.cancel()
	ai.running = false
	return nil
}

// RegisterDevice 注册设备.
func (ai *ActiveInsight) RegisterDevice(device *Device) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if _, exists := ai.devices[device.ID]; exists {
		return fmt.Errorf("device %s already registered", device.ID)
	}

	device.LastSeen = time.Now()
	device.Status = DeviceStatusOnline
	ai.devices[device.ID] = device
	return nil
}

// UnregisterDevice 注销设备.
func (ai *ActiveInsight) UnregisterDevice(deviceID string) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if _, exists := ai.devices[deviceID]; !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	delete(ai.devices, deviceID)
	delete(ai.metrics, deviceID)
	return nil
}

// UpdateDeviceStatus 更新设备状态.
func (ai *ActiveInsight) UpdateDeviceStatus(deviceID string, status DeviceStatus) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	device, exists := ai.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Status = status
	device.LastSeen = time.Now()
	return nil
}

// RecordMetric 记录指标.
func (ai *ActiveInsight) RecordMetric(metric *Metric) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if _, exists := ai.devices[metric.DeviceID]; !exists {
		return fmt.Errorf("device %s not found", metric.DeviceID)
	}

	metric.Timestamp = time.Now()
	ai.metrics[metric.DeviceID] = append(ai.metrics[metric.DeviceID], metric)

	// 限制指标数量
	if len(ai.metrics[metric.DeviceID]) > ai.config.MaxMetrics {
		ai.metrics[metric.DeviceID] = ai.metrics[metric.DeviceID][1:]
	}

	return nil
}

// GetDevice 获取设备信息.
func (ai *ActiveInsight) GetDevice(deviceID string) (*Device, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	device, exists := ai.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	return device, nil
}

// GetDevices 获取所有设备.
func (ai *ActiveInsight) GetDevices() []*Device {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	devices := make([]*Device, 0, len(ai.devices))
	for _, d := range ai.devices {
		devices = append(devices, d)
	}
	return devices
}

// GetMetrics 获取设备指标.
func (ai *ActiveInsight) GetMetrics(deviceID string, metricName string, since time.Time) ([]*Metric, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	if _, exists := ai.devices[deviceID]; !exists {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}

	var result []*Metric
	for _, m := range ai.metrics[deviceID] {
		if m.Timestamp.After(since) && (metricName == "" || m.Name == metricName) {
			result = append(result, m)
		}
	}

	return result, nil
}

// GetAlerts 获取告警列表.
func (ai *ActiveInsight) GetAlerts(deviceID string, severity AlertSeverity, resolved bool) []*Alert {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	var alerts []*Alert
	for _, a := range ai.alerts {
		if deviceID != "" && a.DeviceID != deviceID {
			continue
		}
		if severity != "" && a.Severity != severity {
			continue
		}
		if !resolved && a.ResolvedAt != nil {
			continue
		}
		alerts = append(alerts, a)
	}

	return alerts
}

// ResolveAlert 解决告警.
func (ai *ActiveInsight) ResolveAlert(alertID string) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	for _, a := range ai.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.ResolvedAt = &now
			return nil
		}
	}

	return fmt.Errorf("alert %s not found", alertID)
}

// RegisterCollector 注册指标收集器.
func (ai *ActiveInsight) RegisterCollector(collector Collector) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.collectors[collector.Name()] = collector
}

// GetStats 获取统计信息.
func (ai *ActiveInsight) GetStats() map[string]interface{} {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	onlineCount := 0
	for _, d := range ai.devices {
		if d.Status == DeviceStatusOnline {
			onlineCount++
		}
	}

	totalMetrics := 0
	for _, metrics := range ai.metrics {
		totalMetrics += len(metrics)
	}

	unresolvedAlerts := 0
	for _, a := range ai.alerts {
		if a.ResolvedAt == nil {
			unresolvedAlerts++
		}
	}

	return map[string]interface{}{
		"devices":           len(ai.devices),
		"online_devices":    onlineCount,
		"total_metrics":     totalMetrics,
		"total_alerts":      len(ai.alerts),
		"unresolved_alerts": unresolvedAlerts,
		"collectors":        len(ai.collectors),
		"running":           ai.running,
	}
}

// collectMetrics 收集指标.
func (ai *ActiveInsight) collectMetrics() {
	ticker := time.NewTicker(ai.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ai.ctx.Done():
			return
		case <-ticker.C:
			ai.collectAllMetrics()
		}
	}
}

// collectAllMetrics 收集所有设备指标.
func (ai *ActiveInsight) collectAllMetrics() {
	ai.mu.RLock()
	devices := make([]*Device, 0, len(ai.devices))
	for _, d := range ai.devices {
		devices = append(devices, d)
	}
	ai.mu.RUnlock()

	for _, device := range devices {
		for _, collector := range ai.collectors {
			metrics, err := collector.Collect(ai.ctx, device)
			if err != nil {
				continue
			}

			for _, metric := range metrics {
				ai.RecordMetric(metric)
			}
		}
	}
}

// checkAlerts 检查告警.
func (ai *ActiveInsight) checkAlerts() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ai.ctx.Done():
			return
		case <-ticker.C:
			ai.checkAllAlerts()
		}
	}
}

// checkAllAlerts 检查所有设备告警.
func (ai *ActiveInsight) checkAllAlerts() {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	for _, device := range ai.devices {
		ai.checkDeviceAlerts(device)
	}
}

// checkDeviceAlerts 检查设备告警.
func (ai *ActiveInsight) checkDeviceAlerts(device *Device) {
	if device.Hardware == nil {
		return
	}

	// 检查 CPU 使用率
	if threshold, ok := ai.config.AlertThresholds["cpu"]; ok {
		if device.Hardware.CPUUsage > threshold {
			ai.createAlert(device, AlertTypeCPU, AlertSeverityWarning,
				"CPU 使用率过高",
				fmt.Sprintf("CPU 使用率 %.1f%% 超过阈值 %.1f%%", device.Hardware.CPUUsage, threshold),
				device.Hardware.CPUUsage, threshold)
		}
	}

	// 检查内存使用率
	if threshold, ok := ai.config.AlertThresholds["memory"]; ok && device.Hardware.MemoryTotal > 0 {
		usage := float64(device.Hardware.MemoryUsed) / float64(device.Hardware.MemoryTotal) * 100
		if usage > threshold {
			ai.createAlert(device, AlertTypeMemory, AlertSeverityWarning,
				"内存使用率过高",
				fmt.Sprintf("内存使用率 %.1f%% 超过阈值 %.1f%%", usage, threshold),
				usage, threshold)
		}
	}

	// 检查温度
	if threshold, ok := ai.config.AlertThresholds["temperature"]; ok {
		if device.Hardware.Temperature > threshold {
			ai.createAlert(device, AlertTypeTemp, AlertSeverityCritical,
				"设备温度过高",
				fmt.Sprintf("设备温度 %.1f°C 超过阈值 %.1f°C", device.Hardware.Temperature, threshold),
				device.Hardware.Temperature, threshold)
		}
	}
}

// createAlert 创建告警.
func (ai *ActiveInsight) createAlert(device *Device, alertType AlertType, severity AlertSeverity, title, message string, value, threshold float64) {
	alert := &Alert{
		ID:         fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Type:       alertType,
		Severity:   severity,
		Title:      title,
		Message:    message,
		Value:      value,
		Threshold:  threshold,
		CreatedAt:  time.Now(),
	}

	ai.alerts = append(ai.alerts, alert)
}

// cleanupOldData 清理旧数据.
func (ai *ActiveInsight) cleanupOldData() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ai.ctx.Done():
			return
		case <-ticker.C:
			ai.cleanup()
		}
	}
}

// cleanup 清理过期数据.
func (ai *ActiveInsight) cleanup() {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -ai.config.RetentionDays)

	// 清理过期指标
	for deviceID, metrics := range ai.metrics {
		var valid []*Metric
		for _, m := range metrics {
			if m.Timestamp.After(cutoff) {
				valid = append(valid, m)
			}
		}
		ai.metrics[deviceID] = valid
	}

	// 清理已解决的告警
	var validAlerts []*Alert
	for _, a := range ai.alerts {
		if a.ResolvedAt == nil || a.ResolvedAt.After(cutoff) {
			validAlerts = append(validAlerts, a)
		}
	}
	ai.alerts = validAlerts
}
