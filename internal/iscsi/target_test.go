package iscsi

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestManager creates a test manager with temp directory
func setupTestManager(t *testing.T) (*Manager, string) {
	tmpDir, err := os.MkdirTemp("", "iscsi-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "iscsi-config.json")
	basePath := filepath.Join(tmpDir, "luns")

	mgr, err := NewManager(configPath, basePath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	return mgr, tmpDir
}

func cleanupTestManager(tmpDir string) {
	os.RemoveAll(tmpDir)
}

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
	if mgr.targets == nil {
		t.Error("Targets map should be initialized")
	}
	if mgr.config == nil {
		t.Error("Config should be initialized")
	}
}

func TestCreateTarget(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{
		Name:        "test-target",
		MaxSessions: 8,
	}

	target, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	if target.ID == "" {
		t.Error("Target ID should be generated")
	}
	if target.Name != "test-target" {
		t.Errorf("Expected name 'test-target', got '%s'", target.Name)
	}
	if target.MaxSessions != 8 {
		t.Errorf("Expected max sessions 8, got %d", target.MaxSessions)
	}
	if target.IQN == "" {
		t.Error("IQN should be auto-generated")
	}
	if !target.Enabled {
		t.Error("Target should be enabled by default")
	}
}

func TestCreateTargetWithCustomIQN(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{
		Name: "custom-iqn-target",
		IQN:  "iqn.2024-03.com.example:custom-target",
	}

	target, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	if target.IQN != "iqn.2024-03.com.example:custom-target" {
		t.Errorf("Expected custom IQN, got '%s'", target.IQN)
	}
}

func TestCreateTargetDuplicateName(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{Name: "duplicate-target"}
	_, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("First create should succeed: %v", err)
	}

	_, err = mgr.CreateTarget(input)
	if err != ErrTargetExists {
		t.Errorf("Expected ErrTargetExists, got: %v", err)
	}
}

func TestGetTarget(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{Name: "get-target"}
	created, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	target, err := mgr.GetTarget(created.ID)
	if err != nil {
		t.Fatalf("Failed to get target: %v", err)
	}

	if target.ID != created.ID {
		t.Errorf("Expected ID '%s', got '%s'", created.ID, target.ID)
	}
}

func TestGetTargetNotFound(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	_, err := mgr.GetTarget("non-existent")
	if err != ErrTargetNotFound {
		t.Errorf("Expected ErrTargetNotFound, got: %v", err)
	}
}

func TestListTargets(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	// Create multiple targets
	for i := 0; i < 3; i++ {
		input := TargetInput{Name: string(rune('a' + i))}
		_, err := mgr.CreateTarget(input)
		if err != nil {
			t.Fatalf("Failed to create target: %v", err)
		}
	}

	targets := mgr.ListTargets()
	if len(targets) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(targets))
	}
}

func TestDeleteTarget(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{Name: "delete-target"}
	created, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	err = mgr.DeleteTarget(created.ID)
	if err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	_, err = mgr.GetTarget(created.ID)
	if err != ErrTargetNotFound {
		t.Errorf("Expected ErrTargetNotFound after delete, got: %v", err)
	}
}

func TestUpdateTarget(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	input := TargetInput{Name: "update-target"}
	created, err := mgr.CreateTarget(input)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	update := TargetInput{
		Alias:       "Updated Alias",
		MaxSessions: 32,
	}

	updated, err := mgr.UpdateTarget(created.ID, update)
	if err != nil {
		t.Fatalf("Failed to update target: %v", err)
	}

	if updated.Alias != "Updated Alias" {
		t.Errorf("Expected alias 'Updated Alias', got '%s'", updated.Alias)
	}
	if updated.MaxSessions != 32 {
		t.Errorf("Expected max sessions 32, got %d", updated.MaxSessions)
	}
}

// ========== LUN Tests ==========

func TestAddLUN(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, err := mgr.CreateTarget(TargetInput{Name: "lun-target"})
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	lunInput := LUNInput{
		Name: "test-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 1024, // 1GB
	}

	lun, err := mgr.AddLUN(target.ID, lunInput)
	if err != nil {
		t.Fatalf("Failed to add LUN: %v", err)
	}

	if lun.Name != "test-lun" {
		t.Errorf("Expected name 'test-lun', got '%s'", lun.Name)
	}
	if lun.Type != LUNTypeFile {
		t.Errorf("Expected type 'file', got '%s'", lun.Type)
	}
	if lun.Size != 1024*1024*1024 {
		t.Errorf("Unexpected size: %d", lun.Size)
	}
	if lun.Number != 0 {
		t.Errorf("Expected LUN number 0, got %d", lun.Number)
	}
}

