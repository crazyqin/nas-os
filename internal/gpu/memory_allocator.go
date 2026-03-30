package gpu

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// MemoryAllocator 显存动态分配器
// 实现GPU显存动态分配、碎片整理和超卖管理
type MemoryAllocator struct {
	manager *GPUManager
	
	// 配置
	config *AllocatorConfig
	
	// 分配记录
	allocations map[string]*MemoryAllocation // 按分配ID索引
	byTask      map[string][]*MemoryAllocation // 按任务ID索引
	
	// 等待队列
	waitQueue []*MemoryRequest
	
	// 统计
	stats *AllocatorStats
	
	mu sync.Mutex
}

// AllocatorConfig 分配器配置
type AllocatorConfig struct {
	// 基本配置
	MinAllocationSize uint64 `json:"min_allocation_size"` // 最小分配粒度(bytes)
	MaxAllocationSize uint64 `json:"max_allocation_size"` // 最大分配大小(bytes)
	
	// 超卖配置
	EnableOvercommit  bool   `json:"enable_overcommit"`  // 是否启用超卖
	OvercommitRatio   float64 `json:"overcommit_ratio"`  // 超卖比例(如1.2表示允许120%)
	
	// 预留配置
	ReservedPercent   float64 `json:"reserved_percent"`  // 预留比例(如0.1表示预留10%)
	ReservedAbsolute  uint64  `json:"reserved_absolute"` // 绝对预留量(bytes)
	
	// 碎片整理
	EnableDefrag      bool          `json:"enable_defrag"`    // 是否启用碎片整理
	DefragThreshold   float64       `json:"defrag_threshold"` // 碎片率阈值
	DefragInterval    time.Duration `json:"defrag_interval"`  // 整理间隔
	
	// 回收策略
	IdleTimeout       time.Duration `json:"idle_timeout"`     // 空闲超时回收
	EnableGraceful    bool          `json:"enable_graceful"`  // 优雅回收
}

// DefaultAllocatorConfig 默认配置
func DefaultAllocatorConfig() *AllocatorConfig {
	return &AllocatorConfig{
		MinAllocationSize:  256 * 1024 * 1024,  // 256MB
		MaxAllocationSize:  64 * 1024 * 1024 * 1024, // 64GB
		EnableOvercommit:   false,
		OvercommitRatio:    1.2,
		ReservedPercent:    0.05,
		ReservedAbsolute:   512 * 1024 * 1024, // 512MB
		EnableDefrag:       true,
		DefragThreshold:    0.3,
		DefragInterval:     5 * time.Minute,
		IdleTimeout:        30 * time.Minute,
		EnableGraceful:     true,
	}
}

// MemoryAllocation 显存分配记录
type MemoryAllocation struct {
	ID           string     `json:"id"`
	GPUID        string     `json:"gpu_id"`
	GPUIndex     int        `json:"gpu_index"`
	TaskID       string     `json:"task_id"`
	
	// 分配信息
	Size         uint64     `json:"size"`          // 分配大小(bytes)
	Offset       uint64     `json:"offset"`        // 分配偏移（可选）
	Requested    uint64     `json:"requested"`     // 实际请求大小
	
	// 状态
	Status       AllocationStatus `json:"status"`
	AllocatedAt  time.Time        `json:"allocated_at"`
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"`
	ReleasedAt   *time.Time       `json:"released_at,omitempty"`
	
	// 优先级
	Priority     int              `json:"priority"`
	
	// 碎片信息
	IsFragment   bool             `json:"is_fragment"`
}

// AllocationStatus 分配状态
type AllocationStatus string

const (
	AllocationActive   AllocationStatus = "active"
	AllocationExpired  AllocationStatus = "expired"
	AllocationReleased AllocationStatus = "released"
	AllocationPending  AllocationStatus = "pending"
)

// MemoryRequest 显存请求
type MemoryRequest struct {
	ID         string
	TaskID     string
	GPUID      string        // 可选，指定GPU
	Size       uint64        // 请求大小
	Priority   int           // 优先级
	ExpiresAt  *time.Time    // 超时时间
	Exclusive  bool          // 是否独占
	
	// 回调
	OnAllocated func(alloc *MemoryAllocation)
	OnTimeout   func()
	
	// 内部状态
	submittedAt time.Time
	allocated   *MemoryAllocation
}

