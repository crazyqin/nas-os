// Package gpu GPU调度器
package gpu

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler GPU调度器
type Scheduler struct {
	manager *Manager
	policy  string
	logger  *zap.Logger
	mu      sync.RWMutex
}

// NewScheduler 创建GPU调度器
func NewScheduler(manager *Manager, policy string, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}

	if policy == "" {
		policy = "round-robin"
	}

	return &Scheduler{
		manager: manager,
		policy:  policy,
		logger:  logger,
	}
}

// SetPolicy 设置调度策略
func (s *Scheduler) SetPolicy(policy string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policy = policy
	s.logger.Info("调度策略已更新", zap.String("policy", policy))
}

// GetPolicy 获取当前调度策略
func (s *Scheduler) GetPolicy() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy
}

// SelectGPU 选择合适的GPU设备
func (s *Scheduler) SelectGPU(req *GPUAllocation) (*GPUDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 直接访问manager的devices（不通过ListGPUs避免锁冲突）
	// 注意：调用者应该已经持有manager的锁
	availableGPUs := s.manager.getAvailableGPUsInternal()

	if len(availableGPUs) == 0 {
		return nil, fmt.Errorf("没有可用的GPU设备")
	}

	// 过滤满足显存要求的GPU
	if req.MemoryLimit > 0 {
		var filtered []*GPUDevice
		for _, gpu := range availableGPUs {
			if gpu.MemoryFree >= req.MemoryLimit {
				filtered = append(filtered, gpu)
			}
		}
		availableGPUs = filtered
	}

	if len(availableGPUs) == 0 {
		return nil, fmt.Errorf("没有满足显存要求的GPU设备 (需要 %d MB)", req.MemoryLimit)
	}

	// 根据策略选择GPU
	switch s.policy {
	case "round-robin":
		return s.selectRoundRobin(availableGPUs, req)
	case "priority":
		return s.selectPriority(availableGPUs, req)
	case "exclusive":
		return s.selectExclusive(availableGPUs, req)
	case "least-loaded":
		return s.selectLeastLoaded(availableGPUs, req)
	case "most-memory":
		return s.selectMostMemory(availableGPUs, req)
	default:
		// 默认使用轮询
		return s.selectRoundRobin(availableGPUs, req)
	}
}

// selectRoundRobin 轮询选择策略
func (s *Scheduler) selectRoundRobin(gpus []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	// 按ID排序以确保顺序一致
	sort.Slice(gpus, func(i, j int) bool {
		return gpus[i].ID < gpus[j].ID
	})

	// 使用时间戳作为随机选择（简化版轮询）
	index := time.Now().Unix() % int64(len(gpus))
	selected := gpus[index]

	s.logger.Debug("轮询选择GPU",
		zap.String("gpuId", selected.ID),
		zap.Int("index", int(index)))

	return selected, nil
}

// selectPriority 优先级选择策略
func (s *Scheduler) selectPriority(gpus []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	// 高优先级任务优先选择高性能GPU
	// 低优先级任务选择性能较低的GPU

	// 按CUDA核心数排序（高性能GPU优先）
	sort.Slice(gpus, func(i, j int) bool {
		return gpus[i].CUDAcores > gpus[j].CUDAcores
	})

	// 关键优先级选择最强GPU
	if req.Priority == PriorityCritical {
		return gpus[0], nil
	}

	// 高优先级选择前1/3的GPU
	if req.Priority == PriorityHigh {
		highEnd := len(gpus) / 3
		if highEnd == 0 {
			highEnd = 1
		}
		selected := gpus[rand.Intn(highEnd)]
		return selected, nil
	}

	// 正常优先级选择中等GPU
	if req.Priority == PriorityNormal {
		start := len(gpus) / 3
		end := len(gpus) * 2 / 3
		if start == end {
			start = 0
			end = len(gpus)
		}
		selected := gpus[start+rand.Intn(end-start)]
		return selected, nil
	}

	// 低优先级选择性能较低的GPU（后1/3）
	lowEnd := len(gpus) * 2 / 3
	if lowEnd >= len(gpus) {
		lowEnd = 0
	}
	selected := gpus[lowEnd+rand.Intn(len(gpus)-lowEnd)]
	return selected, nil
}

// selectExclusive 独占选择策略
func (s *Scheduler) selectExclusive(gpus []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	if !req.Exclusive {
		return nil, fmt.Errorf("独占模式需要设置Exclusive标志")
	}

	// 独占模式下选择完全空闲的GPU
	// 按显存利用率排序，选择利用率最低的
	sort.Slice(gpus, func(i, j int) bool {
		memUtilI := float64(gpus[i].MemoryUsed) / float64(gpus[i].MemoryTotal)
		memUtilJ := float64(gpus[j].MemoryUsed) / float64(gpus[j].MemoryTotal)
		return memUtilI < memUtilJ
	})

	// 选择利用率最低的GPU
	selected := gpus[0]

	s.logger.Info("独占模式选择GPU",
		zap.String("gpuId", selected.ID),
		zap.Uint64("memoryFree", selected.MemoryFree))

	return selected, nil
}

