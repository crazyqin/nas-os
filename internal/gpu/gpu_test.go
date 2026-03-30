// Package gpu GPU模块测试
package gpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGPUDevice 测试GPU设备类型
func TestGPUDevice(t *testing.T) {
	device := &GPUDevice{
		ID:          "nvidia0",
		UUID:        "GPU-12345-abc",
		Name:        "NVIDIA GeForce RTX 3080",
		Model:       "RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryUsed:  2048,
		MemoryFree:  8192,
		CUDAcores:   8704,
		Temperature: 65,
		Status:      GPUStatusAvailable,
	}

	assert.Equal(t, "nvidia0", device.ID)
	assert.Equal(t, "nvidia", device.Vendor)
	assert.Equal(t, uint64(10240), device.MemoryTotal)
	assert.Equal(t, GPUStatusAvailable, device.Status)
}

// TestGPUStatus 测试GPU状态常量
func TestGPUStatus(t *testing.T) {
	assert.Equal(t, GPUStatus("available"), GPUStatusAvailable)
	assert.Equal(t, GPUStatus("allocated"), GPUStatusAllocated)
	assert.Equal(t, GPUStatus("busy"), GPUStatusBusy)
	assert.Equal(t, GPUStatus("error"), GPUStatusError)
	assert.Equal(t, GPUStatus("offline"), GPUStatusOffline)
}

// TestAllocationPriority 测试分配优先级
func TestAllocationPriority(t *testing.T) {
	assert.Equal(t, AllocationPriority("low"), PriorityLow)
	assert.Equal(t, AllocationPriority("normal"), PriorityNormal)
	assert.Equal(t, AllocationPriority("high"), PriorityHigh)
	assert.Equal(t, AllocationPriority("critical"), PriorityCritical)
}

// TestDefaultGPUConfig 测试默认配置
func TestDefaultGPUConfig(t *testing.T) {
	config := DefaultGPUConfig()

	assert.True(t, config.GPUEnabled)
	assert.Equal(t, "4g", config.DefaultMemoryLimit)
	assert.Equal(t, 1000, config.DefaultCUDACores)
	assert.Equal(t, "round-robin", config.SchedulerPolicy)
	assert.Equal(t, 10, config.MaxAllocations)
	assert.Equal(t, 30, config.HealthCheckInterval)
	assert.Equal(t, 5, config.MonitorInterval)
}

// TestGPUAllocation 测试GPU分配请求
func TestGPUAllocation(t *testing.T) {
	req := &GPUAllocation{
		ContainerID: "container-123",
		GPUID:       "nvidia0",
		MemoryLimit: 4096,
		CUDALimit:   1000,
		Priority:    PriorityHigh,
		Exclusive:   false,
	}

	assert.Equal(t, "container-123", req.ContainerID)
	assert.Equal(t, uint64(4096), req.MemoryLimit)
	assert.Equal(t, PriorityHigh, req.Priority)
	assert.False(t, req.Exclusive)
}

// TestGPUDeviceFilter 测试GPU设备过滤器
func TestGPUDeviceFilter(t *testing.T) {
	filter := &GPUDeviceFilter{
		Vendor:       "nvidia",
		MinMemory:    8192,
		MinCUDACores: 5000,
		Status:       GPUStatusAvailable,
		OnlyFree:     true,
	}

	assert.Equal(t, "nvidia", filter.Vendor)
	assert.Equal(t, uint64(8192), filter.MinMemory)
	assert.True(t, filter.OnlyFree)
}

// TestManager 测试GPU管理器创建
func TestManager(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false // 禁用GPU避免检测

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	// 测试基本属性
	assert.Equal(t, config, mgr.GetConfig())
	assert.False(t, mgr.IsGPUAvailable()) // GPU禁用时应该返回false

	// 关闭管理器
	err = mgr.Close()
	assert.NoError(t, err)
}