func TestAddMultipleLUNs(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, err := mgr.CreateTarget(TargetInput{Name: "multi-lun-target"})
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	for i := 0; i < 3; i++ {
		lunInput := LUNInput{
			Name: string(rune('a' + i)),
			Type: LUNTypeFile,
			Size: 1024 * 1024 * 100,
		}
		lun, err := mgr.AddLUN(target.ID, lunInput)
		if err != nil {
			t.Fatalf("Failed to add LUN %d: %v", i, err)
		}
		if lun.Number != i {
			t.Errorf("Expected LUN number %d, got %d", i, lun.Number)
		}
	}

	updated, _ := mgr.GetTarget(target.ID)
	if len(updated.LUNs) != 3 {
		t.Errorf("Expected 3 LUNs, got %d", len(updated.LUNs))
	}
}

func TestGetLUN(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "get-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{
		Name: "get-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})

	retrieved, err := mgr.GetLUN(target.ID, lun.ID)
	if err != nil {
		t.Fatalf("Failed to get LUN: %v", err)
	}
	if retrieved.ID != lun.ID {
		t.Errorf("LUN ID mismatch")
	}
}

func TestRemoveLUN(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "remove-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{
		Name: "remove-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})

	err := mgr.RemoveLUN(target.ID, lun.ID)
	if err != nil {
		t.Fatalf("Failed to remove LUN: %v", err)
	}

	_, err = mgr.GetLUN(target.ID, lun.ID)
	if err != ErrLUNNotFound {
		t.Errorf("Expected ErrLUNNotFound, got: %v", err)
	}
}

func TestExpandLUN(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "expand-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{
		Name: "expand-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})

	newSize := int64(1024 * 1024 * 200)
	expanded, err := mgr.ExpandLUN(target.ID, lun.ID, newSize)
	if err != nil {
		t.Fatalf("Failed to expand LUN: %v", err)
	}

	if expanded.Size != newSize {
		t.Errorf("Expected size %d, got %d", newSize, expanded.Size)
	}
}

func TestExpandLUNShrinkError(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "shrink-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{
		Name: "shrink-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})

	_, err := mgr.ExpandLUN(target.ID, lun.ID, 1024*1024*50)
	if err != ErrShrinkNotSupported {
		t.Errorf("Expected ErrShrinkNotSupported, got: %v", err)
	}
}

// ========== CHAP Tests ==========

func TestCHAPManager(t *testing.T) {
	chapMgr := NewCHAPManager()

	input := &CHAPInput{
		Enabled:  true,
		Username: "testuser",
		Secret:   "testsecret1234",
	}

	err := chapMgr.ValidateInput(input)
	if err != nil {
		t.Fatalf("Valid CHAP input should pass: %v", err)
	}

	config := chapMgr.CreateConfig("target-1", input)
	if config == nil {
		t.Fatal("Config should not be nil")
	}
	if !config.Enabled {
		t.Error("Config should be enabled")
	}
	if config.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Username)
	}
}

func TestCHAPValidationErrors(t *testing.T) {
	chapMgr := NewCHAPManager()

	tests := []struct {
		name  string
		input *CHAPInput
	}{
		{
			name: "empty username",
			input: &CHAPInput{
				Enabled: true,
				Secret:  "testsecret1234",
			},
		},
		{
			name: "empty secret",
			input: &CHAPInput{
				Enabled:  true,
				Username: "testuser",
			},
		},
		{
			name: "short secret",
			input: &CHAPInput{
				Enabled:  true,
				Username: "testuser",
				Secret:   "short",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := chapMgr.ValidateInput(tt.input)
			if err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			}
		})
	}
}

func TestCHAPAuthenticate(t *testing.T) {
	chapMgr := NewCHAPManager()

	input := &CHAPInput{
		Enabled:  true,
		Username: "testuser",
		Secret:   "testsecret1234",
	}
	chapMgr.CreateConfig("target-1", input)

	// Test correct credentials
	if !chapMgr.Authenticate("target-1", "testuser", "testsecret1234") {
		t.Error("Authentication should succeed with correct credentials")
	}

	// Test wrong credentials
	if chapMgr.Authenticate("target-1", "testuser", "wrongsecret") {
		t.Error("Authentication should fail with wrong credentials")
	}

	// Test non-existent target (no auth required)
	if !chapMgr.Authenticate("non-existent", "any", "any") {
		t.Error("Authentication should succeed for non-existent target")
	}
}

// ========== IQN Tests ==========

