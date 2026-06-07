// Package gpu GPU透传管理器
package gpu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PassthroughType 透传类型
type PassthroughType string

const (
	// PassthroughDocker Docker容器透传
	PassthroughDocker PassthroughType = "docker"
	// PassthroughLXC LXC容器透传
	PassthroughLXC PassthroughType = "lxc"
	// PassthroughVM 虚拟机透传
	PassthroughVM PassthroughType = "vm"
	// PassthroughKVM KVM虚拟机透传
	PassthroughKVM PassthroughType = "kvm"
)

// PassthroughMode 透传模式
type PassthroughMode string

const (
	// PassthroughModeFull 完全透传
	PassthroughModeFull PassthroughMode = "full"
	// PassthroughModeShared 共享模式
	PassthroughModeShared PassthroughMode = "shared"
	// PassthroughModeVFIO VFIO透传
	PassthroughModeVFIO PassthroughMode = "vfio"
	// PassthroughModeMIG MIG实例
	PassthroughModeMIG PassthroughMode = "mig"
	// PassthroughModeVGPU vGPU虚拟化
	PassthroughModeVGPU PassthroughMode = "vgpu"
)

// PassthroughConfig 透传配置
type PassthroughConfig struct {
	// 目标配置
	Type       PassthroughType `json:"type"`       // 透传类型
	TargetID   string          `json:"targetId"`   // 目标ID (容器名或VM名)
	TargetName string          `json:"targetName"` // 目标名称

	// GPU配置
	GPUID      string `json:"gpuId"`      // GPU设备ID (如 nvidia0)
	GPUIndex   int    `json:"gpuIndex"`   // GPU索引
	GPUUUID    string `json:"gpuUuid"`    // GPU UUID
	PCIAddress string `json:"pciAddress"` // PCI地址

	// 模式和权限
	Mode      PassthroughMode `json:"mode"`      // 透传模式
	Exclusive bool            `json:"exclusive"` // 独占模式
	EnableMIG bool            `json:"enableMig"` // 启用MIG
	MIGGI     int             `json:"migGi"`     // MIG GPU实例
	MIGCI     int             `json:"migCi"`     // MIG计算实例

	// 资源限制
	MemoryLimitMB   uint64 `json:"memoryLimitMb"`   // 显存限制(MB)
	ComputeLimitPct uint64 `json:"computeLimitPct"` // 计算限制(%)
	MaxProcesses    int    `json:"maxProcesses"`    // 最大进程数

	// 功能开关
	EnableCompute  bool `json:"enableCompute"`  // 启用计算
	EnableGraphics bool `json:"enableGraphics"` // 启用图形
	EnableVideo    bool `json:"enableVideo"`    // 启用视频编码解码
	EnableDisplay  bool `json:"enableDisplay"`  // 启用显示

	// MPS配置
	EnableMPS     bool `json:"enableMps"`     // 启用MPS
	MPSThreadPool int  `json:"mpsThreadPool"` // MPS线程池大小

	// 挂载配置
	MountDriver    bool   `json:"mountDriver"`    // 挂载驱动
	MountLibraries bool   `json:"mountLibraries"` // 挂载库文件
	DriverVersion  string `json:"driverVersion"`  // 驱动版本
	CUDAVersion    string `json:"cudaVersion"`    // CUDA版本

	// 时间戳
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PassthroughManager GPU透传管理器
type PassthroughManager struct {
	manager      *Manager
	logger       *zap.Logger
	passthroughs map[string]*PassthroughConfig // key: requestID
	mu           sync.RWMutex

	// 各类型处理器
	dockerHandler PassthroughHandler
	lxcHandler    PassthroughHandler
	vmHandler     PassthroughHandler
}

// PassthroughHandler 透传处理器接口
type PassthroughHandler interface {
	// Attach GPU附加
	Attach(ctx context.Context, config *PassthroughConfig) error
	// Detach GPU分离
	Detach(ctx context.Context, config *PassthroughConfig) error
	// Validate 验证配置
	Validate(config *PassthroughConfig) error
	// GetStatus 获取状态
	GetStatus(ctx context.Context, targetID string) ([]PassthroughConfig, error)
	// GetType 获取处理器类型
	GetType() PassthroughType
}

// NewPassthroughManager 创建透传管理器
func NewPassthroughManager(manager *Manager, logger *zap.Logger) *PassthroughManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	ptm := &PassthroughManager{
		manager:      manager,
		logger:       logger,
		passthroughs: make(map[string]*PassthroughConfig),
	}

	// 注册处理器
	ptm.dockerHandler = NewDockerPassthroughHandler(manager, logger)
	ptm.lxcHandler = NewLXCPassthroughHandler(manager, logger)
	ptm.vmHandler = NewVMPassthroughHandler(manager, logger)

	return ptm
}

