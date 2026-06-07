// Package edgeai 提供推理任务调度器功能
package edgeai

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

// InferScheduler 推理任务调度器
type InferScheduler struct {
	mu            sync.RWMutex
	priorityQueue *PriorityQueue
	processing    map[string]*ScheduledTask
	maxConcurrent int
	maxQueueSize  int
	stats         *SchedulerStats
	stopCh        chan struct{}
}

// ScheduledTask 调度任务
type ScheduledTask struct {
	ID        string
	Request   *InferenceRequest
	Priority  TaskPriority
	CreatedAt time.Time
	StartedAt *time.Time
	Status    TaskStatus
	Device    ComputeDevice
}

// NewInferScheduler 创建推理调度器
func NewInferScheduler(maxConcurrent, maxQueueSize int) *InferScheduler {
	s := &InferScheduler{
		priorityQueue: &PriorityQueue{},
		processing:    make(map[string]*ScheduledTask),
		maxConcurrent: maxConcurrent,
		maxQueueSize:  maxQueueSize,
		stats:         &SchedulerStats{},
		stopCh:        make(chan struct{}),
	}

	heap.Init(s.priorityQueue)

	return s
}

// Start 启动调度器
func (s *InferScheduler) Start() {
	go s.scheduleLoop()
}

// Stop 停止调度器
func (s *InferScheduler) Stop() {
	close(s.stopCh)
}

// Submit 提交任务
func (s *InferScheduler) Submit(request *InferenceRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.priorityQueue.Len() >= s.maxQueueSize {
		return "", fmt.Errorf("队列已满，当前长度: %d", s.priorityQueue.Len())
	}

	task := &ScheduledTask{
		ID:        request.ID,
		Request:   request,
		Priority:  request.Priority,
		CreatedAt: time.Now(),
		Status:    TaskStatusQueued,
	}

	heap.Push(s.priorityQueue, task)
	s.stats.TotalQueued++
	s.stats.QueueLength = s.priorityQueue.Len()

	return request.ID, nil
}

// Cancel 取消任务
func (s *InferScheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从队列中查找并移除
	for i, task := range *s.priorityQueue {
		if task.ID == taskID {
			heap.Remove(s.priorityQueue, i)
			s.stats.QueueLength = s.priorityQueue.Len()
			return nil
		}
	}

	// 从处理中查找并移除
	if task, ok := s.processing[taskID]; ok {
		task.Status = TaskStatusCancelled
		delete(s.processing, taskID)
		s.stats.Processing = len(s.processing)
		return nil
	}

	return fmt.Errorf("任务 %s 不存在", taskID)
}

// GetTask 获取任务状态
func (s *InferScheduler) GetTask(taskID string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 在队列中查找
	for _, task := range *s.priorityQueue {
		if task.ID == taskID {
			return task, nil
		}
	}

	// 在处理中查找
	if task, ok := s.processing[taskID]; ok {
		return task, nil
	}

	return nil, fmt.Errorf("任务 %s 不存在", taskID)
}

// QueueLength 获取队列长度
func (s *InferScheduler) QueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.priorityQueue.Len()
}

// ProcessingCount 获取处理中任务数
func (s *InferScheduler) ProcessingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processing)
}

// GetStats 获取调度器统计
func (s *InferScheduler) GetStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := *s.stats
	stats.QueueLength = s.priorityQueue.Len()
	stats.Processing = len(s.processing)
	return &stats
}

// scheduleLoop 调度循环
func (s *InferScheduler) scheduleLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.schedule()
		}
	}
}

// schedule 执行调度
func (s *InferScheduler) schedule() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否有空闲槽位
	if len(s.processing) >= s.maxConcurrent || s.priorityQueue.Len() == 0 {
		return
	}

	// 取出最高优先级的任务
	item := heap.Pop(s.priorityQueue).(*ScheduledTask)
	now := time.Now()
	item.StartedAt = &now
	item.Status = TaskStatusProcessing
	item.Device = s.selectDevice(item.Request)

	s.processing[item.ID] = item
	s.stats.QueueLength = s.priorityQueue.Len()
	s.stats.Processing = len(s.processing)

	// 计算等待时间
	waitTime := now.Sub(item.CreatedAt)
	s.stats.TotalProcessed++
	s.stats.AvgWaitTime = (s.stats.AvgWaitTime*float64(s.stats.TotalProcessed-1) + float64(waitTime.Milliseconds())) / float64(s.stats.TotalProcessed)
	if float64(waitTime.Milliseconds()) > s.stats.MaxWaitTime {
		s.stats.MaxWaitTime = float64(waitTime.Milliseconds())
	}
}

// selectDevice 选择计算设备
func (s *InferScheduler) selectDevice(request *InferenceRequest) ComputeDevice {
	// 简化实现：根据任务类型选择设备
	switch request.TaskType {
	case TaskTypeClassification, TaskTypeDetection:
		return ComputeDeviceGPU
	case TaskTypeNLP, TaskTypeEmbedding:
		return ComputeDeviceCPU
	default:
		return ComputeDeviceCPU
	}
}

// CompleteTask 完成任务
func (s *InferScheduler) CompleteTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.processing[taskID]; ok {
		task.Status = TaskStatusCompleted
		delete(s.processing, taskID)
		s.stats.Processing = len(s.processing)
	}
}

