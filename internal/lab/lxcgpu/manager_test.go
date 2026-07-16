package lxcgpu

import (
	"testing"
	"time"
)

// ========== 类型与验证测试 ==========

func TestGPUQuotaValidate(t *testing.T) {
	tests := []struct {
		name    string
		quota   GPUQuota
		wantErr bool
	}{
		{
			name:    "有效配额",
			quota:   GPUQuota{MemoryLimitMB: 4096, SMPercent: 50, Priority: 10},
			wantErr: false,
		},
		{
			name:    "零值配额（默认不限制）",
			quota:   GPUQuota{},
			wantErr: false,
		},
		{
			name:    "SM百分比超出范围",
			quota:   GPUQuota{SMPercent: 150},
			wantErr: true,
		},
		{
			name:    "SM百分比为负数",
			quota:   GPUQuota{SMPercent: -10},
			wantErr: true,
		},
		{
			name:    "优先级超出范围",
			quota:   GPUQuota{Priority: 200},
			wantErr: true,
		},
		{
			name:    "显存保证超过限制",
			quota:   GPUQuota{MemoryLimitMB: 1024, MemoryGuarantee: 2048},
			wantErr: true,
		},
		{
			name:    "显存保证等于限制（有效）",
			quota:   GPUQuota{MemoryLimitMB: 2048, MemoryGuarantee: 2048},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.quota.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GPUQuota.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultLXCConfig(t *testing.T) {
	cfg := DefaultLXCConfig()
	if cfg == nil {
		t.Fatal("DefaultLXCConfig() 返回nil")
	}
	if cfg.ConfigPath == "" {
		t.Error("默认ConfigPath不应为空")
	}
	if cfg.DeviceCGroup == "" {
		t.Error("默认DeviceCGroup不应为空")
	}
	if cfg.HotplugSocket == "" {
		t.Error("默认HotplugSocket不应为空")
	}
}

// ========== DeviceManager 测试 ==========

func TestNewDeviceManager(t *testing.T) {
	dm := NewDeviceManager(nil)
	if dm == nil {
		t.Fatal("NewDeviceManager(nil) 返回nil")
	}
	if dm.devices == nil {
		t.Error("devices map未初始化")
	}
	if dm.stopCh == nil {
		t.Error("stopCh未初始化")
	}
}

func TestDeviceManagerGetDeviceNotFound(t *testing.T) {
	dm := NewDeviceManager(nil)
	_, err := dm.GetDevice("0000:01:00.0")
	if err == nil {
		t.Error("期望返回错误，但未返回")
	}
}

func TestDeviceManagerListDevices(t *testing.T) {
	dm := NewDeviceManager(nil)
	devices := dm.ListDevices()
	if devices == nil {
		t.Error("ListDevices() 返回nil")
	}
	if len(devices) != 0 {
		t.Errorf("期望0个设备，得到%d个", len(devices))
	}
}

func TestDeviceManagerListAvailableDevices(t *testing.T) {
	dm := NewDeviceManager(nil)
	devices := dm.ListAvailableDevices()
	if devices == nil {
		t.Error("ListAvailableDevices() 返回nil")
	}
}

func TestDeviceManagerGetContainerGPUs(t *testing.T) {
	dm := NewDeviceManager(nil)
	gpus := dm.GetContainerGPUs("test-container")
	if gpus == nil {
		t.Error("GetContainerGPUs() 返回nil")
	}
	if len(gpus) != 0 {
		t.Errorf("期望0个GPU，得到%d个", len(gpus))
	}
}

func TestDeviceManagerIsDeviceAvailable(t *testing.T) {
	dm := NewDeviceManager(nil)
	available, reason := dm.IsDeviceAvailable("0000:01:00.0", ShareModeExclusive)
	if available {
		t.Error("不存在的设备不应返回可用")
	}
	if reason == "" {
		t.Error("应返回不可用原因")
	}
}

func TestDeviceManagerUpdateDeviceAssignment(t *testing.T) {
	dm := NewDeviceManager(nil)
	assign := &LXCGPUAssignment{
		ID:          "test-assign-1",
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		State:       AssignmentStateActive,
	}
	err := dm.UpdateDeviceAssignment("0000:01:00.0", assign)
	if err == nil {
		t.Error("不存在的设备应返回错误")
	}
}

func TestDeviceManagerRemoveDeviceAssignment(t *testing.T) {
	dm := NewDeviceManager(nil)
	err := dm.RemoveDeviceAssignment("0000:01:00.0", "test-assign-1")
	if err == nil {
		t.Error("不存在的设备应返回错误")
	}
}

func TestDeviceManagerGetDeviceForContainer(t *testing.T) {
	dm := NewDeviceManager(nil)
	devices := dm.GetDeviceForContainer("test-container")
	if devices == nil {
		t.Error("GetDeviceForContainer() 返回nil")
	}
	if len(devices) != 0 {
		t.Errorf("期望0个设备，得到%d个", len(devices))
	}
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager(nil) 返回nil")
	}
	if mgr.config == nil {
		t.Error("config未初始化")
	}
	if mgr.devices == nil {
		t.Error("devices未初始化")
	}
	if mgr.assignments == nil {
		t.Error("assignments未初始化")
	}
	if mgr.containerAssigns == nil {
		t.Error("containerAssigns未初始化")
	}
}

func TestManagerGetDeviceManager(t *testing.T) {
	mgr := NewManager(nil)
	dm := mgr.GetDeviceManager()
	if dm == nil {
		t.Error("GetDeviceManager() 返回nil")
	}
}

func TestManagerAssignGPUInvalidQuota(t *testing.T) {
	mgr := NewManager(nil)
	req := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota: GPUQuota{
			SMPercent: 150, // 无效值
		},
	}
	_, err := mgr.AssignGPU(req)
	if err == nil {
		t.Error("无效配额应返回错误")
	}
}

func TestManagerAssignGPUNonExistentDevice(t *testing.T) {
	mgr := NewManager(nil)
	// 不调用Start()，设备列表为空
	req := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 1024},
	}
	_, err := mgr.AssignGPU(req)
	if err == nil {
		t.Error("不存在的设备应返回错误")
	}
}