// TestManagerWithMockDevices 测试带模拟设备的管理器
func TestManagerWithMockDevices(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 手动添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		Name:        "NVIDIA GeForce RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryUsed:  0,
		MemoryFree:  10240,
		CUDAcores:   8704,
		Temperature: 50,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia0",
	}
	mgr.devices["nvidia1"] = &GPUDevice{
		ID:          "nvidia1",
		Name:        "NVIDIA GeForce RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryUsed:  0,
		MemoryFree:  10240,
		CUDAcores:   8704,
		Temperature: 55,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia1",
	}
	mgr.mu.Unlock()

	// 测试列表
	devices := mgr.ListGPUs(nil)
	assert.Len(t, devices, 2)

	// 测试过滤
	filter := &GPUDeviceFilter{
		Vendor: "nvidia",
	}
	filtered := mgr.ListGPUs(filter)
	assert.Len(t, filtered, 2)

	// 测试获取单个设备
	device, err := mgr.GetGPU("nvidia0")
	require.NoError(t, err)
	assert.Equal(t, "nvidia0", device.ID)

	// 测试不存在的设备
	_, err = mgr.GetGPU("nvidia99")
	assert.Error(t, err)

	mgr.Close()
}

// TestAllocateGPU 测试GPU分配
func TestAllocateGPU(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = true
	config.GPUDevices = []string{} // 禁用自动检测

	// 手动创建manager
	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		Name:        "NVIDIA GeForce RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryUsed:  0,
		MemoryFree:  10240,
		CUDAcores:   8704,
		Temperature: 50,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia0",
	}
	mgr.mu.Unlock()

	// 测试分配
	req := &GPUAllocation{
		ContainerID: "test-container",
		MemoryLimit: 4096,
		Priority:    PriorityNormal,
	}

	result, err := mgr.AllocateGPU(req)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.RequestID)
	assert.Equal(t, "nvidia0", result.GPUID)

	// 验证设备状态
	device, err := mgr.GetGPU("nvidia0")
	require.NoError(t, err)
	assert.True(t, device.Allocated)
	assert.Equal(t, "test-container", device.AllocatedTo)
	assert.Equal(t, GPUStatusAllocated, device.Status)

	// 测试释放
	releaseReq := &GPUReleaseRequest{
		RequestID: result.RequestID,
	}
	err = mgr.ReleaseGPU(releaseReq)
	assert.NoError(t, err)

	// 验证释放后状态
	device, err = mgr.GetGPU("nvidia0")
	require.NoError(t, err)
	assert.False(t, device.Allocated)
	assert.Equal(t, GPUStatusAvailable, device.Status)
}

// TestAllocateGPUExclusive 测试独占分配
func TestAllocateGPUExclusive(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = true
	config.GPUDevices = []string{} // 禁用自动检测
	config.SchedulerPolicy = "exclusive"

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		Name:        "NVIDIA GeForce RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryUsed:  0,
		MemoryFree:  10240,
		CUDAcores:   8704,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia0",
	}
	mgr.mu.Unlock()

	// 独占分配
	req := &GPUAllocation{
		ContainerID: "exclusive-container",
		Priority:    PriorityHigh,
		Exclusive:   true,
	}

	result, err := mgr.AllocateGPU(req)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 再次分配应该失败（独占模式下设备已被占用）
	req2 := &GPUAllocation{
		ContainerID: "another-container",
		Exclusive:   true,
	}

	_, err = mgr.AllocateGPU(req2)
	assert.Error(t, err) // 应该失败，没有可用GPU
}

// TestScheduler 测试调度器
func TestScheduler(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		Name:        "RTX 3080",
		Vendor:      "nvidia",
		MemoryTotal: 10240,
		MemoryFree:  10240,
		CUDAcores:   8704,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia0",
	}
	mgr.devices["nvidia1"] = &GPUDevice{
		ID:          "nvidia1",
		Name:        "RTX 3060",
		Vendor:      "nvidia",
		MemoryTotal: 8192,
		MemoryFree:  8192,
		CUDAcores:   3584,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia1",
	}
	mgr.mu.Unlock()

	scheduler := NewScheduler(mgr, "round-robin", logger)

	// 测试策略设置
	scheduler.SetPolicy("priority")
	assert.Equal(t, "priority", scheduler.GetPolicy())

	// 测试选择GPU
	req := &GPUAllocation{
		Priority: PriorityHigh,
	}

	device, err := scheduler.SelectGPU(req)
	require.NoError(t, err)
	assert.NotNil(t, device)
}

