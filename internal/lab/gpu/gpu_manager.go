// Package gpu GPU管理核心模块
package gpu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager GPU管理器.
type Manager struct {
	config      *GPUConfig
	logger      *zap.Logger
	devices     map[string]*GPUDevice     // GPU设备映射
	allocations map[string]*GPUAllocation // 分配映射
	mu          sync.RWMutex
	monitor     *Monitor
	scheduler   *Scheduler
	nvidia      *NVIDIAProvider
	initialized bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager 创建GPU管理器.
func NewManager(config *GPUConfig, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if config == nil {
		config = DefaultGPUConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	mgr := &Manager{
		config:      config,
		logger:      logger,
		devices:     make(map[string]*GPUDevice),
		allocations: make(map[string]*GPUAllocation),
		ctx:         ctx,
		cancel:      cancel,
	}

	// 只在明确启用且有设备路径时初始化NVIDIA提供者
	// GPUDevices非空表示用户明确指定了GPU设备
	if config.GPUEnabled && len(config.GPUDevices) > 0 {
		if nvidiaProvider, err := NewNVIDIAProvider(logger); err == nil {
			mgr.nvidia = nvidiaProvider
		} else {
			logger.Warn("NVIDIA GPU provider not available", zap.Error(err))
		}
	}

	// 初始化调度器
	mgr.scheduler = NewScheduler(mgr, config.SchedulerPolicy, logger)

	// 初始化监控器
	mgr.monitor = NewMonitor(mgr, config.MonitorInterval, logger)

	// 只在有NVIDIA provider时自动检测GPU设备
	if mgr.nvidia != nil {
		if err := mgr.detectDevices(); err != nil {
			logger.Warn("GPU device detection failed", zap.Error(err))
		}
	}

	mgr.initialized = true
	return mgr, nil
}

// Initialize 初始化GPU管理器.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.GPUEnabled {
		m.logger.Info("GPU功能已禁用")
		return nil
	}

	// 检查NVIDIA驱动是否可用
	if m.nvidia != nil {
		if err := m.nvidia.CheckDriver(); err != nil {
			return fmt.Errorf("NVIDIA驱动检查失败: %w", err)
		}
	}

	// 启动监控器
	if m.monitor != nil {
		go m.monitor.Start(m.ctx)
	}

	// 启动健康检查
	go m.startHealthCheck(m.ctx)

	m.logger.Info("GPU管理器初始化完成",
		zap.Int("devices", len(m.devices)),
		zap.Bool("nvidia", m.nvidia != nil))

	return nil
}

// detectDevices 检测GPU设备.
func (m *Manager) detectDevices() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有设备
	m.devices = make(map[string]*GPUDevice)

	// 检测NVIDIA GPU
	if m.nvidia != nil {
		nvidiaDevices, err := m.nvidia.ListDevices()
		if err != nil {
			m.logger.Warn("NVIDIA设备检测失败", zap.Error(err))
		} else {
			for _, device := range nvidiaDevices {
				m.devices[device.ID] = device
				m.logger.Info("检测到NVIDIA GPU",
					zap.String("id", device.ID),
					zap.String("name", device.Name),
					zap.Uint64("memory", device.MemoryTotal))
			}
		}
	}

	// 检查指定的设备路径
	for _, devicePath := range m.config.GPUDevices {
		if _, err := os.Stat(devicePath); err == nil {
			// 设备存在，如果尚未注册则添加
			deviceID := filepath.Base(devicePath)
			if _, exists := m.devices[deviceID]; !exists {
				m.devices[deviceID] = &GPUDevice{
					ID:         deviceID,
					DevicePath: devicePath,
					Status:     GPUStatusAvailable,
					Allocated:  false,
					Vendor:     "unknown",
				}
			}
		}
	}

	return nil
}

// ListGPUs 列出所有GPU设备.
func (m *Manager) ListGPUs(filter *GPUDeviceFilter) []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*GPUDevice
	for _, device := range m.devices {
		if filter != nil && !m.matchFilter(device, filter) {
			continue
		}
		result = append(result, device)
	}

	return result
}

