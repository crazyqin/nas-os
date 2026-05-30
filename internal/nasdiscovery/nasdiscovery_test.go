package nasdiscovery

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m == nil {
		t.Fatal("Manager 不应为 nil")
	}

	if !m.config.Enabled {
		t.Error("默认配置应启用")
	}

	if m.config.ScanInterval != 60 {
		t.Errorf("默认扫描间隔应为 60，实际: %d", m.config.ScanInterval)
	}

	if m.config.UDPPort != 9999 {
		t.Errorf("默认 UDP 端口应为 9999，实际: %d", m.config.UDPPort)
	}
}

func TestGetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	config := m.GetConfig()
	if config == nil {
		t.Fatal("Config 不应为 nil")
	}

	if !config.MDNSEnabled {
		t.Error("mDNS 应默认启用")
	}

	if !config.SSDPEnabled {
		t.Error("SSDP 应默认启用")
	}

	if !config.AutoAddDevices {
		t.Error("自动添加设备应默认启用")
	}
}

func TestUpdateDeviceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	newConfig := DiscoveryConfig{
		ScanInterval: 120,
		UDPPort:      8888,
		Enabled:      false,
	}

	m.UpdateDeviceConfig(newConfig)

	config := m.GetConfig()
	if config.ScanInterval != 120 {
		t.Errorf("扫描间隔应为 120，实际: %d", config.ScanInterval)
	}

	if config.UDPPort != 8888 {
		t.Errorf("UDP 端口应为 8888，实际: %d", config.UDPPort)
	}

	if config.Enabled {
		t.Error("Enabled 应为 false")
	}
}

func TestAddManualDevice(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	device, err := m.AddManualDevice(ctx, "192.168.1.100", "测试NAS")
	if err != nil {
		t.Fatalf("手动添加设备失败: %v", err)
	}

	if device.IP != "192.168.1.100" {
		t.Errorf("IP 不匹配: %s", device.IP)
	}

	if device.Hostname != "测试NAS" {
		t.Errorf("Hostname 不匹配: %s", device.Hostname)
	}

	if !device.ManualAdd {
		t.Error("ManualAdd 应为 true")
	}

	if device.ID == "" {
		t.Error("ID 不应为空")
	}
}

func TestGetDevices(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	// 初始应为空
	devices := m.GetDevices()
	if len(devices) != 0 {
		t.Errorf("初始设备数应为 0，实际: %d", len(devices))
	}

	// 添加设备
	m.AddManualDevice(ctx, "192.168.1.100", "NAS1")
	m.AddManualDevice(ctx, "192.168.1.101", "NAS2")

	devices = m.GetDevices()
	if len(devices) != 2 {
		t.Errorf("设备数应为 2，实际: %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	device, _ := m.AddManualDevice(ctx, "192.168.1.100", "测试NAS")

	found, err := m.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("获取设备失败: %v", err)
	}

	if found.IP != "192.168.1.100" {
		t.Errorf("IP 不匹配: %s", found.IP)
	}

	// 不存在的设备
	_, err = m.GetDevice("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound，实际: %v", err)
	}
}

func TestRemoveDevice(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	device, _ := m.AddManualDevice(ctx, "192.168.1.100", "测试NAS")

	err := m.RemoveDevice(device.ID)
	if err != nil {
		t.Fatalf("删除设备失败: %v", err)
	}

	devices := m.GetDevices()
	if len(devices) != 0 {
		t.Errorf("设备数应为 0，实际: %d", len(devices))
	}

	// 删除不存在的设备
	err = m.RemoveDevice("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound，实际: %v", err)
	}
}

func TestMarkTrusted(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	device, _ := m.AddManualDevice(ctx, "192.168.1.100", "测试NAS")

	err := m.MarkTrusted(device.ID, true)
	if err != nil {
		t.Fatalf("标记信任失败: %v", err)
	}

	found, _ := m.GetDevice(device.ID)
	if !found.Trusted {
		t.Error("设备应被标记为受信任")
	}

	// 取消信任
	m.MarkTrusted(device.ID, false)
	found, _ = m.GetDevice(device.ID)
	if found.Trusted {
		t.Error("设备应被标记为不受信任")
	}
}

func TestGetOnlineDevices(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	m.AddManualDevice(ctx, "192.168.1.100", "NAS1")
	m.AddManualDevice(ctx, "192.168.1.101", "NAS2")

	// 手动添加的设备默认在线
	online := m.GetOnlineDevices()
	if len(online) != 2 {
		t.Errorf("在线设备数应为 2，实际: %d", len(online))
	}
}

func TestStartStopDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	ctx := context.Background()

	// 启动发现服务
	err := m.StartDiscovery(ctx)
	if err != nil {
		t.Fatalf("启动发现服务失败: %v", err)
	}

	// 等待一下让服务启动
	time.Sleep(100 * time.Millisecond)

	// 重复启动应返回错误
	err = m.StartDiscovery(ctx)
	if err != ErrAlreadyRunning {
		t.Errorf("期望 ErrAlreadyRunning，实际: %v", err)
	}

	// 停止发现服务
	m.StopDiscovery()
	time.Sleep(100 * time.Millisecond)
}

func TestSaveLoadDevices(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// 创建管理器并添加设备
	m1 := NewManager(tmpDir)
	m1.AddManualDevice(ctx, "192.168.1.100", "NAS1")
	m1.AddManualDevice(ctx, "192.168.1.101", "NAS2")

	// 创建新管理器，应加载已保存的设备
	m2 := NewManager(tmpDir)
	devices := m2.GetDevices()

	if len(devices) != 2 {
		t.Errorf("加载的设备数应为 2，实际: %d", len(devices))
	}
}

func TestDiscoveryDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	NewManager(tmpDir)

	discoveryDir := tmpDir + "/discovery"
	if _, err := os.Stat(discoveryDir); os.IsNotExist(err) {
		t.Error("discovery 目录应该被创建")
	}
}

func TestScanResult(t *testing.T) {
	// 验证扫描结果结构
	result := &ScanResult{
		ID:       "test-id",
		StartTime: time.Now(),
		Status:   ScanStatusCompleted,
	}

	if result.Status != ScanStatusCompleted {
		t.Errorf("扫描状态应为 completed，实际: %s", result.Status)
	}
}
