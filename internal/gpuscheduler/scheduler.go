// Package gpuscheduler 提供 GPU 调度核心业务逻辑
package gpuscheduler

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 常量 ==========

const (
	// DefaultReservedPercent 默认预留资源百分比
	DefaultReservedPercent = 10.0
	// DefaultOvercommitRatio 默认超分配比率
	DefaultOvercommitRatio = 1.0
	// DefaultMaxTemperature 默认最大允许温度（℃）
	DefaultMaxTemperature = 85
	// DeviceRefreshInterval 设备刷新间隔
	DeviceRefreshInterval = 30 * time.Second
	// MaxRetryCount 最大重试次数
	MaxRetryCount = 3
)

// ========== Scheduler 调度器 ==========

// Scheduler GPU 调度器
type Scheduler struct {
	devices     map[string]*GPUDevice     // GPU 设备映射
	allocations map[string]*GPUAllocation // 分配记录映射
	pools       map[string]*GPUPool       // GPU 资源池
	policy      SchedulerPolicy           // 调度策略
	mu          sync.RWMutex              // 读写锁
	logger      *zap.Logger               // 日志
	ctx         context.Context
	cancel      context.CancelFunc
	stopCh      chan struct{}
}

// NewScheduler 创建 GPU 调度器
func NewScheduler(logger *zap.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		devices:     make(map[string]*GPUDevice),
		allocations: make(map[string]*GPUAllocation),
		pools:       make(map[string]*GPUPool),
		policy: SchedulerPolicy{
			Strategy:          StrategyLeastUsed,
			PreemptionEnabled: false,
			ReservedPercent:   DefaultReservedPercent,
			OvercommitRatio:   DefaultOvercommitRatio,
			MaxTemperature:    DefaultMaxTemperature,
			UpdatedAt:         time.Now(),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		stopCh: make(chan struct{}),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.logger.Info("启动 GPU 调度器")

	// 发现 GPU 设备
	if err := s.DiscoverDevices(); err != nil {
		s.logger.Error("GPU 设备发现失败", zap.Error(err))
		// 不阻塞启动，允许后续重试
	}

	// 启动后台刷新协程
	go s.refreshLoop()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.logger.Info("停止 GPU 调度器")
	s.cancel()
	close(s.stopCh)
}

// refreshLoop 定期刷新设备状态
func (s *Scheduler) refreshLoop() {
	ticker := time.NewTicker(DeviceRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.refreshDeviceStatus(); err != nil {
				s.logger.Error("刷新设备状态失败", zap.Error(err))
			}
		}
	}
}

// ========== 设备发现 ==========

// DiscoverDevices 发现 GPU 设备（通过 nvidia-smi）
func (s *Scheduler) DiscoverDevices() error {
	s.logger.Info("开始发现 GPU 设备")

	// 执行 nvidia-smi 查询
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,uuid,memory.total,memory.used,memory.free,temperature.gpu,power.draw,power.limit,compute_cap,driver_version,utilization.gpu,utilization.memory",
		"--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// 如果 nvidia-smi 不可用，使用模拟数据
		s.logger.Warn("nvidia-smi 不可用，使用模拟数据", zap.Error(err))
		return s.loadMockDevices()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 解析输出
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 13 {
			continue
		}

		// 解析字段
		index := parseInt(strings.TrimSpace(fields[0]))
		name := strings.TrimSpace(fields[1])
		uuid := strings.TrimSpace(fields[2])
		memTotal := parseInt64(strings.TrimSpace(fields[3]))
		memUsed := parseInt64(strings.TrimSpace(fields[4]))
		memFree := parseInt64(strings.TrimSpace(fields[5]))
		temp := parseInt(strings.TrimSpace(fields[6]))
		powerDraw := parseFloat(strings.TrimSpace(fields[7]))
		powerLimit := parseFloat(strings.TrimSpace(fields[8]))
		computeCap := strings.TrimSpace(fields[9])
		driverVer := strings.TrimSpace(fields[10])
		utilGPU := parseInt(strings.TrimSpace(fields[11]))
		utilMem := parseInt(strings.TrimSpace(fields[12]))

		device := &GPUDevice{
			ID:             uuid,
			Index:          index,
			Name:           name,
			UUID:           uuid,
			MemoryTotal:    memTotal,
			MemoryUsed:     memUsed,
			MemoryFree:     memFree,
			Temperature:    temp,
			PowerDraw:      powerDraw,
			PowerLimit:     powerLimit,
			ComputeCap:     computeCap,
			DriverVersion:  driverVer,
			UtilizationGPU: utilGPU,
			UtilizationMem: utilMem,
			Status:         s.determineDeviceStatus(temp),
			Allocations:    s.getDeviceAllocations(uuid),
			UpdatedAt:      time.Now(),
		}

		s.devices[uuid] = device
		s.logger.Info("发现 GPU 设备",
			zap.String("id", uuid),
			zap.String("name", name),
			zap.Int64("memory_total", memTotal))
	}

	return nil
}

