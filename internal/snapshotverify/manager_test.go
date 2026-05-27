package snapshotverify

import (
	"testing"
)

func TestNewSnapshotVerifyManager(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected default config to be set")
	}

	if manager.config.MaxConcurrent != 3 {
		t.Errorf("Expected max concurrent to be 3, got %d", manager.config.MaxConcurrent)
	}
}

func TestRegisterVerifier(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	verifier := manager.RegisterVerifier("test-verifier", "hash")
	if verifier == nil {
		t.Fatal("Expected verifier to be created")
	}

	if verifier.Name != "test-verifier" {
		t.Errorf("Expected name 'test-verifier', got '%s'", verifier.Name)
	}

	if verifier.Type != "hash" {
		t.Errorf("Expected type 'hash', got '%s'", verifier.Type)
	}

	if verifier.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", verifier.Status)
	}
}

func TestStartVerification(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	job, err := manager.StartVerification("snapshot-123")
	if err != nil {
		t.Fatalf("Failed to start verification: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be created")
	}

	if job.SnapshotID != "snapshot-123" {
		t.Errorf("Expected snapshot ID 'snapshot-123', got '%s'", job.SnapshotID)
	}

	if job.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", job.Status)
	}
}

func TestGetVerifyJob(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	// 创建一个验证任务
	job, _ := manager.StartVerification("snapshot-123")

	// 获取任务
	fetchedJob, err := manager.GetVerifyJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to get verify job: %v", err)
	}

	if fetchedJob.ID != job.ID {
		t.Errorf("Expected job ID '%s', got '%s'", job.ID, fetchedJob.ID)
	}

	// 测试不存在的任务
	_, err = manager.GetVerifyJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestCalculateHash(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	data := []byte("test data")
	hash := manager.CalculateHash(data)

	if hash == "" {
		t.Error("Expected hash to be generated")
	}

	// 验证相同数据产生相同哈希
	hash2 := manager.CalculateHash(data)
	if hash != hash2 {
		t.Error("Expected same hash for same data")
	}

	// 验证不同数据产生不同哈希
	data2 := []byte("different data")
	hash3 := manager.CalculateHash(data2)
	if hash == hash3 {
		t.Error("Expected different hash for different data")
	}
}

func TestVerifyHash(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	data := []byte("test data")
	hash := manager.CalculateHash(data)

	// 验证正确哈希
	if !manager.VerifyHash(data, hash) {
		t.Error("Expected hash to be valid")
	}

	// 验证错误哈希
	if manager.VerifyHash(data, "invalid-hash") {
		t.Error("Expected hash to be invalid")
	}
}

func TestGetVerifiers(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	// 注册一些验证器
	manager.RegisterVerifier("verifier1", "hash")
	manager.RegisterVerifier("verifier2", "integrity")

	verifiers := manager.GetVerifiers()
	if len(verifiers) != 2 {
		t.Errorf("Expected 2 verifiers, got %d", len(verifiers))
	}
}

func TestGetVerificationHistory(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	// 创建一些验证任务
	manager.StartVerification("snapshot-123")
	manager.StartVerification("snapshot-123")
	manager.StartVerification("snapshot-456")

	history := manager.GetVerificationHistory("snapshot-123")
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}
}

func TestGetVerificationStats(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	// 创建一些验证任务
	manager.StartVerification("snapshot-1")
	manager.StartVerification("snapshot-2")

	stats := manager.GetVerificationStats()
	if stats.TotalJobs != 2 {
		t.Errorf("Expected 2 total jobs, got %d", stats.TotalJobs)
	}

	if stats.RunningJobs != 2 {
		t.Errorf("Expected 2 running jobs, got %d", stats.RunningJobs)
	}
}

func TestVerifySnapshot(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	snapshot := &SnapshotInfo{
		ID:   "snapshot-123",
		Name: "test-snapshot",
		Path: "/path/to/snapshot",
		Size: 1024,
	}

	result, err := manager.VerifySnapshot(snapshot)
	if err != nil {
		t.Fatalf("Failed to verify snapshot: %v", err)
	}

	if !result.IsValid {
		t.Error("Expected snapshot to be valid")
	}

	if !result.HashMatch {
		t.Error("Expected hash to match")
	}

	if !result.IntegrityOK {
		t.Error("Expected integrity to be OK")
	}
}

func TestBatchVerify(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	snapshots := []*SnapshotInfo{
		{ID: "snapshot-1", Name: "snap1"},
		{ID: "snapshot-2", Name: "snap2"},
	}

	results, err := manager.BatchVerify(snapshots)
	if err != nil {
		t.Fatalf("Failed to batch verify: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestVerifyIntegrity(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	report, err := manager.VerifyIntegrity("/path/to/check")
	if err != nil {
		t.Fatalf("Failed to verify integrity: %v", err)
	}

	if report.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", report.Status)
	}
}

func TestRepairSnapshot(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	result, err := manager.RepairSnapshot("snapshot-123")
	if err != nil {
		t.Fatalf("Failed to repair snapshot: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Status)
	}
}

func TestCreateVerificationPolicy(t *testing.T) {
	manager := NewSnapshotVerifyManager(nil)

	policy := &VerificationPolicy{
		Name:     "daily-verify",
		Schedule: "0 0 * * *",
		Snapshots: []string{"snapshot-1", "snapshot-2"},
		Enabled:  true,
	}

	err := manager.CreateVerificationPolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// 测试无效策略
	invalidPolicy := &VerificationPolicy{
		Name: "",
	}
	err = manager.CreateVerificationPolicy(invalidPolicy)
	if err == nil {
		t.Error("Expected error for invalid policy")
	}
}