// AllocatorStats 分配器统计
type AllocatorStats struct {
	// 分配统计
	TotalAllocations   int64   `json:"total_allocations"`
	TotalAllocated     uint64  `json:"total_allocated_bytes"`
	TotalReleased      uint64  `json:"total_released_bytes"`
	ActiveAllocations  int     `json:"active_allocations"`
	
	// 等待队列
	WaitQueueLength    int     `json:"wait_queue_length"`
	WaitQueueTotal     uint64  `json:"wait_queue_total_bytes"`
	
	// 超卖统计
	OvercommitUsed     uint64  `json:"overcommit_used_bytes"`
	OvercommitLimit    uint64  `json:"overcommit_limit_bytes"`
	OvercommitRatio    float64 `json:"overcommit_ratio"`
	
	// 碎片统计
	FragmentCount      int     `json:"fragment_count"`
	FragmentRate       float64 `json:"fragment_rate"` // 碎片率
	
	// 性能
	AvgAllocationTime  int64   `json:"avg_allocation_time_ms"`
	AvgReleaseTime     int64   `json:"avg_release_time_ms"`
	
	// 失败统计
	AllocationFailures int64   `json:"allocation_failures"`
	TimeoutFailures    int64   `json:"timeout_failures"`
}

// NewMemoryAllocator 创建分配器
func NewMemoryAllocator(manager *GPUManager, config *AllocatorConfig) *MemoryAllocator {
	if config == nil {
		config = DefaultAllocatorConfig()
	}
	
	return &MemoryAllocator{
		manager:     manager,
		config:      config,
		allocations: make(map[string]*MemoryAllocation),
		byTask:      make(map[string][]*MemoryAllocation),
		waitQueue:   make([]*MemoryRequest, 0),
		stats:       &AllocatorStats{},
	}
}

// Allocate 分配显存
func (a *MemoryAllocator) Allocate(ctx context.Context, req *MemoryRequest) (*MemoryAllocation, error) {
	// 验证请求大小
	if req.Size < a.config.MinAllocationSize {
		req.Size = a.config.MinAllocationSize
	}
	if req.Size > a.config.MaxAllocationSize {
		return nil, ErrAllocationTooLarge
	}
	
	req.submittedAt = time.Now()
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// 查找合适的GPU
	gpu, err := a.findGPUForAllocation(req)
	if err != nil {
		// 无法立即分配，加入等待队列
		if a.config.EnableGraceful {
			a.addToWaitQueue(req)
			return nil, ErrAllocationQueued
		}
		return nil, err
	}
	
	// 执行分配
	alloc := a.doAllocate(gpu, req)
	
	// 更新统计
	a.stats.TotalAllocations++
	a.stats.TotalAllocated += alloc.Size
	a.stats.ActiveAllocations++
	
	// 记录分配
	a.allocations[alloc.ID] = alloc
	a.byTask[req.TaskID] = append(a.byTask[req.TaskID], alloc)
	
	// 调用回调
	if req.OnAllocated != nil {
		req.OnAllocated(alloc)
	}
	
	return alloc, nil
}

// findGPUForAllocation 查找可分配GPU
func (a *MemoryAllocator) findGPUForAllocation(req *MemoryRequest) (*GPUDevice, error) {
	var candidates []*GPUDevice
	
	a.manager.mu.RLock()
	for _, gpu := range a.manager.gpus {
		// 检查状态
		if gpu.Status == StatusOffline || gpu.Status == StatusError {
			continue
		}
		
		// 指定GPU检查
		if req.GPUID != "" && gpu.ID != req.GPUID {
			continue
		}
		
		// 计算可用显存（考虑预留和超卖）
		available := a.calculateAvailableMemory(gpu)
		
		// 独占模式检查
		if req.Exclusive {
			if gpu.Status != StatusIdle {
				continue
			}
			// 独占需要全部可用显存
			if available < gpu.TotalMemory {
				continue
			}
		} else {
			// 共享模式检查
			if available < req.Size {
				continue
			}
		}
		
		candidates = append(candidates, gpu)
	}
	a.manager.mu.RUnlock()
	
	if len(candidates) == 0 {
		return nil, ErrNoMemoryAvailable
	}
	
	// 选择最佳GPU
	return a.selectBestGPU(candidates, req), nil
}

// calculateAvailableMemory 计算可用显存
func (a *MemoryAllocator) calculateAvailableMemory(gpu *GPUDevice) uint64 {
	available := gpu.AvailableMemory
	
	// 减去预留
	if a.config.ReservedAbsolute > 0 {
		available -= a.config.ReservedAbsolute
	} else if a.config.ReservedPercent > 0 {
		available -= uint64(float64(gpu.TotalMemory) * a.config.ReservedPercent)
	}
	
	// 考虑超卖
	if a.config.EnableOvercommit && a.config.OvercommitRatio > 1 {
		overcommitExtra := uint64(float64(gpu.TotalMemory) * (a.config.OvercommitRatio - 1))
		available += overcommitExtra
		
		// 更新超卖统计
		a.stats.OvercommitLimit = uint64(float64(gpu.TotalMemory) * a.config.OvercommitRatio)
	}
	
	return available
}

