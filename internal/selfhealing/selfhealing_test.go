package selfhealing

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled:           true,
		ScrubInterval:     24,
		IntegrityLevel:    IntegrityLevelStandard,
		AutoRepair:        true,
		ReplicaCount:      3,
		MaxRepairAttempts: 3,
		ChecksumAlgorithm: "sha256",
		AlertThreshold:    10,
	}
	
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if !manager.config.AutoRepair {
		t.Error("Expected AutoRepair to be true")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled:       true,
		ScrubInterval: 1,
		IntegrityLevel: IntegrityLevelBasic,
	}
	
	manager := NewManager(config)
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	manager.Stop()
}

func TestRegisterBlock(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled:           true,
		ChecksumAlgorithm: "sha256",
	}
	
	manager := NewManager(config)
	
	block := &DataBlock{
		Path: "/data/test.txt",
		Size: 1024,
		Checksum: "abc123",
	}
	
	if err := manager.RegisterBlock(block); err != nil {
		t.Fatalf("RegisterBlock failed: %v", err)
	}
	
	if block.ID == "" {
		t.Error("Block ID not generated")
	}
	
	if block.Algorithm != "sha256" {
		t.Errorf("Expected algorithm sha256, got %s", block.Algorithm)
	}
}

func TestAddReplica(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	block := &DataBlock{
		Path:     "/data/test.txt",
		Checksum: "abc123",
	}
	manager.RegisterBlock(block)
	
	replica := &ReplicaInfo{
		ReplicaID: "replica_1",
		Path:      "/backup/test.txt",
		Checksum:  "abc123",
		Healthy:   true,
	}
	
	if err := manager.AddReplica(block.ID, replica); err != nil {
		t.Fatalf("AddReplica failed: %v", err)
	}
	
	if replica.BlockID != block.ID {
		t.Errorf("Expected block ID %s, got %s", block.ID, replica.BlockID)
	}
}

func TestCreateScrubTask(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	task := manager.CreateScrubTask("test-scrub", "/data")
	
	if task == nil {
		t.Fatal("CreateScrubTask returned nil")
	}
	
	if task.ID == "" {
		t.Error("Task ID not generated")
	}
	
	if task.Name != "test-scrub" {
		t.Errorf("Expected name test-scrub, got %s", task.Name)
	}
}

func TestGetScrubTask(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	task := manager.CreateScrubTask("test-scrub", "/data")
	
	got, err := manager.GetScrubTask(task.ID)
	if err != nil {
		t.Fatalf("GetScrubTask failed: %v", err)
	}
	
	if got.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, got.ID)
	}
}

func TestListScrubTasks(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	manager.CreateScrubTask("scrub-1", "/data")
	manager.CreateScrubTask("scrub-2", "/backup")
	
	tasks := manager.ListScrubTasks()
	
	if len(tasks) < 2 {
		t.Errorf("Expected at least 2 tasks, got %d", len(tasks))
	}
}

func TestGetCorruptionRecords(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	records := manager.GetCorruptionRecords()
	
	if records == nil {
		// 初始化后应该是空切片
		records = []CorruptionRecord{}
	}
	
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

func TestGenerateRepairReport(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled:    true,
		AutoRepair: true,
	}
	
	manager := NewManager(config)
	
	report := manager.GenerateRepairReport("test-task")
	
	if report == nil {
		t.Fatal("GenerateRepairReport returned nil")
	}
	
	if report.ScrubTaskID != "test-task" {
		t.Errorf("Expected scrub task ID test-task, got %s", report.ScrubTaskID)
	}
}

func TestGetDashboard(t *testing.T) {
	config := &SelfHealingConfig{
		Enabled:        true,
		IntegrityLevel: IntegrityLevelStandard,
		AutoRepair:     true,
	}
	
	manager := NewManager(config)
	
	dashboard := manager.GetDashboard()
	
	if dashboard["total_blocks"] != 0 {
		t.Errorf("Expected 0 total_blocks, got %v", dashboard["total_blocks"])
	}
	
	if dashboard["integrity_level"] != IntegrityLevelStandard {
		t.Errorf("Expected integrity_level standard, got %v", dashboard["integrity_level"])
	}
}

func TestIntegrityLevels(t *testing.T) {
	levels := []IntegrityLevel{
		IntegrityLevelBasic,
		IntegrityLevelStandard,
		IntegrityLevelStrict,
	}
	
	for _, l := range levels {
		if string(l) == "" {
			t.Errorf("Empty integrity level: %v", l)
		}
	}
}

func TestScrubStatuses(t *testing.T) {
	statuses := []ScrubStatus{
		ScrubStatusPending,
		ScrubStatusRunning,
		ScrubStatusCompleted,
		ScrubStatusFailed,
	}
	
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Empty scrub status: %v", s)
		}
	}
}

func TestRepairStatuses(t *testing.T) {
	statuses := []RepairStatus{
		RepairStatusPending,
		RepairStatusRunning,
		RepairStatusSuccess,
		RepairStatusFailed,
	}
	
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Empty repair status: %v", s)
		}
	}
}
