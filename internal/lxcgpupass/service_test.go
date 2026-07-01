package lxcgpupass

import (
	"testing"
	"time"
)

// ========== 类型与验证测试 ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() 返回 nil")
	}
	if cfg.SysPCIBase == "" {
		t.Error("默认 SysPCIBase 不应为空")
	}
	if cfg.DevBase == "" {
		t.Error("默认 DevBase 不应为空")
	}
	if cfg.LXCBase == "" {
		t.Error("默认 LXCBase 不应为空")
	}
	if cfg.CGroupBase == "" {
		t.Error("默认 CGroupBase 不应为空")
	}
}

func TestValidatePCIAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"有效地址", "0000:01:00.0", false},
		{"有效地址2", "0000:00:00.0", false},
		{"太短", "01:00.0", true},
		{"缺少点", "0000:01:000", true},
		{"空字符串", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePCIAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePCIAddr(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
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
	if GPUVendorUnknown != "unknown" {
		t.Error("GPUVendorUnknown 应为 'unknown'")
	}
}

func TestAssignmentStateConstants(t *testing.T) {
	if AssignmentStateActive != "active" {
		t.Error("AssignmentStateActive 应为 'active'")
	}
	if AssignmentStateInactive != "inactive" {
		t.Error("AssignmentStateInactive 应为 'inactive'")
	}
	if AssignmentStateError != "error" {
		t.Error("AssignmentStateError 应为 'error'")
	}
}

// ========== Service 基础测试 ==========

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService(nil) 返回 nil")
	}
	if svc.config == nil {
		t.Error("config 未初始化")
	}
	if svc.devices == nil {
		t.Error("devices map 未初始化")
	}
	if svc.assignments == nil {
		t.Error("assignments map 未初始化")
	}
	if svc.containerGPU == nil {
		t.Error("containerGPU map 未初始化")
	}
}

func TestNewServiceWithConfig(t *testing.T) {
	cfg := &Config{
		SysPCIBase: "/tmp/sys",
		DevBase:    "/tmp/dev",
		LXCBase:    "/tmp/lxc",
		CGroupBase: "/tmp/cgroup",
	}
	svc := NewService(cfg)
	if svc.config != cfg {
		t.Error("config 应为传入的配置")
	}
}

// ========== 设备检测测试 ==========

func TestDetectDevicesNoSysFS(t *testing.T) {
	// 指向不存在的路径，应返回空列表而非报错
	cfg := &Config{
		SysPCIBase: "/nonexistent/path",
		DevBase:    "/dev",
		LXCBase:    "/tmp/lxc-test",
		CGroupBase: "/tmp/cgroup",
	}
	svc := NewService(cfg)
	devices, err := svc.DetectDevices()
	if err != nil {
		t.Fatalf("DetectDevices() 不应返回错误: %v", err)
	}
	if devices == nil {
		t.Fatal("DetectDevices() 返回 nil")
	}
	if len(devices) != 0 {
		t.Errorf("期望 0 个设备，得到 %d 个", len(devices))
	}
}