// selectBestGPU 选择最佳GPU
func (a *MemoryAllocator) selectBestGPU(candidates []*GPUDevice, req *MemoryRequest) *GPUDevice {
	var best *GPUDevice
	bestScore := -1.0
	
	for _, gpu := range candidates {
		score := a.scoreGPU(gpu, req)
		if score > bestScore {
			bestScore = score
			best = gpu
		}
	}
	
	return best
}

// scoreGPU 评分GPU
func (a *MemoryAllocator) scoreGPU(gpu *GPUDevice, req *MemoryRequest) float64 {
	var score float64
	
	// 显存利用率（优先选择利用率低的）
	score += (100 - float64(gpu.UtilizationMem)) * 0.3
	
	// 碎片率（优先选择碎片少的）
	fragmentRate := a.calculateFragmentRate(gpu.ID)
	score += (100 - fragmentRate) * 0.2
	
	// 匹配度（大小刚好够用最好）
	available := a.calculateAvailableMemory(gpu)
	memRatio := float64(available) / float64(req.Size)
	if memRatio > 2 {
		memRatio = 2 // 防止过度偏好大显存
	}
	score += memRatio * 20
	
	// NUMA亲和性（如果请求有指定）
	// 这里简化处理，实际应该考虑任务的NUMA需求
	
	return score
}

// doAllocate 执行分配
func (a *MemoryAllocator) doAllocate(gpu *GPUDevice, req *MemoryRequest) *MemoryAllocation {
	now := time.Now()
	
	// 更新GPU显存使用
	a.manager.mu.Lock()
	if req.Exclusive {
		gpu.Reserve(req.TaskID)
	} else {
		gpu.Allocate(req.Size, req.TaskID)
	}
	a.manager.mu.Unlock()
	
	// 创建分配记录
	alloc := &MemoryAllocation{
		ID:        GenerateAllocationID(),
		GPUID:     gpu.ID,
		GPUIndex:  gpu.Index,
		TaskID:    req.TaskID,
		Size:      req.Size,
		Requested: req.Size,
		Status:    AllocationActive,
		AllocatedAt: now,
		ExpiresAt: req.ExpiresAt,
		Priority:  req.Priority,
	}
	
	// 更新分配时间统计
	allocationTime := time.Since(req.submittedAt).Milliseconds()
	a.stats.AvgAllocationTime = (a.stats.AvgAllocationTime + allocationTime) / 2
	
	return alloc
}

// addToWaitQueue 加入等待队列
func (a *MemoryAllocator) addToWaitQueue(req *MemoryRequest) {
	a.waitQueue = append(a.waitQueue, req)
	a.stats.WaitQueueLength++
	a.stats.WaitQueueTotal += req.Size
	
	// 设置超时回调
	if req.ExpiresAt != nil {
		go a.waitForAllocation(req)
	}
}

// waitForAllocation 等待分配
func (a *MemoryAllocator) waitForAllocation(req *MemoryRequest) {
	timeout := req.ExpiresAt.Sub(req.submittedAt)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	for {
		select {
		case <-timer.C:
			// 超时
			a.mu.Lock()
			// 从队列移除
			for i, r := range a.waitQueue {
				if r.ID == req.ID {
					a.waitQueue = append(a.waitQueue[:i], a.waitQueue[i+1:]...)
					break
				}
			}
			a.stats.WaitQueueLength--
			a.stats.WaitQueueTotal -= req.Size
			a.stats.TimeoutFailures++
			a.mu.Unlock()
			
			if req.OnTimeout != nil {
				req.OnTimeout()
			}
			return
			
		case <-time.After(100 * time.Millisecond):
			// 检查是否已分配
			a.mu.Lock()
			if req.allocated != nil {
				a.mu.Unlock()
				return
			}
			a.mu.Unlock()
		}
	}
}

