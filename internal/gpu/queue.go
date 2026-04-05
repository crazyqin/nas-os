// Package gpu 任务队列和分配器
package gpu

import (
	"container/list"
	"sync"
	"time"
)

// TaskQueue 任务队列
// 支持优先级队列，高优先级任务优先处理
type TaskQueue struct {
	maxSize  int
	queues   map[AllocationPriority]*list.List // 按优先级分组
	count    int
	mu       sync.RWMutex
}

// NewTaskQueue 创建任务队列
func NewTaskQueue(maxSize int) *TaskQueue {
	return &TaskQueue{
		maxSize: maxSize,
		queues: map[AllocationPriority]*list.List{
			PriorityCritical: list.New(),
			PriorityHigh:     list.New(),
			PriorityNormal:   list.New(),
			PriorityLow:      list.New(),
		},
	}
}

// Enqueue 添加任务到队列
// 按优先级插入对应队列
func (q *TaskQueue) Enqueue(task *GPUTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.maxSize > 0 && q.count >= q.maxSize {
		return NewPoolError("queue_full", "任务队列已满")
	}

	priority := task.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	q.queues[priority].PushBack(task)
	q.count++

	return nil
}

// Dequeue 从队列取出任务
// 按优先级顺序：critical -> high -> normal -> low
func (q *TaskQueue) Dequeue() (*GPUTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	priorities := []AllocationPriority{
		PriorityCritical,
		PriorityHigh,
		PriorityNormal,
		PriorityLow,
	}

	for _, priority := range priorities {
		queue := q.queues[priority]
		if queue.Len() > 0 {
			elem := queue.Front()
			task := elem.Value.(*GPUTask)
			queue.Remove(elem)
			q.count--
			return task, nil
		}
	}

	return nil, NewPoolError("queue_empty", "任务队列空")
}

// Peek 查看队列头部任务（不移除）
func (q *TaskQueue) Peek() *GPUTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	priorities := []AllocationPriority{
		PriorityCritical,
		PriorityHigh,
		PriorityNormal,
		PriorityLow,
	}

	for _, priority := range priorities {
		queue := q.queues[priority]
		if queue.Len() > 0 {
			return queue.Front().Value.(*GPUTask)
		}
	}

	return nil
}

// Remove 移除指定任务
func (q *TaskQueue) Remove(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, queue := range q.queues {
		for elem := queue.Front(); elem != nil; elem = elem.Next() {
			task := elem.Value.(*GPUTask)
			if task.ID == taskID {
				queue.Remove(elem)
				q.count--
				return nil
			}
		}
	}

	return ErrTaskNotFound
}

// Length 获取队列长度
func (q *TaskQueue) Length() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.count
}

// Clear 清空队列
func (q *TaskQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, queue := range q.queues {
		queue.Init()
	}
	q.count = 0
}

// GetAllTasks 获取所有任务
func (q *TaskQueue) GetAllTasks() []*GPUTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var tasks []*GPUTask
	priorities := []AllocationPriority{
		PriorityCritical,
		PriorityHigh,
		PriorityNormal,
		PriorityLow,
	}

	for _, priority := range priorities {
		queue := q.queues[priority]
		for elem := queue.Front(); elem != nil; elem = elem.Next() {
			tasks = append(tasks, elem.Value.(*GPUTask))
		}
	}

	return tasks
}

// PoolAllocator 资源池分配器
type PoolAllocator struct {
	pool    *Pool
	strategy string
	mu      sync.RWMutex
}

// NewPoolAllocator 创建分配器
func NewPoolAllocator(pool *Pool, strategy string) *PoolAllocator {
	if strategy == "" {
		strategy = "least-loaded"
	}
	return &PoolAllocator{
		pool:     pool,
		strategy: strategy,
	}
}

// Allocate 分配GPU设备
func (a *PoolAllocator) Allocate(task *GPUTask) (*PoolDevice, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 获取可用设备列表
	available := a.getAvailableDevices(task)
	if len(available) == 0 {
		return nil, ErrDeviceNotAvailable
	}

	// 根据策略选择设备
	device := a.selectDevice(available, task)

	return device, nil
}

// getAvailableDevices 获取满足条件的可用设备
func (a *PoolAllocator) getAvailableDevices(task *GPUTask) []*PoolDevice {
	var devices []*PoolDevice

	for _, device := range a.pool.devices {
		// 检查是否可用
		if device.Allocated && !task.Exclusive {
			continue
		}

		// 检查显存是否足够
		freeMemory := device.Device.MemoryTotal - device.MemoryUsed
		if task.MemoryRequired > 0 && freeMemory < task.MemoryRequired {
			continue
		}

		// 检查健康状态
		if device.HealthStatus == "critical" {
			continue
		}

		devices = append(devices, device)
	}

	return devices
}

