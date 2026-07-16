package nvmetempool

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.maxFailovers != 100 {
		t.Errorf("expected maxFailovers 100, got %d", m.maxFailovers)
	}
}

func TestAddTarget(t *testing.T) {
	m := NewManager()

	target := &NvmeTarget{
		ID:        "target-1",
		Name:      "Test Target",
		Address:   "192.168.1.100",
		Port:      4420,
		Transport: TransportTCP,
		Subsystem: "nqn.2024-01.com.example:test",
	}

	err := m.AddTarget(target)
	if err != nil {
		t.Fatalf("add target failed: %v", err)
	}

	// 重复添加
	err = m.AddTarget(target)
	if err == nil {
		t.Error("expected error for duplicate target")
	}

	// 空ID
	err = m.AddTarget(&NvmeTarget{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestRemoveTarget(t *testing.T) {
	m := NewManager()

	target := &NvmeTarget{
		ID:      "target-1",
		Name:    "Test Target",
		Address: "192.168.1.100",
		Port:    4420,
	}

	m.AddTarget(target)

	// 移除
	err := m.RemoveTarget("target-1")
	if err != nil {
		t.Fatalf("remove target failed: %v", err)
	}

	// 不存在的
	err = m.RemoveTarget("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent target")
	}
}

func TestGetTarget(t *testing.T) {
	m := NewManager()

	target := &NvmeTarget{
		ID:      "target-1",
		Name:    "Test Target",
		Address: "192.168.1.100",
		Port:    4420,
	}

	m.AddTarget(target)

	got, err := m.GetTarget("target-1")
	if err != nil {
		t.Fatalf("get target failed: %v", err)
	}
	if got.Name != "Test Target" {
		t.Errorf("expected Test Target, got %s", got.Name)
	}

	_, err = m.GetTarget("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent target")
	}
}

func TestListTargets(t *testing.T) {
	m := NewManager()

	targets := m.ListTargets()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddTarget(&NvmeTarget{ID: "t2", Name: "T2", Address: "2.2.2.2", Port: 4420})

	targets = m.ListTargets()
	if len(targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(targets))
	}
}

func TestAddDevice(t *testing.T) {
	m := NewManager()

	// 先添加目标端
	m.AddTarget(&NvmeTarget{ID: "target-1", Name: "T1", Address: "1.1.1.1", Port: 4420})

	device := &NvmeDevice{
		ID:        "device-1",
		Model:     "Samsung 980 PRO",
		Serial:    "S123456789",
		Namespace: "ns1",
		Capacity:  1024 * 1024 * 1024 * 1024, // 1TB
		TargetID:  "target-1",
	}

	err := m.AddDevice(device)
	if err != nil {
		t.Fatalf("add device failed: %v", err)
	}

	// 重复添加
	err = m.AddDevice(device)
	if err == nil {
		t.Error("expected error for duplicate device")
	}

	// 不存在的目标端
	err = m.AddDevice(&NvmeDevice{ID: "d2", Model: "M2", Capacity: 100, TargetID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent target")
	}
}

func TestRemoveDevice(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "target-1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "device-1", Model: "M1", Capacity: 100, TargetID: "target-1"})

	err := m.RemoveDevice("device-1")
	if err != nil {
		t.Fatalf("remove device failed: %v", err)
	}

	err = m.RemoveDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestGetDevice(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "target-1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "device-1", Model: "M1", Capacity: 100, TargetID: "target-1"})

	device, err := m.GetDevice("device-1")
	if err != nil {
		t.Fatalf("get device failed: %v", err)
	}
	if device.Model != "M1" {
		t.Errorf("expected M1, got %s", device.Model)
	}

	_, err = m.GetDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestListDevices(t *testing.T) {
	m := NewManager()

	devices := m.ListDevices()
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})
	m.AddDevice(&NvmeDevice{ID: "d2", Model: "M2", Capacity: 200, TargetID: "t1"})

	devices = m.ListDevices()
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestCreatePool(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "target-1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "device-1", Model: "M1", Capacity: 1024 * 1024 * 1024 * 1024, TargetID: "target-1"})
	m.AddDevice(&NvmeDevice{ID: "device-2", Model: "M2", Capacity: 1024 * 1024 * 1024 * 1024, TargetID: "target-1"})

	pool := &NvmePool{
		ID:      "pool-1",
		Name:    "Test Pool",
		Devices: []string{"device-1", "device-2"},
	}

	err := m.CreatePool(pool)
	if err != nil {
		t.Fatalf("create pool failed: %v", err)
	}

	// 重复创建
	err = m.CreatePool(pool)
	if err == nil {
		t.Error("expected error for duplicate pool")
	}

	// 不存在的设备
	err = m.CreatePool(&NvmePool{ID: "pool-2", Name: "P2", Devices: []string{"nonexistent"}})
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestDeletePool(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})
	m.CreatePool(&NvmePool{ID: "pool-1", Name: "P1", Devices: []string{"d1"}})

	err := m.DeletePool("pool-1")
	if err != nil {
		t.Fatalf("delete pool failed: %v", err)
	}

	err = m.DeletePool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestGetPool(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})
	m.CreatePool(&NvmePool{ID: "pool-1", Name: "P1", Devices: []string{"d1"}})

	pool, err := m.GetPool("pool-1")
	if err != nil {
		t.Fatalf("get pool failed: %v", err)
	}
	if pool.Name != "P1" {
		t.Errorf("expected P1, got %s", pool.Name)
	}

	_, err = m.GetPool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestListPools(t *testing.T) {
	m := NewManager()

	pools := m.ListPools()
	if len(pools) != 0 {
		t.Errorf("expected 0 pools, got %d", len(pools))
	}

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})
	m.AddDevice(&NvmeDevice{ID: "d2", Model: "M2", Capacity: 200, TargetID: "t1"})

	m.CreatePool(&NvmePool{ID: "p1", Name: "P1", Devices: []string{"d1"}})
	m.CreatePool(&NvmePool{ID: "p2", Name: "P2", Devices: []string{"d2"}})

	pools = m.ListPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}