// loadMockDevices 加载模拟设备（用于测试环境）
func (s *Scheduler) loadMockDevices() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mockDevices := []*GPUDevice{
		{
			ID:             "GPU-MOCK-001",
			Index:          0,
			Name:           "NVIDIA GeForce RTX 4090",
			UUID:           "GPU-MOCK-001",
			MemoryTotal:    24576,
			MemoryUsed:     0,
			MemoryFree:     24576,
			Temperature:    35,
			PowerDraw:      50.0,
			PowerLimit:     450.0,
			ComputeCap:     "8.9",
			DriverVersion:  "535.129.03",
			UtilizationGPU: 0,
			UtilizationMem: 0,
			Status:         DeviceStatusOnline,
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "GPU-MOCK-002",
			Index:          1,
			Name:           "NVIDIA GeForce RTX 4090",
			UUID:           "GPU-MOCK-002",
			MemoryTotal:    24576,
			MemoryUsed:     0,
			MemoryFree:     24576,
			Temperature:    38,
			PowerDraw:      55.0,
			PowerLimit:     450.0,
			ComputeCap:     "8.9",
			DriverVersion:  "535.129.03",
			UtilizationGPU: 0,
			UtilizationMem: 0,
			Status:         DeviceStatusOnline,
			UpdatedAt:      time.Now(),
		},
	}

	for _, d := range mockDevices {
		s.devices[d.ID] = d
		s.logger.Info("加载模拟 GPU 设备", zap.String("id", d.ID), zap.String("name", d.Name))
	}

	return nil
}

// refreshDeviceStatus 刷新设备状态
func (s *Scheduler) refreshDeviceStatus() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, device := range s.devices {
		if device.Status == DeviceStatusOffline || device.Status == DeviceStatusMaintenance {
			continue
		}

		// 模拟状态更新（实际环境应调用 nvidia-smi）
		device.Allocations = s.getDeviceAllocations(id)
		device.UpdatedAt = time.Now()

		// 计算已分配显存
		var usedMem int64
		for _, alloc := range device.Allocations {
			if alloc.Status == AllocationStatusActive {
				usedMem += alloc.MemoryMiB
			}
		}
		device.MemoryUsed = usedMem
		device.MemoryFree = device.MemoryTotal - usedMem

		// 检查温度
		if device.Temperature > s.policy.MaxTemperature {
			device.Status = DeviceStatusOverheat
			s.logger.Warn("GPU 温度过高",
				zap.String("device_id", id),
				zap.Int("temperature", device.Temperature))
		}
	}

	return nil
}

// ========== 资源分配 ==========

// Allocate 分配 GPU 资源
func (s *Scheduler) Allocate(req AllocateRequest) (*GPUAllocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证请求
	if req.ContainerID == "" {
		return nil, &ValidationError{Field: "container_id", Message: "不能为空"}
	}
	if req.MemoryMiB <= 0 {
		return nil, &ValidationError{Field: "memory_mib", Message: "必须大于 0"}
	}

	// 设置默认优先级
	if req.Priority == "" {
		req.Priority = PriorityMedium
	}

	// 选择设备
	device, err := s.selectDevice(req)
	if err != nil {
		return nil, err
	}

	// 检查超分配
	if !s.checkOvercommit(device, req.MemoryMiB) {
		return nil, &InsufficientResourceError{
			Message: fmt.Sprintf("设备 %s 显存不足，请求 %d MiB，可用 %d MiB", device.ID, req.MemoryMiB, device.MemoryFree),
		}
	}

	// 创建分配记录
	allocation := &GPUAllocation{
		ID:            generateID(),
		DeviceID:      device.ID,
		ContainerID:   req.ContainerID,
		ContainerName: req.ContainerName,
		VMID:          req.VMID,
		MemoryMiB:     req.MemoryMiB,
		Priority:      req.Priority,
		Status:        AllocationStatusActive,
		Constraint:    req.Constraint,
		CreatedAt:     time.Now(),
		Labels:        req.Labels,
	}

	// 更新设备状态
	device.MemoryUsed += req.MemoryMiB
	device.MemoryFree -= req.MemoryMiB
	device.Allocations = append(device.Allocations, allocation)
	device.UpdatedAt = time.Now()

	// 保存分配记录
	s.allocations[allocation.ID] = allocation

	s.logger.Info("GPU 资源分配成功",
		zap.String("allocation_id", allocation.ID),
		zap.String("device_id", device.ID),
		zap.String("container_id", req.ContainerID),
		zap.Int64("memory_mib", req.MemoryMiB))

	return allocation, nil
}

// Release 释放 GPU 资源
func (s *Scheduler) Release(allocationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allocation, exists := s.allocations[allocationID]
	if !exists {
		return &NotFoundError{Resource: "allocation", ID: allocationID}
	}

	if allocation.Status != AllocationStatusActive {
		return fmt.Errorf("分配 %s 状态为 %s，无法释放", allocationID, allocation.Status)
	}

	// 更新分配状态
	allocation.Status = AllocationStatusReleased

	// 更新设备状态
	device, exists := s.devices[allocation.DeviceID]
	if exists {
		device.MemoryUsed -= allocation.MemoryMiB
		device.MemoryFree += allocation.MemoryMiB
		device.UpdatedAt = time.Now()

		// 从设备分配列表中移除
		for i, alloc := range device.Allocations {
			if alloc.ID == allocationID {
				device.Allocations = append(device.Allocations[:i], device.Allocations[i+1:]...)
				break
			}
		}
	}

	s.logger.Info("GPU 资源释放成功",
		zap.String("allocation_id", allocationID),
		zap.String("device_id", allocation.DeviceID))

	return nil
}

