// Package gpu GPU资源池管理
// 实现GPU资源池、任务队列和资源监控
package gpu

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Pool GPU资源池
// 管理多个GPU设备的统一调度和分配.
type Pool struct {
	manager   *Manager
	config    *PoolConfig
	logger    *zap.Logger
	devices   map[string]*PoolDevice
	queue     *TaskQueue
	allocator *PoolAllocator
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// PoolConfig 资源池配置.
type PoolConfig struct {
	// 资源池名称
	Name string `json:"name"`
	// 最大并发任务数
	MaxConcurrentTasks int `json:"maxConcurrentTasks"`
	// 任务超时时间(秒)
	TaskTimeout int `json:"taskTimeout"`
	// 资源预留比例(%)
	ReservePercent int `json:"reservePercent"`
	// 负载均衡策略
	BalanceStrategy string `json:"balanceStrategy"` // round-robin, least-loaded, most-memory
	// 是否启用MPS
	EnableMPS bool `json:"enableMps"`
	// 监控间隔(秒)
	MonitorInterval int `json:"monitorInterval"`
	// 是否启用自动扩缩容
	EnableAutoScale bool `json:"enableAutoScale"`
	// 自动扩容阈值(%)
	ScaleUpThreshold int `json:"scaleUpThreshold"`
	// 自动缩容阈值(%)
	ScaleDownThreshold int `json:"scaleDownThreshold"`
}

// DefaultPoolConfig 默认资源池配置.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Name:               "default",
		MaxConcurrentTasks: 10,
		TaskTimeout:        3600, // 1小时
		ReservePercent:     10,
		BalanceStrategy:    "least-loaded",
		EnableMPS:          false,
		MonitorInterval:    5,
		EnableAutoScale:    false,
		ScaleUpThreshold:   80,
		ScaleDownThreshold: 20,
	}
}

// PoolDevice 资源池中的GPU设备.
type PoolDevice struct {
	Device       *GPUDevice
	Allocated    bool
	AllocatedTo  string
	AllocatedAt  time.Time
	MemoryUsed   uint64
	MemoryLimit  uint64
	TaskCount    int
	LoadScore    float64
	LastUpdate   time.Time
	HealthStatus string
}

// NewPool 创建GPU资源池.
func NewPool(manager *Manager, config *PoolConfig, logger *zap.Logger) (*Pool, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if config == nil {
		config = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &Pool{
		manager: manager,
		config:  config,
		logger:  logger,
		devices: make(map[string]*PoolDevice),
		queue:   NewTaskQueue(config.MaxConcurrentTasks),
		ctx:     ctx,
		cancel:  cancel,
	}

	// 初始化分配器
	pool.allocator = NewPoolAllocator(pool, config.BalanceStrategy)

	// 从管理器获取设备并初始化
	gpus := manager.ListGPUs(nil)
	for _, gpu := range gpus {
		pool.devices[gpu.ID] = &PoolDevice{
			Device:       gpu,
			Allocated:    false,
			HealthStatus: "healthy",
			LastUpdate:   time.Now(),
		}
	}

	// 启动监控
	go pool.startMonitoring(ctx)

	return pool, nil
}

// AddDevice 添加GPU设备到资源池.
func (p *Pool) AddDevice(gpu *GPUDevice) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[gpu.ID]; exists {
		return nil // 已存在，忽略
	}

	p.devices[gpu.ID] = &PoolDevice{
		Device:       gpu,
		Allocated:    false,
		HealthStatus: "healthy",
		LastUpdate:   time.Now(),
	}

	p.logger.Info("GPU设备添加到资源池",
		zap.String("pool", p.config.Name),
		zap.String("gpuId", gpu.ID),
		zap.String("gpuName", gpu.Name))

	return nil
}

// RemoveDevice 从资源池移除GPU设备.
func (p *Pool) RemoveDevice(gpuID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	device, exists := p.devices[gpuID]
	if !exists {
		return nil // 不存在，忽略
	}

	// 如果设备正在使用，先释放
	if device.Allocated {
		p.releaseDeviceInternal(gpuID)
	}

	delete(p.devices, gpuID)

	p.logger.Info("GPU设备从资源池移除",
		zap.String("pool", p.config.Name),
		zap.String("gpuId", gpuID))

	return nil
}

