package configdrift

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()

	manager := NewManager(tmpDir)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.storageDir != tmpDir {
		t.Errorf("Expected storageDir %s, got %s", tmpDir, manager.storageDir)
	}

	if manager.snapshots == nil {
		t.Error("snapshots map not initialized")
	}
}

func TestNewManagerDefaultDir(t *testing.T) {
	manager := NewManager("")
	if manager.storageDir != "/tmp/configdrift" {
		t.Errorf("Expected default storageDir /tmp/configdrift, got %s", manager.storageDir)
	}
}

func TestTakeSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	snapshot, err := manager.TakeSnapshot("test-label")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snapshot.ID == "" {
		t.Error("Snapshot ID is empty")
	}

	if snapshot.Label != "test-label" {
		t.Errorf("Expected label 'test-label', got '%s'", snapshot.Label)
	}

	if snapshot.Hash == "" {
		t.Error("Snapshot hash is empty")
	}

	if snapshot.Config == nil {
		t.Error("Snapshot config is nil")
	}

	if snapshot.Timestamp.IsZero() {
		t.Error("Snapshot timestamp is zero")
	}
}

func TestGetSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	snapshot, _ := manager.TakeSnapshot("test")

	// 获取存在的快照
	retrieved, err := manager.GetSnapshot(snapshot.ID)
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}

	if retrieved.ID != snapshot.ID {
		t.Errorf("Expected ID %s, got %s", snapshot.ID, retrieved.ID)
	}

	// 获取不存在的快照
	_, err = manager.GetSnapshot("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func TestListSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 创建多个快照
	manager.TakeSnapshot("snap1")
	time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	manager.TakeSnapshot("snap2")
	time.Sleep(10 * time.Millisecond)
	manager.TakeSnapshot("snap3")

	snapshots, err := manager.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}

	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}

	// 验证按时间排序（最新的在前）
	if snapshots[0].Label != "snap3" {
		t.Errorf("Expected first snapshot to be 'snap3', got '%s'", snapshots[0].Label)
	}
}

func TestSetBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	snapshot, _ := manager.TakeSnapshot("baseline")

	// 设置基线
	err := manager.SetBaseline(snapshot.ID)
	if err != nil {
		t.Fatalf("SetBaseline failed: %v", err)
	}

	// 设置不存在的快照为基线
	err = manager.SetBaseline("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func TestGetBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 未设置基线时获取
	_, err := manager.GetBaseline()
	if err == nil {
		t.Error("Expected error when no baseline is set")
	}

	// 设置并获取基线
	snapshot, _ := manager.TakeSnapshot("baseline")
	manager.SetBaseline(snapshot.ID)

	baseline, err := manager.GetBaseline()
	if err != nil {
		t.Fatalf("GetBaseline failed: %v", err)
	}

	if baseline.ID != snapshot.ID {
		t.Errorf("Expected baseline ID %s, got %s", snapshot.ID, baseline.ID)
	}
}

func TestCompareSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 创建第一个快照
	snap1, _ := manager.TakeSnapshot("baseline")

	// 修改配置并创建第二个快照
	// 注意：由于 readSystemConfig 是模拟的，我们需要手动修改
	snap2 := &ConfigSnapshot{
		ID:        "snap2",
		Timestamp: time.Now(),
		Config: map[string]interface{}{
			"hostname": "nas-server-modified",
			"network": map[string]interface{}{
				"interface": "eth0",
				"ip":        "192.168.1.200",
				"gateway":   "192.168.1.1",
			},
		},
		Label: "modified",
	}
	snap2.Hash = hashConfig(snap2.Config)
	manager.snapshots[snap2.ID] = snap2

	// 比较快照
	report, err := manager.CompareSnapshots(snap1.ID, snap2.ID)
	if err != nil {
		t.Fatalf("CompareSnapshots failed: %v", err)
	}

	if report.BaselineID != snap1.ID {
		t.Errorf("Expected baseline ID %s, got %s", snap1.ID, report.BaselineID)
	}

	if report.CurrentID != snap2.ID {
		t.Errorf("Expected current ID %s, got %s", snap2.ID, report.CurrentID)
	}

	if len(report.Changes) == 0 {
		t.Error("Expected changes but got none")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("Report generated time is zero")
	}
}

func TestCompareSnapshotsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	_, err := manager.CompareSnapshots("non-existent1", "non-existent2")
	if err == nil {
		t.Error("Expected error for non-existent snapshots")
	}
}

func TestDetectDrift(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 未设置基线时检测
	_, err := manager.DetectDrift()
	if err == nil {
		t.Error("Expected error when no baseline is set")
	}

	// 设置基线后检测
	manager.TakeSnapshot("baseline")
	snapshots, _ := manager.ListSnapshots()
	manager.SetBaseline(snapshots[0].ID)

	report, err := manager.DetectDrift()
	if err != nil {
		t.Fatalf("DetectDrift failed: %v", err)
	}

	if report == nil {
		t.Fatal("Report is nil")
	}
}

func TestDeepCompare(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	baseline := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]interface{}{
			"a": "1",
			"b": "2",
		},
	}

	current := map[string]interface{}{
		"key1": "value1-modified",
		"key3": "value3",
		"nested": map[string]interface{}{
			"a": "1",
			"c": "3",
		},
	}

	changes := manager.deepCompare(baseline, current, "")

	// 应该有: key1 修改, key2 删除, key3 新增, nested.b 删除, nested.c 新增
	if len(changes) < 4 {
		t.Errorf("Expected at least 4 changes, got %d", len(changes))
	}

	// 验证变更类型
	changeTypes := make(map[string]ChangeType)
	for _, change := range changes {
		changeTypes[change.Path] = change.Type
	}

	if changeTypes["key1"] != ChangeModify {
		t.Errorf("Expected key1 to be Modify, got %s", changeTypes["key1"])
	}

	if changeTypes["key2"] != ChangeDelete {
		t.Errorf("Expected key2 to be Delete, got %s", changeTypes["key2"])
	}

	if changeTypes["key3"] != ChangeAdd {
		t.Errorf("Expected key3 to be Add, got %s", changeTypes["key3"])
	}
}

func TestCalculateDriftScore(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	baseline := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// 无变更
	changes := []ConfigChange{}
	score := manager.calculateDriftScore(changes, baseline)
	if score != 0 {
		t.Errorf("Expected score 0, got %f", score)
	}

	// 有变更
	changes = []ConfigChange{
		{Path: "key1", Type: ChangeModify},
		{Path: "key2", Type: ChangeDelete},
	}
	score = manager.calculateDriftScore(changes, baseline)
	if score <= 0 || score > 100 {
		t.Errorf("Expected score between 0 and 100, got %f", score)
	}
}

func TestDetermineSeverity(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 测试低严重程度
	changes := []ConfigChange{
		{Path: "some.path", Type: ChangeModify},
	}
	severity := manager.determineSeverity(5, changes)
	if severity != SeverityLow {
		t.Errorf("Expected Low severity, got %s", severity.String())
	}

	// 测试中等严重程度
	severity = manager.determineSeverity(15, changes)
	if severity != SeverityMedium {
		t.Errorf("Expected Medium severity, got %s", severity.String())
	}

	// 测试高严重程度
	severity = manager.determineSeverity(35, changes)
	if severity != SeverityHigh {
		t.Errorf("Expected High severity, got %s", severity.String())
	}

	// 测试严重程度（关键配置变更）
	changes = []ConfigChange{
		{Path: "hostname", Type: ChangeModify},
	}
	severity = manager.determineSeverity(10, changes)
	if severity != SeverityCritical {
		t.Errorf("Expected Critical severity for hostname change, got %s", severity.String())
	}
}