// selectDevice 根据策略选择设备
func (s *Scheduler) selectDevice(req AllocateRequest) (*GPUDevice, error) {
	// 过滤可用设备
	available := s.filterAvailableDevices(req)
	if len(available) == 0 {
		return nil, &InsufficientResourceError{Message: "没有可用的 GPU 设备"}
	}

	// 根据策略选择
	switch s.policy.Strategy {
	case StrategyRoundRobin:
		return s.selectRoundRobin(available), nil
	case StrategyLeastUsed:
		return s.selectLeastUsed(available), nil
	case StrategyPriority:
		return s.selectByPriority(available, req.Priority), nil
	case StrategyAffinity:
		return s.selectByAffinity(available, req.Constraint), nil
	case StrategyBinPacking:
		return s.selectBinPacking(available, req.MemoryMiB), nil
	default:
		return s.selectLeastUsed(available), nil
	}
}

// filterAvailableDevices 过滤可用设备
func (s *Scheduler) filterAvailableDevices(req AllocateRequest) []*GPUDevice {
	var available []*GPUDevice

	for _, device := range s.devices {
		// 检查设备状态
		if device.Status != DeviceStatusOnline {
			continue
		}

		// 检查温度
		if device.Temperature > s.policy.MaxTemperature {
			continue
		}

		// 检查亲和性约束
		if req.Constraint != nil {
			if !s.matchesConstraint(device, req.Constraint) {
				continue
			}
		}

		available = append(available, device)
	}

	return available
}

// matchesConstraint 检查设备是否满足约束
func (s *Scheduler) matchesConstraint(device *GPUDevice, constraint *AffinityConstraint) bool {
	// 检查排除列表
	for _, id := range constraint.ExcludedDeviceIDs {
		if device.ID == id {
			return false
		}
	}

	// 如果有偏好列表，优先选择
	if len(constraint.PreferredDeviceIDs) > 0 {
		for _, id := range constraint.PreferredDeviceIDs {
			if device.ID == id {
				return true
			}
		}
		return false
	}

	// 检查标签选择器
	if len(constraint.DeviceLabelSelector) > 0 {
		for k, v := range constraint.DeviceLabelSelector {
			if device.Labels[k] != v {
				return false
			}
		}
	}

	return true
}

// selectRoundRobin 轮询选择
func (s *Scheduler) selectRoundRobin(devices []*GPUDevice) *GPUDevice {
	// 按索引排序后轮询
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Index < devices[j].Index
	})

	// 简单轮询：选择分配数最少的
	var selected *GPUDevice
	minAllocations := -1
	for _, d := range devices {
		count := len(d.Allocations)
		if minAllocations < 0 || count < minAllocations {
			minAllocations = count
			selected = d
		}
	}
	return selected
}

// selectLeastUsed 最少使用选择
func (s *Scheduler) selectLeastUsed(devices []*GPUDevice) *GPUDevice {
	var selected *GPUDevice
	maxFree := int64(0)

	for _, d := range devices {
		if d.MemoryFree > maxFree {
			maxFree = d.MemoryFree
			selected = d
		}
	}
	return selected
}

// selectByPriority 按优先级选择
func (s *Scheduler) selectByPriority(devices []*GPUDevice, priority Priority) *GPUDevice {
	// 高优先级任务优先使用空闲最多的设备
	if priority == PriorityHigh {
		return s.selectLeastUsed(devices)
	}

	// 中低优先级任务使用利用率较高的设备（装箱策略）
	var selected *GPUDevice
	minFree := int64(1<<63 - 1)

	for _, d := range devices {
		if d.MemoryFree < minFree && d.MemoryFree > 0 {
			minFree = d.MemoryFree
			selected = d
		}
	}

	if selected == nil {
		selected = s.selectLeastUsed(devices)
	}
	return selected
}

// selectByAffinity 亲和性选择
func (s *Scheduler) selectByAffinity(devices []*GPUDevice, constraint *AffinityConstraint) *GPUDevice {
	if constraint == nil || len(constraint.PreferredDeviceIDs) == 0 {
		return s.selectLeastUsed(devices)
	}

	// 优先选择偏好设备
	for _, id := range constraint.PreferredDeviceIDs {
		for _, d := range devices {
			if d.ID == id {
				return d
			}
		}
	}

	// 回退到最少使用
	return s.selectLeastUsed(devices)
}

// selectBinPacking 装箱策略选择
func (s *Scheduler) selectBinPacking(devices []*GPUDevice, requiredMiB int64) *GPUDevice {
	// 选择最接近但满足需求的设备（减少碎片）
	var selected *GPUDevice
	bestFit := int64(1<<63 - 1)

	for _, d := range devices {
		if d.MemoryFree >= requiredMiB {
			fit := d.MemoryFree - requiredMiB
			if fit < bestFit {
				bestFit = fit
				selected = d
			}
		}
	}

	if selected == nil {
		selected = s.selectLeastUsed(devices)
	}
	return selected
}

// checkOvercommit 检查超分配
func (s *Scheduler) checkOvercommit(device *GPUDevice, requestMiB int64) bool {
	if s.policy.OvercommitRatio <= 1.0 {
		// 不允许超分配
		return device.MemoryFree >= requestMiB
	}

	// 允许超分配
	maxMemory := int64(float64(device.MemoryTotal) * s.policy.OvercommitRatio)
	return (device.MemoryUsed + requestMiB) <= maxMemory
}

// ========== 查询接口 ==========

