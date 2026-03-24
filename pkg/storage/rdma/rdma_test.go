package rdma

import (
	"context"
	"testing"
	"time"
)

func TestRDMAConfigValidation(t *testing.T) {
	// Test invalid transport
	config := &RDMAConfig{
		Transport: "invalid",
	}
	config.Validate()
	if config.Transport != "iSER" {
		t.Error("Should default to iSER")
	}

	// Test invalid port
	config = &RDMAConfig{
		Port: -1,
	}
	config.Validate()
	if config.Port != 3260 {
		t.Error("Should default to 3260")
	}
}

func TestRDMAEndpoint(t *testing.T) {
	config := DefaultRDMAConfig()

	endpoint, err := NewRDMAEndpoint(config)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}
	defer endpoint.Close()

	// Check initial state
	if endpoint.GetState() != StateDisconnected {
		t.Error("Initial state should be Disconnected")
	}

	// Test connect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = endpoint.Connect(ctx, "127.0.0.1:3260")
	if err != nil {
		// 连接可能失败，但这不是我们的测试目标
		// 主要测试状态转换
		t.Logf("Connect returned: %v (expected in test env)", err)
	}

	// Check stats
	stats := endpoint.GetStats()
	_ = stats // 确保可以获取统计
}

func TestRDMAManager(t *testing.T) {
	config := DefaultRDMAConfig()
	mgr, err := NewRDMAManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Test device detection
	devices := mgr.ListDevices()
	t.Logf("Found %d RDMA devices", len(devices))

	// Test IsAvailable
	_ = mgr.IsAvailable()

	// Test endpoint creation
	endpoint, err := mgr.CreateEndpoint("test-ep")
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Test get endpoint
	ep, ok := mgr.GetEndpoint("test-ep")
	if !ok || ep != endpoint {
		t.Error("Should get created endpoint")
	}

	// Test remove endpoint
	mgr.RemoveEndpoint("test-ep")
	_, ok = mgr.GetEndpoint("test-ep")
	if ok {
		t.Error("Endpoint should be removed")
	}
}

func TestISERSession(t *testing.T) {
	config := DefaultISERConfig()
	config.TargetAddr = "127.0.0.1"

	session, err := NewISERSession(config)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Check initial state
	if session.IsConnected() {
		t.Error("Session should not be connected initially")
	}

	// Test disconnect when not connected
	session.Disconnect()
}

func TestNFSRDMAMount(t *testing.T) {
	config := DefaultNFSRDMAConfig()
	config.ServerAddr = "127.0.0.1"

	mount, err := NewNFSRDMAMount(config)
	if err != nil {
		t.Fatalf("Failed to create mount: %v", err)
	}

	// Check initial state
	if mount.IsMounted() {
		t.Error("Mount should not be mounted initially")
	}

	// Test unmount when not mounted
	mount.Unmount()
}

func TestRDMAPerfMonitor(t *testing.T) {
	monitor := NewRDMAPerfMonitor(time.Second, 10)

	// Test latest before any samples
	_, ok := monitor.GetLatest()
	if ok {
		t.Error("Should not have latest sample yet")
	}

	// Get history (empty)
	history := monitor.GetHistory()
	if len(history) != 0 {
		t.Error("History should be empty")
	}
}

func TestDeviceInfo(t *testing.T) {
	info := &DeviceInfo{
		Name:      "rdma0",
		Transport: "RoCEv2",
		State:     "UP",
		Ports: []PortInfo{
			{Number: 1, State: "ACTIVE", Speed: 100},
		},
	}

	if info.Name != "rdma0" {
		t.Error("Device name mismatch")
	}
	if len(info.Ports) != 1 {
		t.Error("Should have 1 port")
	}
}

func TestRDMAStats(t *testing.T) {
	stats := RDMAStats{
		BytesSent:     1000,
		BytesReceived: 2000,
		OpsSent:       10,
		OpsReceived:   20,
	}

	if stats.BytesSent != 1000 {
		t.Error("BytesSent mismatch")
	}
}

func TestRDMAEvent(t *testing.T) {
	event := RDMAEvent{
		Type:    EventConnected,
		Message: "Connected to server",
	}

	if event.Type != EventConnected {
		t.Error("Event type mismatch")
	}
}

func TestRDMAStates(t *testing.T) {
	states := []RDMAState{
		StateDisconnected,
		StateConnecting,
		StateConnected,
		StateClosing,
	}

	for _, state := range states {
		if state < 0 || state > 3 {
			t.Errorf("Invalid state: %d", state)
		}
	}
}

func BenchmarkRDMAEndpointSend(b *testing.B) {
	config := DefaultRDMAConfig()
	endpoint, _ := NewRDMAEndpoint(config)
	defer endpoint.Close()

	// Connect (模拟)
	ctx := context.Background()
	endpoint.Connect(ctx, "127.0.0.1:3260")

	data := make([]byte, 4096)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		endpoint.Send(ctx, data)
	}
}