// TestSchedulerPolicies 测试不同调度策略
func TestSchedulerPolicies(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备（不同负载）
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		MemoryTotal: 10240,
		MemoryUsed:  2000,
		MemoryFree:  8240,
		CUDAcores:   8704,
		PowerLimit:  300,
		PowerUsage:  100,
		Temperature: 60,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia0",
	}
	mgr.devices["nvidia1"] = &GPUDevice{
		ID:          "nvidia1",
		MemoryTotal: 10240,
		MemoryUsed:  5000,
		MemoryFree:  5240,
		CUDAcores:   8704,
		PowerLimit:  300,
		PowerUsage:  200,
		Temperature: 70,
		Status:      GPUStatusAvailable,
		Allocated:   false,
		DevicePath:  "/dev/nvidia1",
	}
	mgr.mu.Unlock()

	// 测试least-loaded策略
	scheduler := NewScheduler(mgr, "least-loaded", logger)
	req := &GPUAllocation{Priority: PriorityNormal}
	device, err := scheduler.SelectGPU(req)
	require.NoError(t, err)
	// 应该选择负载较低的nvidia0
	assert.Equal(t, "nvidia0", device.ID)

	// 测试most-memory策略
	scheduler.SetPolicy("most-memory")
	device, err = scheduler.SelectGPU(req)
	require.NoError(t, err)
	// 应该选择可用显存更多的nvidia0
	assert.Equal(t, "nvidia0", device.ID)
}

// TestAllocationPolicyRoundRobin 测试轮询策略
func TestAllocationPolicyRoundRobin(t *testing.T) {
	policy := &AllocationPolicyRoundRobin{}

	devices := []*GPUDevice{
		{ID: "nvidia0", MemoryTotal: 10240, MemoryFree: 10240},
		{ID: "nvidia1", MemoryTotal: 10240, MemoryFree: 10240},
		{ID: "nvidia2", MemoryTotal: 10240, MemoryFree: 10240},
	}

	req := &GPUAllocation{Priority: PriorityNormal}

	device, err := policy.SelectGPU(devices, req)
	require.NoError(t, err)
	assert.NotNil(t, device)

	assert.Equal(t, "round-robin", policy.Name())
}

// TestAllocationPolicyLeastLoaded 测试最小负载策略
func TestAllocationPolicyLeastLoaded(t *testing.T) {
	policy := &AllocationPolicyLeastLoaded{}

	devices := []*GPUDevice{
		{ID: "nvidia0", MemoryTotal: 10240, MemoryUsed: 1000},
		{ID: "nvidia1", MemoryTotal: 10240, MemoryUsed: 5000},
		{ID: "nvidia2", MemoryTotal: 10240, MemoryUsed: 2000},
	}

	req := &GPUAllocation{Priority: PriorityNormal}

	device, err := policy.SelectGPU(devices, req)
	require.NoError(t, err)
	// 应该选择内存使用最低的nvidia0
	assert.Equal(t, "nvidia0", device.ID)

	assert.Equal(t, "least-loaded", policy.Name())
}

// TestAllocationPolicyPriority 测试优先级策略
func TestAllocationPolicyPriority(t *testing.T) {
	policy := &AllocationPolicyPriority{}

	devices := []*GPUDevice{
		{ID: "nvidia0", CUDAcores: 8704},   // 高性能
		{ID: "nvidia1", CUDAcores: 3584},   // 低性能
		{ID: "nvidia2", CUDAcores: 5000},   // 中性能
	}

	// 高优先级应该选择高性能GPU
	req := &GPUAllocation{Priority: PriorityHigh}
	device, err := policy.SelectGPU(devices, req)
	require.NoError(t, err)
	assert.Equal(t, "nvidia0", device.ID) // CUDA cores最高的

	// 低优先级应该选择低性能GPU
	req = &GPUAllocation{Priority: PriorityLow}
	device, err = policy.SelectGPU(devices, req)
	require.NoError(t, err)
	assert.Equal(t, "nvidia1", device.ID) // CUDA cores最低的

	assert.Equal(t, "priority", policy.Name())
}

// TestAllocationPolicyExclusive 测试独占策略
func TestAllocationPolicyExclusive(t *testing.T) {
	policy := &AllocationPolicyExclusive{}

	devices := []*GPUDevice{
		{ID: "nvidia0", CUDAcores: 8704, Allocated: false},
		{ID: "nvidia1", CUDAcores: 3584, Allocated: false},
	}

	req := &GPUAllocation{Priority: PriorityNormal, Exclusive: true}
	device, err := policy.SelectGPU(devices, req)
	require.NoError(t, err)
	assert.NotNil(t, device)

	// 非独占请求应该失败
	req = &GPUAllocation{Priority: PriorityNormal, Exclusive: false}
	_, err = policy.SelectGPU(devices, req)
	assert.Error(t, err)

	assert.Equal(t, "exclusive", policy.Name())
}