// ListDevices 列出所有 GPU 设备
func (s *Scheduler) ListDevices() []*GPUDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*GPUDevice, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, d)
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Index < devices[j].Index
	})

	return devices
}

// GetDevice 获取指定 GPU 设备
func (s *Scheduler) GetDevice(deviceID string) (*GPUDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return nil, &NotFoundError{Resource: "device", ID: deviceID}
	}

	return device, nil
}

// GetAllocation 获取分配记录
func (s *Scheduler) GetAllocation(allocationID string) (*GPUAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allocation, exists := s.allocations[allocationID]
	if !exists {
		return nil, &NotFoundError{Resource: "allocation", ID: allocationID}
	}

	return allocation, nil
}

// ListAllocations 列出所有分配记录
func (s *Scheduler) ListAllocations() []*GPUAllocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allocations := make([]*GPUAllocation, 0, len(s.allocations))
	for _, a := range s.allocations {
		allocations = append(allocations, a)
	}

	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].CreatedAt.After(allocations[j].CreatedAt)
	})

	return allocations
}

// GetStats 获取 GPU 使用统计
func (s *Scheduler) GetStats() *GPUStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &GPUStats{
		TotalDevices: len(s.devices),
		UpdatedAt:    time.Now(),
		Policy:       s.policy,
	}

	var totalMem, usedMem int64
	var onlineDevices int

	for _, device := range s.devices {
		totalMem += device.MemoryTotal
		usedMem += device.MemoryUsed

		if device.Status == DeviceStatusOnline {
			onlineDevices++
		}

		memUtil := float64(0)
		if device.MemoryTotal > 0 {
			memUtil = float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
		}

		stats.DeviceStats = append(stats.DeviceStats, DeviceStatsEntry{
			DeviceID:          device.ID,
			Name:              device.Name,
			Temperature:       device.Temperature,
			PowerDraw:         device.PowerDraw,
			MemoryUtilization: memUtil,
			GPUUtilization:    device.UtilizationGPU,
			AllocationCount:   len(device.Allocations),
		})
	}

	stats.OnlineDevices = onlineDevices
	stats.TotalMemoryMiB = totalMem
	stats.UsedMemoryMiB = usedMem
	stats.FreeMemoryMiB = totalMem - usedMem

	if totalMem > 0 {
		stats.MemoryUtilization = float64(usedMem) / float64(totalMem) * 100
	}

	// 统计分配数
	for _, alloc := range s.allocations {
		stats.TotalAllocations++
		if alloc.Status == AllocationStatusActive {
			stats.ActiveAllocations++
		}
	}

	return stats
}

// UpdatePolicy 更新调度策略
func (s *Scheduler) UpdatePolicy(req UpdatePolicyRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Strategy != "" {
		s.policy.Strategy = req.Strategy
	}
	if req.PreemptionEnabled != nil {
		s.policy.PreemptionEnabled = *req.PreemptionEnabled
	}
	if req.ReservedPercent != nil {
		if *req.ReservedPercent < 0 || *req.ReservedPercent > 100 {
			return &ValidationError{Field: "reserved_percent", Message: "必须在 0-100 之间"}
		}
		s.policy.ReservedPercent = *req.ReservedPercent
	}
	if req.OvercommitRatio != nil {
		if *req.OvercommitRatio < 1.0 {
			return &ValidationError{Field: "overcommit_ratio", Message: "必须 >= 1.0"}
		}
		s.policy.OvercommitRatio = *req.OvercommitRatio
	}
	if req.MaxTemperature != nil {
		if *req.MaxTemperature < 0 || *req.MaxTemperature > 120 {
			return &ValidationError{Field: "max_temperature", Message: "必须在 0-120 之间"}
		}
		s.policy.MaxTemperature = *req.MaxTemperature
	}

	s.policy.UpdatedAt = time.Now()

	s.logger.Info("调度策略已更新", zap.Any("policy", s.policy))
	return nil
}

// GetPolicy 获取当前调度策略
func (s *Scheduler) GetPolicy() SchedulerPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy
}

// ========== 资源池管理 ==========

// CreatePool 创建 GPU 资源池
// 资源池将多个 GPU 设备组合成一个逻辑单元，支持统一调度
// pool: 资源池配置，DeviceIDs 必须包含有效的设备 ID
// 返回: 创建的资源池或错误
func (s *Scheduler) CreatePool(pool *GPUPool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证资源池 ID
	if pool.ID == "" {
		return &ValidationError{Field: "pool_id", Message: "不能为空"}
	}

	// 检查 ID 是否已存在
	if _, exists := s.pools[pool.ID]; exists {
		return &ValidationError{Field: "pool_id", Message: fmt.Sprintf("资源池 %s 已存在", pool.ID)}
	}

	// 验证设备是否存在
	for _, deviceID := range pool.DeviceIDs {
		if _, exists := s.devices[deviceID]; !exists {
			return &NotFoundError{Resource: "device", ID: deviceID}
		}
	}

	// 初始化资源池
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	// 计算资源池总量
	var totalMem int64
	for _, deviceID := range pool.DeviceIDs {
		if device, exists := s.devices[deviceID]; exists {
			totalMem += device.MemoryTotal
		}
	}
	pool.TotalMemoryMiB = totalMem
	pool.FreeMemoryMiB = totalMem

	// 如果未指定策略，使用调度器默认策略
	if pool.Policy.Strategy == "" {
		pool.Policy = s.policy
	}

	s.pools[pool.ID] = pool

	s.logger.Info("创建 GPU 资源池",
		zap.String("pool_id", pool.ID),
		zap.String("name", pool.Name),
		zap.Int("device_count", len(pool.DeviceIDs)),
		zap.Int64("total_memory_mib", totalMem))

	return nil
}