func TestManagerAssignGPUDuplicateAssignment(t *testing.T) {
	mgr := NewManager(nil)
	// 手动注入设备
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Test GPU",
		VRAM:       8192,
		Available:  true,
		UpdatedAt:  time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	// 第一次分配
	req := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 1024},
	}
	_, err := mgr.AssignGPU(req)
	if err != nil {
		t.Fatalf("第一次分配失败: %v", err)
	}

	// 重复分配应失败
	_, err = mgr.AssignGPU(req)
	if err == nil {
		t.Error("重复分配应返回错误")
	}
}

func TestManagerAssignGPUMPSModeMultiple(t *testing.T) {
	mgr := NewManager(nil)
	// 注入支持MPS的设备
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Test GPU",
		VRAM:       16384,
		Available:  true,
		Capabilities: GPUCapabilities{
			SupportsMPS:  true,
			MaxInstances: 4,
		},
		UpdatedAt: time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	// MPS模式下可以分配给多个容器
	req1 := &AssignGPURequest{
		ContainerID: "container-1",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeMPS,
		GPUQuota:    GPUQuota{MemoryLimitMB: 2048, SMPercent: 25},
	}
	_, err := mgr.AssignGPU(req1)
	if err != nil {
		t.Fatalf("MPS分配1失败: %v", err)
	}

	req2 := &AssignGPURequest{
		ContainerID: "container-2",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeMPS,
		GPUQuota:    GPUQuota{MemoryLimitMB: 2048, SMPercent: 25},
	}
	_, err = mgr.AssignGPU(req2)
	if err != nil {
		t.Fatalf("MPS分配2失败: %v", err)
	}

	// 验证分配数量
	assignments := mgr.ListAllAssignments()
	if len(assignments) != 2 {
		t.Errorf("期望2个分配记录，得到%d个", len(assignments))
	}
}

func TestManagerAssignGPUVRAMExceeded(t *testing.T) {
	mgr := NewManager(nil)
	// 注入一个显存较小的设备
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Small GPU",
		VRAM:       2048, // 2GB显存
		Available:  true,
		UpdatedAt:  time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	// 请求超过显存的配额
	req := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 4096}, // 请求4GB，设备只有2GB
	}
	_, err := mgr.AssignGPU(req)
	if err == nil {
		t.Error("超出显存的配额应返回错误")
	}
}

