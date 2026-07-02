package gpumanager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// HealthMonitor GPU健康监控器.
type HealthMonitor struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	config    *HealthThresholds
	devices   map[string]*GPUDevice
	statuses  map[string]*GPUHealthStatus
	warnings  []string
	errors    []string
	interval  time.Duration
	lastCheck time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewHealthMonitor 创建健康监控器.
func NewHealthMonitor(config *HealthThresholds, interval time.Duration, logger *slog.Logger) *HealthMonitor {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultHealthThresholds()
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		logger:   logger,
		config:   config,
		devices:  make(map[string]*GPUDevice),
		statuses: make(map[string]*GPUHealthStatus),
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动健康监控.
func (m *HealthMonitor) Start(devices map[string]*GPUDevice) {
	m.mu.Lock()
	m.devices = devices
	m.mu.Unlock()

	m.logger.Info("GPU健康监控启动", "interval", m.interval)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("GPU健康监控停止")
			return
		case <-ticker.C:
			m.CheckAll()
		}
	}
}

// Stop 停止健康监控.
func (m *HealthMonitor) Stop() {
	m.cancel()
}

// UpdateDevices 更新设备列表.
func (m *HealthMonitor) UpdateDevices(devices map[string]*GPUDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices = devices
}

// CheckAll 检查所有GPU健康状态.
func (m *HealthMonitor) CheckAll() *GPUHealthReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := &GPUHealthReport{
		Timestamp: time.Now(),
		Devices:   make([]*GPUHealthStatus, 0),
		Warnings:  make([]string, 0),
		Errors:    make([]string, 0),
	}

	overallStatus := StatusHealthy

	for id, device := range m.devices {
		status := m.checkDevice(device)
		m.statuses[id] = status
		report.Devices = append(report.Devices, status)

		// 更新整体状态
		switch status.Status {
		case StatusCritical, StatusError:
			overallStatus = StatusCritical
		case StatusWarning:
			if overallStatus != StatusCritical {
				overallStatus = StatusWarning
			}
		}

		// 收集警告和错误
		report.Warnings = append(report.Warnings, status.Warnings...)
		report.Errors = append(report.Errors, status.Errors...)
	}

	report.OverallStatus = overallStatus
	m.warnings = report.Warnings
	m.errors = report.Errors
	m.lastCheck = time.Now()

	return report
}

// checkDevice 检查单个GPU设备健康状态.
func (m *HealthMonitor) checkDevice(device *GPUDevice) *GPUHealthStatus {
	status := &GPUHealthStatus{
		DeviceID:    device.ID,
		DeviceName:  device.Name,
		Status:      StatusHealthy,
		Temperature: device.Temperature,
		Warnings:    make([]string, 0),
		Errors:      make([]string, 0),
		LastUpdated: time.Now(),
	}

	// 检查温度
	status.TempStatus = m.checkTemperature(device.Temperature, status)

	// 检查功耗
	status.PowerStatus = m.checkPower(device.PowerUsage, device.PowerLimit, status)

	// 检查显存
	status.MemoryStatus = m.checkMemory(device.MemoryUsed, device.MemoryTotal, status)

	// 检查设备状态
	if device.Status == StatusError || device.Status == StatusOffline {
		status.Status = StatusError
		status.Errors = append(status.Errors, fmt.Sprintf("设备 %s 状态异常: %s", device.ID, device.Status))
	}

	// 检查驱动状态
	if !device.DriverOK {
		status.Status = StatusWarning
		status.Warnings = append(status.Warnings, fmt.Sprintf("设备 %s 驱动异常", device.ID))
	}

	// 确定最终状态
	if len(status.Errors) > 0 {
		status.Status = StatusCritical
	} else if len(status.Warnings) > 0 {
		status.Status = StatusWarning
	}

	return status
}

// checkTemperature 检查温度.
func (m *HealthMonitor) checkTemperature(temp int, status *GPUHealthStatus) string {
	if temp <= 0 {
		return "unknown"
	}

	if temp >= m.config.TempCritical {
		status.Errors = append(status.Errors,
			fmt.Sprintf("温度严重过高: %d°C (阈值: %d°C)", temp, m.config.TempCritical))
		return "critical"
	}

	if temp >= m.config.TempWarning {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("温度偏高: %d°C (阈值: %d°C)", temp, m.config.TempWarning))
		return "warm"
	}

	if temp >= 60 {
		return "warm"
	}

	return "normal"
}

// checkPower 检查功耗.
func (m *HealthMonitor) checkPower(usage, limit uint64, status *GPUHealthStatus) string {
	if limit == 0 {
		return "unknown"
	}

	utilization := float64(usage) / float64(limit) * 100

	if utilization >= m.config.PowerCritical {
		status.Errors = append(status.Errors,
			fmt.Sprintf("功耗已达上限: %dW/%dW (%.1f%%)", usage, limit, utilization))
		return "overlimit"
	}

	if utilization >= m.config.PowerWarning {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("功耗偏高: %dW/%dW (%.1f%%)", usage, limit, utilization))
		return "high"
	}

	return "normal"
}

// checkMemory 检查显存.
func (m *HealthMonitor) checkMemory(used, total uint64, status *GPUHealthStatus) string {
	if total == 0 {
		return "unknown"
	}

	utilization := float64(used) / float64(total) * 100

	if utilization >= m.config.MemCritical {
		status.Errors = append(status.Errors,
			fmt.Sprintf("显存即将耗尽: %dMB/%dMB (%.1f%%)", used, total, utilization))
		return "full"
	}

	if utilization >= m.config.MemWarning {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("显存使用率高: %dMB/%dMB (%.1f%%)", used, total, utilization))
		return "high"
	}

	return "normal"
}

// GetDeviceStatus 获取单个设备健康状态.
func (m *HealthMonitor) GetDeviceStatus(deviceID string) (*GPUHealthStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.statuses[deviceID]
	return status, ok
}

// GetWarnings 获取当前警告.
func (m *HealthMonitor) GetWarnings() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.warnings
}

// GetErrors 获取当前错误.
func (m *HealthMonitor) GetErrors() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errors
}

// SetThresholds 更新阈值.
func (m *HealthMonitor) SetThresholds(config *HealthThresholds) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// SetInterval 更新检查间隔.
func (m *HealthMonitor) SetInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = interval
}

// GetLastCheckTime 获取最后检查时间.
func (m *HealthMonitor) GetLastCheckTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastCheck
}