// DeletePool 删除 GPU 资源池
// 仅当资源池无活跃分配时允许删除
func (s *Scheduler) DeletePool(poolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolID]
	if !exists {
		return &NotFoundError{Resource: "pool", ID: poolID}
	}

	// 检查是否有活跃分配
	if pool.AllocationCount > 0 {
		return fmt.Errorf("资源池 %s 仍有 %d 个活跃分配，无法删除", poolID, pool.AllocationCount)
	}

	delete(s.pools, poolID)

	s.logger.Info("删除 GPU 资源池", zap.String("pool_id", poolID))
	return nil
}

// GetPool 获取资源池信息
func (s *Scheduler) GetPool(poolID string) (*GPUPool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool, exists := s.pools[poolID]
	if !exists {
		return nil, &NotFoundError{Resource: "pool", ID: poolID}
	}

	// 刷新资源池统计
	pool = s.refreshPoolStats(pool)
	return pool, nil
}

// ListPools 列出所有资源池
func (s *Scheduler) ListPools() []*GPUPool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pools := make([]*GPUPool, 0, len(s.pools))
	for _, pool := range s.pools {
		// 刷新每个池的统计
		refreshed := s.refreshPoolStats(pool)
		pools = append(pools, refreshed)
	}

	sort.Slice(pools, func(i, j int) bool {
		return pools[i].CreatedAt.Before(pools[j].CreatedAt)
	})

	return pools
}

// AddDeviceToPool 将设备添加到资源池
func (s *Scheduler) AddDeviceToPool(poolID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolID]
	if !exists {
		return &NotFoundError{Resource: "pool", ID: poolID}
	}

	device, exists := s.devices[deviceID]
	if !exists {
		return &NotFoundError{Resource: "device", ID: deviceID}
	}

	// 检查设备是否已在池中
	for _, id := range pool.DeviceIDs {
		if id == deviceID {
			return fmt.Errorf("设备 %s 已在资源池 %s 中", deviceID, poolID)
		}
	}

	pool.DeviceIDs = append(pool.DeviceIDs, deviceID)
	pool.TotalMemoryMiB += device.MemoryTotal
	pool.FreeMemoryMiB += device.MemoryFree
	pool.UpdatedAt = time.Now()

	s.logger.Info("设备添加到资源池",
		zap.String("pool_id", poolID),
		zap.String("device_id", deviceID))

	return nil
}

// RemoveDeviceFromPool 从资源池移除设备
// 仅当设备无活跃分配时允许移除
func (s *Scheduler) RemoveDeviceFromPool(poolID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolID]
	if !exists {
		return &NotFoundError{Resource: "pool", ID: poolID}
	}

	device, exists := s.devices[deviceID]
	if !exists {
		return &NotFoundError{Resource: "device", ID: deviceID}
	}

	// 检查设备是否有活跃分配
	allocations := s.getDeviceAllocations(deviceID)
	if len(allocations) > 0 {
		return fmt.Errorf("设备 %s 仍有 %d 个活跃分配，无法从资源池移除", deviceID, len(allocations))
	}

	// 从池中移除设备
	for i, id := range pool.DeviceIDs {
		if id == deviceID {
			pool.DeviceIDs = append(pool.DeviceIDs[:i], pool.DeviceIDs[i+1:]...)
			break
		}
	}

	pool.TotalMemoryMiB -= device.MemoryTotal
	pool.FreeMemoryMiB -= device.MemoryFree
	pool.UpdatedAt = time.Now()

	s.logger.Info("设备从资源池移除",
		zap.String("pool_id", poolID),
		zap.String("device_id", deviceID))

	return nil
}

// AllocateFromPool 从指定资源池分配 GPU 资源
// 仅在资源池内的设备中进行调度
func (s *Scheduler) AllocateFromPool(poolID string, req AllocateRequest) (*GPUAllocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[poolID]
	if !exists {
		return nil, &NotFoundError{Resource: "pool", ID: poolID}
	}

	// 验证请求
	if req.ContainerID == "" {
		return nil, &ValidationError{Field: "container_id", Message: "不能为空"}
	}
	if req.MemoryMiB <= 0 {
		return nil, &ValidationError{Field: "memory_mib", Message: "必须大于 0"}
	}

	if req.Priority == "" {
		req.Priority = PriorityMedium
	}

	// 在资源池范围内选择设备
	poolDevices := s.getPoolDevices(pool)
	if len(poolDevices) == 0 {
		return nil, &InsufficientResourceError{Message: fmt.Sprintf("资源池 %s 无可用设备", poolID)}
	}

	// 应用资源池策略选择设备
	device, err := s.selectDeviceFromPool(poolDevices, req, pool.Policy)
	if err != nil {
		return nil, err
	}

	// 检查超分配
	if !s.checkOvercommitWithPolicy(device, req.MemoryMiB, pool.Policy) {
		return nil, &InsufficientResourceError{
			Message: fmt.Sprintf("设备 %s 显存不足，请求 %d MiB，可用 %d MiB", device.ID, req.MemoryMiB, device.MemoryFree),
		}
	}

	// 创建分配记录
	allocation := &GPUAllocation{
		ID:            generateID(),
		DeviceID:      device.ID,
		ContainerID:   req.ContainerID,
		ContainerName: req.ContainerName,
		VMID:          req.VMID,
		MemoryMiB:     req.MemoryMiB,
		Priority:      req.Priority,
		Status:        AllocationStatusActive,
		Constraint:    req.Constraint,
		CreatedAt:     time.Now(),
		Labels:        req.Labels,
	}

	// 更新设备状态
	device.MemoryUsed += req.MemoryMiB
	device.MemoryFree -= req.MemoryMiB
	device.Allocations = append(device.Allocations, allocation)
	device.UpdatedAt = time.Now()

	// 更新资源池统计
	pool.UsedMemoryMiB += req.MemoryMiB
	pool.FreeMemoryMiB -= req.MemoryMiB
	pool.AllocationCount++
	pool.UpdatedAt = time.Now()

	// 保存分配记录
	s.allocations[allocation.ID] = allocation

	s.logger.Info("从资源池分配 GPU 资源成功",
		zap.String("pool_id", poolID),
		zap.String("allocation_id", allocation.ID),
		zap.String("device_id", device.ID),
		zap.Int64("memory_mib", req.MemoryMiB))

	return allocation, nil
}

