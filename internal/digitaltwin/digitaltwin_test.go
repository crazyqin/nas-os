package digitaltwin

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:           true,
		MaxSnapshots:      10,
		MaxTwins:          5,
		SnapshotRetention: 30,
		AutoSnapshot:      true,
		SnapshotInterval:  24,
		Dr演练Enabled:     true,
	}
	
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if !manager.config.AutoSnapshot {
		t.Error("Expected AutoSnapshot to be true")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:       true,
		MaxSnapshots:  10,
		MaxTwins:      5,
		AutoSnapshot:  false,
	}
	
	manager := NewManager(config)
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	manager.Stop()
}

func TestCreateSnapshot(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	snapshot, err := manager.CreateSnapshot("test-snapshot", "Test snapshot", SnapshotTypeConfig, nil)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	if snapshot.ID == "" {
		t.Error("Snapshot ID not generated")
	}
	
	if snapshot.Name != "test-snapshot" {
		t.Errorf("Expected name test-snapshot, got %s", snapshot.Name)
	}
	
	if snapshot.Version != "2.527.0" {
		t.Errorf("Expected version 2.527.0, got %s", snapshot.Version)
	}
}

func TestGetSnapshot(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	got, err := manager.GetSnapshot(snapshot.ID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	
	if got.ID != snapshot.ID {
		t.Errorf("Expected snapshot ID %s, got %s", snapshot.ID, got.ID)
	}
}

func TestListSnapshots(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	manager.CreateSnapshot("snap-1", "Snapshot 1", SnapshotTypeConfig, nil)
	manager.CreateSnapshot("snap-2", "Snapshot 2", SnapshotTypeFull, nil)
	
	snapshots := manager.ListSnapshots()
	
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	if err := manager.DeleteSnapshot(snapshot.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	
	_, err := manager.GetSnapshot(snapshot.ID)
	if err == nil {
		t.Error("Expected error for deleted snapshot")
	}
}

func TestCreateVirtualTwin(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
		MaxTwins:     5,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	resources := TwinResources{
		CPU:     2,
		Memory:  4096,
		Storage: 100 * 1024 * 1024 * 1024,
	}
	
	twin, err := manager.CreateVirtualTwin("test-twin", "Test twin", snapshot.ID, resources)
	if err != nil {
		t.Fatalf("CreateVirtualTwin failed: %v", err)
	}
	
	if twin.ID == "" {
		t.Error("Twin ID not generated")
	}
	
	if twin.Status != TwinStatusCreating {
		t.Errorf("Expected status creating, got %s", twin.Status)
	}
}

func TestGetVirtualTwin(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
		MaxTwins:     5,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	twin, _ := manager.CreateVirtualTwin("test-twin", "Test", snapshot.ID, TwinResources{CPU: 2, Memory: 4096})
	
	got, err := manager.GetVirtualTwin(twin.ID)
	if err != nil {
		t.Fatalf("GetVirtualTwin failed: %v", err)
	}
	
	if got.ID != twin.ID {
		t.Errorf("Expected twin ID %s, got %s", twin.ID, got.ID)
	}
}

func TestListVirtualTwins(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
		MaxTwins:     5,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	manager.CreateVirtualTwin("twin-1", "Twin 1", snapshot.ID, TwinResources{CPU: 2, Memory: 4096})
	manager.CreateVirtualTwin("twin-2", "Twin 2", snapshot.ID, TwinResources{CPU: 4, Memory: 8192})
	
	twins := manager.ListVirtualTwins()
	
	if len(twins) != 2 {
		t.Errorf("Expected 2 twins, got %d", len(twins))
	}
}

func TestStartStopVirtualTwin(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
		MaxTwins:     5,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	twin, _ := manager.CreateVirtualTwin("test-twin", "Test", snapshot.ID, TwinResources{CPU: 2, Memory: 4096})
	
	// 等待创建完成
	time.Sleep(3 * time.Second)
	
	if err := manager.StartVirtualTwin(twin.ID); err != nil {
		t.Fatalf("StartVirtualTwin failed: %v", err)
	}
	
	if twin.Status != TwinStatusRunning {
		t.Errorf("Expected status running, got %s", twin.Status)
	}
	
	if err := manager.StopVirtualTwin(twin.ID); err != nil {
		t.Fatalf("StopVirtualTwin failed: %v", err)
	}
	
	if twin.Status != TwinStatusStopped {
		t.Errorf("Expected status stopped, got %s", twin.Status)
	}
}

func TestDestroyVirtualTwin(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
		MaxTwins:     5,
	}
	
	manager := NewManager(config)
	
	snapshot, _ := manager.CreateSnapshot("test-snapshot", "Test", SnapshotTypeConfig, nil)
	
	twin, _ := manager.CreateVirtualTwin("test-twin", "Test", snapshot.ID, TwinResources{CPU: 2, Memory: 4096})
	
	if err := manager.DestroyVirtualTwin(twin.ID); err != nil {
		t.Fatalf("DestroyVirtualTwin failed: %v", err)
	}
	
	_, err := manager.GetVirtualTwin(twin.ID)
	if err == nil {
		t.Error("Expected error for destroyed twin")
	}
}

func TestCompareSnapshots(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	data1 := map[string]interface{}{
		"hostname": "nas-old",
		"version":  "2.526.0",
	}
	
	data2 := map[string]interface{}{
		"hostname": "nas-new",
		"version":  "2.527.0",
		"new_key":  "new_value",
	}
	
	snapshot1, _ := manager.CreateSnapshot("snap-1", "Snapshot 1", SnapshotTypeConfig, data1)
	snapshot2, _ := manager.CreateSnapshot("snap-2", "Snapshot 2", SnapshotTypeConfig, data2)
	
	result, err := manager.CompareSnapshots(snapshot1.ID, snapshot2.ID)
	if err != nil {
		t.Fatalf("CompareSnapshots failed: %v", err)
	}
	
	if result.TotalDiffs == 0 {
		t.Error("Expected differences, got 0")
	}
}

func TestGenerateTopology(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:      true,
		MaxSnapshots: 10,
	}
	
	manager := NewManager(config)
	
	topology := manager.GenerateTopology()
	
	if topology == nil {
		t.Fatal("GenerateTopology returned nil")
	}
	
	if len(topology.Nodes) == 0 {
		t.Error("Expected nodes in topology")
	}
}