// matchFilter 匹配设备过滤器.
func (m *Manager) matchFilter(device *GPUDevice, filter *GPUDeviceFilter) bool {
	if filter.Vendor != "" && device.Vendor != filter.Vendor {
		return false
	}
	if filter.Model != "" && !strings.Contains(device.Name, filter.Model) {
		return false
	}
	if filter.MinMemory > 0 && device.MemoryTotal < filter.MinMemory {
		return false
	}
	if filter.MinCUDACores > 0 && device.CUDAcores < filter.MinCUDACores {
		return false
	}
	if filter.Status != "" && device.Status != filter.Status {
		return false
	}
	if filter.OnlyFree && device.Allocated {
		return false
	}
	for _, excludeID := range filter.ExcludeIDs {
		if device.ID == excludeID {
			return false
		}
	}
	return true
}

// GetGPU 获取GPU设备详情.
func (m *Manager) GetGPU(id string) (*GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("GPU设备不存在: %s", id)
	}

	return device, nil
}

// getAvailableGPUsInternal 内部方法获取可用GPU列表（不获取锁，用于调度器）
// 注意：调用者必须已持有m.mu锁.
func (m *Manager) getAvailableGPUsInternal() []*GPUDevice {
	var result []*GPUDevice
	for _, device := range m.devices {
		if device.Status == GPUStatusAvailable && !device.Allocated {
			result = append(result, device)
		}
	}
	return result
}

// AllocateGPU 分配GPU给容器/VM.
func (m *Manager) AllocateGPU(req *GPUAllocation) (*GPUAllocationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.GPUEnabled {
		return nil, fmt.Errorf("GPU功能已禁用")
	}

	if len(m.allocations) >= m.config.MaxAllocations {
		return nil, fmt.Errorf("已达到最大分配数量限制")
	}

	// 使用调度器选择GPU
	device, err := m.scheduler.SelectGPU(req)
	if err != nil {
		return nil, fmt.Errorf("无法选择合适的GPU: %w", err)
	}

	// 设置显存限制
	memoryLimit := req.MemoryLimit
	if memoryLimit == 0 {
		memoryLimit = parseMemoryLimit(m.config.DefaultMemoryLimit)
	}

	// 设置CUDA核心限制
	cudaLimit := req.CUDALimit
	if cudaLimit == 0 {
		cudaLimit = m.config.DefaultCUDACores
	}

	// 确保限制不超过设备容量
	if memoryLimit > device.MemoryTotal {
		memoryLimit = device.MemoryTotal * 80 / 100 // 限制为80%
	}

	// 创建分配记录
	req.RequestID = generateRequestID()
	req.GPUID = device.ID // 设置GPU设备ID
	req.MemoryLimit = memoryLimit
	req.CUDALimit = cudaLimit
	req.CreatedAt = time.Now()

	// 更新设备状态
	device.Allocated = true
	device.AllocatedTo = req.ContainerID
	device.AllocatedAt = req.CreatedAt
	device.Status = GPUStatusAllocated

	// 保存分配记录
	m.allocations[req.RequestID] = req

	// 构建设备路径
	devicePaths := m.buildDevicePaths(device)

	m.logger.Info("GPU已分配",
		zap.String("requestId", req.RequestID),
		zap.String("gpuId", device.ID),
		zap.String("containerId", req.ContainerID),
		zap.Uint64("memoryLimit", memoryLimit))

	return &GPUAllocationResult{
		Success:     true,
		RequestID:   req.RequestID,
		GPUID:       device.ID,
		DevicePaths: devicePaths,
		MemoryLimit: memoryLimit,
		CUDALimit:   cudaLimit,
		Message:     fmt.Sprintf("GPU %s 已成功分配给容器 %s", device.Name, req.ContainerID),
	}, nil
}

// ReleaseGPU 释放GPU.
func (m *Manager) ReleaseGPU(req *GPUReleaseRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.RequestID != "" {
		// 通过请求ID释放
		alloc, exists := m.allocations[req.RequestID]
		if !exists {
			return fmt.Errorf("分配请求不存在: %s", req.RequestID)
		}

		// 更新设备状态
		if device, exists := m.devices[alloc.GPUID]; exists {
			device.Allocated = false
			device.AllocatedTo = ""
			device.AllocatedAt = time.Time{}
			device.Status = GPUStatusAvailable

			m.logger.Info("GPU已释放",
				zap.String("requestId", req.RequestID),
				zap.String("gpuId", device.ID))
		}

		delete(m.allocations, req.RequestID)
		return nil
	}

	// 通过容器ID释放所有相关GPU
	for reqID, alloc := range m.allocations {
		if alloc.ContainerID == req.ContainerID {
			if device, exists := m.devices[alloc.GPUID]; exists {
				device.Allocated = false
				device.AllocatedTo = ""
				device.AllocatedAt = time.Time{}
				device.Status = GPUStatusAvailable
			}
			delete(m.allocations, reqID)

			m.logger.Info("GPU已释放",
				zap.String("containerId", req.ContainerID),
				zap.String("gpuId", alloc.GPUID))
		}
	}

	return nil
}

