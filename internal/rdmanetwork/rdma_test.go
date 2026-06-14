package rdmanetwork

import (
	"testing"
	"time"
)

func TestRDMANetworkManager_Create(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.config.MTU != config.MTU {
		t.Errorf("MTU = %d, want %d", manager.config.MTU, config.MTU)
	}
}

func TestRDMANetworkManager_StartStop(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)

	err := manager.Start()
	if err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	if !manager.running {
		t.Error("expected manager to be running")
	}

	err = manager.Stop()
	if err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}

	if manager.running {
		t.Error("expected manager to be stopped")
	}
}

func TestRDMANetworkManager_RegisterDevice(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	device := &RDMADevice{
		Name: "mlx5_0",
		GUID: "0000000000000001",
		Type: RDMATypeRoCE,
		Ports: []*RDMAPort{
			{
				ID:    1,
				LID:   1,
				GID:   "fe80::1",
				State: RDMAStateConnected,
				MTU:   4096,
			},
		},
	}

	err := manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	if len(manager.devices) != 1 {
		t.Errorf("devices count = %d, want 1", len(manager.devices))
	}
}

func TestRDMANetworkManager_UnregisterDevice(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	device := &RDMADevice{
		Name: "mlx5_0",
		GUID: "0000000000000001",
		Type: RDMATypeRoCE,
	}

	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected

	err := manager.UnregisterDevice("mlx5_0")
	if err != nil {
		t.Fatalf("failed to unregister device: %v", err)
	}

	if len(manager.devices) != 0 {
		t.Errorf("devices count = %d, want 0", len(manager.devices))
	}
}

