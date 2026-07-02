package nvmeofenhanced

import (
	"testing"
)

func TestNewNVMeOFManager(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	if m == nil {
		t.Fatal("NewNVMeOFManager returned nil")
	}
}

func TestCreateSubsystem(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	subsystem := &NVMeSubsystem{
		ID:          "sub1",
		NQN:         "nqn.2024-01.com.example:sub1",
		Alias:       "测试子系统",
		Description: "测试NVMe子系统",
		Transport:   TransportTCP,
		IPAddress:   "192.168.1.100",
		Port:        4420,
	}

	err := m.CreateSubsystem(subsystem)
	if err != nil {
		t.Fatalf("CreateSubsystem failed: %v", err)
	}

	// 验证子系统已创建
	retrieved, err := m.GetSubsystem("sub1")
	if err != nil {
		t.Fatalf("GetSubsystem failed: %v", err)
	}

	if retrieved.NQN != "nqn.2024-01.com.example:sub1" {
		t.Errorf("Expected NQN 'nqn.2024-01.com.example:sub1', got '%s'", retrieved.NQN)
	}
}

func TestCreateDuplicateSubsystem(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	subsystem := &NVMeSubsystem{
		ID:  "sub1",
		NQN: "nqn.2024-01.com.example:sub1",
	}

	m.CreateSubsystem(subsystem)

	err := m.CreateSubsystem(subsystem)
	if err != ErrSubsystemExists {
		t.Errorf("Expected ErrSubsystemExists, got %v", err)
	}
}

func TestCreateNamespace(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	// 先创建子系统
	subsystem := &NVMeSubsystem{
		ID:  "sub1",
		NQN: "nqn.2024-01.com.example:sub1",
	}
	m.CreateSubsystem(subsystem)

	// 创建命名空间
	namespace := &NVMeNamespace{
		ID:          "ns1",
		SubsystemID: "sub1",
		NSID:        1,
		UUID:        "uuid-001",
		SizeBytes:   1024 * 1024 * 1024 * 100, // 100GB
		BlockSize:   4096,
	}

	err := m.CreateNamespace(namespace)
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}

	// 验证命名空间已创建
	retrieved, err := m.GetNamespace("ns1")
	if err != nil {
		t.Fatalf("GetNamespace failed: %v", err)
	}

	if retrieved.NSID != 1 {
		t.Errorf("Expected NSID 1, got %d", retrieved.NSID)
	}
}

func TestConnectController(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	// 先创建子系统
	subsystem := &NVMeSubsystem{
		ID:  "sub1",
		NQN: "nqn.2024-01.com.example:sub1",
	}
	m.CreateSubsystem(subsystem)

	// 连接控制器
	controller := &NVMeController{
		ID:          "ctrl1",
		SubsystemID: "sub1",
		CNTLID:      1,
		Model:       "NVMe Controller",
		Serial:      "SN001",
		Transport:   TransportTCP,
		IPAddress:   "192.168.1.101",
		Port:        4420,
	}

	err := m.ConnectController(controller)
	if err != nil {
		t.Fatalf("ConnectController failed: %v", err)
	}

	// 验证控制器已连接
	retrieved, err := m.GetController("ctrl1")
	if err != nil {
		t.Fatalf("GetController failed: %v", err)
	}

	if retrieved.Model != "NVMe Controller" {
		t.Errorf("Expected model 'NVMe Controller', got '%s'", retrieved.Model)
	}
}

func TestAddNetworkInterface(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	iface := &NetworkInterface{
		ID:        "eth0",
		Name:      "eth0",
		Speed:     Speed400G,
		IPAddress: "192.168.1.10",
		Subnet:    "255.255.255.0",
		Gateway:   "192.168.1.1",
		MTU:       9000,
		IsRDMA:    true,
		IsOnline:  true,
	}

	err := m.AddNetworkInterface(iface)
	if err != nil {
		t.Fatalf("AddNetworkInterface failed: %v", err)
	}

	// 验证接口已添加
	retrieved, err := m.GetNetworkInterface("eth0")
	if err != nil {
		t.Fatalf("GetNetworkInterface failed: %v", err)
	}

	if retrieved.Speed != Speed400G {
		t.Errorf("Expected speed 400g, got '%s'", retrieved.Speed)
	}
}

func TestConfigureRDMA(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	rdmaConfig := RDMAConfig{
		Enabled:        true,
		Device:         "mlx5_0",
		QueuePairCount: 512,
		MaxInlineData:  512,
		MaxSendWR:      2048,
		MaxRecvWR:      2048,
		MaxSGE:         32,
		UseGRH:         true,
	}

	err := m.ConfigureRDMA(rdmaConfig)
	if err != nil {
		t.Fatalf("ConfigureRDMA failed: %v", err)
	}

	// 验证配置已更新
	retrieved := m.GetRDMAConfig()
	if retrieved.Device != "mlx5_0" {
		t.Errorf("Expected device 'mlx5_0', got '%s'", retrieved.Device)
	}
}

