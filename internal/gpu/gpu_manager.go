package gpu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GPUManager GPU资源池管理器
// 参考TrueNAS 25.10多GPU支持和Unraid灵活硬件混用策略
type GPUManager struct {
	gpus       map[string]*GPUDevice // GPU池，按UUID索引
	gpuByIndex map[int]*GPUDevice    // GPU池，按索引索引
	
	// 配置
	config *ManagerConfig
	
	// 监控
	monitorInterval time.Duration
	stopMonitor    chan struct{}
	monitorRunning bool
	
	// 事件
	eventHandlers []GPUEventHandler
	
	// 日志
	logger *zap.Logger
	
	mu sync.RWMutex
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 监控配置
	MonitorInterval   time.Duration `json:"monitor_interval"`
	EnableMonitoring  bool          `json:"enable_monitoring"`
	
	// 调度配置
	DefaultTimeout    time.Duration `json:"default_timeout"`
	MaxRetryCount    int           `json:"max_retry_count"`
	RetryDelay       time.Duration `json:"retry_delay"`
	
	// 显存管理
	MemoryReserve    uint64        `json:"memory_reserve"`    // 预留显存(bytes)
	EnableOvercommit bool          `json:"enable_overcommit"` // 允许超卖
	
	// 存储
	StateFilePath    string        `json:"state_file_path"`
	
	// 模拟模式（用于测试）
	SimulationMode   bool          `json:"simulation_mode"`
	SimulatedGPUCount int          `json:"simulated_gpu_count"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MonitorInterval:   10 * time.Second,
		EnableMonitoring:  true,
		DefaultTimeout:    5 * time.Minute,
		MaxRetryCount:     3,
		RetryDelay:        time.Second,
		MemoryReserve:     512 * 1024 * 1024, // 512MB
		EnableOvercommit: false,
		StateFilePath:     "/var/lib/gpu-manager/state.json",
	}
}

// GPUEventHandler GPU事件处理器
type GPUEventHandler interface {
	OnGPUAdded(gpu *GPUDevice)
	OnGPURemoved(gpu *GPUDevice)
	OnGPUStatusChange(gpu *GPUDevice, oldStatus, newStatus GPUStatus)
	OnGPUAllocated(gpu *GPUDevice, taskID string, memory uint64)
	OnGPUReleased(gpu *GPUDevice, taskID string, memory uint64)
}

// NewGPUManager 创建GPU管理器
func NewGPUManager(config *ManagerConfig, logger *zap.Logger) (*GPUManager, error) {
	if config == nil {
		config = DefaultManagerConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	
	m := &GPUManager{
		gpus:           make(map[string]*GPUDevice),
		gpuByIndex:     make(map[int]*GPUDevice),
		config:         config,
		monitorInterval: config.MonitorInterval,
		stopMonitor:    make(chan struct{}),
		logger:         logger,
	}
	
	// 初始化GPU检测
	if config.SimulationMode {
		if err := m.initSimulatedGPUs(config.SimulatedGPUCount); err != nil {
			return nil, fmt.Errorf("初始化模拟GPU失败: %w", err)
		}
	} else {
		if err := m.detectGPUs(); err != nil {
			m.logger.Warn("GPU检测失败，将以空池启动", zap.Error(err))
		}
	}
	
	// 加载状态
	if config.StateFilePath != "" {
		if err := m.loadState(); err != nil {
			m.logger.Debug("加载GPU状态失败", zap.Error(err))
		}
	}
	
	// 启动监控
	if config.EnableMonitoring {
		go m.startMonitoring()
	}
	
	return m, nil
}

// initSimulatedGPUs 初始化模拟GPU（用于测试）
func (m *GPUManager) initSimulatedGPUs(count int) error {
	architectures := []Architecture{ArchAmpere, ArchAda, ArchHopper, ArchBlackwell}
	
	for i := 0; i < count; i++ {
		arch := architectures[i%len(architectures)]
		var name string
		var memory uint64
		var compute string
		
		switch arch {
		case ArchAmpere:
			name = fmt.Sprintf("NVIDIA RTX A%d00", (i+3)*10)
			memory = 48 * 1024 * 1024 * 1024 // 48GB
			compute = "8.0"
		case ArchAda:
			name = fmt.Sprintf("NVIDIA RTX %d90", 40+i*10)
			memory = 24 * 1024 * 1024 * 1024 // 24GB
			compute = "8.9"
		case ArchHopper:
			name = fmt.Sprintf("NVIDIA H100-%d", i+1)
			memory = 80 * 1024 * 1024 * 1024 // 80GB
			compute = "9.0"
		case ArchBlackwell:
			name = fmt.Sprintf("NVIDIA B%d0", (i+2)*10)
			memory = 192 * 1024 * 1024 * 1024 // 192GB
			compute = "10.0"
		}
		
		gpu := &GPUDevice{
			ID:                 fmt.Sprintf("GPU-%d-%s", i, strings.ToLower(string(arch))),
			Index:              i,
			Name:               name,
			Architecture:       arch,
			TotalMemory:        memory,
			AvailableMemory:    memory,
			ComputeCapability:  compute,
			CUDACores:          10000 + i*1000,
			TensorCores:        300 + i*50,
			SMCount:            100 + i*10,
			Status:             StatusIdle,
			Labels: map[string]string{
				"type":  "simulated",
				"arch":  string(arch),
				"index": fmt.Sprintf("%d", i),
			},
			DriverVersion: "535.129.03",
			CUDAVersion:   "12.2",
			NUMANode:      i / 2, // 模拟NUMA节点
		}
		
		m.gpus[gpu.ID] = gpu
		m.gpuByIndex[gpu.Index] = gpu
	}
	
	m.logger.Info("初始化模拟GPU完成", zap.Int("count", count))
	return nil
}

// detectGPUs 检测系统GPU
func (m *GPUManager) detectGPUs() error {
	// 尝试使用nvidia-smi检测
	gpus, err := m.detectNvidiaGPUs()
	if err == nil && len(gpus) > 0 {
		m.mu.Lock()
		for _, gpu := range gpus {
			m.gpus[gpu.ID] = gpu
			m.gpuByIndex[gpu.Index] = gpu
		}
		m.mu.Unlock()
		m.logger.Info("检测到NVIDIA GPU", zap.Int("count", len(gpus)))
		return nil
	}
	
	return fmt.Errorf("未检测到GPU: %w", err)
}

// detectNvidiaGPUs 使用nvidia-smi检测NVIDIA GPU
func (m *GPUManager) detectNvidiaGPUs() ([]*GPUDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,uuid,name,memory.total,memory.free,memory.used,temperature.gpu,power.draw,power.limit,utilization.gpu,utilization.memory,compute_cap,driver_version,cuda_version,pcie.link.gen.current", "--format=csv,noheader,nounits")
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi执行失败: %w", err)
	}
	
	var gpus []*GPUDevice
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		gpu, err := m.parseNvidiaSMILine(line)
		if err != nil {
			m.logger.Warn("解析GPU信息失败", zap.String("line", line), zap.Error(err))
			continue
		}
		
		if gpu != nil {
			// 解析架构
			gpu.Architecture = m.detectArchitecture(gpu.Name, gpu.ComputeCapability)
			gpu.Status = StatusIdle
			gpus = append(gpus, gpu)
		}
	}
	
	return gpus, nil
}

// parseNvidiaSMILine 解析nvidia-smi输出行
func (m *GPUManager) parseNvidiaSMILine(line string) (*GPUDevice, error) {
	fields := strings.Split(line, ", ")
	if len(fields) < 14 {
		return nil, fmt.Errorf("字段数量不足: %d", len(fields))
	}
	
	index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil {
		return nil, fmt.Errorf("解析索引失败: %w", err)
	}
	
	uuid := strings.TrimSpace(fields[1])
	name := strings.TrimSpace(fields[2])
	
	totalMem, err := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析总显存失败: %w", err)
	}
	totalMem *= 1024 * 1024 // MB to bytes
	
	freeMem, err := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析可用显存失败: %w", err)
	}
	freeMem *= 1024 * 1024
	
	usedMem, err := strconv.ParseUint(strings.TrimSpace(fields[5]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析已用显存失败: %w", err)
	}
	usedMem *= 1024 * 1024
	
	temp, _ := strconv.ParseUint(strings.TrimSpace(fields[6]), 10, 32)
	powerDraw, _ := strconv.ParseFloat(strings.TrimSpace(fields[7]), 64)
	powerLimit, _ := strconv.ParseFloat(strings.TrimSpace(fields[8]), 64)
	gpuUtil, _ := strconv.ParseUint(strings.TrimSpace(fields[9]), 10, 32)
	memUtil, _ := strconv.ParseUint(strings.TrimSpace(fields[10]), 10, 32)
	computeCap := strings.TrimSpace(fields[11])
	driverVer := strings.TrimSpace(fields[12])
	cudaVer := strings.TrimSpace(fields[13])
	
	return &GPUDevice{
		ID:                 uuid,
		Index:              index,
		Name:               name,
		TotalMemory:        totalMem,
		AvailableMemory:    freeMem,
		UsedMemory:         usedMem,
		ComputeCapability:  computeCap,
		Temperature:        uint(temp),
		PowerUsage:         uint(powerDraw),
		PowerLimit:         uint(powerLimit),
		UtilizationGPU:     uint(gpuUtil),
		UtilizationMem:      uint(memUtil),
		DriverVersion:      driverVer,
		CUDAVersion:        cudaVer,
		Labels:             make(map[string]string),
	}, nil
}

// detectArchitecture 根据名称和计算能力检测GPU架构
func (m *GPUManager) detectArchitecture(name, computeCap string) Architecture {
	nameLower := strings.ToLower(name)
	
	// Blackwell架构 - TrueNAS 25.10支持
	if strings.Contains(nameLower, "b100") || strings.Contains(nameLower, "b200") ||
		strings.Contains(nameLower, "gb") || strings.Contains(nameLower, "blackwell") {
		return ArchBlackwell
	}
	
	// Hopper架构
	if strings.Contains(nameLower, "h100") || strings.Contains(nameLower, "h200") ||
		strings.Contains(nameLower, "hopper") {
		return ArchHopper
	}
	
	// Ada架构
	if strings.Contains(nameLower, "rtx 40") || strings.Contains(nameLower, "ada") ||
		strings.Contains(nameLower, "l40") || strings.Contains(nameLower, "l4") {
		return ArchAda
	}
	
	// Ampere架构
	if strings.Contains(nameLower, "rtx 30") || strings.Contains(nameLower, "a100") ||
		strings.Contains(nameLower, "a10") || strings.Contains(nameLower, "a30") ||
		strings.Contains(nameLower, "ampere") {
		return ArchAmpere
	}
	
	// 根据计算能力推断
	switch computeCap {
	case "10.0", "10.1":
		return ArchBlackwell
	case "9.0", "9.1":
		return ArchHopper
	case "8.9":
		return ArchAda
	case "8.0", "8.6", "8.7":
		return ArchAmpere
	}
	
	return ArchUnknown
}

// startMonitoring 启动监控
func (m *GPUManager) startMonitoring() {
	m.mu.Lock()
	m.monitorRunning = true
	m.mu.Unlock()
	
	ticker := time.NewTicker(m.monitorInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopMonitor:
			return
		case <-ticker.C:
			m.updateMetrics()
		}
	}
}

// updateMetrics 更新GPU指标
func (m *GPUManager) updateMetrics() {
	if m.config.SimulationMode {
		m.updateSimulatedMetrics()
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=index,temperature.gpu,power.draw,utilization.gpu,utilization.memory,memory.used", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		m.logger.Debug("更新GPU指标失败", zap.Error(err))
		return
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		fields := strings.Split(line, ", ")
		if len(fields) < 6 {
			continue
		}
		
		index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		
		m.mu.RLock()
		gpu, exists := m.gpuByIndex[index]
		m.mu.RUnlock()
		
		if !exists {
			continue
		}
		
		temp, _ := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 32)
		power, _ := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		gpuUtil, _ := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 32)
		memUtil, _ := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 32)
		memUsed, _ := strconv.ParseUint(strings.TrimSpace(fields[5]), 10, 64)
		memUsed *= 1024 * 1024
		
		gpu.UpdateMetrics(uint(temp), uint(power), uint(gpuUtil), uint(memUtil), 0)
		
		m.mu.Lock()
		gpu.UsedMemory = memUsed
		gpu.AvailableMemory = gpu.TotalMemory - memUsed
		m.mu.Unlock()
	}
}

// updateSimulatedMetrics 更新模拟GPU指标
func (m *GPUManager) updateSimulatedMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, gpu := range m.gpus {
		// 模拟随机波动
		gpu.Temperature = 40 + uint(time.Now().Unix()%30)
		gpu.UtilizationGPU = uint(time.Now().Unix() % 100)
		gpu.UtilizationMem = uint(time.Now().Unix() % 80)
		gpu.PowerUsage = 100 + uint(time.Now().Unix()%300)
	}
}

// Stop 停止管理器
func (m *GPUManager) Stop() {
	close(m.stopMonitor)
	
	if m.config.StateFilePath != "" {
		if err := m.saveState(); err != nil {
			m.logger.Warn("保存GPU状态失败", zap.Error(err))
		}
	}
}

// loadState 加载状态
func (m *GPUManager) loadState() error {
	data, err := os.ReadFile(m.config.StateFilePath)
	if err != nil {
		return err
	}
	
	var state struct {
		GPUs []*GPUDevice `json:"gpus"`
	}
	
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	m.mu.Lock()
	for _, gpu := range state.GPUs {
		if _, exists := m.gpus[gpu.ID]; !exists {
			m.gpus[gpu.ID] = gpu
			m.gpuByIndex[gpu.Index] = gpu
		}
	}
	m.mu.Unlock()
	
	return nil
}

// saveState 保存状态
func (m *GPUManager) saveState() error {
	m.mu.RLock()
	gpus := make([]*GPUDevice, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		gpus = append(gpus, gpu.Clone())
	}
	m.mu.RUnlock()
	
	state := struct {
		GPUs []*GPUDevice `json:"gpus"`
	}{
		GPUs: gpus,
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	dir := filepath.Dir(m.config.StateFilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	
	return os.WriteFile(m.config.StateFilePath, data, 0640)
}

// ========== GPU池操作 ==========

// ListGPUs 列出所有GPU
func (m *GPUManager) ListGPUs() []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*GPUDevice, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		result = append(result, gpu.Clone())
	}
	return result
}

// GetGPU 获取指定GPU
func (m *GPUManager) GetGPU(id string) *GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if gpu, exists := m.gpus[id]; exists {
		return gpu.Clone()
	}
	return nil
}

// GetGPUByIndex 按索引获取GPU
func (m *GPUManager) GetGPUByIndex(index int) *GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if gpu, exists := m.gpuByIndex[index]; exists {
		return gpu.Clone()
	}
	return nil
}

// GetAvailableGPUs 获取可用GPU列表
func (m *GPUManager) GetAvailableGPUs() []*GPUDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*GPUDevice, 0)
	for _, gpu := range m.gpus {
		if gpu.Status == StatusIdle {
			result = append(result, gpu.Clone())
		}
	}
	return result
}

// Count 统计GPU数量
func (m *GPUManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.gpus)
}

// CountByStatus 按状态统计GPU数量
func (m *GPUManager) CountByStatus(status GPUStatus) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	count := 0
	for _, gpu := range m.gpus {
		if gpu.Status == status {
			count++
		}
	}
	return count
}

// TotalMemory 获取总显存
func (m *GPUManager) TotalMemory() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var total uint64
	for _, gpu := range m.gpus {
		total += gpu.TotalMemory
	}
	return total
}

// AvailableMemory 获取可用显存
func (m *GPUManager) AvailableMemory() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var available uint64
	for _, gpu := range m.gpus {
		available += gpu.AvailableMemory
	}
	return available
}

// ========== 资源分配与释放 ==========

// AllocateGPU 分配GPU
func (m *GPUManager) AllocateGPU(ctx context.Context, req *GPUAllocationRequest) (*GPUAllocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 查找合适的GPU
	candidates := m.findCandidates(req)
	if len(candidates) == 0 {
		return nil, ErrNoGPUAvailable
	}
	
	// 选择最优GPU
	selected := m.selectBestGPU(candidates, req)
	if selected == nil {
		return nil, ErrNoGPUAvailable
	}
	
	// 执行分配
	allocation, err := m.doAllocate(ctx, selected, req)
	if err != nil {
		return nil, err
	}
	
	// 触发事件
	m.notifyAllocated(selected, allocation)
	
	return allocation, nil
}

// findCandidates 查找候选GPU
func (m *GPUManager) findCandidates(req *GPUAllocationRequest) []*GPUDevice {
	var candidates []*GPUDevice
	
	for _, gpu := range m.gpus {
		// 检查状态
		if gpu.Status != StatusIdle && gpu.Status != StatusBusy {
			continue
		}
		
		// 检查是否满足需求
		if !gpu.MatchesRequirements(req.Requirements) {
			continue
		}
		
		// 检查显存
		requiredMemory := req.MemorySize
		if requiredMemory == 0 {
			requiredMemory = 1024 * 1024 * 1024 // 默认1GB
		}
		
		available := gpu.AvailableMemory
		if !m.config.EnableOvercommit {
			available -= m.config.MemoryReserve
		}
		
		if available < requiredMemory {
			continue
		}
		
		// 检查独占模式
		if req.Requirements != nil && req.Requirements.ExclusiveMode {
			if gpu.Status != StatusIdle {
				continue
			}
		}
		
		candidates = append(candidates, gpu)
	}
	
	return candidates
}

// selectBestGPU 选择最优GPU
func (m *GPUManager) selectBestGPU(candidates []*GPUDevice, req *GPUAllocationRequest) *GPUDevice {
	if len(candidates) == 0 {
		return nil
	}
	
	// 按评分排序
	best := candidates[0]
	bestScore := m.scoreGPU(best, req)
	
	for _, gpu := range candidates[1:] {
		score := m.scoreGPU(gpu, req)
		if score > bestScore {
			best = gpu
			bestScore = score
		}
	}
	
	return best
}

// scoreGPU 计算GPU评分
func (m *GPUManager) scoreGPU(gpu *GPUDevice, req *GPUAllocationRequest) float64 {
	var score float64
	
	// 显存利用率越低越好（倾向于使用空闲GPU）
	memUtilScore := 100 - float64(gpu.UtilizationMem)
	score += memUtilScore * 0.3
	
	// GPU利用率越低越好
	gpuUtilScore := 100 - float64(gpu.UtilizationGPU)
	score += gpuUtilScore * 0.2
	
	// 显存匹配度（剩余显存刚好够用最好）
	if req.MemorySize > 0 {
		memFit := float64(gpu.AvailableMemory) / float64(req.MemorySize)
		if memFit > 2 {
			memFit = 2 // 避免过度偏好大显存
		}
		score += memFit * 20
	}
	
	// 温度越低越好
	tempScore := 100 - float64(gpu.Temperature)
	score += tempScore * 0.1
	
	// NUMA亲和性
	if req.Requirements != nil && req.Requirements.NUMANode >= 0 {
		if gpu.NUMANode == req.Requirements.NUMANode {
			score += 50
		}
	}
	
	// 架构匹配加分
	if req.Requirements != nil && len(req.Requirements.Architectures) > 0 {
		for _, arch := range req.Requirements.Architectures {
			if gpu.Architecture == arch {
				score += 30
				break
			}
		}
	}
	
	return score
}

// doAllocate 执行分配
func (m *GPUManager) doAllocate(ctx context.Context, gpu *GPUDevice, req *GPUAllocationRequest) (*GPUAllocation, error) {
	memory := req.MemorySize
	if memory == 0 {
		memory = 1024 * 1024 * 1024 // 默认1GB
	}
	
	// 独占模式
	if req.Requirements != nil && req.Requirements.ExclusiveMode {
		if !gpu.Reserve(req.TaskID) {
			return nil, ErrGPUReserved
		}
	} else {
		// 共享模式
		if !gpu.Allocate(memory, req.TaskID) {
			return nil, ErrGPUAllocationFailed
		}
	}
	
	allocation := &GPUAllocation{
		ID:          GenerateAllocationID(),
		GPUID:       gpu.ID,
		GPUIndex:    gpu.Index,
		TaskID:      req.TaskID,
		MemorySize:  memory,
		Exclusive:   req.Requirements != nil && req.Requirements.ExclusiveMode,
		AllocatedAt: time.Now(),
		ExpiresAt:   req.ExpiresAt,
		Priority:    req.Priority,
	}
	
	return allocation, nil
}

// ReleaseGPU 释放GPU
func (m *GPUManager) ReleaseGPU(allocationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 查找分配（从调度器获取）
	// 这里简化处理，直接按taskID查找
	for _, gpu := range m.gpus {
		if gpu.ReservedBy != "" {
			gpu.Unreserve()
			m.notifyReleased(gpu, gpu.ReservedBy, 0)
			return nil
		}
	}
	
	return ErrAllocationNotFound
}

// ReleaseGPUByTaskID 按任务ID释放GPU
func (m *GPUManager) ReleaseGPUByTaskID(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, gpu := range m.gpus {
		if gpu.ReservedBy == taskID {
			if gpu.Status == StatusReserved {
				gpu.Unreserve()
				m.notifyReleased(gpu, taskID, 0)
			}
			return nil
		}
	}
	
	return ErrAllocationNotFound
}

// ========== 事件处理 ==========

// RegisterEventHandler 注册事件处理器
func (m *GPUManager) RegisterEventHandler(handler GPUEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

func (m *GPUManager) notifyAllocated(gpu *GPUDevice, alloc *GPUAllocation) {
	for _, h := range m.eventHandlers {
		h.OnGPUAllocated(gpu, alloc.TaskID, alloc.MemorySize)
	}
}

func (m *GPUManager) notifyReleased(gpu *GPUDevice, taskID string, memory uint64) {
	for _, h := range m.eventHandlers {
		h.OnGPUReleased(gpu, taskID, memory)
	}
}

// ========== 辅助函数 ==========

// GenerateAllocationID 生成分配ID
func GenerateAllocationID() string {
	return fmt.Sprintf("alloc-%d", time.Now().UnixNano())
}

// 错误定义
var (
	ErrNoGPUAvailable     = fmt.Errorf("没有可用的GPU")
	ErrGPUReserved        = fmt.Errorf("GPU已被预留")
	ErrGPUAllocationFailed = fmt.Errorf("GPU分配失败")
	ErrAllocationNotFound  = fmt.Errorf("分配记录不存在")
)

// GPUAllocationRequest GPU分配请求
type GPUAllocationRequest struct {
	TaskID       string            `json:"task_id"`
	MemorySize   uint64            `json:"memory_size"`   // 显存需求(bytes)
	Priority     int               `json:"priority"`      // 优先级
	Requirements *GPURequirements `json:"requirements"`
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"`
}

