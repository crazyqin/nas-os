package nfsoverrdma

import (
	"net"
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("管理器创建失败")
	}
}

func TestConfigureRDMA(t *testing.T) {
	manager := NewManager()

	config := &RDMAConfig{
		Enabled:    true,
		Provider:   ProviderRoCE,
		DeviceName: "mlx5_0",
		Port:       1,
		MTU:        4096,
	}

	if err := manager.ConfigureRDMA(config); err != nil {
		t.Fatalf("配置RDMA失败: %v", err)
	}

	stats := manager.GetRDMAStats()
	if stats.DeviceName != "mlx5_0" {
		t.Errorf("期望设备名 mlx5_0，实际 %s", stats.DeviceName)
	}
}

func TestCreateExport(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{
		ID:   "export-1",
		Path: "/data/share",
		Name: "测试共享",
		Mode: ExportModeReadWrite,
	}

	if err := manager.CreateExport(export); err != nil {
		t.Fatalf("创建导出失败: %v", err)
	}

	if len(manager.exports) != 1 {
		t.Errorf("期望1个导出，实际 %d", len(manager.exports))
	}

	// 重复创建应失败
	if err := manager.CreateExport(export); err != ErrExportExists {
		t.Errorf("期望 ErrExportExists，实际 %v", err)
	}
}

func TestGetExport(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{
		ID:   "export-1",
		Path: "/data/share",
		Name: "测试共享",
	}

	manager.CreateExport(export)

	got, err := manager.GetExport("export-1")
	if err != nil {
		t.Fatalf("获取导出失败: %v", err)
	}
	if got.Name != "测试共享" {
		t.Errorf("期望名称 测试共享，实际 %s", got.Name)
	}

	_, err = manager.GetExport("nonexistent")
	if err != ErrExportNotFound {
		t.Errorf("期望 ErrExportNotFound，实际 %v", err)
	}
}

func TestDeleteExport(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{
		ID:   "export-1",
		Path: "/data/share",
		Name: "测试共享",
	}

	manager.CreateExport(export)
	manager.AddClient(&NFSClient{
		ID:       "client-1",
		ExportID: "export-1",
		ClientIP: net.ParseIP("192.168.1.100"),
	})

	if err := manager.DeleteExport("export-1"); err != nil {
		t.Fatalf("删除导出失败: %v", err)
	}

	if len(manager.exports) != 0 {
		t.Errorf("期望0个导出，实际 %d", len(manager.exports))
	}
	if len(manager.clients) != 0 {
		t.Errorf("期望0个客户端，实际 %d", len(manager.clients))
	}
}

func TestAddClient(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{
		ID:   "export-1",
		Path: "/data/share",
		Name: "测试共享",
	}
	manager.CreateExport(export)

	client := &NFSClient{
		ID:       "client-1",
		ExportID: "export-1",
		ClientIP: net.ParseIP("192.168.1.100"),
		Hostname: "test-host",
	}

	if err := manager.AddClient(client); err != nil {
		t.Fatalf("添加客户端失败: %v", err)
	}

	if len(manager.clients) != 1 {
		t.Errorf("期望1个客户端，实际 %d", len(manager.clients))
	}

	got, err := manager.GetClient("client-1")
	if err != nil {
		t.Fatalf("获取客户端失败: %v", err)
	}
	if got.Status != ConnStatusConnected {
		t.Errorf("期望状态 connected，实际 %s", got.Status)
	}
}

func TestRemoveClient(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{ID: "export-1", Path: "/data", Name: "test"}
	manager.CreateExport(export)
	manager.AddClient(&NFSClient{ID: "client-1", ExportID: "export-1", ClientIP: net.ParseIP("192.168.1.1")})

	if err := manager.RemoveClient("client-1"); err != nil {
		t.Fatalf("移除客户端失败: %v", err)
	}

	if len(manager.clients) != 0 {
		t.Errorf("期望0个客户端，实际 %d", len(manager.clients))
	}
}

func TestEnableRDMAOnExport(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{ID: "export-1", Path: "/data", Name: "test", Transport: TransportTCP}
	manager.CreateExport(export)

	config := &RDMAConfig{
		Enabled:    true,
		Provider:   ProviderRoCE,
		DeviceName: "mlx5_0",
	}

	if err := manager.EnableRDMAOnExport("export-1", config); err != nil {
		t.Fatalf("启用RDMA失败: %v", err)
	}

	got, _ := manager.GetExport("export-1")
	if got.Transport != TransportRDMA {
		t.Errorf("期望传输类型 rdma，实际 %s", got.Transport)
	}
}

func TestGetServerStats(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{ID: "export-1", Path: "/data", Name: "test"}
	manager.CreateExport(export)

	stats := manager.GetServerStats()
	totalExports := stats["total_exports"].(int)
	if totalExports != 1 {
		t.Errorf("期望1个导出，实际 %d", totalExports)
	}
}

func TestSimulateRDMAConnection(t *testing.T) {
	manager := NewManager()

	result, err := manager.SimulateRDMAConnection("192.168.1.1", 2049)
	if err != nil {
		t.Fatalf("模拟连接失败: %v", err)
	}

	if !result.Success {
		t.Error("连接应成功")
	}
	if result.LatencyMs <= 0 {
		t.Error("延迟应大于0")
	}
}

func TestGetPerformanceMetrics(t *testing.T) {
	manager := NewManager()

	export := &NFSExport{ID: "export-1", Path: "/data", Name: "test"}
	manager.CreateExport(export)

	metrics := manager.GetPerformanceMetrics("export-1")
	if metrics == nil {
		t.Fatal("性能指标不应为nil")
	}
	if metrics.ExportID != "export-1" {
		t.Errorf("期望导出ID export-1，实际 %s", metrics.ExportID)
	}
}
