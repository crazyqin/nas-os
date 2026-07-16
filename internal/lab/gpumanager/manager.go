package gpumanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager 多厂商GPU管理器.
type Manager struct {
	mu          sync.RWMutex
	config      *GPUManagerConfig
	logger      *slog.Logger
	detector    *Detector
	healthMon   *HealthMonitor
	capChecker  *CapabilityChecker
	devices     map[string]*GPUDevice
	initialized bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager 创建GPU管理器.
func NewManager(config *GPUManagerConfig, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultGPUManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	mgr := &Manager{
		config:     config,
		logger:     logger,
		detector:   NewDetector(logger),
		capChecker: NewCapabilityChecker(logger),
		devices:    make(map[string]*GPUDevice),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 初始化健康监控器
	if config.HealthCheckEnabled {
		mgr.healthMon = NewHealthMonitor(
			config.HealthThresholds,
			config.ScanInterval,
			logger,
		)
	}

	return mgr, nil
}

// Initialize 初始化GPU管理器.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		m.logger.Info("GPU管理器已禁用")
		return nil
	}

	m.logger.Info("GPU管理器初始化中...")

	// 检测GPU设备
	devices, err := m.detector.DetectAll(m.ctx)
	if err != nil {
		return fmt.Errorf("GPU检测失败: %w", err)
	}

	// 应用设备过滤
	filtered := m.applyFilter(devices)

	// 更新设备列表
	for _, device := range filtered {
		m.devices[device.ID] = device
	}

	// 启动健康监控
	if m.healthMon != nil {
		go m.healthMon.Start(m.devices)
	}

	// 启动定期扫描
	go m.startPeriodicScan()

	m.initialized = true

	m.logger.Info("GPU管理器初始化完成",
		"devices", len(m.devices),
		"nvidia", countVendor(m.devices, VendorNVIDIA),
		"amd", countVendor(m.devices, VendorAMD),
		"intel", countVendor(m.devices, VendorIntel),
	)

	return nil
}

// applyFilter 应用设备过滤器.
func (m *Manager) applyFilter(devices []*GPUDevice) []*GPUDevice {
	if len(m.config.DeviceFilter) == 0 {
		return devices
	}

	filterSet := make(map[GPUVendor]bool)
	for _, v := range m.config.DeviceFilter {
		filterSet[v] = true
	}

	var filtered []*GPUDevice
	for _, device := range devices {
		if filterSet[device.Vendor] {
			filtered = append(filtered, device)
		}
	}

	return filtered
}

// startPeriodicScan 启动定期扫描.
func (m *Manager) startPeriodicScan() {
	if m.config.ScanInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.refreshDevices()
		}
	}
}

// refreshDevices 刷新设备列表.
func (m *Manager) refreshDevices() {
	m.mu.Lock()
	defer m.mu.Unlock()

	devices, err := m.detector.DetectAll(m.ctx)
	if err != nil {
		m.logger.Warn("GPU刷新失败", "error", err)
		return
	}

	filtered := m.applyFilter(devices)

	// 更新设备列表
	newDevices := make(map[string]*GPUDevice)
	for _, device := range filtered {
		// 保留旧设备的能力信息
		if old, exists := m.devices[device.ID]; exists {
			if device.Capabilities == nil {
				device.Capabilities = old.Capabilities
			}
		}
		newDevices[device.ID] = device
	}

	m.devices = newDevices

	// 更新健康监控器
	if m.healthMon != nil {
		m.healthMon.UpdateDevices(newDevices)
	}

	m.logger.Debug("GPU设备刷新完成", "devices", len(newDevices))
}

// Stop 停止GPU管理器.
func (m *Manager) Stop() {
	m.cancel()

	if m.healthMon != nil {
		m.healthMon.Stop()
	}

	m.logger.Info("GPU管理器已停止")
}

// ListDevices 列出所有GPU设备.
func (m *Manager) ListDevices() []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetDevice 获取单个GPU设备.
func (m *Manager) GetDevice(id string) (*GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("GPU设备不存在: %s", id)
	}
	return device, nil
}