// AttachGPU GPU附加到目标
func (ptm *PassthroughManager) AttachGPU(ctx context.Context, config *PassthroughConfig) (string, error) {
	// 验证配置
	handler := ptm.getHandler(config.Type)
	if handler == nil {
		return "", fmt.Errorf("不支持的透传类型: %s", config.Type)
	}

	if err := handler.Validate(config); err != nil {
		return "", fmt.Errorf("配置验证失败: %w", err)
	}

	// 检查GPU可用性
	if err := ptm.checkGPUAvailability(config); err != nil {
		return "", fmt.Errorf("GPU可用性检查失败: %w", err)
	}

	// 分配GPU
	allocReq := &GPUAllocation{
		ContainerID: config.TargetID,
		GPUID:       config.GPUID,
		MemoryLimit: config.MemoryLimitMB,
		Exclusive:   config.Exclusive,
		Priority:    PriorityNormal,
	}

	result, err := ptm.manager.AllocateGPU(allocReq)
	if err != nil {
		return "", fmt.Errorf("GPU分配失败: %w", err)
	}

	config.GPUID = result.GPUID
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	// 执行透传
	if err := handler.Attach(ctx, config); err != nil {
		// 回滚分配
		ptm.manager.ReleaseGPU(&GPUReleaseRequest{
			RequestID:   result.RequestID,
			ContainerID: config.TargetID,
		})
		return "", fmt.Errorf("GPU透传失败: %w", err)
	}

	// 保存配置
	ptm.mu.Lock()
	ptm.passthroughs[result.RequestID] = config
	ptm.mu.Unlock()

	ptm.logger.Info("GPU透传完成",
		zap.String("requestId", result.RequestID),
		zap.String("type", string(config.Type)),
		zap.String("targetId", config.TargetID),
		zap.String("gpuId", config.GPUID))

	return result.RequestID, nil
}

// DetachGPU GPU分离
func (ptm *PassthroughManager) DetachGPU(ctx context.Context, requestID string) error {
	ptm.mu.Lock()
	config, exists := ptm.passthroughs[requestID]
	if !exists {
		ptm.mu.Unlock()
		return fmt.Errorf("透传配置不存在: %s", requestID)
	}
	ptm.mu.Unlock()

	// 获取处理器
	handler := ptm.getHandler(config.Type)
	if handler == nil {
		return fmt.Errorf("不支持的透传类型: %s", config.Type)
	}

	// 执行分离
	if err := handler.Detach(ctx, config); err != nil {
		ptm.logger.Error("GPU分离失败", zap.Error(err))
		// 继续释放GPU
	}

	// 释放GPU
	if err := ptm.manager.ReleaseGPU(&GPUReleaseRequest{
		RequestID:   requestID,
		ContainerID: config.TargetID,
	}); err != nil {
		ptm.logger.Warn("GPU释放失败", zap.Error(err))
	}

	// 删除配置
	ptm.mu.Lock()
	delete(ptm.passthroughs, requestID)
	ptm.mu.Unlock()

	ptm.logger.Info("GPU分离完成",
		zap.String("requestId", requestID),
		zap.String("targetId", config.TargetID))

	return nil
}

// GetPassthroughStatus 获取透传状态
func (ptm *PassthroughManager) GetPassthroughStatus(ctx context.Context, targetID string, targetType PassthroughType) ([]PassthroughConfig, error) {
	handler := ptm.getHandler(targetType)
	if handler == nil {
		return nil, fmt.Errorf("不支持的透传类型: %s", targetType)
	}

	return handler.GetStatus(ctx, targetID)
}

// ListPassthroughs 列出所有透传
func (ptm *PassthroughManager) ListPassthroughs(filter *PassthroughFilter) []*PassthroughConfig {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	var result []*PassthroughConfig
	for _, config := range ptm.passthroughs {
		if filter != nil && !ptm.matchFilter(config, filter) {
			continue
		}
		result = append(result, config)
	}

	return result
}

// PassthroughFilter 透传过滤器
type PassthroughFilter struct {
	Type     PassthroughType
	GPUID    string
	TargetID string
	Mode     PassthroughMode
}