// selectLeastLoaded 最小负载选择策略
func (s *Scheduler) selectLeastLoaded(gpus []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	// 综合考虑显存使用率和功耗
	sort.Slice(gpus, func(i, j int) bool {
		// 计算负载评分（越低越好）
		scoreI := s.calculateLoadScore(gpus[i])
		scoreJ := s.calculateLoadScore(gpus[j])
		return scoreI < scoreJ
	})

	// 选择负载最低的GPU
	selected := gpus[0]

	s.logger.Debug("最小负载选择GPU",
		zap.String("gpuId", selected.ID),
		zap.Float64("loadScore", s.calculateLoadScore(selected)))

	return selected, nil
}

// selectMostMemory 最大显存选择策略
func (s *Scheduler) selectMostMemory(gpus []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	// 按可用显存排序
	sort.Slice(gpus, func(i, j int) bool {
		return gpus[i].MemoryFree > gpus[j].MemoryFree
	})

	// 选择可用显存最多的GPU
	selected := gpus[0]

	s.logger.Debug("最大显存选择GPU",
		zap.String("gpuId", selected.ID),
		zap.Uint64("memoryFree", selected.MemoryFree))

	return selected, nil
}

// calculateLoadScore 计算GPU负载评分
func (s *Scheduler) calculateLoadScore(gpu *GPUDevice) float64 {
	// 考虑因素：
	// 1. 显存使用率 (权重 0.4)
	// 2. 功耗率 (权重 0.3)
	// 3. 温度 (权重 0.2)
	// 4. CUDA核心压力 (权重 0.1) - 基于分配情况估算

	memUtil := 0.0
	if gpu.MemoryTotal > 0 {
		memUtil = float64(gpu.MemoryUsed) / float64(gpu.MemoryTotal)
	}

	powerUtil := 0.0
	if gpu.PowerLimit > 0 {
		powerUtil = float64(gpu.PowerUsage) / float64(gpu.PowerLimit)
	}

	tempUtil := float64(gpu.Temperature) / 100.0 // 假设100°C为上限

	// 综合评分
	score := memUtil*0.4 + powerUtil*0.3 + tempUtil*0.2 + 0.1 // 最后0.1为基础负载

	return score
}

// AllocationPolicyRoundRobin 轮询分配策略实现
type AllocationPolicyRoundRobin struct{}

func (p *AllocationPolicyRoundRobin) SelectGPU(devices []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("没有可用GPU")
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})

	index := time.Now().Unix() % int64(len(devices))
	return devices[index], nil
}

func (p *AllocationPolicyRoundRobin) Name() string {
	return "round-robin"
}

// AllocationPolicyLeastLoaded 最小负载分配策略实现
type AllocationPolicyLeastLoaded struct{}

func (p *AllocationPolicyLeastLoaded) SelectGPU(devices []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("没有可用GPU")
	}

	// 简化版：选择显存使用率最低的
	sort.Slice(devices, func(i, j int) bool {
		memUtilI := float64(devices[i].MemoryUsed) / float64(devices[i].MemoryTotal)
		memUtilJ := float64(devices[j].MemoryUsed) / float64(devices[j].MemoryTotal)
		return memUtilI < memUtilJ
	})

	return devices[0], nil
}

func (p *AllocationPolicyLeastLoaded) Name() string {
	return "least-loaded"
}

// AllocationPolicyPriority 优先级分配策略实现
type AllocationPolicyPriority struct{}

func (p *AllocationPolicyPriority) SelectGPU(devices []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("没有可用GPU")
	}

	// 按CUDA核心数排序
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CUDAcores > devices[j].CUDAcores
	})

	// 高优先级选择高性能GPU
	if req.Priority == PriorityCritical || req.Priority == PriorityHigh {
		return devices[0], nil
	}

	// 低优先级选择低性能GPU
	if req.Priority == PriorityLow {
		return devices[len(devices)-1], nil
	}

	// 正常优先级随机选择
	return devices[rand.Intn(len(devices))], nil
}

func (p *AllocationPolicyPriority) Name() string {
	return "priority"
}

// AllocationPolicyExclusive 独占分配策略实现
type AllocationPolicyExclusive struct{}

func (p *AllocationPolicyExclusive) SelectGPU(devices []*GPUDevice, req *GPUAllocation) (*GPUDevice, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("没有可用GPU")
	}

	if !req.Exclusive {
		return nil, fmt.Errorf("独占策略需要Exclusive标志")
	}

	// 选择完全空闲、性能最好的GPU
	sort.Slice(devices, func(i, j int) bool {
		// 先按分配状态排序（空闲优先）
		if devices[i].Allocated != devices[j].Allocated {
			return !devices[i].Allocated
		}
		// 然后按CUDA核心数排序
		return devices[i].CUDAcores > devices[j].CUDAcores
	})

	return devices[0], nil
}

func (p *AllocationPolicyExclusive) Name() string {
	return "exclusive"
}
