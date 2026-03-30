package gpu

import (
	"sync"
	"time"
)

// GPU架构类型 - 参考TrueNAS 25.10对Blackwell架构的支持
type Architecture string

const (
	ArchAmpere    Architecture = "ampere"     // RTX 30系列, A100等
	ArchAda       Architecture = "ada"        // RTX 40系列, L40等
	ArchHopper    Architecture = "hopper"     // H100, H200等
	ArchBlackwell Architecture = "blackwell"  // B100, B200等 - TrueNAS 25.10支持
	ArchUnknown   Architecture = "unknown"
)

// GPU状态
type GPUStatus string

const (
	StatusIdle      GPUStatus = "idle"
	StatusBusy      GPUStatus = "busy"
	StatusReserved  GPUStatus = "reserved"
	StatusOffline   GPUStatus = "offline"
	StatusError     GPUStatus = "error"
)

// GPU设备信息
type GPUDevice struct {
	ID           string       `json:"id"`            // GPU UUID
	Index        int          `json:"index"`         // GPU索引（物理位置）
	Name         string       `json:"name"`          // GPU名称
	Architecture Architecture `json:"architecture"`  // 架构类型
	
	// 显存信息
	TotalMemory     uint64 `json:"total_memory"`     // 总显存(bytes)
	UsedMemory      uint64 `json:"used_memory"`      // 已用显存(bytes)
	AvailableMemory uint64 `json:"available_memory"` // 可用显存(bytes)
	
	// 计算能力
	ComputeCapability string `json:"compute_capability"` // 如 8.0, 9.0
	CUDACores        int    `json:"cuda_cores"`         // CUDA核心数
	TensorCores      int    `json:"tensor_cores"`       // Tensor核心数
	SMCount          int    `json:"sm_count"`           // SM数量
	
	// 性能指标
	Temperature    uint   `json:"temperature"`    // 温度(摄氏度)
	PowerUsage     uint   `json:"power_usage"`   // 功耗(W)
	PowerLimit     uint   `json:"power_limit"`   // 功耗限制(W)
	UtilizationGPU uint   `json:"utilization_gpu"` // GPU利用率(%)
	UtilizationMem uint   `json:"utilization_mem"` // 显存利用率(%)
	FanSpeed       uint   `json:"fan_speed"`     // 风扇转速(%)
	
	// 状态管理
	Status      GPUStatus `json:"status"`
	ReservedBy  string     `json:"reserved_by,omitempty"` // 预留者任务ID
	ReservedAt  *time.Time `json:"reserved_at,omitempty"`
	
	// 亲和性标签 - 参考Unraid灵活混用策略
	Labels map[string]string `json:"labels,omitempty"`
	
	// 驱动信息
	DriverVersion  string `json:"driver_version"`
	CUDAVersion    string `json:"cuda_version"`
	PCIeBandwidth  string `json:"pcie_bandwidth"` // PCIe带宽
	
	// NUMA信息（多GPU系统）
	NUMANode int `json:"numa_node"`
	
	mu sync.RWMutex
}

// CanAllocate 检查是否可以分配显存
func (g *GPUDevice) CanAllocate(memory uint64) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return g.Status != StatusOffline && 
		g.Status != StatusError &&
		g.AvailableMemory >= memory
}

// Allocate 分配显存
func (g *GPUDevice) Allocate(memory uint64, taskID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.Status == StatusOffline || g.Status == StatusError {
		return false
	}
	
	if g.AvailableMemory < memory {
		return false
	}
	
	g.UsedMemory += memory
	g.AvailableMemory -= memory
	g.Status = StatusBusy
	g.ReservedBy = taskID
	now := time.Now()
	g.ReservedAt = &now
	
	return true
}

// Release 释放显存
func (g *GPUDevice) Release(memory uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.UsedMemory < memory {
		g.UsedMemory = 0
	} else {
		g.UsedMemory -= memory
	}
	
	g.AvailableMemory = g.TotalMemory - g.UsedMemory
	g.ReservedBy = ""
	g.ReservedAt = nil
	
	if g.UsedMemory == 0 {
		g.Status = StatusIdle
	}
}

// Reserve 预留GPU
func (g *GPUDevice) Reserve(taskID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.Status != StatusIdle {
		return false
	}
	
	g.Status = StatusReserved
	g.ReservedBy = taskID
	now := time.Now()
	g.ReservedAt = &now
	
	return true
}

// Unreserve 取消预留
func (g *GPUDevice) Unreserve() {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.Status == StatusReserved {
		g.Status = StatusIdle
		g.ReservedBy = ""
		g.ReservedAt = nil
	}
}