func TestManagerUnassignGPUNotFound(t *testing.T) {
	mgr := NewManager(nil)
	req := &UnassignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
	}
	err := mgr.UnassignGPU(req)
	if err == nil {
		t.Error("不存在的分配应返回错误")
	}
}

func TestManagerUnassignGPUSuccess(t *testing.T) {
	mgr := NewManager(nil)
	// 注入设备
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Test GPU",
		VRAM:       8192,
		Available:  true,
		UpdatedAt:  time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	// 先分配
	assignReq := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 1024},
	}
	_, err := mgr.AssignGPU(assignReq)
	if err != nil {
		t.Fatalf("分配失败: %v", err)
	}

	// 取消分配
	unassignReq := &UnassignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
	}
	err = mgr.UnassignGPU(unassignReq)
	if err != nil {
		t.Fatalf("取消分配失败: %v", err)
	}

	// 验证分配已移除
	assignments := mgr.GetContainerAssignments("test-container")
	if len(assignments) != 0 {
		t.Errorf("期望0个分配记录，得到%d个", len(assignments))
	}
}

func TestManagerUpdateQuota(t *testing.T) {
	mgr := NewManager(nil)
	// 注入设备
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Test GPU",
		VRAM:       8192,
		Available:  true,
		UpdatedAt:  time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	// 先分配
	assignReq := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 1024},
	}
	_, err := mgr.AssignGPU(assignReq)
	if err != nil {
		t.Fatalf("分配失败: %v", err)
	}

	// 更新配额
	updateReq := &UpdateQuotaRequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		GPUQuota:    GPUQuota{MemoryLimitMB: 2048, SMPercent: 50},
	}
	err = mgr.UpdateQuota(updateReq)
	if err != nil {
		t.Fatalf("更新配额失败: %v", err)
	}

	// 验证配额已更新
	assignments := mgr.GetContainerAssignments("test-container")
	if len(assignments) != 1 {
		t.Fatalf("期望1个分配记录，得到%d个", len(assignments))
	}
	if assignments[0].GPUQuota.MemoryLimitMB != 2048 {
		t.Errorf("期望显存限制2048MB，得到%dMB", assignments[0].GPUQuota.MemoryLimitMB)
	}
}

func TestManagerUpdateQuotaInvalid(t *testing.T) {
	mgr := NewManager(nil)
	req := &UpdateQuotaRequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		GPUQuota:    GPUQuota{SMPercent: -1},
	}
	err := mgr.UpdateQuota(req)
	if err == nil {
		t.Error("无效配额应返回错误")
	}
}

func TestManagerHotplugInvalidAction(t *testing.T) {
	mgr := NewManager(nil)
	req := &HotplugRequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		Action:      "invalid",
	}
	_, err := mgr.HotplugGPU(req)
	if err == nil {
		t.Error("无效action应返回错误")
	}
}

func TestManagerHotplugNoAssignment(t *testing.T) {
	mgr := NewManager(nil)
	req := &HotplugRequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		Action:      "attach",
	}
	_, err := mgr.HotplugGPU(req)
	if err == nil {
		t.Error("无分配记录的热插拔应返回错误")
	}
}

func TestManagerGetContainerAssignments(t *testing.T) {
	mgr := NewManager(nil)
	assignments := mgr.GetContainerAssignments("nonexistent")
	if assignments == nil {
		t.Error("GetContainerAssignments() 返回nil")
	}
	if len(assignments) != 0 {
		t.Errorf("期望0个分配记录，得到%d个", len(assignments))
	}
}

func TestManagerListAllAssignments(t *testing.T) {
	mgr := NewManager(nil)
	assignments := mgr.ListAllAssignments()
	if assignments == nil {
		t.Error("ListAllAssignments() 返回nil")
	}
	if len(assignments) != 0 {
		t.Errorf("期望0个分配记录，得到%d个", len(assignments))
	}
}

func TestManagerGetDashboard(t *testing.T) {
	mgr := NewManager(nil)
	dashboard := mgr.GetDashboard()
	if dashboard == nil {
		t.Fatal("GetDashboard() 返回nil")
	}
	if dashboard.GPUs == nil {
		t.Error("GPUs列表为nil")
	}
	if dashboard.Assignments == nil {
		t.Error("Assignments列表为nil")
	}
}