func TestUpdateMetrics(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	// 先创建子系统
	subsystem := &NVMeSubsystem{
		ID:  "sub1",
		NQN: "nqn.2024-01.com.example:sub1",
	}
	m.CreateSubsystem(subsystem)

	// 更新指标
	metrics := &PerformanceMetrics{
		SubsystemID:         "sub1",
		ReadIOPS:            100000,
		WriteIOPS:           50000,
		TotalIOPS:           150000,
		ReadThroughputMBps:  3200,
		WriteThroughputMBps: 1600,
		TotalThroughputMBps: 4800,
		ReadLatencyUs:       50,
		WriteLatencyUs:      100,
		AvgLatencyUs:        75,
	}

	err := m.UpdateMetrics("sub1", metrics)
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// 验证指标已更新
	retrieved, err := m.GetMetrics("sub1")
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if retrieved.ReadIOPS != 100000 {
		t.Errorf("Expected ReadIOPS 100000, got %d", retrieved.ReadIOPS)
	}
}

func TestGetSubsystemStats(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	// 创建子系统
	subsystem := &NVMeSubsystem{
		ID:        "sub1",
		NQN:       "nqn.2024-01.com.example:sub1",
		Transport: TransportTCP,
		IsOnline:  true,
	}
	m.CreateSubsystem(subsystem)

	// 创建命名空间
	namespace := &NVMeNamespace{
		ID:          "ns1",
		SubsystemID: "sub1",
		NSID:        1,
	}
	m.CreateNamespace(namespace)

	// 获取统计
	stats := m.GetSubsystemStats("sub1")

	if stats["id"] != "sub1" {
		t.Errorf("Expected id 'sub1', got '%v'", stats["id"])
	}

	if stats["namespace_count"] != 1 {
		t.Errorf("Expected namespace_count 1, got %v", stats["namespace_count"])
	}
}

func TestGetGlobalStats(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	// 创建一些数据
	m.CreateSubsystem(&NVMeSubsystem{ID: "sub1", Transport: TransportTCP})
	m.CreateSubsystem(&NVMeSubsystem{ID: "sub2", Transport: TransportRDMA})
	m.AddNetworkInterface(&NetworkInterface{ID: "eth0", Speed: Speed400G})

	stats := m.GetGlobalStats()

	if stats["total_subsystems"] != 2 {
		t.Errorf("Expected total_subsystems 2, got %v", stats["total_subsystems"])
	}

	if stats["total_interfaces"] != 1 {
		t.Errorf("Expected total_interfaces 1, got %v", stats["total_interfaces"])
	}
}

func TestListSubsystems(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	m.CreateSubsystem(&NVMeSubsystem{ID: "sub1", Transport: TransportTCP})
	m.CreateSubsystem(&NVMeSubsystem{ID: "sub2", Transport: TransportRDMA})
	m.CreateSubsystem(&NVMeSubsystem{ID: "sub3", Transport: TransportTCP})

	// 列出所有
	all := m.ListSubsystems("")
	if len(all) != 3 {
		t.Errorf("Expected 3 subsystems, got %d", len(all))
	}

	// 按传输类型筛选
	tcpOnly := m.ListSubsystems(TransportTCP)
	if len(tcpOnly) != 2 {
		t.Errorf("Expected 2 TCP subsystems, got %d", len(tcpOnly))
	}
}

func TestDeleteSubsystem(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewNVMeOFManager(config)

	m.CreateSubsystem(&NVMeSubsystem{ID: "sub1", Transport: TransportTCP})

	err := m.DeleteSubsystem("sub1")
	if err != nil {
		t.Fatalf("DeleteSubsystem failed: %v", err)
	}

	_, err = m.GetSubsystem("sub1")
	if err != ErrSubsystemNotFound {
		t.Errorf("Expected ErrSubsystemNotFound, got %v", err)
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		speed    NetworkSpeed
		expected string
	}{
		{Speed10G, "10 GbE"},
		{Speed25G, "25 GbE"},
		{Speed40G, "40 GbE"},
		{Speed100G, "100 GbE"},
		{Speed200G, "200 GbE"},
		{Speed400G, "400 GbE"},
	}

	for _, test := range tests {
		result := FormatSpeed(test.speed)
		if result != test.expected {
			t.Errorf("FormatSpeed(%s) = %s, want %s", test.speed, result, test.expected)
		}
	}
}

func TestFormatTransport(t *testing.T) {
	tests := []struct {
		transport TransportType
		expected  string
	}{
		{TransportTCP, "NVMe/TCP"},
		{TransportRDMA, "NVMe/RDMA"},
		{TransportFC, "NVMe/FC"},
		{TransportIB, "NVMe/IB"},
	}

	for _, test := range tests {
		result := FormatTransport(test.transport)
		if result != test.expected {
			t.Errorf("FormatTransport(%s) = %s, want %s", test.transport, result, test.expected)
		}
	}
}