// selectDevice 根据策略选择设备
func (a *PoolAllocator) selectDevice(devices []*PoolDevice, task *GPUTask) *PoolDevice {
	switch a.strategy {
	case "round-robin":
		return a.selectRoundRobin(devices)
	case "least-loaded":
		return a.selectLeastLoaded(devices)
	case "most-memory":
		return a.selectMostMemory(devices)
	case "priority":
		return a.selectPriority(devices, task)
	default:
		return a.selectLeastLoaded(devices)
	}
}

// selectRoundRobin 轮询选择
func (a *PoolAllocator) selectRoundRobin(devices []*PoolDevice) *PoolDevice {
	// 简化：选择第一个可用设备
	if len(devices) > 0 {
		return devices[0]
	}
	return nil
}

// selectLeastLoaded 选择负载最低的设备
func (a *PoolAllocator) selectLeastLoaded(devices []*PoolDevice) *PoolDevice {
	if len(devices) == 0 {
		return nil
	}

	// 按负载评分排序，选择最低的
	var selected *PoolDevice
	minScore := float64(100)

	for _, device := range devices {
		score := device.LoadScore
		if score < minScore {
			minScore = score
			selected = device
		}
	}

	return selected
}

// selectMostMemory 选择可用显存最多的设备
func (a *PoolAllocator) selectMostMemory(devices []*PoolDevice) *PoolDevice {
	if len(devices) == 0 {
		return nil
	}

	var selected *PoolDevice
	maxMemory := uint64(0)

	for _, device := range devices {
		freeMemory := device.Device.MemoryTotal - device.MemoryUsed
		if freeMemory > maxMemory {
			maxMemory = freeMemory
			selected = device
		}
	}

	return selected
}

// selectPriority 根据任务优先级选择设备
func (a *PoolAllocator) selectPriority(devices []*PoolDevice, task *GPUTask) *PoolDevice {
	if len(devices) == 0 {
		return nil
	}

	// 高优先级任务选择高性能设备
	// 按CUDA核心数排序
	var sorted []*PoolDevice
	sorted = append(sorted, devices...)

	// 简单排序（按CUDA核心数降序）
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Device.CUDAcores > sorted[i].Device.CUDAcores {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	switch task.Priority {
	case PriorityCritical, PriorityHigh:
		// 选择最高性能设备
		return sorted[0]
	case PriorityLow:
		// 选择性能最低设备
		return sorted[len(sorted)-1]
	default:
		// 随机选择中等性能设备
		mid := len(sorted) / 2
		if mid >= len(sorted) {
			mid = 0
		}
		return sorted[mid]
	}
}

// SetStrategy 设置分配策略
func (a *PoolAllocator) SetStrategy(strategy string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.strategy = strategy
}

// GetStrategy 获取当前策略
func (a *PoolAllocator) GetStrategy() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.strategy
}

// GetPoolStatus 获取资源池状态
func (p *Pool) GetPoolMetrics() *PoolMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metrics := &PoolMetrics{
		Timestamp:     time.Now(),
		DeviceCount:   len(p.devices),
		ActiveTasks:   0,
		QueueLength:   p.queue.Length(),
		TotalMemory:   0,
		UsedMemory:    0,
		Utilization:   0,
	}

	for _, device := range p.devices {
		metrics.TotalMemory += device.Device.MemoryTotal
		metrics.UsedMemory += device.MemoryUsed

		if device.Allocated {
			metrics.ActiveTasks++
		}

		metrics.DeviceMetrics = append(metrics.DeviceMetrics, DeviceMetric{
			ID:         device.Device.ID,
			Name:       device.Device.Name,
			MemoryUsed: device.MemoryUsed,
			MemoryFree: device.Device.MemoryTotal - device.MemoryUsed,
			Temperature: device.Device.Temperature,
			PowerUsage: device.Device.PowerUsage,
			LoadScore:  device.LoadScore,
			Health:     device.HealthStatus,
		})
	}

	if metrics.TotalMemory > 0 {
		metrics.Utilization = float64(metrics.UsedMemory) / float64(metrics.TotalMemory) * 100
	}

	return metrics
}

// PoolMetrics 资源池指标
type PoolMetrics struct {
	Timestamp     time.Time       `json:"timestamp"`
	DeviceCount   int             `json:"deviceCount"`
	ActiveTasks   int             `json:"activeTasks"`
	QueueLength   int             `json:"queueLength"`
	TotalMemory   uint64          `json:"totalMemory"`
	UsedMemory    uint64          `json:"usedMemory"`
	Utilization   float64         `json:"utilization"`
	DeviceMetrics []DeviceMetric  `json:"deviceMetrics"`
}

// DeviceMetric 设备指标
type DeviceMetric struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MemoryUsed  uint64    `json:"memoryUsed"`
	MemoryFree  uint64    `json:"memoryFree"`
	Temperature int       `json:"temperature"`
	PowerUsage  uint64    `json:"powerUsage"`
	LoadScore   float64   `json:"loadScore"`
	Health      string    `json:"health"`
}