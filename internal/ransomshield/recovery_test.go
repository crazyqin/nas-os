package ransomshield

import (
	"os"
	"testing"
	"time"
)

func TestNewRecoveryManager(t *testing.T) {
	config := RecoveryConfig{
		MaxTotalPoints:     100,
		MaxDiskUsageGB:     10,
		CleanupInterval:    1 * time.Hour,
		VerifyOnCreate:     true,
		CompressionEnabled: false,
	}

	rm := NewRecoveryManager(config)
	if rm == nil {
		t.Fatal("NewRecoveryManager returned nil")
	}

	if len(rm.policies) == 0 {
		t.Error("expected default policies")
	}
}

func TestRecoveryManager_CreateRecoveryPoint(t *testing.T) {
	tmpDir := t.TempDir()
	config := RecoveryConfig{MaxTotalPoints: 50}
	rm := NewRecoveryManager(config)

	// 设置快照回调（模拟）
	rm.SetSnapshotFunc(func(path string) (string, error) {
		snapshotDir := tmpDir + "/snapshots/snap1"
		os.MkdirAll(snapshotDir, 0755)
		return snapshotDir, nil
	})

	// 创建测试目录
	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/test.txt", []byte("test data"), 0644)

	rp, err := rm.CreateRecoveryPoint("test-snapshot", testDir, "测试快照", RecoveryTypeManual, ThreatLevelNone)
	if err != nil {
		t.Fatalf("CreateRecoveryPoint failed: %v", err)
	}

	if rp.Name != "test-snapshot" {
		t.Errorf("expected name 'test-snapshot', got '%s'", rp.Name)
	}

	if rp.Status != RecoveryStatusReady {
		t.Errorf("expected status ready, got %s", rp.Status)
	}

	if rp.FilesCount < 1 {
		t.Error("expected FilesCount >= 1")
	}
}

func TestRecoveryManager_CreateAutoRecoveryPoint(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRecoveryManager(RecoveryConfig{})

	rm.SetSnapshotFunc(func(path string) (string, error) {
		return tmpDir + "/auto-snap", nil
	})

	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/important.txt", []byte("data"), 0644)

	rp, err := rm.CreateAutoRecoveryPoint(testDir+"/important.txt", "threat-123", ThreatLevelCritical)
	if err != nil {
		t.Fatalf("CreateAutoRecoveryPoint failed: %v", err)
	}

	if rp.Type != RecoveryTypeAuto {
		t.Errorf("expected type auto, got %s", rp.Type)
	}

	if rp.ThreatLevel != ThreatLevelCritical {
		t.Errorf("expected critical threat level, got %s", rp.ThreatLevel.String())
	}
}

func TestRecoveryManager_ListRecoveryPoints(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRecoveryManager(RecoveryConfig{})

	rm.SetSnapshotFunc(func(path string) (string, error) {
		return tmpDir + "/snap", nil
	})

	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/a.txt", []byte("a"), 0644)

	rm.CreateRecoveryPoint("snap1", testDir, "", RecoveryTypeManual, ThreatLevelNone)
	rm.CreateRecoveryPoint("snap2", testDir, "", RecoveryTypeAuto, ThreatLevelHigh)

	all := rm.ListRecoveryPoints("", "", 0)
	if len(all) != 2 {
		t.Errorf("expected 2 recovery points, got %d", len(all))
	}

	autoOnly := rm.ListRecoveryPoints("", RecoveryTypeAuto, 0)
	if len(autoOnly) != 1 {
		t.Errorf("expected 1 auto recovery point, got %d", len(autoOnly))
	}
}

func TestRecoveryManager_Restore(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRecoveryManager(RecoveryConfig{})

	snapDir := tmpDir + "/snap"
	os.MkdirAll(snapDir, 0755)
	os.WriteFile(snapDir+"/recovered.txt", []byte("recovered"), 0644)

	rm.SetSnapshotFunc(func(path string) (string, error) {
		return snapDir, nil
	})

	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)

	rp, _ := rm.CreateRecoveryPoint("test", testDir, "", RecoveryTypeManual, ThreatLevelNone)

	// 试运行
	result, err := rm.Restore(rp.ID, testDir, true)
	if err != nil {
		t.Fatalf("DryRun restore failed: %v", err)
	}
	if !result.DryRun {
		t.Error("expected DryRun to be true")
	}
	if result.Status != "dry_run_ok" {
		t.Errorf("expected dry_run_ok status, got %s", result.Status)
	}
}

func TestRecoveryManager_RestoreNonExistent(t *testing.T) {
	rm := NewRecoveryManager(RecoveryConfig{})
	rm.SetSnapshotFunc(func(path string) (string, error) {
		return "/tmp/snap", nil
	})

	_, err := rm.Restore("nonexistent", "/tmp/target", false)
	if err == nil {
		t.Error("expected error for non-existent recovery point")
	}
}