// matchFilter 匹配过滤器
func (ptm *PassthroughManager) matchFilter(config *PassthroughConfig, filter *PassthroughFilter) bool {
	if filter.Type != "" && config.Type != filter.Type {
		return false
	}
	if filter.GPUID != "" && config.GPUID != filter.GPUID {
		return false
	}
	if filter.TargetID != "" && config.TargetID != filter.TargetID {
		return false
	}
	if filter.Mode != "" && config.Mode != filter.Mode {
		return false
	}
	return true
}

// getHandler 获取处理器
func (ptm *PassthroughManager) getHandler(type_ PassthroughType) PassthroughHandler {
	switch type_ {
	case PassthroughDocker:
		return ptm.dockerHandler
	case PassthroughLXC:
		return ptm.lxcHandler
	case PassthroughVM, PassthroughKVM:
		return ptm.vmHandler
	default:
		return nil
	}
}

// checkGPUAvailability 检查GPU可用性
func (ptm *PassthroughManager) checkGPUAvailability(config *PassthroughConfig) error {
	if config.GPUID == "" && config.GPUUUID != "" {
		// 通过UUID查找GPU
		devices := ptm.manager.ListGPUs(nil)
		for _, device := range devices {
			if device.UUID == config.GPUUUID {
				config.GPUID = device.ID
				break
			}
		}
	}

	if config.GPUID == "" {
		return fmt.Errorf("无法确定GPU设备")
	}

	device, err := ptm.manager.GetGPU(config.GPUID)
	if err != nil {
		return fmt.Errorf("获取GPU信息失败: %w", err)
	}

	if device.Allocated && !config.Exclusive {
		return fmt.Errorf("GPU %s 已被分配", config.GPUID)
	}

	// 检查显存限制
	if config.MemoryLimitMB > 0 && config.MemoryLimitMB > device.MemoryTotal {
		return fmt.Errorf("显存限制 %d MB 超过GPU总显存 %d MB", config.MemoryLimitMB, device.MemoryTotal)
	}

	return nil
}

// DockerPassthroughHandler Docker容器透传处理器
type DockerPassthroughHandler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewDockerPassthroughHandler 创建Docker透传处理器
func NewDockerPassthroughHandler(manager *Manager, logger *zap.Logger) *DockerPassthroughHandler {
	return &DockerPassthroughHandler{
		manager: manager,
		logger:  logger,
	}
}

// GetType 获取处理器类型
func (h *DockerPassthroughHandler) GetType() PassthroughType {
	return PassthroughDocker
}

// Validate 验证配置
func (h *DockerPassthroughHandler) Validate(config *PassthroughConfig) error {
	if config.TargetID == "" {
		return fmt.Errorf("容器ID不能为空")
	}
	if config.GPUID == "" && config.GPUIndex < 0 && config.GPUUUID == "" {
		return fmt.Errorf("必须指定GPU设备")
	}
	return nil
}

// Attach GPU附加
func (h *DockerPassthroughHandler) Attach(ctx context.Context, config *PassthroughConfig) error {
	// Docker GPU附加通过ContainerGPUConfig处理
	// 实际实现需要调用Docker API或CLI
	// 这里主要配置环境变量和设备挂载
	h.logger.Info("Docker GPU附加",
		zap.String("containerId", config.TargetID),
		zap.String("gpuId", config.GPUID))

	// 构建GPU配置
	gpuConfig := DefaultContainerGPUConfig()
	gpuConfig.NvidiaRuntime = true

	if config.GPUIndex >= 0 {
		gpuConfig.GPUIndices = []int{config.GPUIndex}
	}
	if config.GPUUUID != "" {
		gpuConfig.GPUUUIDs = []string{config.GPUUUID}
	}

	if config.MemoryLimitMB > 0 {
		gpuConfig.MemoryLimit = config.MemoryLimitMB
	}

	if config.EnableCompute {
		gpuConfig.EnvVars["NVIDIA_DRIVER_CAPABILITIES"] = "compute,utility"
	}
	if config.EnableGraphics {
		gpuConfig.EnvVars["NVIDIA_DRIVER_CAPABILITIES"] += ",graphics"
	}
	if config.EnableVideo {
		gpuConfig.EnvVars["NVIDIA_DRIVER_CAPABILITIES"] += ",video"
	}

	h.logger.Debug("GPU配置", zap.Any("config", gpuConfig))

	return nil
}

// Detach GPU分离
func (h *DockerPassthroughHandler) Detach(ctx context.Context, config *PassthroughConfig) error {
	h.logger.Info("Docker GPU分离",
		zap.String("containerId", config.TargetID),
		zap.String("gpuId", config.GPUID))

	return nil
}

