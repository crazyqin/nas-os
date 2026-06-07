package lxcorchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler 资源调度器
type Scheduler struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	orchestrator *Orchestrator
	allocations  map[string]*ResourceAllocation
	totalCPU     int
	totalMemory  int64
	totalIO      int
	usedCPU      int
	usedMemory   int64
	usedIO       int
}

// ResourceAllocation 资源分配记录
type ResourceAllocation struct {
	ContainerID string         `json:"container_id"`
	CPUShares   int            `json:"cpu_shares"`
	CPUQuota    int            `json:"cpu_quota"`
	MemoryLimit int64          `json:"memory_limit"`
	MemorySwap  int64          `json:"memory_swap"`
	IOWeight    int            `json:"io_weight"`
	AllocatedAt time.Time      `json:"allocated_at"`
	Resources   ResourceLimits `json:"resources"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	TotalCPU        int     `json:"total_cpu"`        // CPU 核心数 * 1024
	TotalMemory     int64   `json:"total_memory"`     // 总内存字节
	TotalIO         int     `json:"total_io"`         // IO 权重总和
	OvercommitRatio float64 `json:"overcommit_ratio"` // 超配比例
}

// NewScheduler 创建资源调度器
func NewScheduler(logger *zap.Logger, orchestrator *Orchestrator) *Scheduler {
	return &Scheduler{
		logger:       logger,
		orchestrator: orchestrator,
		allocations:  make(map[string]*ResourceAllocation),
		totalCPU:     4096,                   // 默认 4 核
		totalMemory:  8 * 1024 * 1024 * 1024, // 默认 8GB
		totalIO:      1000,
	}
}

// Run 运行调度器
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rebalance(ctx)
		}
	}
}

// AllocateResources 分配资源
func (s *Scheduler) AllocateResources(container *ContainerInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config := container.Config.Resources

	// 计算资源需求
	cpuShares := config.CPUShares
	if cpuShares == 0 {
		cpuShares = 1024 // 默认份额
	}
	if cpuShares < 10 || cpuShares > 1024 {
		return fmt.Errorf("invalid CPU shares: %d (must be 10-1024)", cpuShares)
	}

	cpuQuota := config.CPUQuota
	if cpuQuota == 0 {
		cpuQuota = s.totalCPU / 4 // 默认 25% CPU
	}

	memoryLimit := config.MemoryLimit
	if memoryLimit == 0 {
		memoryLimit = 512 * 1024 * 1024 // 默认 512MB
	}

	memorySwap := config.MemorySwap
	if memorySwap == 0 {
		memorySwap = memoryLimit * 2
	}

	ioWeight := config.IOWeight
	if ioWeight == 0 {
		ioWeight = 500
	}
	if ioWeight < 10 || ioWeight > 1000 {
		return fmt.Errorf("invalid IO weight: %d (must be 10-1000)", ioWeight)
	}

	// 检查资源是否足够
	if s.usedMemory+memoryLimit > int64(float64(s.totalMemory)*1.5) {
		return fmt.Errorf("insufficient memory: requested %d MB, available %d MB",
			memoryLimit/(1024*1024), (s.totalMemory-s.usedMemory)/(1024*1024))
	}

	// 分配资源
	allocation := &ResourceAllocation{
		ContainerID: container.Config.ID,
		CPUShares:   cpuShares,
		CPUQuota:    cpuQuota,
		MemoryLimit: memoryLimit,
		MemorySwap:  memorySwap,
		IOWeight:    ioWeight,
		AllocatedAt: time.Now(),
		Resources: ResourceLimits{
			CPUShares:   cpuShares,
			CPUQuota:    cpuQuota,
			MemoryLimit: memoryLimit,
			MemorySwap:  memorySwap,
			IOWeight:    ioWeight,
		},
	}

	s.allocations[container.Config.ID] = allocation
	s.usedCPU += cpuShares
	s.usedMemory += memoryLimit
	s.usedIO += ioWeight

	// 更新容器资源配置
	container.Config.Resources = allocation.Resources

	s.logger.Info("resources allocated",
		zap.String("container_id", container.Config.ID),
		zap.Int("cpu_shares", cpuShares),
		zap.Int64("memory_mb", memoryLimit/(1024*1024)),
		zap.Int("io_weight", ioWeight),
	)

	return nil
}

// ReleaseResources 释放资源
func (s *Scheduler) ReleaseResources(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	allocation, exists := s.allocations[containerID]
	if !exists {
		return
	}

	s.usedCPU -= allocation.CPUShares
	s.usedMemory -= allocation.MemoryLimit
	s.usedIO -= allocation.IOWeight

	delete(s.allocations, containerID)

	s.logger.Info("resources released",
		zap.String("container_id", containerID),
		zap.Int64("memory_mb", allocation.MemoryLimit/(1024*1024)),
	)
}

// UpdateResources 更新资源分配
func (s *Scheduler) UpdateResources(container *ContainerInstance, newLimits ResourceLimits) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allocation, exists := s.allocations[container.Config.ID]
	if !exists {
		return fmt.Errorf("container not allocated: %s", container.Config.ID)
	}

	// 计算差值
	cpuDelta := newLimits.CPUShares - allocation.CPUShares
	memoryDelta := newLimits.MemoryLimit - allocation.MemoryLimit
	ioDelta := newLimits.IOWeight - allocation.IOWeight

	// 检查资源是否足够
	if s.usedMemory+memoryDelta > int64(float64(s.totalMemory)*1.5) {
		return fmt.Errorf("insufficient memory for update")
	}

	// 应用更新
	s.usedCPU += cpuDelta
	s.usedMemory += memoryDelta
	s.usedIO += ioDelta

	allocation.CPUShares = newLimits.CPUShares
	allocation.CPUQuota = newLimits.CPUQuota
	allocation.MemoryLimit = newLimits.MemoryLimit
	allocation.MemorySwap = newLimits.MemorySwap
	allocation.IOWeight = newLimits.IOWeight
	allocation.Resources = newLimits

	container.Config.Resources = newLimits

	s.logger.Info("resources updated",
		zap.String("container_id", container.Config.ID),
		zap.Int64("memory_delta_mb", memoryDelta/(1024*1024)),
	)

	return nil
}

// GetAllocation 获取资源分配
func (s *Scheduler) GetAllocation(containerID string) (*ResourceAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allocation, exists := s.allocations[containerID]
	if !exists {
		return nil, fmt.Errorf("container not allocated: %s", containerID)
	}

	return allocation, nil
}

// GetResourceUsage 获取资源使用情况
func (s *Scheduler) GetResourceUsage() *ResourceUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &ResourceUsage{
		TotalCPU:        float64(s.usedCPU) / float64(s.totalCPU) * 100,
		TotalMemory:     s.usedMemory,
		AvailableCPU:    float64(s.totalCPU-s.usedCPU) / float64(s.totalCPU) * 100,
		AvailableMemory: s.totalMemory - s.usedMemory,
	}
}

// GetTotalResources 获取总资源
func (s *Scheduler) GetTotalResources() (cpu int, memory int64, io int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalCPU, s.totalMemory, s.totalIO
}

// SetTotalResources 设置总资源
func (s *Scheduler) SetTotalResources(cpu int, memory int64, io int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalCPU = cpu
	s.totalMemory = memory
	s.totalIO = io
	s.logger.Info("total resources updated",
		zap.Int("cpu", cpu),
		zap.Int64("memory_mb", memory/(1024*1024)),
		zap.Int("io", io),
	)
}

// rebalance 重新平衡资源
func (s *Scheduler) rebalance(ctx context.Context) {
	s.mu.RLock()
	allocations := make([]*ResourceAllocation, 0, len(s.allocations))
	for _, a := range s.allocations {
		allocations = append(allocations, a)
	}
	s.mu.RUnlock()

	if len(allocations) == 0 {
		return
	}

	// 检查资源使用率
	usage := s.GetResourceUsage()
	if usage.AvailableMemory < 100*1024*1024 { // 少于 100MB
		s.logger.Warn("low memory available",
			zap.Int64("available_mb", usage.AvailableMemory/(1024*1024)),
		)
	}

	// 记录资源状态
	s.logger.Debug("scheduler rebalance",
		zap.Float64("cpu_usage_percent", usage.TotalCPU),
		zap.Int64("memory_usage_mb", usage.TotalMemory/(1024*1024)),
		zap.Int("container_count", len(allocations)),
	)
}

// ValidateResourceLimits 验证资源限制
func ValidateResourceLimits(limits ResourceLimits) error {
	if limits.CPUShares != 0 && (limits.CPUShares < 10 || limits.CPUShares > 1024) {
		return fmt.Errorf("invalid CPU shares: %d (must be 10-1024)", limits.CPUShares)
	}

	if limits.CPUQuota < 0 {
		return fmt.Errorf("invalid CPU quota: %d (must be >= 0)", limits.CPUQuota)
	}

	if limits.MemoryLimit < 0 {
		return fmt.Errorf("invalid memory limit: %d (must be >= 0)", limits.MemoryLimit)
	}

	if limits.MemorySwap < 0 {
		return fmt.Errorf("invalid memory swap: %d (must be >= 0)", limits.MemorySwap)
	}

	if limits.MemorySwap != 0 && limits.MemoryLimit != 0 && limits.MemorySwap < limits.MemoryLimit {
		return fmt.Errorf("memory swap (%d) must be >= memory limit (%d)", limits.MemorySwap, limits.MemoryLimit)
	}

	if limits.IOWeight != 0 && (limits.IOWeight < 10 || limits.IOWeight > 1000) {
		return fmt.Errorf("invalid IO weight: %d (must be 10-1000)", limits.IOWeight)
	}

	if limits.ProcLimit < 0 {
		return fmt.Errorf("invalid proc limit: %d (must be >= 0)", limits.ProcLimit)
	}

	if limits.OpenFiles < 0 {
		return fmt.Errorf("invalid open files limit: %d (must be >= 0)", limits.OpenFiles)
	}

	return nil
}

// EstimateResourceUsage 估算资源使用
func EstimateResourceUsage(config ContainerConfig) ResourceLimits {
	limits := config.Resources

	// 设置默认值
	if limits.CPUShares == 0 {
		limits.CPUShares = 1024
	}
	if limits.CPUQuota == 0 {
		limits.CPUQuota = 100000 // 100ms per 100ms = 100% CPU
	}
	if limits.MemoryLimit == 0 {
		limits.MemoryLimit = 512 * 1024 * 1024 // 512MB
	}
	if limits.MemorySwap == 0 {
		limits.MemorySwap = limits.MemoryLimit * 2
	}
	if limits.IOWeight == 0 {
		limits.IOWeight = 500
	}
	if limits.ProcLimit == 0 {
		limits.ProcLimit = 256
	}
	if limits.OpenFiles == 0 {
		limits.OpenFiles = 1024
	}

	return limits
}