// SubmitTask 提交GPU任务到队列.
func (p *Pool) SubmitTask(task *GPUTask) (*TaskResult, error) {
	if task == nil {
		return nil, ErrInvalidTask
	}

	// 验证任务参数
	if err := p.validateTask(task); err != nil {
		return nil, err
	}

	// 生成任务ID
	task.ID = generateTaskID()
	task.SubmitTime = time.Now()
	task.Status = TaskStatusPending

	// 添加到任务队列
	if err := p.queue.Enqueue(task); err != nil {
		return nil, err
	}

	p.logger.Info("GPU任务已提交",
		zap.String("pool", p.config.Name),
		zap.String("taskId", task.ID),
		zap.String("taskType", task.Type),
		zap.Uint64("memoryReq", task.MemoryRequired))

	// 尝试立即分配
	go p.tryAllocateTask()

	return &TaskResult{
		TaskID:     task.ID,
		Status:     TaskStatusPending,
		Message:    "任务已提交，等待分配",
		SubmitTime: task.SubmitTime,
	}, nil
}

// validateTask 验证任务参数.
func (p *Pool) validateTask(task *GPUTask) error {
	if task.Type == "" {
		return ErrInvalidTaskType
	}

	if task.MemoryRequired > 0 {
		// 检查是否有足够显存的设备
		p.mu.RLock()
		hasDevice := false
		for _, device := range p.devices {
			freeMemory := device.Device.MemoryTotal - device.MemoryUsed
			if freeMemory >= task.MemoryRequired {
				hasDevice = true
				break
			}
		}
		p.mu.RUnlock()

		if !hasDevice {
			return ErrInsufficientMemory
		}
	}

	if task.Priority == "" {
		task.Priority = PriorityNormal
	}

	return nil
}

// tryAllocateTask 尝试分配任务.
func (p *Pool) tryAllocateTask() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查队列中的待处理任务
	task := p.queue.Peek()
	if task == nil {
		return
	}

	// 尝试分配GPU
	device, err := p.allocator.Allocate(task)
	if err != nil {
		p.logger.Debug("暂时无法分配GPU",
			zap.String("taskId", task.ID),
			zap.Error(err))
		return
	}

	// 从队列中取出任务
	task, _ = p.queue.Dequeue()

	// 分配设备
	p.allocateDeviceInternal(device.Device.ID, task)

	// 启动任务执行
	go p.executeTask(task, device)
}

// allocateDeviceInternal 内部设备分配（已持有锁）.
func (p *Pool) allocateDeviceInternal(deviceID string, task *GPUTask) {
	device := p.devices[deviceID]
	device.Allocated = true
	device.AllocatedTo = task.ID
	device.AllocatedAt = time.Now()
	device.MemoryUsed += task.MemoryRequired
	device.TaskCount++
	device.LastUpdate = time.Now()

	task.Status = TaskStatusRunning
	task.StartTime = time.Now()
	task.AssignedGPU = deviceID

	p.logger.Info("GPU设备已分配",
		zap.String("pool", p.config.Name),
		zap.String("taskId", task.ID),
		zap.String("gpuId", deviceID))
}

// releaseDeviceInternal 内部设备释放（已持有锁）.
func (p *Pool) releaseDeviceInternal(deviceID string) {
	device := p.devices[deviceID]
	device.Allocated = false
	device.AllocatedTo = ""
	device.AllocatedAt = time.Time{}
	device.TaskCount--
	device.LastUpdate = time.Now()
}

