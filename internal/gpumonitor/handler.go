package gpumonitor

import (
	"fmt"
	"sync"
	"time"
)

// GPUVendor GPU厂商
type GPUVendor string

const (
	VendorNVIDIA GPUVendor = "nvidia"
	VendorAMD    GPUVendor = "amd"
	VendorIntel  GPUVendor = "intel"
)

// GPUStatus GPU状态
type GPU struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Vendor       GPUVendor  `json:"vendor"`
	Driver       string     `json:"driver"`
	MemTotalMB   int        `json:"memTotalMb"`
	MemUsedMB    int        `json:"memUsedMb"`
	MemFreeMB    int        `json:"memFreeMb"`
	TempCelsius  int        `json:"tempCelsius"`
	FanSpeedRPM  int        `json:"fanSpeedRpm"`
	PowerDrawW   int        `json:"powerDrawW"`
	PowerLimitW  int        `json:"powerLimitW"`
	ClockCoreMHz int        `json:"clockCoreMHz"`
	ClockMemMHz  int        `json:"clockMemMHz"`
	UtilGPU      int        `json:"utilGpu"`
	UtilMem      int        `json:"utilMem"`
	ProcessCount int        `json:"processCount"`
	Healthy      bool       `json:"healthy"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// GPUProcess GPU进程
type GPUProcess struct {
	PID        int    `json:"pid"`
	Name       string `json:"name"`
	GPUID      string `json:"gpuId"`
	MemMB      int    `json:"memMb"`
	Type       string `json:"type"`
}

// GPUStats GPU历史统计
type GPUStats struct {
	GPUID       string    `json:"gpuId"`
	Timestamp   time.Time `json:"timestamp"`
	AvgTemp     float64   `json:"avgTemp"`
	MaxTemp     int       `json:"maxTemp"`
	AvgUtil     float64   `json:"avgUtil"`
	AvgMemUsed  float64   `json:"avgMemUsed"`
	AvgPower    float64   `json:"avgPower"`
}

// Monitor GPU监控器
type Monitor struct {
	mu       sync.RWMutex
	gpus     map[string]*GPU
	processes map[string][]*GPUProcess
	history   map[string][]*GPUStats
	alerts    []GPUAlert
}

// GPUAlert GPU告警
type GPUAlert struct {
	GPUID     string    `json:"gpuId"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewMonitor 创建监控器
func NewMonitor() *Monitor {
	return &Monitor{
		gpus:     make(map[string]*GPU),
		processes: make(map[string][]*GPUProcess),
		history:   make(map[string][]*GPUStats),
	}
}

// GetGPUs 获取所有GPU
func (m *Monitor) GetGPUs() []*GPU {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gpus := make([]*GPU, 0, len(m.gpus))
	for _, g := range m.gpus {
		gpus = append(gpus, g)
	}
	return gpus
}

// GetGPU 获取指定GPU
func (m *Monitor) GetGPU(id string) (*GPU, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gpu, ok := m.gpus[id]
	if !ok {
		return nil, fmt.Errorf("gpu %s not found", id)
	}
	return gpu, nil
}

// GetGPUProcesses 获取GPU进程
func (m *Monitor) GetGPUProcesses(gpuID string) ([]*GPUProcess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.gpus[gpuID]; !ok {
		return nil, fmt.Errorf("gpu %s not found", gpuID)
	}
	procs := m.processes[gpuID]
	if procs == nil {
		return []*GPUProcess{}, nil
	}
	return procs, nil
}

// GetGPUHistory 获取GPU历史统计
func (m *Monitor) GetGPUHistory(gpuID string, hours int) ([]*GPUStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.gpus[gpuID]; !ok {
		return nil, fmt.Errorf("gpu %s not found", gpuID)
	}
	history := m.history[gpuID]
	if history == nil {
		return []*GPUStats{}, nil
	}
	return history, nil
}

// GetAlerts 获取GPU告警
func (m *Monitor) GetAlerts() []GPUAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// SetPowerLimit 设置功耗限制
func (m *Monitor) SetPowerLimit(gpuID string, watts int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gpu, ok := m.gpus[gpuID]
	if !ok {
		return fmt.Errorf("gpu %s not found", gpuID)
	}
	gpu.PowerLimitW = watts
	return nil
}

// GetGPUSummary 获取GPU概要
func (m *Monitor) GetGPUSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.gpus)
	healthy := 0
	totalMemMB := 0
	usedMemMB := 0
	for _, g := range m.gpus {
		if g.Healthy {
			healthy++
		}
		totalMemMB += g.MemTotalMB
		usedMemMB += g.MemUsedMB
	}
	return map[string]interface{}{
		"total":      total,
		"healthy":    healthy,
		"unhealthy":  total - healthy,
		"totalMemMb": totalMemMB,
		"usedMemMb":  usedMemMB,
		"alerts":     len(m.alerts),
	}
}