// GetGPUStats 获取GPU统计信息.
func (m *Manager) GetGPUStats() (*GPUStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &GPUStats{
		Allocations: make([]GPUAllocation, 0),
	}

	stats.TotalGPUs = len(m.devices)
	stats.AllocatedGPUs = len(m.allocations)
	stats.AvailableGPUs = stats.TotalGPUs - stats.AllocatedGPUs

	for _, alloc := range m.allocations {
		stats.Allocations = append(stats.Allocations, *alloc)
	}

	// 计算内存统计
	var totalMemory, usedMemory, freeMemory uint64
	var avgTemp, avgPower int
	var gpuUtilization float64

	for _, device := range m.devices {
		totalMemory += device.MemoryTotal
		usedMemory += device.MemoryUsed
		freeMemory += device.MemoryFree
		avgTemp += device.Temperature
		avgPower += int(device.PowerUsage)

		// GPU利用率 (基于内存使用)
		if device.MemoryTotal > 0 {
			gpuUtilization += float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
		}
	}

	stats.TotalMemory = totalMemory
	stats.UsedMemory = usedMemory
	stats.FreeMemory = freeMemory

	if stats.TotalGPUs > 0 {
		stats.AvgTemperature = avgTemp / stats.TotalGPUs
		stats.AvgPowerUsage = uint64(avgPower / stats.TotalGPUs)
		stats.Utilization = gpuUtilization / float64(stats.TotalGPUs)
	}

	// 健康状态
	stats.HealthStatus = m.getHealthStatus()

	return stats, nil
}

// getHealthStatus 获取健康状态.
func (m *Manager) getHealthStatus() GPUHealthStatus {
	status := GPUHealthStatus{
		Status:    "healthy",
		Warnings:  make([]string, 0),
		Errors:    make([]string, 0),
		LastCheck: time.Now(),
		DriverOK:  true,
		DevicesOK: make(map[string]bool),
	}

	// 检查NVIDIA驱动
	if m.nvidia != nil {
		if err := m.nvidia.CheckDriver(); err != nil {
			status.DriverOK = false
			status.Status = "warning"
			status.Warnings = append(status.Warnings, "NVIDIA驱动异常: "+err.Error())
		}
	}

	// 检查各GPU设备
	for id, device := range m.devices {
		status.DevicesOK[id] = true

		// 温度警告 (>85°C)
		if device.Temperature > 85 {
			status.DevicesOK[id] = false
			status.Status = "warning"
			status.Warnings = append(status.Warnings,
				fmt.Sprintf("GPU %s 温度过高: %d°C", id, device.Temperature))
		}

		// 功耗异常
		if device.PowerUsage > device.PowerLimit {
			status.DevicesOK[id] = false
			status.Status = "warning"
			status.Warnings = append(status.Warnings,
				fmt.Sprintf("GPU %s 功耗超标: %dW > %dW", id, device.PowerUsage, device.PowerLimit))
		}

		// 设备错误状态
		if device.Status == GPUStatusError {
			status.DevicesOK[id] = false
			status.Status = "critical"
			status.Errors = append(status.Errors,
				fmt.Sprintf("GPU %s 处于错误状态", id))
		}
	}

	return status
}

// buildDevicePaths 构建设备路径列表.
func (m *Manager) buildDevicePaths(device *GPUDevice) []string {
	paths := []string{device.DevicePath}

	// NVIDIA设备需要额外路径
	if device.Vendor == "nvidia" {
		// 添加NVIDIA UVM设备
		uvmPath := "/dev/nvidia-uvm"
		if _, err := os.Stat(uvmPath); err == nil {
			paths = append(paths, uvmPath)
		}

		// 添加NVIDIA UVM Tools设备
		uvmToolsPath := "/dev/nvidia-uvm-tools"
		if _, err := os.Stat(uvmToolsPath); err == nil {
			paths = append(paths, uvmToolsPath)
		}

		// 添加NVIDIA CTL设备
		ctlPath := "/dev/nvidiactl"
		if _, err := os.Stat(ctlPath); err == nil {
			paths = append(paths, ctlPath)
		}
	}

	return paths
}