// executeTask 执行任务.
func (p *Pool) executeTask(task *GPUTask, device *PoolDevice) {
	// 设置超时
	timeout := time.Duration(p.config.TaskTimeout) * time.Second
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	// 执行任务（调用任务执行函数）
	result := &TaskResult{
		TaskID:    task.ID,
		GPUID:     device.Device.ID,
		Status:    TaskStatusRunning,
		StartTime: task.StartTime,
	}

	// 模拟任务执行（实际应该调用任务处理函数）
	select {
	case <-ctx.Done():
		// 任务超时
		result.Status = TaskStatusTimeout
		result.Message = "任务执行超时"
	case <-time.After(100 * time.Millisecond):
		// 任务完成（简化）
		result.Status = TaskStatusCompleted
		result.Message = "任务执行完成"
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 更新任务状态
	p.mu.Lock()
	task.Status = result.Status
	task.EndTime = result.EndTime
	task.Result = result

	// 释放设备
	p.releaseDeviceInternal(device.Device.ID)
	p.mu.Unlock()

	p.logger.Info("GPU任务执行完成",
		zap.String("pool", p.config.Name),
		zap.String("taskId", task.ID),
		zap.String("status", string(result.Status)),
		zap.Duration("duration", result.Duration))

	// 尝试分配下一个任务
	go p.tryAllocateTask()
}

// GetPoolStatus 获取资源池状态.
func (p *Pool) GetPoolStatus() *PoolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := &PoolStatus{
		Name:        p.config.Name,
		DeviceCount: len(p.devices),
		Devices:     make([]PoolDeviceStatus, 0),
		QueueLength: p.queue.Length(),
	}

	var totalMemory, usedMemory uint64
	var activeTasks, healthyDevices int

	for id, device := range p.devices {
		deviceStatus := PoolDeviceStatus{
			ID:          id,
			Name:        device.Device.Name,
			Allocated:   device.Allocated,
			AllocatedTo: device.AllocatedTo,
			MemoryTotal: device.Device.MemoryTotal,
			MemoryUsed:  device.MemoryUsed,
			MemoryFree:  device.Device.MemoryTotal - device.MemoryUsed,
			TaskCount:   device.TaskCount,
			LoadScore:   device.LoadScore,
			Health:      device.HealthStatus,
		}
		status.Devices = append(status.Devices, deviceStatus)

		totalMemory += device.Device.MemoryTotal
		usedMemory += device.MemoryUsed

		if device.Allocated {
			activeTasks++
		}
		if device.HealthStatus == "healthy" {
			healthyDevices++
		}
	}

	status.TotalMemory = totalMemory
	status.UsedMemory = usedMemory
	status.FreeMemory = totalMemory - usedMemory
	status.ActiveTasks = activeTasks
	status.HealthyDevices = healthyDevices

	if totalMemory > 0 {
		status.Utilization = float64(usedMemory) / float64(totalMemory) * 100
	}

	return status
}

// startMonitoring 启动监控.
func (p *Pool) startMonitoring(ctx context.Context) {
	if p.config.MonitorInterval <= 0 {
		p.config.MonitorInterval = 5
	}

	ticker := time.NewTicker(time.Duration(p.config.MonitorInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.updateDeviceStatus()
		}
	}
}

// updateDeviceStatus 更新设备状态.
func (p *Pool) updateDeviceStatus() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 从管理器获取最新设备信息
	gpus := p.manager.ListGPUs(nil)
	for _, gpu := range gpus {
		if device, exists := p.devices[gpu.ID]; exists {
			// 更新设备信息（保留分配状态）
			device.Device.MemoryUsed = gpu.MemoryUsed
			device.Device.MemoryFree = gpu.MemoryFree
			device.Device.Temperature = gpu.Temperature
			device.Device.PowerUsage = gpu.PowerUsage
			device.Device.Status = gpu.Status
			device.HealthStatus = p.calculateHealthStatus(gpu)
			device.LoadScore = p.calculateLoadScore(device)
			device.LastUpdate = time.Now()
		}
	}
}

// calculateHealthStatus 计算健康状态.
func (p *Pool) calculateHealthStatus(gpu *GPUDevice) string {
	if gpu.Status == GPUStatusError {
		return "critical"
	}

	if gpu.Temperature > 85 {
		return "warning"
	}

	if gpu.PowerUsage > gpu.PowerLimit {
		return "warning"
	}

	return "healthy"
}

// calculateLoadScore 计算负载评分.
func (p *Pool) calculateLoadScore(device *PoolDevice) float64 {
	gpu := device.Device

	memUtil := 0.0
	if gpu.MemoryTotal > 0 {
		memUtil = float64(gpu.MemoryUsed) / float64(gpu.MemoryTotal)
	}

	powerUtil := 0.0
	if gpu.PowerLimit > 0 {
		powerUtil = float64(gpu.PowerUsage) / float64(gpu.PowerLimit)
	}

	tempUtil := float64(gpu.Temperature) / 100.0

	// 综合评分：显存40% + 功耗30% + 温度20% + 任务数10%
	score := memUtil*0.4 + powerUtil*0.3 + tempUtil*0.2 + float64(device.TaskCount)*0.1

	return score
}