func TestValidateIQN(t *testing.T) {
	tests := []struct {
		iqn     string
		valid   bool
	}{
		{"iqn.2024-03.com.example:target1", true},
		{"iqn.2024-03.com.example.nas:target1", true},
		{"", true}, // Empty is valid (auto-generated)
		{"invalid-iqn", false},
		{"iqn.invalid-format", false},
	}

	for _, tt := range tests {
		err := ValidateIQN(tt.iqn)
		if (err == nil) != tt.valid {
			t.Errorf("IQN '%s' validation: expected valid=%v, got err=%v", tt.iqn, tt.valid, err)
		}
	}
}

func TestGenerateIQN(t *testing.T) {
	iqn, err := GenerateIQN("example.com", "test")
	if err != nil {
		t.Fatalf("Failed to generate IQN: %v", err)
	}

	if len(iqn) < 10 {
		t.Errorf("IQN too short: %s", iqn)
	}

	// Should contain reversed domain
	if len(iqn) < 10 || iqn[:10] != "iqn.2024-0" {
		t.Errorf("IQN format incorrect: %s", iqn)
	}
}

func TestNormalizeIQN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"IQN.2024-03.COM.EXAMPLE:TARGET", "iqn.2024-03.com.example:target"},
		{"iqn.2024-03.com.example:target", "iqn.2024-03.com.example:target"},
	}

	for _, tt := range tests {
		result := NormalizeIQN(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeIQN(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

// ========== LUN Snapshot Tests ==========

func TestCreateLUNSnapshot(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "snapshot-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{
		Name: "snapshot-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})

	snapshot, err := mgr.CreateLUNSnapshot(target.ID, lun.ID, LUNSnapshotInput{
		Name: "test-snapshot",
	})
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snapshot.Name != "test-snapshot" {
		t.Errorf("Expected snapshot name 'test-snapshot', got '%s'", snapshot.Name)
	}
	if snapshot.LUNNumber != lun.Number {
		t.Errorf("LUN number mismatch")
	}
}

// ========== Enable/Disable Tests ==========

func TestEnableDisableTarget(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "toggle-target"})

	// Disable
	err := mgr.DisableTarget(target.ID)
	if err != nil {
		t.Fatalf("Failed to disable target: %v", err)
	}

	disabled, _ := mgr.GetTarget(target.ID)
	if disabled.Enabled {
		t.Error("Target should be disabled")
	}

	// Enable
	err = mgr.EnableTarget(target.ID)
	if err != nil {
		t.Fatalf("Failed to enable target: %v", err)
	}

	enabled, _ := mgr.GetTarget(target.ID)
	if !enabled.Enabled {
		t.Error("Target should be enabled")
	}
}

// ========== Persistence Tests ==========

func TestConfigPersistence(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	// Create target
	target, _ := mgr.CreateTarget(TargetInput{Name: "persist-target"})

	// Create new manager to test loading
	configPath := filepath.Join(tmpDir, "iscsi-config.json")
	basePath := filepath.Join(tmpDir, "luns")
	newMgr, err := NewManager(configPath, basePath)
	if err != nil {
		t.Fatalf("Failed to create new manager: %v", err)
	}

	// Verify target was loaded
	loaded, err := newMgr.GetTarget(target.ID)
	if err != nil {
		t.Fatalf("Failed to load persisted target: %v", err)
	}

	if loaded.Name != "persist-target" {
		t.Errorf("Expected name 'persist-target', got '%s'", loaded.Name)
	}
}

// ========== Target Status Tests ==========

func TestGetTargetStatus(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "status-target"})

	status, err := mgr.GetTargetStatus(target.ID)
	if err != nil {
		t.Fatalf("Failed to get target status: %v", err)
	}

	if status.IQN != target.IQN {
		t.Errorf("IQN mismatch")
	}
	if status.LUNCount != 0 {
		t.Errorf("Expected 0 LUNs, got %d", status.LUNCount)
	}
}

// ========== Benchmark Tests ==========

func BenchmarkCreateTarget(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "iscsi-bench-*")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "iscsi-config.json")
	basePath := filepath.Join(tmpDir, "luns")
	mgr, _ := NewManager(configPath, basePath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateTarget(TargetInput{Name: time.Now().String()})
	}
}

func BenchmarkAddLUN(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "iscsi-bench-*")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "iscsi-config.json")
	basePath := filepath.Join(tmpDir, "luns")
	mgr, _ := NewManager(configPath, basePath)
	target, _ := mgr.CreateTarget(TargetInput{Name: "bench-target"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.AddLUN(target.ID, LUNInput{
			Name: time.Now().String(),
			Type: LUNTypeFile,
			Size: 1024 * 1024 * 100,
		})
	}
}