func TestManagerGetContainerGPUStatsNoAssignment(t *testing.T) {
	mgr := NewManager(nil)
	_, err := mgr.GetContainerGPUStats("nonexistent")
	if err == nil {
		t.Error("无分配的容器应返回错误")
	}
}

// ========== 容器生命周期联动测试 ==========

func TestManagerOnContainerStart(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.OnContainerStart("nonexistent")
	if err != nil {
		t.Errorf("OnContainerStart不应返回错误: %v", err)
	}
}

func TestManagerOnContainerStop(t *testing.T) {
	mgr := NewManager(nil)
	err := mgr.OnContainerStop("nonexistent")
	if err != nil {
		t.Errorf("OnContainerStop不应返回错误: %v", err)
	}
}

func TestManagerOnContainerDelete(t *testing.T) {
	mgr := NewManager(nil)
	// 注入设备和分配
	device := &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Vendor:     GPUVendorNVIDIA,
		Model:      "Test GPU",
		VRAM:       8192,
		Available:  true,
		UpdatedAt:  time.Now(),
	}
	mgr.devices.mu.Lock()
	mgr.devices.devices["0000:01:00.0"] = device
	mgr.devices.mu.Unlock()

	assignReq := &AssignGPURequest{
		ContainerID: "test-container",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeExclusive,
		GPUQuota:    GPUQuota{MemoryLimitMB: 1024},
	}
	_, err := mgr.AssignGPU(assignReq)
	if err != nil {
		t.Fatalf("分配失败: %v", err)
	}

	// 删除容器应清理所有GPU分配
	err = mgr.OnContainerDelete("test-container")
	if err != nil {
		t.Fatalf("OnContainerDelete失败: %v", err)
	}

	// 验证分配已清理
	assignments := mgr.GetContainerAssignments("test-container")
	if len(assignments) != 0 {
		t.Errorf("期望0个分配记录，得到%d个", len(assignments))
	}
}

// ========== 辅助方法测试 ==========

func TestNormalizePCIAddr(t *testing.T) {
	dm := NewDeviceManager(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"00000000:01:00.0", "0000:01:00.0"},
		{"0000:01:00.0", "0000:01:00.0"},
		{"  0000:02:00.0  ", "0000:02:00.0"},
	}

	for _, tt := range tests {
		result := dm.normalizePCIAddr(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePCIAddr(%q) = %q, 期望 %q", tt.input, result, tt.expected)
		}
	}
}

func TestGPUVendorConstants(t *testing.T) {
	if GPUVendorNVIDIA != "nvidia" {
		t.Error("GPUVendorNVIDIA 应为 'nvidia'")
	}
	if GPUVendorAMD != "amd" {
		t.Error("GPUVendorAMD 应为 'amd'")
	}
	if GPUVendorIntel != "intel" {
		t.Error("GPUVendorIntel 应为 'intel'")
	}
}

func TestShareModeConstants(t *testing.T) {
	if ShareModeExclusive != "exclusive" {
		t.Error("ShareModeExclusive 应为 'exclusive'")
	}
	if ShareModeTimeslice != "timeslice" {
		t.Error("ShareModeTimeslice 应为 'timeslice'")
	}
	if ShareModeMPS != "mps" {
		t.Error("ShareModeMPS 应为 'mps'")
	}
}

func TestAssignmentStateConstants(t *testing.T) {
	states := []AssignmentState{
		AssignmentStatePending,
		AssignmentStateActive,
		AssignmentStateInactive,
		AssignmentStateError,
	}
	for _, state := range states {
		if state == "" {
			t.Error("AssignmentState常量不应为空")
		}
	}
}

func TestHotplugStateConstants(t *testing.T) {
	states := []HotplugState{
		HotplugStateIdle,
		HotplugStateAttaching,
		HotplugStateAttached,
		HotplugStateDetaching,
		HotplugStateError,
	}
	for _, state := range states {
		if state == "" {
			t.Error("HotplugState常量不应为空")
		}
	}
}

// ========== Handler 测试 ==========

func TestNewHandler(t *testing.T) {
	mgr := NewManager(nil)
	handler := NewHandler(mgr)
	if handler == nil {
		t.Fatal("NewHandler() 返回nil")
	}
	if handler.manager != mgr {
		t.Error("Handler的manager应为传入的Manager")
	}
}