// CancelTask 取消任务.
func (p *Pool) CancelTask(taskID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 从队列中移除
	if err := p.queue.Remove(taskID); err == nil {
		p.logger.Info("任务已从队列中取消",
			zap.String("taskId", taskID))
		return nil
	}

	// 如果任务正在执行，释放设备
	for _, device := range p.devices {
		if device.AllocatedTo == taskID {
			p.releaseDeviceInternal(device.Device.ID)
			p.logger.Info("正在执行的任务已取消",
				zap.String("taskId", taskID),
				zap.String("gpuId", device.Device.ID))
			return nil
		}
	}

	return ErrTaskNotFound
}

// Close 关闭资源池.
func (p *Pool) Close() error {
	p.cancel()

	// 清空队列
	p.queue.Clear()

	// 释放所有设备
	p.mu.Lock()
	for id := range p.devices {
		p.releaseDeviceInternal(id)
	}
	p.mu.Unlock()

	p.logger.Info("GPU资源池已关闭",
		zap.String("pool", p.config.Name))

	return nil
}

// PoolStatus 资源池状态.
type PoolStatus struct {
	Name           string             `json:"name"`
	DeviceCount    int                `json:"deviceCount"`
	HealthyDevices int                `json:"healthyDevices"`
	ActiveTasks    int                `json:"activeTasks"`
	QueueLength    int                `json:"queueLength"`
	TotalMemory    uint64             `json:"totalMemory"`
	UsedMemory     uint64             `json:"usedMemory"`
	FreeMemory     uint64             `json:"freeMemory"`
	Utilization    float64            `json:"utilization"`
	Devices        []PoolDeviceStatus `json:"devices"`
}

// PoolDeviceStatus 设备状态.
type PoolDeviceStatus struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Allocated   bool    `json:"allocated"`
	AllocatedTo string  `json:"allocatedTo"`
	MemoryTotal uint64  `json:"memoryTotal"`
	MemoryUsed  uint64  `json:"memoryUsed"`
	MemoryFree  uint64  `json:"memoryFree"`
	TaskCount   int     `json:"taskCount"`
	LoadScore   float64 `json:"loadScore"`
	Health      string  `json:"health"`
}

// GPUTask GPU任务.
type GPUTask struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"` // compute, inference, encode, decode
	Priority        AllocationPriority `json:"priority"`
	MemoryRequired  uint64             `json:"memoryRequired"`  // 显存需求(MB)
	ComputeRequired int                `json:"computeRequired"` // 计算需求
	Duration        time.Duration      `json:"duration"`        // 预估执行时间
	Exclusive       bool               `json:"exclusive"`       // 是否独占
	ContainerID     string             `json:"containerId"`     // 关联容器
	SubmitTime      time.Time          `json:"submitTime"`
	StartTime       time.Time          `json:"startTime"`
	EndTime         time.Time          `json:"endTime"`
	Status          TaskStatus         `json:"status"`
	AssignedGPU     string             `json:"assignedGpu"`
	Result          *TaskResult        `json:"result"`
	Params          map[string]string  `json:"params"` // 任务参数
}

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimeout   TaskStatus = "timeout"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskResult 任务结果.
type TaskResult struct {
	TaskID     string        `json:"taskId"`
	GPUID      string        `json:"gpuId"`
	Status     TaskStatus    `json:"status"`
	Message    string        `json:"message"`
	SubmitTime time.Time     `json:"submitTime"`
	StartTime  time.Time     `json:"startTime"`
	EndTime    time.Time     `json:"endTime"`
	Duration   time.Duration `json:"duration"`
	Output     string        `json:"output"`
}

// 错误定义.
var (
	ErrInvalidTask        = NewPoolError("invalid_task", "无效的任务")
	ErrInvalidTaskType    = NewPoolError("invalid_task_type", "无效的任务类型")
	ErrInsufficientMemory = NewPoolError("insufficient_memory", "显存不足")
	ErrTaskNotFound       = NewPoolError("task_not_found", "任务不存在")
	ErrDeviceNotAvailable = NewPoolError("device_not_available", "设备不可用")
)

// PoolError 资源池错误.
type PoolError struct {
	Code    string
	Message string
}

func NewPoolError(code, message string) *PoolError {
	return &PoolError{Code: code, Message: message}
}

func (e *PoolError) Error() string {
	return e.Message
}

// generateTaskID 生成任务ID.
func generateTaskID() string {
	return "gpu-task-" + time.Now().Format("20060102-150405")
}