// PriorityQueue 优先级队列
type PriorityQueue []*ScheduledTask

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 优先级高的排在前面
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// 同优先级按创建时间排序
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*ScheduledTask)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return item
}

// ResourceScheduler 资源调度器
type ResourceScheduler struct {
	mu          sync.RWMutex
	cpuCores    int
	gpuDevices  int
	memoryTotal int64
	memoryUsed  int64
	cpuUsage    float64
	gpuUsage    float64
	taskLimits  map[ComputeDevice]int
	taskCounts  map[ComputeDevice]int
}

// NewResourceScheduler 创建资源调度器
func NewResourceScheduler(cpuCores, gpuDevices int, memoryTotal int64) *ResourceScheduler {
	return &ResourceScheduler{
		cpuCores:    cpuCores,
		gpuDevices:  gpuDevices,
		memoryTotal: memoryTotal,
		taskLimits: map[ComputeDevice]int{
			ComputeDeviceCPU: cpuCores * 2,
			ComputeDeviceGPU: gpuDevices * 4,
			ComputeDeviceNPU: 2,
		},
		taskCounts: map[ComputeDevice]int{
			ComputeDeviceCPU: 0,
			ComputeDeviceGPU: 0,
			ComputeDeviceNPU: 0,
		},
	}
}

// Allocate 分配资源
func (rs *ResourceScheduler) Allocate(device ComputeDevice, memoryNeeded int64) (bool, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// 检查任务数限制
	if rs.taskCounts[device] >= rs.taskLimits[device] {
		return false, fmt.Errorf("设备 %s 任务数已达上限", device)
	}

	// 检查内存限制
	if device == ComputeDeviceGPU && memoryNeeded > 0 {
		if rs.memoryUsed+memoryNeeded > rs.memoryTotal {
			return false, fmt.Errorf("GPU 内存不足")
		}
		rs.memoryUsed += memoryNeeded
	}

	rs.taskCounts[device]++
	return true, nil
}

// Release 释放资源
func (rs *ResourceScheduler) Release(device ComputeDevice, memoryUsed int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.taskCounts[device] > 0 {
		rs.taskCounts[device]--
	}

	if device == ComputeDeviceGPU && memoryUsed > 0 {
		rs.memoryUsed -= memoryUsed
		if rs.memoryUsed < 0 {
			rs.memoryUsed = 0
		}
	}
}

// GetUsage 获取资源使用情况
func (rs *ResourceScheduler) GetUsage() map[ComputeDevice]float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	usage := make(map[ComputeDevice]float64)
	for device, count := range rs.taskCounts {
		limit := rs.taskLimits[device]
		if limit > 0 {
			usage[device] = float64(count) / float64(limit) * 100
		}
	}

	return usage
}

// UpdateMetrics 更新资源指标
func (rs *ResourceScheduler) UpdateMetrics(cpuUsage, gpuUsage float64, memoryUsed int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.cpuUsage = cpuUsage
	rs.gpuUsage = gpuUsage
	rs.memoryUsed = memoryUsed
}

// GetMetrics 获取资源指标
func (rs *ResourceScheduler) GetMetrics() (cpuUsage, gpuUsage float64, memoryUsed int64) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return rs.cpuUsage, rs.gpuUsage, rs.memoryUsed
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	mu       sync.RWMutex
	devices  []ComputeDevice
	counts   map[ComputeDevice]int
	strategy string // round_robin, least_connections, weighted
	index    int
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(devices []ComputeDevice, strategy string) *LoadBalancer {
	if strategy == "" {
		strategy = "least_connections"
	}

	counts := make(map[ComputeDevice]int)
	for _, device := range devices {
		counts[device] = 0
	}

	return &LoadBalancer{
		devices:  devices,
		counts:   counts,
		strategy: strategy,
	}
}

// Select 选择设备
func (lb *LoadBalancer) Select() ComputeDevice {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.devices) == 0 {
		return ComputeDeviceCPU
	}

	switch lb.strategy {
	case "round_robin":
		return lb.selectRoundRobin()
	case "least_connections":
		return lb.selectLeastConnections()
	default:
		return lb.selectLeastConnections()
	}
}

// selectRoundRobin 轮询选择
func (lb *LoadBalancer) selectRoundRobin() ComputeDevice {
	device := lb.devices[lb.index%len(lb.devices)]
	lb.index++
	return device
}

// selectLeastConnections 最少连接选择
func (lb *LoadBalancer) selectLeastConnections() ComputeDevice {
	minCount := int(^uint(0) >> 1)
	selected := lb.devices[0]

	for _, device := range lb.devices {
		count := lb.counts[device]
		if count < minCount {
			minCount = count
			selected = device
		}
	}

	return selected
}

// Increment 增加设备计数
func (lb *LoadBalancer) Increment(device ComputeDevice) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.counts[device]++
}

// Decrement 减少设备计数
func (lb *LoadBalancer) Decrement(device ComputeDevice) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.counts[device] > 0 {
		lb.counts[device]--
	}
}

// GetCounts 获取设备计数
func (lb *LoadBalancer) GetCounts() map[ComputeDevice]int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	counts := make(map[ComputeDevice]int)
	for k, v := range lb.counts {
		counts[k] = v
	}

	return counts
}