// getPoolDevices 获取资源池内的设备列表
func (s *Scheduler) getPoolDevices(pool *GPUPool) []*GPUDevice {
	var devices []*GPUDevice
	for _, deviceID := range pool.DeviceIDs {
		if device, exists := s.devices[deviceID]; exists {
			devices = append(devices, device)
		}
	}
	return devices
}

// selectDeviceFromPool 从池内设备中选择最佳设备
func (s *Scheduler) selectDeviceFromPool(devices []*GPUDevice, req AllocateRequest, policy SchedulerPolicy) (*GPUDevice, error) {
	// 过滤可用设备
	var available []*GPUDevice
	for _, device := range devices {
		if device.Status != DeviceStatusOnline {
			continue
		}
		if device.Temperature > policy.MaxTemperature {
			continue
		}
		if req.Constraint != nil && !s.matchesConstraint(device, req.Constraint) {
			continue
		}
		available = append(available, device)
	}

	if len(available) == 0 {
		return nil, &InsufficientResourceError{Message: "资源池内无可用设备"}
	}

	// 根据策略选择
	switch policy.Strategy {
	case StrategyRoundRobin:
		return s.selectRoundRobin(available), nil
	case StrategyLeastUsed:
		return s.selectLeastUsed(available), nil
	case StrategyPriority:
		return s.selectByPriority(available, req.Priority), nil
	case StrategyAffinity:
		return s.selectByAffinity(available, req.Constraint), nil
	case StrategyBinPacking:
		return s.selectBinPacking(available, req.MemoryMiB), nil
	default:
		return s.selectLeastUsed(available), nil
	}
}

// checkOvercommitWithPolicy 使用指定策略检查超分配
func (s *Scheduler) checkOvercommitWithPolicy(device *GPUDevice, requestMiB int64, policy SchedulerPolicy) bool {
	if policy.OvercommitRatio <= 1.0 {
		return device.MemoryFree >= requestMiB
	}
	maxMemory := int64(float64(device.MemoryTotal) * policy.OvercommitRatio)
	return (device.MemoryUsed + requestMiB) <= maxMemory
}

// refreshPoolStats 刷新资源池统计信息
func (s *Scheduler) refreshPoolStats(pool *GPUPool) *GPUPool {
	var totalMem, usedMem, freeMem int64
	var allocCount int

	for _, deviceID := range pool.DeviceIDs {
		if device, exists := s.devices[deviceID]; exists {
			totalMem += device.MemoryTotal
			usedMem += device.MemoryUsed
			freeMem += device.MemoryFree
			allocCount += len(device.Allocations)
		}
	}

	pool.TotalMemoryMiB = totalMem
	pool.UsedMemoryMiB = usedMem
	pool.FreeMemoryMiB = freeMem
	pool.AllocationCount = allocCount

	return pool
}

// ========== 抢占机制 ==========