func TestRecoveryManager_DeleteRecoveryPoint(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRecoveryManager(RecoveryConfig{})

	rm.SetSnapshotFunc(func(path string) (string, error) {
		return tmpDir + "/snap", nil
	})
	rm.SetDeleteFunc(func(snapshotID string) error {
		return nil
	})

	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/a.txt", []byte("a"), 0644)

	rp, _ := rm.CreateRecoveryPoint("to-delete", testDir, "", RecoveryTypeManual, ThreatLevelNone)

	if err := rm.DeleteRecoveryPoint(rp.ID); err != nil {
		t.Fatalf("DeleteRecoveryPoint failed: %v", err)
	}

	_, found := rm.GetRecoveryPoint(rp.ID)
	if found {
		t.Error("recovery point should be deleted")
	}
}

func TestRecoveryManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRecoveryManager(RecoveryConfig{})

	rm.SetSnapshotFunc(func(path string) (string, error) {
		return tmpDir + "/snap", nil
	})

	testDir := tmpDir + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/a.txt", []byte("a"), 0644)

	rm.CreateRecoveryPoint("s1", testDir, "", RecoveryTypeManual, ThreatLevelNone)
	rm.CreateRecoveryPoint("s2", testDir, "", RecoveryTypeAuto, ThreatLevelHigh)

	stats := rm.GetStats()
	if stats.TotalCreated != 2 {
		t.Errorf("expected TotalCreated 2, got %d", stats.TotalCreated)
	}

	if stats.ActivePoints != 2 {
		t.Errorf("expected ActivePoints 2, got %d", stats.ActivePoints)
	}
}

func TestRecoveryManager_GetPolicies(t *testing.T) {
	rm := NewRecoveryManager(RecoveryConfig{})

	policies := rm.GetPolicies()
	if len(policies) == 0 {
		t.Error("expected default policies")
	}

	// 检查策略包含 hourly, daily, pre-threat
	policyIDs := make(map[string]bool)
	for _, p := range policies {
		policyIDs[p.ID] = true
	}

	if !policyIDs["hourly"] {
		t.Error("expected hourly policy")
	}
	if !policyIDs["daily"] {
		t.Error("expected daily policy")
	}
	if !policyIDs["pre-threat"] {
		t.Error("expected pre-threat policy")
	}
}

func TestRecoveryManager_AddPolicy(t *testing.T) {
	rm := NewRecoveryManager(RecoveryConfig{})

	initialCount := len(rm.GetPolicies())

	customPolicy := SnapshotPolicy{
		ID:       "custom",
		Name:     "Custom Policy",
		Enabled:  true,
		Paths:    []string{"/custom"},
		Interval: 30 * time.Minute,
		Type:     RecoveryTypeScheduled,
	}
	rm.AddPolicy(customPolicy)

	newCount := len(rm.GetPolicies())
	if newCount != initialCount+1 {
		t.Errorf("expected %d policies, got %d", initialCount+1, newCount)
	}
}

func TestRecoveryManager_SetCallbacks(t *testing.T) {
	rm := NewRecoveryManager(RecoveryConfig{})

	snapshotCalled := false
	restoreCalled := false
	deleteCalled := false

	rm.SetSnapshotFunc(func(path string) (string, error) {
		snapshotCalled = true
		return "/tmp/snap", nil
	})
	rm.SetRestoreFunc(func(snapshotID, targetPath string) error {
		restoreCalled = true
		return nil
	})
	rm.SetDeleteFunc(func(snapshotID string) error {
		deleteCalled = true
		return nil
	})

	testDir := t.TempDir() + "/data"
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testDir+"/a.txt", []byte("a"), 0644)

	rp, _ := rm.CreateRecoveryPoint("test", testDir, "", RecoveryTypeManual, ThreatLevelNone)
	if !snapshotCalled {
		t.Error("snapshot callback not called")
	}

	rm.Restore(rp.ID, testDir, false)
	if !restoreCalled {
		t.Error("restore callback not called")
	}

	rm.DeleteRecoveryPoint(rp.ID)
	if !deleteCalled {
		t.Error("delete callback not called")
	}
}

func TestRecoveryPoint_Types(t *testing.T) {
	types := []RecoveryType{
		RecoveryTypeAuto,
		RecoveryTypeManual,
		RecoveryTypeScheduled,
		RecoveryTypePreemptive,
	}

	for _, rt := range types {
		if string(rt) == "" {
			t.Errorf("RecoveryType should not be empty: %v", rt)
		}
	}
}

func TestRecoveryStatus_Values(t *testing.T) {
	statuses := []RecoveryStatus{
		RecoveryStatusReady,
		RecoveryStatusCreating,
		RecoveryStatusExpired,
		RecoveryStatusRollback,
	}

	for _, rs := range statuses {
		if string(rs) == "" {
			t.Errorf("RecoveryStatus should not be empty: %v", rs)
		}
	}
}
