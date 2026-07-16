package rdmanfs

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.state != NFSStateStopped {
		t.Errorf("初始状态 = %q, want %q", m.state, NFSStateStopped)
	}
	if m.nfsConfig.Port != 2049 {
		t.Errorf("默认端口 = %d, want 2049", m.nfsConfig.Port)
	}
}

func TestDetectRDMADevices(t *testing.T) {
	m := NewManager(nil)
	devices, err := m.DetectRDMADevices(context.Background())
	if err != nil {
		t.Fatalf("DetectRDMADevices 失败: %v", err)
	}
	// 当前是模拟实现，返回空列表
	if len(devices) != 0 {
		t.Errorf("模拟检测应返回 0 设备, got %d", len(devices))
	}
}

func TestConfigureNFSRDMA(t *testing.T) {
	m := NewManager(nil)

	cfg := NFSRDMAConfig{
		Enabled:        true,
		Device:         "mlx5_0",
		ExportRoot:     "/export/nfsrdma",
		Port:           2049,
		NFSVersion:     "4.2",
		MaxConnections: 64,
		ReadWriteSize:  4 * 1024 * 1024,
		AuthType:       "krb5",
	}
	if err := m.ConfigureNFSRDMA(cfg); err != nil {
		t.Fatalf("ConfigureNFSRDMA 失败: %v", err)
	}

	got := m.GetNFSRDMAConfig()
	if got.Device != "mlx5_0" {
		t.Errorf("设备 = %q, want mlx5_0", got.Device)
	}
	if got.AuthType != "krb5" {
		t.Errorf("认证类型 = %q, want krb5", got.AuthType)
	}
}

func TestConfigureInvalidPort(t *testing.T) {
	m := NewManager(nil)
	cfg := NFSRDMAConfig{Port: 0, ExportRoot: "/export"}
	if err := m.ConfigureNFSRDMA(cfg); err == nil {
		t.Fatal("无效端口应返回错误")
	}
	cfg = NFSRDMAConfig{Port: 99999, ExportRoot: "/export"}
	if err := m.ConfigureNFSRDMA(cfg); err == nil {
		t.Fatal("超出范围端口应返回错误")
	}
}

func TestConfigureEmptyExportRoot(t *testing.T) {
	m := NewManager(nil)
	cfg := NFSRDMAConfig{Port: 2049, ExportRoot: ""}
	if err := m.ConfigureNFSRDMA(cfg); err == nil {
		t.Fatal("空导出根目录应返回错误")
	}
}

func TestStartStopService(t *testing.T) {
	m := NewManager(nil)

	// 未启用时应启动失败
	if err := m.StartService(context.Background()); err == nil {
		t.Fatal("未启用时应启动失败")
	}

	// 启用并配置
	m.ConfigureNFSRDMA(NFSRDMAConfig{
		Enabled:    true,
		Device:     "mlx5_0",
		ExportRoot: "/export",
		Port:       2049,
	})

	if err := m.StartService(context.Background()); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}
	if m.GetServiceState() != NFSStateRunning {
		t.Errorf("状态 = %q, want %q", m.GetServiceState(), NFSStateRunning)
	}

	// 重复启动
	if err := m.StartService(context.Background()); err == nil {
		t.Fatal("重复启动应返回错误")
	}

	if err := m.StopService(context.Background()); err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}
	if m.GetServiceState() != NFSStateStopped {
		t.Errorf("状态 = %q, want %q", m.GetServiceState(), NFSStateStopped)
	}
}

func TestAddRemoveExport(t *testing.T) {
	m := NewManager(nil)

	export := NFSExport{
		Path:     "/export/data",
		Client:   "192.168.1.0/24",
		ReadOnly: false,
		Sync:     true,
	}
	if err := m.AddExport(export); err != nil {
		t.Fatalf("AddExport 失败: %v", err)
	}

	exports := m.ListExports()
	if len(exports) != 1 {
		t.Fatalf("导出数 = %d, want 1", len(exports))
	}

	if err := m.RemoveExport("/export/data"); err != nil {
		t.Fatalf("RemoveExport 失败: %v", err)
	}
	if len(m.ListExports()) != 0 {
		t.Error("删除后导出列表应空")
	}
}

func TestRemoveNonexistentExport(t *testing.T) {
	m := NewManager(nil)
	if err := m.RemoveExport("/nonexistent"); err == nil {
		t.Fatal("删除不存在的导出应返回错误")
	}
}

func TestAddExportEmptyPath(t *testing.T) {
	m := NewManager(nil)
	export := NFSExport{Path: "", Client: "*"}
	if err := m.AddExport(export); err == nil {
		t.Fatal("空路径导出应返回错误")
	}
}

func TestCollectStats(t *testing.T) {
	m := NewManager(nil)
	stats, err := m.CollectStats(context.Background())
	if err != nil {
		t.Fatalf("CollectStats 失败: %v", err)
	}
	if stats == nil {
		t.Fatal("stats 不应为 nil")
	}
	if stats.CollectAt.IsZero() {
		t.Error("采集时间不应为零")
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager(nil)
	m.ConfigureNFSRDMA(NFSRDMAConfig{
		Enabled:    true,
		Device:     "mlx5_0",
		ExportRoot: "/export",
		Port:       2049,
	})
	m.StartService(context.Background())

	status := m.GetStatus()
	if status.NFSRDMA != NFSStateRunning {
		t.Errorf("NFS RDMA 状态 = %q, want %q", status.NFSRDMA, NFSStateRunning)
	}
	if status.Uptime == nil {
		t.Error("运行中应有 uptime")
	}
}
