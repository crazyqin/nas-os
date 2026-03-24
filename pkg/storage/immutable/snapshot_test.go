package immutable

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// helper function to create test config
func testConfig(t *testing.T) *ImmutableConfig {
	config := DefaultImmutableConfig()
	config.SnapshotPath = t.TempDir()
	config.ConfigPath = filepath.Join(t.TempDir(), "immutable.json")
	return config
}

func TestNewSnapshotManager(t *testing.T) {
	m, err := NewSnapshotManager(testConfig(t))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer m.Close()

	if m.config == nil {
		t.Error("Config should not be nil")
	}
}

func TestCreateSnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	opts := CreateSnapshotOptions{
		ID:          "snap-001",
		Name:        "test-snapshot",
		Volume:      "volume-1",
		Description: "Test snapshot",
		CreatedBy:   "admin",
		Size:        1024 * 1024 * 1024, // 1GB
	}

	snap, err := m.CreateSnapshot(context.Background(), opts)
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snap.ID != "snap-001" {
		t.Errorf("ID mismatch: got %s", snap.ID)
	}

	if snap.State == StateCreating {
		t.Error("State should not be creating after creation")
	}
}

func TestCreateDuplicateSnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	opts := CreateSnapshotOptions{
		ID:        "snap-002",
		Name:      "duplicate-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}

	_, err := m.CreateSnapshot(context.Background(), opts)
	if err != nil {
		t.Fatalf("Failed to create first snapshot: %v", err)
	}

	// Try to create duplicate
	_, err = m.CreateSnapshot(context.Background(), opts)
	if err != ErrSnapshotAlreadyExists {
		t.Errorf("Expected ErrSnapshotAlreadyExists, got %v", err)
	}
}

func TestLockSnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot
	opts := CreateSnapshotOptions{
		ID:        "snap-003",
		Name:      "lock-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Lock snapshot
	err := m.LockSnapshot(context.Background(), "snap-003", LockTypeHard, 0, "admin")
	if err != nil {
		t.Fatalf("Failed to lock snapshot: %v", err)
	}

	// Verify lock
	snap, _ := m.GetSnapshot("snap-003")
	if snap.LockType != LockTypeHard {
		t.Errorf("Lock type should be hard, got %s", snap.LockType)
	}
	if snap.State != StateLocked {
		t.Errorf("State should be locked, got %s", snap.State)
	}
}

func TestLockImmutableSnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create and lock snapshot
	opts := CreateSnapshotOptions{
		ID:        "snap-004",
		Name:      "immutable-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)
	m.LockSnapshot(context.Background(), "snap-004", LockTypePermanent, 0, "admin")

	// Try to delete locked snapshot
	err := m.DeleteSnapshot(context.Background(), "snap-004", "admin")
	if err != ErrSnapshotImmutable {
		t.Errorf("Expected ErrSnapshotImmutable, got %v", err)
	}
}

func TestUnlockSnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create and lock with soft lock
	opts := CreateSnapshotOptions{
		ID:        "snap-005",
		Name:      "unlock-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)
	m.LockSnapshot(context.Background(), "snap-005", LockTypeSoft, 0, "admin")

	// Unlock
	err := m.UnlockSnapshot(context.Background(), "snap-005", "admin", "Test unlock")
	if err != nil {
		t.Fatalf("Failed to unlock snapshot: %v", err)
	}

	// Verify unlock
	snap, _ := m.GetSnapshot("snap-005")
	if snap.LockType != LockTypeNone {
		t.Errorf("Lock type should be none, got %s", snap.LockType)
	}
}