// Preempt 抢占低优先级任务的 GPU 资源
// 当高优先级任务无法分配时，抢占低优先级任务释放资源
// req: 高优先级任务的分配请求
// 返回: 被抢占的分配记录列表和错误
func (s *Scheduler) Preempt(req AllocateRequest) ([]*GPUAllocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.policy.PreemptionEnabled {
		return nil, &PolicyViolationError{Message: "抢占功能未启用"}
	}

	// 仅高优先级任务可触发抢占
	if req.Priority != PriorityHigh {
		return nil, &PolicyViolationError{Message: "仅高优先级任务可触发抢占"}
	}

	// 尝试正常分配
	device, err := s.selectDevice(req)
	if err == nil && s.checkOvercommit(device, req.MemoryMiB) {
		// 无需抢占，直接返回空列表
		return nil, nil
	}

	// 找到可抢占的低优先级分配
	preemptable := s.findPreemptableAllocations(req.MemoryMiB)
	if len(preemptable) == 0 {
		return nil, &InsufficientResourceError{Message: "无低优先级任务可抢占"}
	}

	// 执行抢占
	var preempted []*GPUAllocation
	var freedMem int64

	for _, alloc := range preemptable {
		if freedMem >= req.MemoryMiB {
			break
		}

		// 更新分配状态
		alloc.Status = AllocationStatusPreempted

		// 释放设备资源
		if device, exists := s.devices[alloc.DeviceID]; exists {
			device.MemoryUsed -= alloc.MemoryMiB
			device.MemoryFree += alloc.MemoryMiB
			device.UpdatedAt = time.Now()

			// 从设备分配列表移除
			for i, d := range device.Allocations {
				if d.ID == alloc.ID {
					device.Allocations = append(device.Allocations[:i], device.Allocations[i+1:]...)
					break
				}
			}
		}

		freedMem += alloc.MemoryMiB
		preempted = append(preempted, alloc)

		s.logger.Warn("GPU 资源被抢占",
			zap.String("preempted_allocation", alloc.ID),
			zap.String("container_id", alloc.ContainerID),
			zap.Int64("memory_mib", alloc.MemoryMiB),
			zap.String("priority", string(alloc.Priority)))
	}

	if len(preempted) > 0 {
		s.logger.Info("抢占完成",
			zap.Int("preempted_count", len(preempted)),
			zap.Int64("freed_memory_mib", freedMem))
	}

	return preempted, nil
}

// findPreemptableAllocations 找到可抢占的分配
// 按优先级从低到高排序，选择最旧的分配
func (s *Scheduler) findPreemptableAllocations(requiredMiB int64) []*GPUAllocation {
	var preemptable []*GPUAllocation

	// 收集所有低优先级和中优先级的活跃分配
	for _, alloc := range s.allocations {
		if alloc.Status != AllocationStatusActive {
			continue
		}
		// 低优先级和中优先级可被高优先级抢占
		if alloc.Priority == PriorityLow || alloc.Priority == PriorityMedium {
			preemptable = append(preemptable, alloc)
		}
	}

	// 按优先级排序（低优先级先被抢占）
	sort.Slice(preemptable, func(i, j int) bool {
		priorityOrder := map[Priority]int{
			PriorityLow:    0,
			PriorityMedium: 1,
			PriorityHigh:   2,
		}
		if priorityOrder[preemptable[i].Priority] != priorityOrder[preemptable[j].Priority] {
			return priorityOrder[preemptable[i].Priority] < priorityOrder[preemptable[j].Priority]
		}
		// 同优先级按创建时间排序（最旧的先被抢占）
		return preemptable[i].CreatedAt.Before(preemptable[j].CreatedAt)
	})

	// 选择足够的分配以满足需求
	var selected []*GPUAllocation
	var totalMem int64
	for _, alloc := range preemptable {
		selected = append(selected, alloc)
		totalMem += alloc.MemoryMiB
		if totalMem >= requiredMiB {
			break
		}
	}

	return selected
}

// ========== 健康检查 ==========

// HealthCheck 执行设备健康检查
// 检查温度、功耗、显存使用率等指标
// 返回: 健康检查结果
func (s *Scheduler) HealthCheck() *HealthCheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &HealthCheckResult{
		Timestamp: time.Now(),
		Devices:   make([]DeviceHealth, 0, len(s.devices)),
	}

	var healthy, unhealthy, warning int

	for _, device := range s.devices {
		health := s.checkDeviceHealth(device)
		result.Devices = append(result.Devices, health)

		switch health.Status {
		case "healthy":
			healthy++
		case "unhealthy":
			unhealthy++
		case "warning":
			warning++
		}
	}

	result.HealthyDevices = healthy
	result.UnhealthyDevices = unhealthy
	result.WarningDevices = warning
	result.TotalDevices = len(s.devices)

	if unhealthy > 0 {
		result.OverallStatus = "unhealthy"
	} else if warning > 0 {
		result.OverallStatus = "warning"
	} else {
		result.OverallStatus = "healthy"
	}

	return result
}