func TestGetPoolPerformance(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})
	m.CreatePool(&NvmePool{ID: "pool-1", Name: "P1", Devices: []string{"d1"}})

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	perf, err := m.GetPoolPerformance("pool-1")
	if err != nil {
		t.Fatalf("get pool performance failed: %v", err)
	}
	if perf.PoolID != "pool-1" {
		t.Errorf("expected pool-1, got %s", perf.PoolID)
	}
	if perf.Metrics == nil {
		t.Fatal("expected non-nil metrics")
	}

	_, err = m.GetPoolPerformance("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestGetFailoverEvents(t *testing.T) {
	m := NewManager()

	events := m.GetFailoverEvents()
	if events == nil {
		t.Error("expected empty events, got nil")
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestDiscoverTargets(t *testing.T) {
	m := NewManager()

	targets, err := m.DiscoverTargets("192.168.1.100", TransportTCP)
	if err != nil {
		t.Fatalf("discover targets failed: %v", err)
	}
	if len(targets) == 0 {
		t.Error("expected at least 1 target")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	m.Stop()

	// 重复停止不应 panic
	m.Stop()
}

func TestUpdateDeviceStatus(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 100, TargetID: "t1"})

	err := m.UpdateDeviceStatus("d1", DeviceStatusFault)
	if err != nil {
		t.Fatalf("update device status failed: %v", err)
	}

	device, _ := m.GetDevice("d1")
	if device.Status != DeviceStatusFault {
		t.Errorf("expected fault status, got %s", device.Status)
	}

	err = m.UpdateDeviceStatus("nonexistent", DeviceStatusOnline)
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestUpdatePoolUsage(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 1024 * 1024 * 1024 * 1024, TargetID: "t1"})
	m.CreatePool(&NvmePool{ID: "pool-1", Name: "P1", Devices: []string{"d1"}})

	err := m.UpdatePoolUsage("pool-1", 512*1024*1024*1024) // 512GB
	if err != nil {
		t.Fatalf("update pool usage failed: %v", err)
	}

	pool, _ := m.GetPool("pool-1")
	if pool.UsedSpace != 512*1024*1024*1024 {
		t.Errorf("expected 512GB used, got %d", pool.UsedSpace)
	}

	err = m.UpdatePoolUsage("nonexistent", 100)
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestGetPoolUsage(t *testing.T) {
	m := NewManager()

	m.AddTarget(&NvmeTarget{ID: "t1", Name: "T1", Address: "1.1.1.1", Port: 4420})
	m.AddDevice(&NvmeDevice{ID: "d1", Model: "M1", Capacity: 1024 * 1024 * 1024 * 1024, TargetID: "t1"})
	m.CreatePool(&NvmePool{ID: "pool-1", Name: "P1", Devices: []string{"d1"}})

	m.UpdatePoolUsage("pool-1", 512*1024*1024*1024)

	usage, err := m.GetPoolUsage("pool-1")
	if err != nil {
		t.Fatalf("get pool usage failed: %v", err)
	}
	if usage != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", usage)
	}

	_, err = m.GetPoolUsage("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}