// Release 释放显存
func (a *MemoryAllocator) Release(allocID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	alloc, exists := a.allocations[allocID]
	if !exists {
		return ErrAllocationNotFound
	}
	
	if alloc.Status != AllocationActive {
		return ErrAllocationAlreadyReleased
	}
	
	// 执行释放
	a.manager.mu.Lock()
	gpu := a.manager.gpus[alloc.GPUID]
	if gpu != nil {
		gpu.Release(alloc.Size)
	}
	a.manager.mu.Unlock()
	
	// 更新分配记录
	now := time.Now()
	alloc.Status = AllocationReleased
	alloc.ReleasedAt = &now
	
	// 更新统计
	a.stats.TotalReleased += alloc.Size
	a.stats.ActiveAllocations--
	
	// 更新释放时间
	releaseTime := time.Since(alloc.AllocatedAt).Milliseconds()
	a.stats.AvgReleaseTime = (a.stats.AvgReleaseTime + releaseTime) / 2
	
	// 处理等待队列
	a.processWaitQueue()
	
	return nil
}

// ReleaseByTask 按任务释放
func (a *MemoryAllocator) ReleaseByTask(taskID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	allocs, exists := a.byTask[taskID]
	if !exists || len(allocs) == 0 {
		return ErrAllocationNotFound
	}
	
	// 释放所有属于该任务的分配
	for _, alloc := range allocs {
		if alloc.Status != AllocationActive {
			continue
		}
		
		a.manager.mu.Lock()
		gpu := a.manager.gpus[alloc.GPUID]
		if gpu != nil {
			gpu.Release(alloc.Size)
		}
		a.manager.mu.Unlock()
		
		now := time.Now()
		alloc.Status = AllocationReleased
		alloc.ReleasedAt = &now
		
		a.stats.TotalReleased += alloc.Size
		a.stats.ActiveAllocations--
	}
	
	// 删除任务记录
	delete(a.byTask, taskID)
	
	// 处理等待队列
	a.processWaitQueue()
	
	return nil
}

// processWaitQueue 处理等待队列
func (a *MemoryAllocator) processWaitQueue() {
	// 按优先级排序等待队列
	a.sortWaitQueue()
	
	processed := 0
	for i := 0; i < len(a.waitQueue) && processed < 3; i++ {
		req := a.waitQueue[i]
		
		gpu, err := a.findGPUForAllocation(req)
		if err != nil {
			continue
		}
		
		alloc := a.doAllocate(gpu, req)
		req.allocated = alloc
		
		// 移除等待队列
		a.waitQueue = append(a.waitQueue[:i], a.waitQueue[i+1:]...)
		i--
		processed++
		
		// 更新统计
		a.stats.WaitQueueLength--
		a.stats.WaitQueueTotal -= req.Size
		
		// 调用回调
		if req.OnAllocated != nil {
			req.OnAllocated(alloc)
		}
	}
}

// sortWaitQueue 排序等待队列
func (a *MemoryAllocator) sortWaitQueue() {
	// 按优先级排序（高优先级在前）
	for i := 0; i < len(a.waitQueue)-1; i++ {
		for j := i + 1; j < len(a.waitQueue); j++ {
			if a.waitQueue[j].Priority > a.waitQueue[i].Priority {
				a.waitQueue[i], a.waitQueue[j] = a.waitQueue[j], a.waitQueue[i]
			}
		}
	}
}

// ========== 碎片整理 ==========

// Defragment 碎片整理
func (a *MemoryAllocator) Defragment() error {
	if !a.config.EnableDefrag {
		return nil
	}
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// 分析每个GPU的碎片情况
	for gpuID := range a.manager.gpus {
		fragmentRate := a.calculateFragmentRate(gpuID)
		
		if fragmentRate > a.config.DefragThreshold {
			a.defragmentGPU(gpuID)
		}
	}
	
	return nil
}

// calculateFragmentRate 计算碎片率
func (a *MemoryAllocator) calculateFragmentRate(gpuID string) float64 {
	var activeAllocs []*MemoryAllocation
	for _, alloc := range a.allocations {
		if alloc.GPUID == gpuID && alloc.Status == AllocationActive {
			activeAllocs = append(activeAllocs, alloc)
		}
	}
	
	if len(activeAllocs) == 0 {
		return 0
	}
	
	// 碎片率 = 小于最小分配粒度的分配比例
	fragmentCount := 0
	for _, alloc := range activeAllocs {
		if alloc.Size < a.config.MinAllocationSize*2 {
			alloc.IsFragment = true
			fragmentCount++
		}
	}
	
	return float64(fragmentCount) / float64(len(activeAllocs))
}

// defragmentGPU 对GPU进行碎片整理
func (a *MemoryAllocator) defragmentGPU(gpuID string) {
	// 实际碎片整理需要迁移数据，这里简化处理
	// 只更新统计信息
	
	var fragmentCount int
	for _, alloc := range a.allocations {
		if alloc.GPUID == gpuID && alloc.Status == AllocationActive && alloc.IsFragment {
			fragmentCount++
		}
	}
	
	a.stats.FragmentCount = fragmentCount
	a.stats.FragmentRate = a.calculateFragmentRate(gpuID)
}

