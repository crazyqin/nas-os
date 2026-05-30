// Package gpuds 提供 GPU Direct Storage 功能
package gpuds

import (
	"fmt"
	"sync"
	"time"
)

// Manager GPU Direct Storage 管理器
type Manager struct {
	mu       sync.RWMutex
	devices  map[string]*GPUDevice
	buffers  map[string]*DirectBuffer
	jobs     map[string]*TransferJob
	config   GPUDSConfig
	stats    TransferStats
}

// NewManager 创建 GPU Direct Storage 管理器
func NewManager(config GPUDSConfig) *Manager {
	// 设置默认值
	if config.MaxBuffers == 0 {
		config.MaxBuffers = 64
	}
	if config.DefaultBufSize == 0 {
		config.DefaultBufSize = 64 * 1024 * 1024 // 64MB
	}
	if config.MaxTransferSize == 0 {
		config.MaxTransferSize = 1024 * 1024 * 1024 // 1GB
	}
	if config.IOQueueDepth == 0 {
		config.IOQueueDepth = 32
	}

	return &Manager{
		devices: make(map[string]*GPUDevice),
		buffers: make(map[string]*DirectBuffer),
		jobs:    make(map[string]*TransferJob),
		config:  config,
	}
}

// DetectGPU 检测 GPU 设备
func (m *Manager) DetectGPU() ([]GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]GPUDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices, nil
}

// RegisterGPU 注册 GPU 设备
func (m *Manager) RegisterGPU(device GPUDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("gpu-%d", time.Now().UnixNano())
	}

	if _, exists := m.devices[device.ID]; exists {
		return fmt.Errorf("GPU device already exists: %s", device.ID)
	}

	device.State = GPUStateActive
	device.LastSeen = time.Now()
	device.CreatedAt = time.Now()

	m.devices[device.ID] = &device
	return nil
}

// GetGPU 获取 GPU 设备详情
func (m *Manager) GetGPU(id string) (*GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("GPU device not found: %s", id)
	}
	return device, nil
}

// ListGPUs 列出所有 GPU 设备
func (m *Manager) ListGPUs() []GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]GPUDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// CreateBuffer 创建直接传输缓冲区
func (m *Manager) CreateBuffer(gpuDeviceID string, size int64) (*DirectBuffer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 GPU 设备存在
	device, ok := m.devices[gpuDeviceID]
	if !ok {
		return nil, fmt.Errorf("GPU device not found: %s", gpuDeviceID)
	}

	if !device.GPDSupported {
		return nil, fmt.Errorf("GPU device %s does not support Direct Storage", gpuDeviceID)
	}

	// 检查缓冲区数量限制
	allocatedCount := 0
	for _, buf := range m.buffers {
		if buf.GPUDeviceID == gpuDeviceID && buf.State != BufferStateFree {
			allocatedCount++
		}
	}
	if allocatedCount >= m.config.MaxBuffers {
		return nil, fmt.Errorf("max buffers reached for device %s: %d", gpuDeviceID, m.config.MaxBuffers)
	}

	// 检查 GPU 内存是否足够
	if size > device.MemoryFree {
		return nil, fmt.Errorf("insufficient GPU memory: requested %d, available %d", size, device.MemoryFree)
	}

	buffer := &DirectBuffer{
		ID:          fmt.Sprintf("buf-%s-%d", gpuDeviceID, time.Now().UnixNano()),
		GPUDeviceID: gpuDeviceID,
		Size:        size,
		State:       BufferStateAllocated,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}

	m.buffers[buffer.ID] = buffer

	// 更新 GPU 内存使用
	device.MemoryUsed += size
	device.MemoryFree -= size

	return buffer, nil
}

// FreeBuffer 释放缓冲区
func (m *Manager) FreeBuffer(bufferID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	buffer, ok := m.buffers[bufferID]
	if !ok {
		return fmt.Errorf("buffer not found: %s", bufferID)
	}

	if buffer.State == BufferStateInUse {
		return fmt.Errorf("buffer %s is in use", bufferID)
	}

	// 更新 GPU 内存
	if device, ok := m.devices[buffer.GPUDeviceID]; ok {
		device.MemoryUsed -= buffer.Size
		device.MemoryFree += buffer.Size
	}

	delete(m.buffers, bufferID)
	return nil
}

// GetBuffer 获取缓冲区详情
func (m *Manager) GetBuffer(bufferID string) (*DirectBuffer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buffer, ok := m.buffers[bufferID]
	if !ok {
		return nil, fmt.Errorf("buffer not found: %s", bufferID)
	}
	return buffer, nil
}

// ListBuffers 列出缓冲区
func (m *Manager) ListBuffers(gpuDeviceID string) []DirectBuffer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var buffers []DirectBuffer
	for _, b := range m.buffers {
		if gpuDeviceID == "" || b.GPUDeviceID == gpuDeviceID {
			buffers = append(buffers, *b)
		}
	}
	return buffers
}