// GetDeviceByVendor 按厂商获取GPU设备.
func (m *Manager) GetDeviceByVendor(vendor GPUVendor) []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*GPUDevice
	for _, device := range m.devices {
		if device.Vendor == vendor {
			result = append(result, device)
		}
	}
	return result
}

// GetHealthReport 获取健康报告.
func (m *Manager) GetHealthReport() *GPUHealthReport {
	if m.healthMon == nil {
		return &GPUHealthReport{
			Timestamp:     time.Now(),
			OverallStatus: StatusOffline,
			Warnings:      []string{"健康监控未启用"},
		}
	}

	return m.healthMon.CheckAll()
}

// GetCapabilityReport 获取能力报告.
func (m *Manager) GetCapabilityReport() *GPUCapabilityReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}

	return m.capChecker.GenerateCapabilityReport(m.ctx, devices)
}

// CheckTranscodeCapabilities 检测转码能力.
func (m *Manager) CheckTranscodeCapabilities(deviceID string) (*TranscodeCapability, error) {
	m.mu.RLock()
	device, ok := m.devices[deviceID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("GPU设备不存在: %s", deviceID)
	}

	return m.capChecker.CheckTranscodeCapabilities(m.ctx, device), nil
}

// CheckInferenceCapabilities 检测AI推理能力.
func (m *Manager) CheckInferenceCapabilities(deviceID string) (*InferenceCapability, error) {
	m.mu.RLock()
	device, ok := m.devices[deviceID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("GPU设备不存在: %s", deviceID)
	}

	return m.capChecker.CheckInferenceCapabilities(m.ctx, device), nil
}

// GetAllTranscodeCapabilities 获取所有设备转码能力.
func (m *Manager) GetAllTranscodeCapabilities() []*TranscodeCapability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TranscodeCapability
	for _, device := range m.devices {
		caps := m.capChecker.CheckTranscodeCapabilities(m.ctx, device)
		result = append(result, caps)
	}
	return result
}

// GetAllInferenceCapabilities 获取所有设备推理能力.
func (m *Manager) GetAllInferenceCapabilities() []*InferenceCapability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*InferenceCapability
	for _, device := range m.devices {
		caps := m.capChecker.CheckInferenceCapabilities(m.ctx, device)
		result = append(result, caps)
	}
	return result
}

// Refresh 手动刷新设备列表.
func (m *Manager) Refresh() error {
	m.refreshDevices()
	return nil
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *GPUManagerConfig {
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config *GPUManagerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config

	// 更新健康监控器阈值
	if m.healthMon != nil && config.HealthThresholds != nil {
		m.healthMon.SetThresholds(config.HealthThresholds)
	}
}

// ExportJSON 导出设备信息为JSON.
func (m *Manager) ExportJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}

	return json.MarshalIndent(devices, "", "  ")
}

// IsInitialized 检查是否已初始化.
func (m *Manager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// DeviceCount 获取设备数量.
func (m *Manager) DeviceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.devices)
}

// HasNVIDIA 检查是否有NVIDIA GPU.
func (m *Manager) HasNVIDIA() bool {
	return countVendor(m.devices, VendorNVIDIA) > 0
}

// HasAMD 检查是否有AMD GPU.
func (m *Manager) HasAMD() bool {
	return countVendor(m.devices, VendorAMD) > 0
}

// HasIntel 检查是否有Intel GPU.
func (m *Manager) HasIntel() bool {
	return countVendor(m.devices, VendorIntel) > 0
}

// HasTranscodeCapable 检查是否有转码能力的GPU.
func (m *Manager) HasTranscodeCapable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.devices {
		if device.Capabilities != nil && device.Capabilities.TranscodeCapable {
			return true
		}
	}
	return false
}

// HasInferenceCapable 检查是否有推理能力的GPU.
func (m *Manager) HasInferenceCapable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.devices {
		if device.Capabilities != nil && device.Capabilities.InferenceCapable {
			return true
		}
	}
	return false
}

// 辅助函数

func countVendor(devices map[string]*GPUDevice, vendor GPUVendor) int {
	count := 0
	for _, device := range devices {
		if device.Vendor == vendor {
			count++
		}
	}
	return count
}