func TestGetDevicesEmpty(t *testing.T) {
	svc := NewService(nil)
	devices := svc.GetDevices()
	if devices == nil {
		t.Fatal("GetDevices() 返回 nil")
	}
	if len(devices) != 0 {
		t.Errorf("期望 0 个设备，得到 %d 个", len(devices))
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.GetDevice("0000:01:00.0")
	if err == nil {
		t.Error("不存在的设备应返回错误")
	}
}

// ========== GPU 分配测试 ==========

// injectDevice 向 Service 注入测试设备（绕过 sysfs）
func injectDevice(svc *Service, pciAddr string, vendor GPUVendor) *GPUDevice {
	d := &GPUDevice{
		PCIAddress: pciAddr,
		Vendor:     vendor,
		Model:      "Test GPU",
		VRAM:       8192,
		Available:  true,
		Assigned:   false,
		UpdatedAt:  time.Now(),
	}
	svc.devices[pciAddr] = d
	return d
}

func TestAssignGPUSuccess(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)

	req := &AssignRequest{
		ContainerID: "lxc-100",
		GPUPCIAddr:  "0000:01:00.0",
	}
	assignment, err := svc.AssignGPU(req)
	if err != nil {
		t.Fatalf("分配失败: %v", err)
	}
	if assignment == nil {
		t.Fatal("返回 nil 分配记录")
	}
	if assignment.ContainerID != "lxc-100" {
		t.Errorf("ContainerID 期望 lxc-100，得到 %s", assignment.ContainerID)
	}
	if assignment.GPUPCIAddr != "0000:01:00.0" {
		t.Errorf("GPUPCIAddr 期望 0000:01:00.0，得到 %s", assignment.GPUPCIAddr)
	}
	if assignment.State != AssignmentStateActive {
		t.Errorf("State 期望 active，得到 %s", assignment.State)
	}

	// 验证设备状态已更新
	device, _ := svc.GetDevice("0000:01:00.0")
	if !device.Assigned {
		t.Error("设备 Assigned 应为 true")
	}
	if device.Available {
		t.Error("设备 Available 应为 false")
	}
	if device.ContainerID != "lxc-100" {
		t.Errorf("ContainerID 期望 lxc-100，得到 %s", device.ContainerID)
	}
}

func TestAssignGPUDuplicate(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)

	req := &AssignRequest{
		ContainerID: "lxc-100",
		GPUPCIAddr:  "0000:01:00.0",
	}
	_, err := svc.AssignGPU(req)
	if err != nil {
		t.Fatalf("第一次分配失败: %v", err)
	}

	// 重复分配应失败
	_, err = svc.AssignGPU(req)
	if err == nil {
		t.Error("重复分配应返回错误")
	}
}

func TestAssignGPUNotExist(t *testing.T) {
	svc := NewService(nil)
	req := &AssignRequest{
		ContainerID: "lxc-100",
		GPUPCIAddr:  "0000:99:99.9",
	}
	_, err := svc.AssignGPU(req)
	if err == nil {
		t.Error("不存在的设备应返回错误")
	}
}

func TestAssignGPUInvalidPCIAddr(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "bad", GPUVendorNVIDIA)

	req := &AssignRequest{
		ContainerID: "lxc-100",
		GPUPCIAddr:  "bad",
	}
	_, err := svc.AssignGPU(req)
	if err == nil {
		t.Error("无效 PCI 地址应返回错误")
	}
}

func TestAssignGPUMultipleContainers(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)
	injectDevice(svc, "0000:02:00.0", GPUVendorAMD)

	// 容器 1 分配 GPU 1
	req1 := &AssignRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"}
	_, err := svc.AssignGPU(req1)
	if err != nil {
		t.Fatalf("容器1分配失败: %v", err)
	}

	// 容器 2 分配 GPU 2
	req2 := &AssignRequest{ContainerID: "lxc-200", GPUPCIAddr: "0000:02:00.0"}
	_, err = svc.AssignGPU(req2)
	if err != nil {
		t.Fatalf("容器2分配失败: %v", err)
	}

	// 验证状态
	status := svc.GetStatus()
	if status.TotalGPUs != 2 {
		t.Errorf("期望 2 个 GPU，得到 %d", status.TotalGPUs)
	}
	if status.AssignedGPUs != 2 {
		t.Errorf("期望 2 个已分配，得到 %d", status.AssignedGPUs)
	}
	if status.AvailableGPUs != 0 {
		t.Errorf("期望 0 个可用，得到 %d", status.AvailableGPUs)
	}
	if len(status.Assignments) != 2 {
		t.Errorf("期望 2 条分配记录，得到 %d", len(status.Assignments))
	}
}

// ========== GPU 移除测试 ==========

func TestRemoveGPUSuccess(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)

	// 先分配
	_, _ = svc.AssignGPU(&AssignRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"})

	// 移除
	req := &RemoveRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"}
	err := svc.RemoveGPU(req)
	if err != nil {
		t.Fatalf("移除失败: %v", err)
	}

	// 验证设备已释放
	device, _ := svc.GetDevice("0000:01:00.0")
	if device.Assigned {
		t.Error("设备 Assigned 应为 false")
	}
	if !device.Available {
		t.Error("设备 Available 应为 true")
	}
	if device.ContainerID != "" {
		t.Errorf("ContainerID 应为空，得到 %s", device.ContainerID)
	}

	// 验证分配记录已删除
	status := svc.GetStatus()
	if len(status.Assignments) != 0 {
		t.Errorf("期望 0 条分配记录，得到 %d", len(status.Assignments))
	}
}