// ========== 自动回收 ==========

// RunGC 运行垃圾回收（回收过期分配）
func (a *MemoryAllocator) RunGC() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	now := time.Now()
	
	for allocID, alloc := range a.allocations {
		if alloc.Status != AllocationActive {
			continue
		}
		
		// 检查过期
		if alloc.ExpiresAt != nil && now.After(*alloc.ExpiresAt) {
			// 执行释放
			a.manager.mu.Lock()
			gpu := a.manager.gpus[alloc.GPUID]
			if gpu != nil {
				gpu.Release(alloc.Size)
			}
			a.manager.mu.Unlock()
			
			alloc.Status = AllocationExpired
			alloc.ReleasedAt = &now
			
			a.stats.TotalReleased += alloc.Size
			a.stats.ActiveAllocations--
			
			delete(a.allocations, allocID)
		}
		
		// 检查空闲超时（如果任务已完成）
		// 这里简化处理，实际应该结合任务状态
	}
	
	// 处理等待队列
	a.processWaitQueue()
}

// ========== 统计和监控 ==========

// GetStats 获取统计
func (a *MemoryAllocator) GetStats() *AllocatorStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// 更新实时统计
	a.stats.ActiveAllocations = 0
	for _, alloc := range a.allocations {
		if alloc.Status == AllocationActive {
			a.stats.ActiveAllocations++
		}
	}
	
	return a.stats
}

// GetAllocation 获取分配
func (a *MemoryAllocator) GetAllocation(allocID string) (*MemoryAllocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	alloc, exists := a.allocations[allocID]
	if !exists {
		return nil, ErrAllocationNotFound
	}
	
	return alloc, nil
}

// ListAllocations 列出分配
func (a *MemoryAllocator) ListAllocations(gpuID string, status AllocationStatus) []*MemoryAllocation {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	var result []*MemoryAllocation
	for _, alloc := range a.allocations {
		if gpuID != "" && alloc.GPUID != gpuID {
			continue
		}
		if status != "" && alloc.Status != status {
			continue
		}
		result = append(result, alloc)
	}
	
	return result
}

// ========== 错误定义 ==========

var (
	ErrAllocationTooLarge      = fmt.Errorf("分配请求过大")
	ErrNoMemoryAvailable       = fmt.Errorf("没有可用显存")
	ErrAllocationQueued        = fmt.Errorf("分配请求已入队等待")
	ErrAllocationNotFound      = fmt.Errorf("分配记录不存在")
	ErrAllocationAlreadyReleased = fmt.Errorf("分配已释放")
)

// GenerateMemoryAllocID 生成分配ID
func GenerateMemoryAllocID() string {
	return fmt.Sprintf("memalloc-%d", time.Now().UnixNano())
}

// ========== 辅助函数 ==========

// ParseMemorySize 解析显存大小字符串
func ParseMemorySize(sizeStr string) (uint64, error) {
	sizeStr = strings.TrimSpace(strings.ToLower(sizeStr))
	
	var multiplier uint64 = 1
	
	// 处理单位
	if strings.HasSuffix(sizeStr, "gb") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "gb")
	} else if strings.HasSuffix(sizeStr, "g") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "g")
	} else if strings.HasSuffix(sizeStr, "mb") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "mb")
	} else if strings.HasSuffix(sizeStr, "m") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "m")
	} else if strings.HasSuffix(sizeStr, "kb") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "kb")
	} else if strings.HasSuffix(sizeStr, "k") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "k")
	} else if strings.HasSuffix(sizeStr, "b") {
		multiplier = 1
		sizeStr = strings.TrimSuffix(sizeStr, "b")
	}
	
	// 处理小数
	sizeFloat, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析显存大小失败: %w", err)
	}
	
	return uint64(sizeFloat * float64(multiplier)), nil
}

// FormatMemorySize 格式化显存大小
func FormatMemorySize(size uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	
	if size >= TB {
		return fmt.Sprintf("%.2fTB", float64(size)/float64(TB))
	}
	if size >= GB {
		return fmt.Sprintf("%.2fGB", float64(size)/float64(GB))
	}
	if size >= MB {
		return fmt.Sprintf("%.2fMB", float64(size)/float64(MB))
	}
	if size >= KB {
		return fmt.Sprintf("%.2fKB", float64(size)/float64(KB))
	}
	return fmt.Sprintf("%dB", size)
}