func TestTimedLock(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot
	opts := CreateSnapshotOptions{
		ID:        "snap-006",
		Name:      "timed-lock-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Lock with duration
	duration := 1 * time.Hour
	err := m.LockSnapshot(context.Background(), "snap-006", LockTypeTimed, duration, "admin")
	if err != nil {
		t.Fatalf("Failed to lock snapshot: %v", err)
	}

	// Verify lock expiry
	snap, _ := m.GetSnapshot("snap-006")
	if snap.LockExpiry == nil {
		t.Error("Lock expiry should be set")
	}

	// Try to unlock before expiry (should fail without approval)
	err = m.UnlockSnapshot(context.Background(), "snap-006", "unauthorized", "early release")
	if err == nil {
		t.Error("Should not be able to unlock timed lock before expiry")
	}
}

func TestProcessTimeLocks(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot with short timed lock
	opts := CreateSnapshotOptions{
		ID:        "snap-007",
		Name:      "expiring-lock-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Lock with 1 nanosecond duration (will expire immediately)
	m.LockSnapshot(context.Background(), "snap-007", LockTypeTimed, 1*time.Nanosecond, "admin")

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Process time locks
	expired, err := m.ProcessTimeLocks(context.Background())
	if err != nil {
		t.Fatalf("Failed to process time locks: %v", err)
	}

	if len(expired) < 1 {
		t.Error("Should have at least one expired lock")
	}

	// Verify snapshot is unlocked
	snap, _ := m.GetSnapshot("snap-007")
	if snap.LockType != LockTypeNone {
		t.Errorf("Lock should be none after expiry, got %s", snap.LockType)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	config := testConfig(t)
	config.DefaultPolicy.MinRetention = 0
	config.DefaultPolicy.AutoLock = false // 禁用自动锁定以便测试删除

	m, _ := NewSnapshotManager(config)
	defer m.Close()

	opts := CreateSnapshotOptions{
		ID:        "snap-008",
		Name:      "delete-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Delete
	err := m.DeleteSnapshot(context.Background(), "snap-008", "admin")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestListSnapshots(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create multiple snapshots
	for i := 0; i < 3; i++ {
		opts := CreateSnapshotOptions{
			ID:        string(rune('a' + i)),
			Name:      "list-test",
			Volume:    "volume-1",
			CreatedBy: "admin",
		}
		m.CreateSnapshot(context.Background(), opts)
	}

	// List all
	all := m.ListSnapshots("", "")
	if len(all) < 3 {
		t.Errorf("Expected at least 3 snapshots, got %d", len(all))
	}

	// List by volume
	volume1 := m.ListSnapshots("volume-1", "")
	if len(volume1) < 3 {
		t.Errorf("Expected at least 3 snapshots for volume-1, got %d", len(volume1))
	}
}

func TestRansomwareCheck(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create protected snapshot
	opts := CreateSnapshotOptions{
		ID:                  "snap-009",
		Name:                "ransomware-test",
		Volume:              "volume-1",
		CreatedBy:           "admin",
		RansomwareProtection: true,
	}
	m.CreateSnapshot(context.Background(), opts)

	// Check for ransomware activity
	result, err := m.CheckRansomwareActivity(context.Background(), "snap-009")
	if err != nil {
		t.Fatalf("Ransomware check failed: %v", err)
	}

	if !result.Protected {
		t.Error("Snapshot should be protected")
	}
}

func TestVerifySnapshot(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot
	opts := CreateSnapshotOptions{
		ID:        "snap-010",
		Name:      "verify-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Verify
	result, err := m.VerifySnapshot(context.Background(), "snap-010")
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	if result.SnapshotID != "snap-010" {
		t.Errorf("Snapshot ID mismatch: got %s", result.SnapshotID)
	}
}

func TestAccessRecord(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot
	opts := CreateSnapshotOptions{
		ID:        "snap-011",
		Name:      "access-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	// Record access
	err := m.RecordAccess("snap-011", "user1", "read", true, "192.168.1.1")
	if err != nil {
		t.Fatalf("Failed to record access: %v", err)
	}

	// Verify access was recorded
	snap, _ := m.GetSnapshot("snap-011")
	if snap.ReadCount != 1 {
		t.Errorf("Read count should be 1, got %d", snap.ReadCount)
	}
	if len(snap.AccessLog) != 1 {
		t.Errorf("Access log should have 1 entry, got %d", len(snap.AccessLog))
	}
}

func TestGetStats(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create multiple snapshots
	for i := 0; i < 5; i++ {
		opts := CreateSnapshotOptions{
			ID:        string(rune('0' + i)),
			Name:      "stats-test",
			Volume:    "volume-1",
			CreatedBy: "admin",
			Size:      1024,
		}
		m.CreateSnapshot(context.Background(), opts)
	}

	stats := m.GetStats()
	if stats.Total < 5 {
		t.Errorf("Total should be at least 5, got %d", stats.Total)
	}

	if stats.TotalSize < 5*1024 {
		t.Errorf("Total size should be at least %d, got %d", 5*1024, stats.TotalSize)
	}
}

func TestRetentionPolicy(t *testing.T) {
	policy := DefaultRetentionPolicy()

	if policy.Name != "default" {
		t.Error("Policy name should be default")
	}

	if policy.MinRetention < 0 {
		t.Error("Min retention should not be negative")
	}

	if policy.KeepCount <= 0 {
		t.Error("Keep count should be positive")
	}
}

func TestImmutableConfig(t *testing.T) {
	config := DefaultImmutableConfig()

	if config.DefaultPolicy == nil {
		t.Error("Default policy should not be nil")
	}

	if !config.EnableRansomwareProtection {
		t.Error("Ransomware protection should be enabled by default")
	}
}

func TestIntegrityVerifier(t *testing.T) {
	v := NewIntegrityVerifier()

	data := []byte("test data")
	checksum, err := v.CalculateChecksum(data)
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	if checksum == "" {
		t.Error("Checksum should not be empty")
	}

	// Verify
	valid, err := v.VerifyChecksum(data, checksum)
	if err != nil {
		t.Fatalf("Failed to verify checksum: %v", err)
	}

	if !valid {
		t.Error("Checksum should be valid")
	}

	// Test invalid checksum
	valid, _ = v.VerifyChecksum([]byte("different data"), checksum)
	if valid {
		t.Error("Checksum should be invalid for different data")
	}
}

func TestLockTypes(t *testing.T) {
	types := []LockType{
		LockTypeNone,
		LockTypeSoft,
		LockTypeHard,
		LockTypeTimed,
		LockTypePermanent,
		LockTypeCompliance,
	}

	for _, lt := range types {
		if !isValidLockType(lt) {
			t.Errorf("Lock type %s should be valid", lt)
		}
	}

	// Invalid type
	if isValidLockType("invalid") {
		t.Error("Invalid lock type should not be valid")
	}
}

func TestSnapshotStates(t *testing.T) {
	states := []SnapshotState{
		StateCreating,
		StateActive,
		StateLocked,
		StateExpiring,
		StateExpired,
		StateDeleted,
		StateCorrupted,
	}

	for _, s := range states {
		if s == "" {
			t.Error("State should not be empty")
		}
	}
}

func TestGetExpiringSnapshots(t *testing.T) {
	m, _ := NewSnapshotManager(testConfig(t))
	defer m.Close()

	// Create snapshot with timed lock
	opts := CreateSnapshotOptions{
		ID:        "snap-expiring",
		Name:      "expiring-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)
	m.LockSnapshot(context.Background(), "snap-expiring", LockTypeTimed, 1*time.Hour, "admin")

	// Get expiring within 2 hours
	expiring := m.GetExpiringSnapshots(2 * time.Hour)
	if len(expiring) < 1 {
		t.Error("Should have at least one expiring snapshot")
	}
}

func BenchmarkCreateSnapshot(b *testing.B) {
	config := DefaultImmutableConfig()
	config.SnapshotPath = b.TempDir()
	config.ConfigPath = filepath.Join(b.TempDir(), "immutable.json")

	m, _ := NewSnapshotManager(config)
	defer m.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts := CreateSnapshotOptions{
			ID:        string(rune(i)),
			Name:      "bench-test",
			Volume:    "volume-1",
			CreatedBy: "admin",
		}
		m.CreateSnapshot(context.Background(), opts)
	}
}

func BenchmarkGetSnapshot(b *testing.B) {
	config := DefaultImmutableConfig()
	config.SnapshotPath = b.TempDir()
	config.ConfigPath = filepath.Join(b.TempDir(), "immutable.json")

	m, _ := NewSnapshotManager(config)
	defer m.Close()

	opts := CreateSnapshotOptions{
		ID:        "bench-snap",
		Name:      "bench-test",
		Volume:    "volume-1",
		CreatedBy: "admin",
	}
	m.CreateSnapshot(context.Background(), opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.GetSnapshot("bench-snap")
	}
}