func TestRemoveGPUNotAssigned(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)

	req := &RemoveRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"}
	err := svc.RemoveGPU(req)
	if err == nil {
		t.Error("移除未分配的 GPU 应返回错误")
	}
}

func TestRemoveGPUAllForContainer(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)
	injectDevice(svc, "0000:02:00.0", GPUVendorAMD)

	// 分配两个 GPU 给同一容器
	_, _ = svc.AssignGPU(&AssignRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"})
	_, _ = svc.AssignGPU(&AssignRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:02:00.0"})

	// 移除该容器所有 GPU（不指定 PCI 地址）
	req := &RemoveRequest{ContainerID: "lxc-100"}
	err := svc.RemoveGPU(req)
	if err != nil {
		t.Fatalf("移除失败: %v", err)
	}

	// 验证两个设备都已释放
	d1, _ := svc.GetDevice("0000:01:00.0")
	if d1.Assigned {
		t.Error("GPU1 Assigned 应为 false")
	}
	d2, _ := svc.GetDevice("0000:02:00.0")
	if d2.Assigned {
		t.Error("GPU2 Assigned 应为 false")
	}

	// 验证状态
	status := svc.GetStatus()
	if status.AssignedGPUs != 0 {
		t.Errorf("期望 0 个已分配，得到 %d", status.AssignedGPUs)
	}
}

// ========== 状态查询测试 ==========

func TestGetStatusEmpty(t *testing.T) {
	svc := NewService(nil)
	status := svc.GetStatus()
	if status == nil {
		t.Fatal("GetStatus() 返回 nil")
	}
	if status.TotalGPUs != 0 {
		t.Errorf("期望 0 个 GPU，得到 %d", status.TotalGPUs)
	}
	if status.Assignments == nil {
		t.Error("Assignments 不应为 nil")
	}
}

func TestGetContainerGPUs(t *testing.T) {
	svc := NewService(nil)
	injectDevice(svc, "0000:01:00.0", GPUVendorNVIDIA)

	// 未分配时返回空列表
	gpus := svc.GetContainerGPUs("lxc-100")
	if len(gpus) != 0 {
		t.Errorf("期望 0 个 GPU，得到 %d", len(gpus))
	}

	// 分配后返回列表
	_, _ = svc.AssignGPU(&AssignRequest{ContainerID: "lxc-100", GPUPCIAddr: "0000:01:00.0"})
	gpus = svc.GetContainerGPUs("lxc-100")
	if len(gpus) != 1 {
		t.Errorf("期望 1 个 GPU，得到 %d", len(gpus))
	}
	if gpus[0].PCIAddress != "0000:01:00.0" {
		t.Errorf("PCI 地址期望 0000:01:00.0，得到 %s", gpus[0].PCIAddress)
	}
}

// ========== Handler 测试 ==========

func TestNewHandler(t *testing.T) {
	svc := NewService(nil)
	handler := NewHandler(svc)
	if handler == nil {
		t.Fatal("NewHandler() 返回 nil")
	}
	if handler.service != svc {
		t.Error("Handler 的 service 应为传入的 Service")
	}
}

// ========== 辅助函数测试 ==========

func TestIdentifyVendor(t *testing.T) {
	tests := []struct {
		vendorID string
		want     GPUVendor
	}{
		{"0x10de", GPUVendorNVIDIA},
		{"0x1002", GPUVendorAMD},
		{"0x8086", GPUVendorIntel},
		{"0x1234", GPUVendorUnknown},
		{"", GPUVendorUnknown},
	}
	for _, tt := range tests {
		got := identifyVendor(tt.vendorID)
		if got != tt.want {
			t.Errorf("identifyVendor(%q) = %v, 期望 %v", tt.vendorID, got, tt.want)
		}
	}
}