// checkDeviceHealth 检查单个设备健康状态
func (s *Scheduler) checkDeviceHealth(device *GPUDevice) DeviceHealth {
	health := DeviceHealth{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Checks:     make([]HealthCheckItem, 0),
	}

	var issues []string

	// 温度检查
	tempStatus := "healthy"
	if device.Temperature > s.policy.MaxTemperature {
		tempStatus = "unhealthy"
		issues = append(issues, fmt.Sprintf("温度过高: %d°C (阈值: %d°C)", device.Temperature, s.policy.MaxTemperature))
	} else if device.Temperature > s.policy.MaxTemperature-10 {
		tempStatus = "warning"
		issues = append(issues, fmt.Sprintf("温度接近阈值: %d°C", device.Temperature))
	}
	health.Checks = append(health.Checks, HealthCheckItem{
		Name:    "temperature",
		Status:  tempStatus,
		Value:   fmt.Sprintf("%d°C", device.Temperature),
		Message: fmt.Sprintf("阈值: %d°C", s.policy.MaxTemperature),
	})

	// 显存使用率检查
	memUtil := float64(0)
	if device.MemoryTotal > 0 {
		memUtil = float64(device.MemoryUsed) / float64(device.MemoryTotal) * 100
	}
	memStatus := "healthy"
	if memUtil > 95 {
		memStatus = "warning"
		issues = append(issues, fmt.Sprintf("显存使用率过高: %.1f%%", memUtil))
	}
	health.Checks = append(health.Checks, HealthCheckItem{
		Name:    "memory_utilization",
		Status:  memStatus,
		Value:   fmt.Sprintf("%.1f%%", memUtil),
		Message: fmt.Sprintf("已用: %d MiB / 总: %d MiB", device.MemoryUsed, device.MemoryTotal),
	})

	// 功耗检查
	powerStatus := "healthy"
	if device.PowerLimit > 0 && device.PowerDraw > device.PowerLimit*0.95 {
		powerStatus = "warning"
		issues = append(issues, fmt.Sprintf("功耗接近上限: %.1fW / %.1fW", device.PowerDraw, device.PowerLimit))
	}
	health.Checks = append(health.Checks, HealthCheckItem{
		Name:    "power_draw",
		Status:  powerStatus,
		Value:   fmt.Sprintf("%.1fW", device.PowerDraw),
		Message: fmt.Sprintf("上限: %.1fW", device.PowerLimit),
	})

	// 设备状态检查
	statusCheck := "healthy"
	if device.Status != DeviceStatusOnline {
		statusCheck = "unhealthy"
		issues = append(issues, fmt.Sprintf("设备状态异常: %s", device.Status))
	}
	health.Checks = append(health.Checks, HealthCheckItem{
		Name:   "device_status",
		Status: statusCheck,
		Value:  string(device.Status),
	})

	// 综合状态
	if len(issues) > 0 {
		hasUnhealthy := false
		for _, check := range health.Checks {
			if check.Status == "unhealthy" {
				hasUnhealthy = true
				break
			}
		}
		if hasUnhealthy {
			health.Status = "unhealthy"
		} else {
			health.Status = "warning"
		}
		health.Issues = issues
	} else {
		health.Status = "healthy"
	}

	return health
}

// ========== 过期清理 ==========

// CleanupExpired 清理过期的分配记录
// 检查所有分配的 ExpiresAt 字段，释放已过期的资源
// 返回: 清理的分配数量
func (s *Scheduler) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var cleaned int

	for _, alloc := range s.allocations {
		// 跳过非活跃状态
		if alloc.Status != AllocationStatusActive {
			continue
		}

		// 检查是否过期
		if alloc.ExpiresAt != nil && alloc.ExpiresAt.Before(now) {
			// 更新分配状态
			alloc.Status = AllocationStatusExpired

			// 释放设备资源
			if device, exists := s.devices[alloc.DeviceID]; exists {
				device.MemoryUsed -= alloc.MemoryMiB
				device.MemoryFree += alloc.MemoryMiB
				device.UpdatedAt = time.Now()

				// 从设备分配列表移除
				for i, d := range device.Allocations {
					if d.ID == alloc.ID {
						device.Allocations = append(device.Allocations[:i], device.Allocations[i+1:]...)
						break
					}
				}
			}

			// 更新资源池统计
			for _, pool := range s.pools {
				for _, deviceID := range pool.DeviceIDs {
					if deviceID == alloc.DeviceID {
						pool.UsedMemoryMiB -= alloc.MemoryMiB
						pool.FreeMemoryMiB += alloc.MemoryMiB
						pool.AllocationCount--
						pool.UpdatedAt = now
						break
					}
				}
			}

			cleaned++

			s.logger.Info("清理过期分配",
				zap.String("allocation_id", alloc.ID),
				zap.String("container_id", alloc.ContainerID),
				zap.Time("expired_at", *alloc.ExpiresAt))
		}
	}

	if cleaned > 0 {
		s.logger.Info("过期清理完成", zap.Int("cleaned_count", cleaned))
	}

	return cleaned
}

// StartCleanupWorker 启动定期清理协程
// interval: 清理间隔
func (s *Scheduler) StartCleanupWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.CleanupExpired()
			}
		}
	}()

	s.logger.Info("启动过期清理协程", zap.Duration("interval", interval))
}

// SetAllocationExpiry 设置分配的过期时间
func (s *Scheduler) SetAllocationExpiry(allocationID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alloc, exists := s.allocations[allocationID]
	if !exists {
		return &NotFoundError{Resource: "allocation", ID: allocationID}
	}

	if alloc.Status != AllocationStatusActive {
		return fmt.Errorf("分配 %s 状态为 %s，无法设置过期时间", allocationID, alloc.Status)
	}

	alloc.ExpiresAt = &expiresAt

	s.logger.Info("设置分配过期时间",
		zap.String("allocation_id", allocationID),
		zap.Time("expires_at", expiresAt))

	return nil
}

// ========== 辅助函数 ==========

// getDeviceAllocations 获取设备的分配列表
func (s *Scheduler) getDeviceAllocations(deviceID string) []*GPUAllocation {
	var allocations []*GPUAllocation
	for _, alloc := range s.allocations {
		if alloc.DeviceID == deviceID && alloc.Status == AllocationStatusActive {
			allocations = append(allocations, alloc)
		}
	}
	return allocations
}

// determineDeviceStatus 根据温度确定设备状态
func (s *Scheduler) determineDeviceStatus(temp int) DeviceStatus {
	if temp > s.policy.MaxTemperature {
		return DeviceStatusOverheat
	}
	return DeviceStatusOnline
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("alloc-%d", time.Now().UnixNano())
}

// parseInt 安全解析整数
func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

// parseInt64 安全解析 64 位整数
func parseInt64(s string) int64 {
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

// parseFloat 安全解析浮点数
func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}