// GetStatus 获取状态
func (h *DockerPassthroughHandler) GetStatus(ctx context.Context, targetID string) ([]PassthroughConfig, error) {
	allocations := h.manager.GetContainerGPUAllocations(targetID)
	var configs []PassthroughConfig
	for _, alloc := range allocations {
		configs = append(configs, PassthroughConfig{
			Type:     PassthroughDocker,
			TargetID: targetID,
			GPUID:    alloc.GPUID,
			Mode:     PassthroughModeShared,
		})
	}
	return configs, nil
}

// LXCPassthroughHandler LXC容器透传处理器
type LXCPassthroughHandler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewLXCPassthroughHandler 创建LXC透传处理器
func NewLXCPassthroughHandler(manager *Manager, logger *zap.Logger) *LXCPassthroughHandler {
	return &LXCPassthroughHandler{
		manager: manager,
		logger:  logger,
	}
}

// GetType 获取处理器类型
func (h *LXCPassthroughHandler) GetType() PassthroughType {
	return PassthroughLXC
}

// Validate 验证配置
func (h *LXCPassthroughHandler) Validate(config *PassthroughConfig) error {
	if config.TargetID == "" && config.TargetName == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	if config.GPUID == "" && config.GPUIndex < 0 && config.GPUUUID == "" && config.PCIAddress == "" {
		return fmt.Errorf("必须指定GPU设备")
	}
	return nil
}

// Attach GPU附加
func (h *LXCPassthroughHandler) Attach(ctx context.Context, config *PassthroughConfig) error {
	h.logger.Info("LXC GPU附加",
		zap.String("containerName", config.TargetName),
		zap.String("gpuId", config.GPUID))

	// LXC GPU附加通过lxc config device add
	// 实际实现需要调用Incus/LXD CLI或API

	return nil
}

// Detach GPU分离
func (h *LXCPassthroughHandler) Detach(ctx context.Context, config *PassthroughConfig) error {
	h.logger.Info("LXC GPU分离",
		zap.String("containerName", config.TargetName),
		zap.String("gpuId", config.GPUID))

	return nil
}

// GetStatus 获取状态
func (h *LXCPassthroughHandler) GetStatus(ctx context.Context, targetID string) ([]PassthroughConfig, error) {
	allocations := h.manager.GetContainerGPUAllocations(targetID)
	var configs []PassthroughConfig
	for _, alloc := range allocations {
		configs = append(configs, PassthroughConfig{
			Type:     PassthroughLXC,
			TargetID: targetID,
			GPUID:    alloc.GPUID,
			Mode:     PassthroughModeShared,
		})
	}
	return configs, nil
}

// VMPassthroughHandler 虚拟机透传处理器
type VMPassthroughHandler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewVMPassthroughHandler 创建VM透传处理器
func NewVMPassthroughHandler(manager *Manager, logger *zap.Logger) *VMPassthroughHandler {
	return &VMPassthroughHandler{
		manager: manager,
		logger:  logger,
	}
}

// GetType 获取处理器类型
func (h *VMPassthroughHandler) GetType() PassthroughType {
	return PassthroughVM
}

// Validate 验证配置
func (h *VMPassthroughHandler) Validate(config *PassthroughConfig) error {
	if config.TargetID == "" && config.TargetName == "" {
		return fmt.Errorf("虚拟机名称不能为空")
	}
	if config.PCIAddress == "" && config.GPUID == "" {
		return fmt.Errorf("必须指定PCI地址或GPU设备")
	}

	// VM透传需要VFIO支持
	if config.Mode == PassthroughModeVFIO || config.Mode == PassthroughModeFull {
		// 验证IOMMU是否启用
		// 实际实现需要检查内核参数
	}

	return nil
}

// Attach GPU附加
func (h *VMPassthroughHandler) Attach(ctx context.Context, config *PassthroughConfig) error {
	h.logger.Info("VM GPU附加",
		zap.String("vmName", config.TargetName),
		zap.String("pciAddress", config.PCIAddress))

	// VM GPU透传需要:
	// 1. 确保设备绑定到vfio-pci驱动
	// 2. 更新VM XML配置添加PCI设备
	// 3. 对于MIG模式，需要先创建MIG实例

	if config.EnableMIG {
		h.logger.Info("MIG模式启用",
			zap.Int("gi", config.MIGGI),
			zap.Int("ci", config.MIGCI))
	}

	return nil
}

// Detach GPU分离
func (h *VMPassthroughHandler) Detach(ctx context.Context, config *PassthroughConfig) error {
	h.logger.Info("VM GPU分离",
		zap.String("vmName", config.TargetName),
		zap.String("pciAddress", config.PCIAddress))

	// VM GPU分离需要:
	// 1. 更新VM XML移除PCI设备
	// 2. 将设备恢复到原始驱动

	return nil
}