func TestGetDriftHistory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 初始历史应为空
	history, _ := manager.GetDriftHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d items", len(history))
	}

	// 创建快照并比较
	snap1, _ := manager.TakeSnapshot("snap1")
	snap2 := &ConfigSnapshot{
		ID:        "snap2",
		Timestamp: time.Now(),
		Config: map[string]interface{}{
			"key": "modified",
		},
	}
	snap2.Hash = hashConfig(snap2.Config)
	manager.snapshots[snap2.ID] = snap2

	manager.CompareSnapshots(snap1.ID, snap2.ID)

	history, _ = manager.GetDriftHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 history item, got %d", len(history))
	}
}

func TestExportReport(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	report := DriftReport{
		BaselineID: "baseline",
		CurrentID:  "current",
		Changes: []ConfigChange{
			{Path: "key1", OldValue: "old", NewValue: "new", Type: ChangeModify},
		},
		DriftScore:  25.0,
		Severity:    SeverityMedium,
		GeneratedAt: time.Now(),
	}

	data, err := manager.ExportReport(report)
	if err != nil {
		t.Fatalf("ExportReport failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Exported report is empty")
	}

	// 验证是有效的 JSON
	var parsed DriftReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Exported report is not valid JSON: %v", err)
	}
}

func TestAutoRollback(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	snapshot, _ := manager.TakeSnapshot("test")

	// 回滚到存在的快照
	err := manager.AutoRollback(snapshot.ID)
	if err != nil {
		t.Fatalf("AutoRollback failed: %v", err)
	}

	// 回滚到不存在的快照
	err = manager.AutoRollback("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent snapshot")
	}
}

func TestDriftSeverityString(t *testing.T) {
	tests := []struct {
		severity DriftSeverity
		expected string
	}{
		{SeverityLow, "Low"},
		{SeverityMedium, "Medium"},
		{SeverityHigh, "High"},
		{SeverityCritical, "Critical"},
		{DriftSeverity(99), "Unknown"},
	}

	for _, test := range tests {
		if test.severity.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.severity.String())
		}
	}
}

func TestHashConfig(t *testing.T) {
	config1 := map[string]interface{}{
		"key": "value",
	}
	config2 := map[string]interface{}{
		"key": "value",
	}
	config3 := map[string]interface{}{
		"key": "different",
	}

	hash1 := hashConfig(config1)
	hash2 := hashConfig(config2)
	hash3 := hashConfig(config3)

	if hash1 != hash2 {
		t.Error("Same config should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different config should produce different hash")
	}
}

func TestCountConfigKeys(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	config := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nested1": "value1",
			"nested2": "value2",
		},
		"key3": "value3",
	}

	count := manager.countConfigKeys(config)
	if count != 5 { // key1, key2, nested1, nested2, key3
		t.Errorf("Expected 5 keys, got %d", count)
	}
}

func TestSnapshotPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建管理器并添加快照
	manager1 := NewManager(tmpDir)
	snapshot, _ := manager1.TakeSnapshot("persistent")

	// 创建新的管理器实例
	manager2 := NewManager(tmpDir)

	// 验证快照被加载
	snapshots, _ := manager2.ListSnapshots()
	if len(snapshots) != 1 {
		t.Errorf("Expected 1 loaded snapshot, got %d", len(snapshots))
	}

	if snapshots[0].ID != snapshot.ID {
		t.Errorf("Expected snapshot ID %s, got %s", snapshot.ID, snapshots[0].ID)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(time.Millisecond)
	id2 := generateID()

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func BenchmarkTakeSnapshot(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.TakeSnapshot("benchmark")
	}
}

func BenchmarkCompareSnapshots(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir)

	snap1, _ := manager.TakeSnapshot("snap1")
	snap2 := &ConfigSnapshot{
		ID:        "snap2",
		Timestamp: time.Now(),
		Config: map[string]interface{}{
			"key": "modified",
		},
	}
	snap2.Hash = hashConfig(snap2.Config)
	manager.snapshots[snap2.ID] = snap2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CompareSnapshots(snap1.ID, snap2.ID)
	}
}