// startHealthCheck 启动健康检查.
func (m *Manager) startHealthCheck(ctx context.Context) {
	if m.config.HealthCheckInterval <= 0 {
		m.config.HealthCheckInterval = 30
	}

	ticker := time.NewTicker(time.Duration(m.config.HealthCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHealth()
		}
	}
}

// checkHealth 检查GPU健康状态.
func (m *Manager) checkHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查NVIDIA驱动和设备
	if m.nvidia != nil {
		devices, err := m.nvidia.ListDevices()
		if err != nil {
			m.logger.Error("健康检查失败", zap.Error(err))
			return
		}

		for _, updatedDevice := range devices {
			if device, exists := m.devices[updatedDevice.ID]; exists {
				// 更新设备状态（保留分配信息）
				device.MemoryUsed = updatedDevice.MemoryUsed
				device.MemoryFree = updatedDevice.MemoryFree
				device.Temperature = updatedDevice.Temperature
				device.PowerUsage = updatedDevice.PowerUsage
				device.Status = updatedDevice.Status
			}
		}
	}
}

// GetContainerGPUAllocations 获取容器/VM的GPU分配.
func (m *Manager) GetContainerGPUAllocations(containerID string) []*GPUAllocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allocations []*GPUAllocation
	for _, alloc := range m.allocations {
		if alloc.ContainerID == containerID {
			allocations = append(allocations, alloc)
		}
	}

	return allocations
}

// Close 关闭GPU管理器.
func (m *Manager) Close() error {
	m.cancel()

	// 释放所有分配
	m.mu.Lock()
	for reqID, alloc := range m.allocations {
		if device, exists := m.devices[alloc.GPUID]; exists {
			device.Allocated = false
			device.AllocatedTo = ""
			device.AllocatedAt = time.Time{}
			device.Status = GPUStatusAvailable
		}
		delete(m.allocations, reqID)
	}
	m.mu.Unlock()

	m.logger.Info("GPU管理器已关闭")
	return nil
}

// RefreshDevices 刷新GPU设备列表.
func (m *Manager) RefreshDevices() error {
	return m.detectDevices()
}

// generateRequestID 生成请求ID.
func generateRequestID() string {
	return fmt.Sprintf("gpu-req-%d", time.Now().UnixNano())
}

// parseMemoryLimit 解析内存限制字符串.
func parseMemoryLimit(limit string) uint64 {
	limit = strings.TrimSpace(strings.ToLower(limit))

	var value uint64
	var unit string

	_, _ = fmt.Sscanf(limit, "%d%s", &value, &unit)

	switch unit {
	case "k", "kb":
		return value / 1024 // 转换为MB
	case "m", "mb":
		return value
	case "g", "gb":
		return value * 1024
	case "t", "tb":
		return value * 1024 * 1024
	default:
		return value
	}
}

// IsGPUAvailable 检查GPU是否可用.
func (m *Manager) IsGPUAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.devices) > 0
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *GPUConfig {
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config *GPUConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	m.scheduler.SetPolicy(config.SchedulerPolicy)
	m.monitor.SetInterval(config.MonitorInterval)

	return nil
}

// ExportConfig 导出配置为JSON.
func (m *Manager) ExportConfig() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.MarshalIndent(m.config, "", "  ")
}

// ImportConfig 从JSON导入配置.
func (m *Manager) ImportConfig(data []byte) error {
	var config GPUConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("配置解析失败: %w", err)
	}

	return m.UpdateConfig(&config)
}

// CheckNVidiaDriver 检查NVIDIA驱动状态.
func (m *Manager) CheckNVidiaDriver() error {
	if m.nvidia == nil {
		return fmt.Errorf("NVIDIA提供者未初始化")
	}
	return m.nvidia.CheckDriver()
}

// GetNvidiaSMIOutput 获取nvidia-smi原始输出.
func (m *Manager) GetNvidiaSMIOutput() (string, error) {
	if m.nvidia == nil {
		return "", fmt.Errorf("NVIDIA提供者未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nvidia-smi执行失败: %w", err)
	}

	return string(output), nil
}