// GetStatus 获取状态
func (h *VMPassthroughHandler) GetStatus(ctx context.Context, targetID string) ([]PassthroughConfig, error) {
	allocations := h.manager.GetContainerGPUAllocations(targetID)
	var configs []PassthroughConfig
	for _, alloc := range allocations {
		mode := PassthroughModeShared
		configs = append(configs, PassthroughConfig{
			Type:     PassthroughVM,
			TargetID: targetID,
			GPUID:    alloc.GPUID,
			Mode:     mode,
		})
	}
	return configs, nil
}

// GPUResourceManager GPU资源管理器
type GPUResourceManager struct {
	manager        *Manager
	passthroughMgr *PassthroughManager
	logger         *zap.Logger
	resourceQuotas map[string]*GPUResourceQuota // key: targetID
	mu             sync.RWMutex
}

// GPUResourceQuota GPU资源配额
type GPUResourceQuota struct {
	TargetID      string             `json:"targetId"`
	MaxGPUs       int                `json:"maxGpus"`       // 最大GPU数
	MaxMemoryMB   uint64             `json:"maxMemoryMb"`   // 最大显存(MB)
	MaxComputePct uint64             `json:"maxComputePct"` // 最大计算百分比
	ReservedGPUs  []string           `json:"reservedGpus"`  // 保留GPU列表
	Priority      AllocationPriority `json:"priority"`      // 优先级
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// NewGPUResourceManager 创建GPU资源管理器
func NewGPUResourceManager(manager *Manager, ptm *PassthroughManager, logger *zap.Logger) *GPUResourceManager {
	return &GPUResourceManager{
		manager:        manager,
		passthroughMgr: ptm,
		logger:         logger,
		resourceQuotas: make(map[string]*GPUResourceQuota),
	}
}

// SetQuota 设置资源配额
func (grm *GPUResourceManager) SetQuota(targetID string, quota *GPUResourceQuota) error {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	quota.TargetID = targetID
	quota.CreatedAt = time.Now()
	quota.UpdatedAt = time.Now()

	grm.resourceQuotas[targetID] = quota

	grm.logger.Info("GPU配额设置",
		zap.String("targetId", targetID),
		zap.Int("maxGpus", quota.MaxGPUs),
		zap.Uint64("maxMemoryMb", quota.MaxMemoryMB))

	return nil
}

// GetQuota 获取资源配额
func (grm *GPUResourceManager) GetQuota(targetID string) (*GPUResourceQuota, error) {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	quota, exists := grm.resourceQuotas[targetID]
	if !exists {
		return nil, fmt.Errorf("配额不存在: %s", targetID)
	}

	return quota, nil
}

// CheckQuota 检查配额
func (grm *GPUResourceManager) CheckQuota(targetID string, request *PassthroughConfig) error {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	quota, exists := grm.resourceQuotas[targetID]
	if !exists {
		return nil // 无配额限制
	}

	// 检查GPU数量限制
	currentAllocs := grm.manager.GetContainerGPUAllocations(targetID)
	if quota.MaxGPUs > 0 && len(currentAllocs) >= quota.MaxGPUs {
		return fmt.Errorf("已达最大GPU数量限制: %d", quota.MaxGPUs)
	}

	// 检查显存限制
	if quota.MaxMemoryMB > 0 && request.MemoryLimitMB > quota.MaxMemoryMB {
		return fmt.Errorf("请求显存 %d MB 超过配额限制 %d MB", request.MemoryLimitMB, quota.MaxMemoryMB)
	}

	// 检查计算限制
	if quota.MaxComputePct > 0 && request.ComputeLimitPct > quota.MaxComputePct {
		return fmt.Errorf("请求计算限制 %d%% 超过配额限制 %d%%", request.ComputeLimitPct, quota.MaxComputePct)
	}

	return nil
}

// DeleteQuota 删除配额
func (grm *GPUResourceManager) DeleteQuota(targetID string) error {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	delete(grm.resourceQuotas, targetID)

	grm.logger.Info("GPU配额删除", zap.String("targetId", targetID))

	return nil
}

// ListQuotas 列出所有配额
func (grm *GPUResourceManager) ListQuotas() []*GPUResourceQuota {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	var quotas []*GPUResourceQuota
	for _, quota := range grm.resourceQuotas {
		quotas = append(quotas, quota)
	}

	return quotas
}