// TestGPUStats 测试GPU统计
func TestGPUStats(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		MemoryTotal: 10240,
		MemoryUsed:  2048,
		MemoryFree:  8192,
		Temperature: 65,
		PowerUsage:  150,
		Status:      GPUStatusAvailable,
		Allocated:   false,
	}
	mgr.devices["nvidia1"] = &GPUDevice{
		ID:          "nvidia1",
		MemoryTotal: 8192,
		MemoryUsed:  1024,
		MemoryFree:  7168,
		Temperature: 70,
		PowerUsage:  200,
		Status:      GPUStatusAllocated,
		Allocated:   true,
		AllocatedTo: "container-1",
	}
	mgr.allocations["req-1"] = &GPUAllocation{
		RequestID:   "req-1",
		ContainerID: "container-1",
		GPUID:       "nvidia1",
	}
	mgr.mu.Unlock()

	stats, err := mgr.GetGPUStats()
	require.NoError(t, err)

	assert.Equal(t, 2, stats.TotalGPUs)
	assert.Equal(t, 1, stats.AvailableGPUs)
	assert.Equal(t, 1, stats.AllocatedGPUs)
	assert.Equal(t, uint64(10240+8192), stats.TotalMemory)
	assert.Equal(t, uint64(2048+1024), stats.UsedMemory)
	assert.Equal(t, uint64(8192+7168), stats.FreeMemory)
	assert.Len(t, stats.Allocations, 1)
}

// TestContainerGPUConfig 测试容器GPU配置
func TestContainerGPUConfig(t *testing.T) {
	config := DefaultContainerGPUConfig()

	assert.False(t, config.GPUAll)
	assert.True(t, config.IncludeUVM)
	assert.True(t, config.IncludeCtl)
	assert.True(t, config.NvidiaRuntime)
	assert.False(t, config.NvidiaCDI)

	// 自定义配置
	customConfig := &ContainerGPUConfig{
		GPUAll:        true,
		MemoryLimit:   4096,
		ComputeLimit:  50,
		NvidiaRuntime: true,
	}

	assert.True(t, customConfig.GPUAll)
	assert.Equal(t, uint64(4096), customConfig.MemoryLimit)
	assert.Equal(t, uint64(50), customConfig.ComputeLimit)
}

// TestGenerateDockerGPUArgs 测试生成Docker GPU参数
func TestGenerateDockerGPUArgs(t *testing.T) {
	// 测试全部GPU
	config := &ContainerGPUConfig{
		GPUAll:        true,
		NvidiaRuntime: true,
	}

	args := GenerateDockerGPUArgs(config, nil)
	assert.Contains(t, args, "--gpus")
	assert.Contains(t, args, "all")

	// 测试指定GPU索引
	config = &ContainerGPUConfig{
		GPUIndices:    []int{0, 1},
		NvidiaRuntime: true,
	}

	args = GenerateDockerGPUArgs(config, nil)
	assert.Contains(t, args, "--gpus")
	// 应包含device=0,1
	hasDeviceArg := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--gpus" && args[i+1] == "device=0,1" {
			hasDeviceArg = true
			break
		}
	}
	assert.True(t, hasDeviceArg)

	// 测试显存限制
	config = &ContainerGPUConfig{
		GPUAll:        false,
		GPUIndices:    []int{0},
		MemoryLimit:   4096,
		NvidiaRuntime: true,
	}

	args = GenerateDockerGPUArgs(config, nil)
	// 应包含显存限制参数
	assert.Contains(t, args, "--gpus")
}

// TestGenerateDockerComposeGPUConfig 测试生成Docker Compose配置
func TestGenerateDockerComposeGPUConfig(t *testing.T) {
	config := &ContainerGPUConfig{
		GPUAll:        true,
		NvidiaRuntime: true,
	}

	composeConfig := GenerateDockerComposeGPUConfig(config, nil)
	assert.NotNil(t, composeConfig)

	// 检查deploy配置
	deploy, ok := composeConfig["deploy"]
	assert.True(t, ok)
	assert.NotNil(t, deploy)
}