// GPUAllocation GPU分配记录
type GPUAllocation struct {
	ID          string     `json:"id"`
	GPUID       string     `json:"gpu_id"`
	GPUIndex    int        `json:"gpu_index"`
	TaskID      string     `json:"task_id"`
	MemorySize  uint64     `json:"memory_size"`
	Exclusive   bool       `json:"exclusive"`
	AllocatedAt time.Time  `json:"allocated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Priority    int        `json:"priority"`
}

// GPUStats GPU统计信息
type GPUStats struct {
	TotalGPUs      int     `json:"total_gpus"`
	IdleGPUs       int     `json:"idle_gpus"`
	BusyGPUs       int     `json:"busy_gpus"`
	ReservedGPUs   int     `json:"reserved_gpus"`
	OfflineGPUs    int     `json:"offline_gpus"`
	TotalMemory    uint64  `json:"total_memory"`
	UsedMemory     uint64  `json:"used_memory"`
	AvailableMemory uint64 `json:"available_memory"`
	AvgTemperature uint    `json:"avg_temperature"`
	AvgUtilization uint    `json:"avg_utilization"`
}

// GetStats 获取统计信息
func (m *GPUManager) GetStats() *GPUStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := &GPUStats{}
	var totalTemp, totalUtil uint
	
	for _, gpu := range m.gpus {
		stats.TotalGPUs++
		stats.TotalMemory += gpu.TotalMemory
		stats.UsedMemory += gpu.UsedMemory
		stats.AvailableMemory += gpu.AvailableMemory
		totalTemp += gpu.Temperature
		totalUtil += gpu.UtilizationGPU
		
		switch gpu.Status {
		case StatusIdle:
			stats.IdleGPUs++
		case StatusBusy:
			stats.BusyGPUs++
		case StatusReserved:
			stats.ReservedGPUs++
		case StatusOffline:
			stats.OfflineGPUs++
		}
	}
	
	if stats.TotalGPUs > 0 {
		stats.AvgTemperature = totalTemp / uint(stats.TotalGPUs)
		stats.AvgUtilization = totalUtil / uint(stats.TotalGPUs)
	}
	
	return stats
}