func TestGetDashboard(t *testing.T) {
	config := &DigitalTwinConfig{
		Enabled:       true,
		MaxSnapshots:  10,
		MaxTwins:      5,
		AutoSnapshot:  true,
		Dr演练Enabled: true,
	}
	
	manager := NewManager(config)
	
	dashboard := manager.GetDashboard()
	
	if dashboard["snapshots_count"] != 0 {
		t.Errorf("Expected 0 snapshots_count, got %v", dashboard["snapshots_count"])
	}
	
	if dashboard["auto_snapshot"] != true {
		t.Error("Expected auto_snapshot to be true")
	}
}

func TestSnapshotTypes(t *testing.T) {
	types := []SnapshotType{
		SnapshotTypeFull,
		SnapshotTypeConfig,
		SnapshotTypeStorage,
		SnapshotTypeNetwork,
		SnapshotTypeService,
	}
	
	for _, st := range types {
		if string(st) == "" {
			t.Errorf("Empty snapshot type: %v", st)
		}
	}
}

func TestTwinStatuses(t *testing.T) {
	statuses := []TwinStatus{
		TwinStatusCreating,
		TwinStatusReady,
		TwinStatusRunning,
		TwinStatusStopped,
		TwinStatusError,
		TwinStatusDestroyed,
	}
	
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Empty twin status: %v", s)
		}
	}
}