// Transfer 发起传输任务
func (m *Manager) Transfer(source, destination TransferEndpoint, size int64) (*TransferJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查传输大小限制
	if size > m.config.MaxTransferSize {
		return nil, fmt.Errorf("transfer size %d exceeds max %d", size, m.config.MaxTransferSize)
	}

	// 验证源和目标
	if err := m.validateEndpoint(source); err != nil {
		return nil, fmt.Errorf("invalid source: %v", err)
	}
	if err := m.validateEndpoint(destination); err != nil {
		return nil, fmt.Errorf("invalid destination: %v", err)
	}

	now := time.Now()
	job := &TransferJob{
		ID:          fmt.Sprintf("transfer-%d", now.UnixNano()),
		Source:      source,
		Destination: destination,
		Size:        size,
		State:       TransferStatePending,
		StartTime:   now,
	}

	m.jobs[job.ID] = job
	m.stats.TotalTransfers++
	m.stats.ActiveJobs++

	// 模拟启动传输
	job.State = TransferStateRunning

	return job, nil
}

// validateEndpoint 验证传输端点
func (m *Manager) validateEndpoint(ep TransferEndpoint) error {
	switch ep.Type {
	case "gpu":
		if _, ok := m.devices[ep.DeviceID]; !ok {
			return fmt.Errorf("GPU device not found: %s", ep.DeviceID)
		}
	case "nvme":
		// NVMe 设备验证（简化）
		if ep.Path == "" {
			return fmt.Errorf("NVMe path required")
		}
	case "host":
		// 主机内存端点
		if ep.DeviceID == "" {
			return fmt.Errorf("host device ID required")
		}
	default:
		return fmt.Errorf("unknown endpoint type: %s", ep.Type)
	}
	return nil
}

// CompleteTransfer 完成传输任务
func (m *Manager) CompleteTransfer(jobID string, transferred int64, bandwidth float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("transfer job not found: %s", jobID)
	}

	now := time.Now()
	job.State = TransferStateCompleted
	job.Transferred = transferred
	job.Bandwidth = bandwidth
	job.EndTime = &now
	job.Duration = now.Sub(job.StartTime).Milliseconds()

	m.stats.SuccessfulTransfers++
	m.stats.TotalBytes += transferred
	m.stats.ActiveJobs--

	if bandwidth > m.stats.MaxBandwidth {
		m.stats.MaxBandwidth = bandwidth
	}

	return nil
}

// FailTransfer 标记传输失败
func (m *Manager) FailTransfer(jobID string, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("transfer job not found: %s", jobID)
	}

	now := time.Now()
	job.State = TransferStateFailed
	job.ErrorMessage = errMsg
	job.EndTime = &now
	job.Duration = now.Sub(job.StartTime).Milliseconds()

	m.stats.FailedTransfers++
	m.stats.ActiveJobs--

	return nil
}

// CancelTransfer 取消传输
func (m *Manager) CancelTransfer(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("transfer job not found: %s", jobID)
	}

	if job.State != TransferStatePending && job.State != TransferStateRunning {
		return fmt.Errorf("cannot cancel transfer in state: %s", job.State)
	}

	now := time.Now()
	job.State = TransferStateCancelled
	job.EndTime = &now
	job.Duration = now.Sub(job.StartTime).Milliseconds()

	m.stats.ActiveJobs--

	return nil
}

// GetTransferJob 获取传输任务详情
func (m *Manager) GetTransferJob(jobID string) (*TransferJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("transfer job not found: %s", jobID)
	}
	return job, nil
}

// ListTransferJobs 列出传输任务
func (m *Manager) ListTransferJobs(state TransferState) []TransferJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []TransferJob
	for _, j := range m.jobs {
		if state == "" || j.State == state {
			jobs = append(jobs, *j)
		}
	}
	return jobs
}

// GetTransferStats 获取传输统计
func (m *Manager) GetTransferStats() TransferStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.AllocatedBuffers = len(m.buffers)
	return stats
}

// GetBandwidthStats 获取带宽统计
func (m *Manager) GetBandwidthStats(deviceID string) (*BandwidthStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[deviceID]; !ok {
		return nil, fmt.Errorf("GPU device not found: %s", deviceID)
	}

	// 计算该设备相关的带宽统计
	var totalBandwidth float64
	var count int
	var maxBW, minBW float64

	for _, job := range m.jobs {
		if (job.Source.DeviceID == deviceID || job.Destination.DeviceID == deviceID) &&
			job.State == TransferStateCompleted {
			totalBandwidth += job.Bandwidth
			count++
			if job.Bandwidth > maxBW {
				maxBW = job.Bandwidth
			}
			if minBW == 0 || job.Bandwidth < minBW {
				minBW = job.Bandwidth
			}
		}
	}

	stats := &BandwidthStats{
		DeviceID:     deviceID,
		MaxBandwidth: maxBW,
		MinBandwidth: minBW,
		SampleCount:  count,
		LastMeasured: time.Now(),
	}

	if count > 0 {
		stats.AvgBandwidth = totalBandwidth / float64(count)
	}

	return stats, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() GPUDSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config GPUDSConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}