// TestValidateGPUConfig 测试验证GPU配置
func TestValidateGPUConfig(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加模拟设备
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		UUID:        "GPU-12345",
		MemoryTotal: 10240,
		Status:      GPUStatusAvailable,
	}
	mgr.mu.Unlock()

	// 有效配置
	validConfig := &ContainerGPUConfig{
		GPUIndices:  []int{0},
		MemoryLimit: 4096,
	}

	err = ValidateGPUConfig(validConfig, mgr)
	assert.NoError(t, err)

	// 无效的GPU索引
	invalidConfig := &ContainerGPUConfig{
		GPUIndices: []int{99},
	}

	err = ValidateGPUConfig(invalidConfig, mgr)
	assert.Error(t, err)

	// 无效的GPU UUID
	invalidConfig = &ContainerGPUConfig{
		GPUUUIDs: []string{"invalid-uuid"},
	}

	err = ValidateGPUConfig(invalidConfig, mgr)
	assert.Error(t, err)

	// 超出显存限制
	invalidConfig = &ContainerGPUConfig{
		GPUIndices:  []int{0},
		MemoryLimit: 99999, // 超过设备显存
	}

	err = ValidateGPUConfig(invalidConfig, mgr)
	assert.Error(t, err)
}

// TestMergeContainerGPUConfig 测试合并GPU配置
func TestMergeContainerGPUConfig(t *testing.T) {
	base := &ContainerGPUConfig{
		GPUIndices:    []int{0},
		MemoryLimit:   2048,
		NvidiaRuntime: true,
	}

	override := &ContainerGPUConfig{
		GPUIndices:    []int{1},
		MemoryLimit:   4096,
		IncludeUVM:    true,
	}

	result := MergeContainerGPUConfig(base, override)

	// GPU索引应该合并
	assert.ElementsMatch(t, []int{0, 1}, result.GPUIndices)
	// MemoryLimit应该使用override的值
	assert.Equal(t, uint64(4096), result.MemoryLimit)
	// IncludeUVM应该合并
	assert.True(t, result.IncludeUVM)
}

// TestParseMemoryLimit 测试解析内存限制
func TestParseMemoryLimit(t *testing.T) {
	assert.Equal(t, uint64(1024), parseMemoryLimit("1g"))
	assert.Equal(t, uint64(512), parseMemoryLimit("512m"))
	assert.Equal(t, uint64(1024*1024), parseMemoryLimit("1t"))
	assert.Equal(t, uint64(100), parseMemoryLimit("100"))
	assert.Equal(t, uint64(0), parseMemoryLimit(""))
	assert.Equal(t, uint64(0), parseMemoryLimit("1kb")) // kb转为MB会除以1024，1kb = 0MB
}

// TestMonitor 测试监控器
func TestMonitor(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	monitor := NewMonitor(mgr, 5, logger)
	assert.NotNil(t, monitor)
	assert.True(t, monitor.IsRunning())

	// 设置间隔
	monitor.SetInterval(10)
	assert.Equal(t, 10, monitor.interval)

	// 停止监控
	monitor.Stop()
	assert.False(t, monitor.IsRunning())
}

// TestGetGPUContainerRuntime 测试获取容器运行时
func TestGetGPUContainerRuntime(t *testing.T) {
	// 无GPU配置
	assert.Equal(t, "runc", GetGPUContainerRuntime(nil))

	// NVIDIA运行时
	config := &ContainerGPUConfig{NvidiaRuntime: true}
	assert.Equal(t, "nvidia", GetGPUContainerRuntime(config))

	// CDI模式
	config = &ContainerGPUConfig{NvidiaRuntime: true, NvidiaCDI: true}
	assert.Equal(t, "nvidia-cdi", GetGPUContainerRuntime(config))
}

// TestIsGPUConfigured 测试检查GPU配置
func TestIsGPUConfigured(t *testing.T) {
	// 无配置
	assert.False(t, IsGPUConfigured(nil))

	// 空配置
	config := DefaultContainerGPUConfig()
	assert.False(t, IsGPUConfigured(config))

	// 有GPU配置
	config.GPUAll = true
	assert.True(t, IsGPUConfigured(config))

	config = &ContainerGPUConfig{GPUIndices: []int{0}}
	assert.True(t, IsGPUConfigured(config))
}