func TestRDMANetworkManager_Connect(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册设备
	device := &RDMADevice{
		Name:  "mlx5_0",
		GUID:  "0000000000000001",
		Type:  RDMATypeRoCE,
		State: RDMAStateConnected,
		Ports: []*RDMAPort{
			{
				ID:    1,
				State: RDMAStateConnected,
			},
		},
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected
	manager.devices["mlx5_0"].State = RDMAStateConnected

	// 建立连接
	conn, err := manager.Connect("mlx5_0", "192.168.1.100", 1)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	if conn.ID == "" {
		t.Error("expected connection ID")
	}

	// 等待连接建立
	time.Sleep(200 * time.Millisecond)

	conn, _ = manager.GetConnection(conn.ID)
	if conn.State != RDMAStateConnected {
		t.Errorf("connection state = %d, want %d", conn.State, RDMAStateConnected)
	}
}

func TestRDMANetworkManager_Disconnect(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册设备
	device := &RDMADevice{
		Name:  "mlx5_0",
		GUID:  "0000000000000001",
		Type:  RDMATypeRoCE,
		State: RDMAStateConnected,
		Ports: []*RDMAPort{
			{
				ID:    1,
				State: RDMAStateConnected,
			},
		},
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected
	manager.devices["mlx5_0"].State = RDMAStateConnected

	// 建立连接
	conn, _ := manager.Connect("mlx5_0", "192.168.1.100", 1)
	time.Sleep(200 * time.Millisecond)

	// 断开连接
	err := manager.Disconnect(conn.ID)
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}

	// 验证连接已删除
	_, err = manager.GetConnection(conn.ID)
	if err == nil {
		t.Error("expected error for deleted connection")
	}
}

func TestRDMANetworkManager_SendReceive(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册设备
	device := &RDMADevice{
		Name:  "mlx5_0",
		GUID:  "0000000000000001",
		Type:  RDMATypeRoCE,
		State: RDMAStateConnected,
		Ports: []*RDMAPort{
			{
				ID:    1,
				State: RDMAStateConnected,
			},
		},
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected

	// 建立连接
	conn, _ := manager.Connect("mlx5_0", "192.168.1.100", 1)
	time.Sleep(200 * time.Millisecond)

	// 发送数据
	data := []byte("Hello RDMA!")
	err := manager.Send(conn.ID, data)
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// 接收数据
	recvData, err := manager.Receive(conn.ID)
	if err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	if len(recvData) == 0 {
		t.Error("expected received data")
	}

	// 验证统计
	stats := manager.GetStats()
	if stats.TotalSendBytes != int64(len(data)) {
		t.Errorf("TotalSendBytes = %d, want %d", stats.TotalSendBytes, len(data))
	}
}

func TestRDMANetworkManager_GetDevice(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	device := &RDMADevice{
		Name: "mlx5_0",
		GUID: "0000000000000001",
		Type: RDMATypeRoCE,
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected

	result, err := manager.GetDevice("mlx5_0")
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	if result.Name != "mlx5_0" {
		t.Errorf("device name = %s, want mlx5_0", result.Name)
	}
}

func TestRDMANetworkManager_ListDevices(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册多个设备
	for i := 0; i < 3; i++ {
		device := &RDMADevice{
			Name: "mlx5_" + string(rune('0'+i)),
			GUID: "000000000000000" + string(rune('1'+i)),
			Type: RDMATypeRoCE,
		}
		manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected
	}

	devices := manager.ListDevices()
	if len(devices) != 3 {
		t.Errorf("devices count = %d, want 3", len(devices))
	}
}

func TestRDMANetworkManager_ListConnections(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册设备
	device := &RDMADevice{
		Name:  "mlx5_0",
		GUID:  "0000000000000001",
		Type:  RDMATypeRoCE,
		State: RDMAStateConnected,
		Ports: []*RDMAPort{
			{
				ID:    1,
				State: RDMAStateConnected,
			},
		},
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected

	// 建立多个连接
	for i := 0; i < 3; i++ {
		manager.Connect("mlx5_0", "192.168.1."+string(rune('1'+i)), 1)
	}
	time.Sleep(300 * time.Millisecond)

	connections := manager.ListConnections()
	if len(connections) != 3 {
		t.Errorf("connections count = %d, want 3", len(connections))
	}
}

func TestRDMANetworkManager_GetStats(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	stats := manager.GetStats()

	if stats.TotalDevices != 0 {
		t.Errorf("TotalDevices = %d, want 0", stats.TotalDevices)
	}

	if stats.TotalConnections != 0 {
		t.Errorf("TotalConnections = %d, want 0", stats.TotalConnections)
	}
}

func TestRDMAType_Constants(t *testing.T) {
	if RDMATypeRoCE != 0 {
		t.Errorf("RDMATypeRoCE = %d, want 0", RDMATypeRoCE)
	}

	if RDMATypeInfiniBand != 1 {
		t.Errorf("RDMATypeInfiniBand = %d, want 1", RDMATypeInfiniBand)
	}

	if RDMATypeiWARP != 2 {
		t.Errorf("RDMATypeiWARP = %d, want 2", RDMATypeiWARP)
	}
}

func TestRDMAState_Constants(t *testing.T) {
	if RDMAStateDisconnected != 0 {
		t.Errorf("RDMAStateDisconnected = %d, want 0", RDMAStateDisconnected)
	}

	if RDMAStateConnecting != 1 {
		t.Errorf("RDMAStateConnecting = %d, want 1", RDMAStateConnecting)
	}

	if RDMAStateConnected != 2 {
		t.Errorf("RDMAStateConnected = %d, want 2", RDMAStateConnected)
	}

	if RDMAStateError != 3 {
		t.Errorf("RDMAStateError = %d, want 3", RDMAStateError)
	}
}

func TestDefaultRDMAConfig(t *testing.T) {
	config := DefaultRDMAConfig()

	if config.MTU <= 0 {
		t.Error("expected MTU > 0")
	}

	if config.QueuePairCount <= 0 {
		t.Error("expected QueuePairCount > 0")
	}

	if config.MaxSendWr <= 0 {
		t.Error("expected MaxSendWr > 0")
	}

	if config.MaxRecvWr <= 0 {
		t.Error("expected MaxRecvWr > 0")
	}

	if config.Timeout <= 0 {
		t.Error("expected Timeout > 0")
	}
}

func TestRDMANetworkManager_ConcurrentAccess(t *testing.T) {
	config := DefaultRDMAConfig()
	manager := NewRDMANetworkManager(config)
	manager.Start()
	defer manager.Stop()

	// 注册设备
	device := &RDMADevice{
		Name:  "mlx5_0",
		GUID:  "0000000000000001",
		Type:  RDMATypeRoCE,
		State: RDMAStateConnected,
		Ports: []*RDMAPort{
			{
				ID:    1,
				State: RDMAStateConnected,
			},
		},
	}
	manager.RegisterDevice(device)
	manager.devices["mlx5_0"].State = RDMAStateConnected

	// 并发连接
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			manager.Connect("mlx5_0", "192.168.1."+string(rune('1'+i)), 1)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(500 * time.Millisecond)

	connections := manager.ListConnections()
	if len(connections) != 10 {
		t.Errorf("connections count = %d, want 10", len(connections))
	}
}