// UpdateMetrics 更新性能指标
func (g *GPUDevice) UpdateMetrics(temp, power, gpuUtil, memUtil, fan uint) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.Temperature = temp
	g.PowerUsage = power
	g.UtilizationGPU = gpuUtil
	g.UtilizationMem = memUtil
	g.FanSpeed = fan
}

// MatchesRequirements 检查是否满足需求
func (g *GPUDevice) MatchesRequirements(req *GPURequirements) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// 检查架构
	if len(req.Architectures) > 0 {
		found := false
		for _, arch := range req.Architectures {
			if g.Architecture == arch {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// 检查显存
	if req.MinMemory > 0 && g.TotalMemory < req.MinMemory {
		return false
	}
	
	// 检查计算能力
	if req.MinComputeCapability != "" && g.ComputeCapability < req.MinComputeCapability {
		return false
	}
	
	// 检查CUDA核心
	if req.MinCUDACores > 0 && g.CUDACores < req.MinCUDACores {
		return false
	}
	
	// 检查标签
	for k, v := range req.Labels {
		if g.Labels[k] != v {
			return false
		}
	}
	
	// 检查NUMA亲和性
	if req.NUMANode >= 0 && g.NUMANode != req.NUMANode {
		return false
	}
	
	return true
}

// GPURequirements GPU需求规格
type GPURequirements struct {
	Architectures       []Architecture     `json:"architectures,omitempty"`
	MinMemory           uint64             `json:"min_memory,omitempty"`           // 最小显存(bytes)
	MaxMemory           uint64             `json:"max_memory,omitempty"`           // 最大显存(bytes)
	MinComputeCapability string            `json:"min_compute_capability,omitempty"`
	MinCUDACores        int                `json:"min_cuda_cores,omitempty"`
	MinTensorCores      int                `json:"min_tensor_cores,omitempty"`
	Labels              map[string]string  `json:"labels,omitempty"`
	NUMANode            int                `json:"numa_node,omitempty"`            // -1表示不限制
	ExclusiveMode       bool               `json:"exclusive_mode"`                 // 是否独占模式
}

// Clone 克隆GPU设备信息
func (g *GPUDevice) Clone() *GPUDevice {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	labels := make(map[string]string)
	for k, v := range g.Labels {
		labels[k] = v
	}
	
	return &GPUDevice{
		ID:                 g.ID,
		Index:              g.Index,
		Name:               g.Name,
		Architecture:       g.Architecture,
		TotalMemory:        g.TotalMemory,
		UsedMemory:        g.UsedMemory,
		AvailableMemory:   g.AvailableMemory,
		ComputeCapability: g.ComputeCapability,
		CUDACores:         g.CUDACores,
		TensorCores:       g.TensorCores,
		SMCount:           g.SMCount,
		Temperature:       g.Temperature,
		PowerUsage:        g.PowerUsage,
		PowerLimit:        g.PowerLimit,
		UtilizationGPU:    g.UtilizationGPU,
		UtilizationMem:    g.UtilizationMem,
		FanSpeed:          g.FanSpeed,
		Status:            g.Status,
		ReservedBy:        g.ReservedBy,
		ReservedAt:        g.ReservedAt,
		Labels:            labels,
		DriverVersion:     g.DriverVersion,
		CUDAVersion:       g.CUDAVersion,
		PCIeBandwidth:     g.PCIeBandwidth,
		NUMANode:          g.NUMANode,
	}
}

// ToMap 转换为map便于JSON序列化
func (g *GPUDevice) ToMap() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return map[string]interface{}{
		"id":                g.ID,
		"index":             g.Index,
		"name":              g.Name,
		"architecture":      g.Architecture,
		"total_memory":      g.TotalMemory,
		"used_memory":       g.UsedMemory,
		"available_memory":  g.AvailableMemory,
		"compute_capability": g.ComputeCapability,
		"cuda_cores":        g.CUDACores,
		"tensor_cores":      g.TensorCores,
		"sm_count":          g.SMCount,
		"temperature":       g.Temperature,
		"power_usage":       g.PowerUsage,
		"power_limit":       g.PowerLimit,
		"utilization_gpu":   g.UtilizationGPU,
		"utilization_mem":   g.UtilizationMem,
		"fan_speed":         g.FanSpeed,
		"status":            g.Status,
		"reserved_by":       g.ReservedBy,
		"labels":            g.Labels,
		"numa_node":         g.NUMANode,
	}
}