// TestCalculateLoadScore 测试负载评分计算
func TestCalculateLoadScore(t *testing.T) {
	logger := zap.NewNop()
	scheduler := NewScheduler(nil, "least-loaded", logger)

	// 低负载设备
	lowLoadGPU := &GPUDevice{
		MemoryTotal: 10240,
		MemoryUsed:  1000,
		PowerLimit:  300,
		PowerUsage:  50,
		Temperature: 50,
	}

	lowScore := scheduler.calculateLoadScore(lowLoadGPU)

	// 高负载设备
	highLoadGPU := &GPUDevice{
		MemoryTotal: 10240,
		MemoryUsed:  8000,
		PowerLimit:  300,
		PowerUsage:  280,
		Temperature: 85,
	}

	highScore := scheduler.calculateLoadScore(highLoadGPU)

	// 低负载评分应该更低
	assert.Less(t, lowScore, highScore)
}

// TestGenerateRequestID 测试请求ID生成
func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	assert.NotEmpty(t, id1)
	assert.Contains(t, id1, "gpu-req-")
	assert.NotEqual(t, id1, id2) // 应该唯一
}

// TestReleaseGPUByContainer 测试通过容器ID释放GPU
func TestReleaseGPUByContainer(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加设备和分配
	mgr.mu.Lock()
	mgr.devices["nvidia0"] = &GPUDevice{
		ID:          "nvidia0",
		MemoryTotal: 10240,
		Status:      GPUStatusAllocated,
		Allocated:   true,
		AllocatedTo: "container-1",
	}
	mgr.devices["nvidia1"] = &GPUDevice{
		ID:          "nvidia1",
		MemoryTotal: 10240,
		Status:      GPUStatusAllocated,
		Allocated:   true,
		AllocatedTo: "container-1",
	}
	mgr.allocations["req-1"] = &GPUAllocation{
		RequestID:   "req-1",
		ContainerID: "container-1",
		GPUID:       "nvidia0",
	}
	mgr.allocations["req-2"] = &GPUAllocation{
		RequestID:   "req-2",
		ContainerID: "container-1",
		GPUID:       "nvidia1",
	}
	mgr.mu.Unlock()

	// 通过容器ID释放所有GPU
	releaseReq := &GPUReleaseRequest{
		ContainerID: "container-1",
	}

	err = mgr.ReleaseGPU(releaseReq)
	assert.NoError(t, err)

	// 验证所有设备已释放
	assert.Len(t, mgr.allocations, 0)
	for _, device := range mgr.devices {
		assert.False(t, device.Allocated)
	}
}

// TestGetContainerGPUAllocations 测试获取容器GPU分配
func TestGetContainerGPUAllocations(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 添加分配记录
	mgr.mu.Lock()
	mgr.allocations["req-1"] = &GPUAllocation{
		RequestID:   "req-1",
		ContainerID: "container-1",
		GPUID:       "nvidia0",
	}
	mgr.allocations["req-2"] = &GPUAllocation{
		RequestID:   "req-2",
		ContainerID: "container-2",
		GPUID:       "nvidia1",
	}
	mgr.mu.Unlock()

	// 获取特定容器的分配
	allocations := mgr.GetContainerGPUAllocations("container-1")
	assert.Len(t, allocations, 1)
	assert.Equal(t, "nvidia0", allocations[0].GPUID)

	// 获取另一个容器
	allocations = mgr.GetContainerGPUAllocations("container-2")
	assert.Len(t, allocations, 1)

	// 获取不存在容器的分配
	allocations = mgr.GetContainerGPUAllocations("container-99")
	assert.Len(t, allocations, 0)
}

// TestConfigImportExport 测试配置导入导出
func TestConfigImportExport(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultGPUConfig()
	config.GPUEnabled = false

	mgr, err := NewManager(config, logger)
	require.NoError(t, err)

	// 导出配置
	data, err := mgr.ExportConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// 导入配置
	newConfig := DefaultGPUConfig()
	newConfig.SchedulerPolicy = "priority"
	data, err = mgr.ExportConfig()
	require.NoError(t, err)

	err = mgr.ImportConfig(data)
	assert.NoError(t, err)
}