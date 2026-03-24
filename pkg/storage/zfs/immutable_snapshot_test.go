package zfs

import (
	"context"
	"testing"
	"time"
)

var ctx = context.Background()

func TestImmutablePolicy(t *testing.T) {
	policy := DefaultImmutablePolicy()

	if policy.DefaultLockType != LockTypeSoft {
		t.Error("Default lock type should be soft")
	}
	if policy.DefaultRetentionDays != 365 {
		t.Error("Default retention should be 365 days")
	}
	if !policy.RequireApproval {
		t.Error("Should require approval by default")
	}
}

func TestSnapshotCreateOptions(t *testing.T) {
	opts := SnapshotCreateOptions{
		Name:      "test-snapshot",
		Immutable: true,
		LockType:  LockTypeHard,
	}

	if opts.Name != "test-snapshot" {
		t.Error("Name mismatch")
	}
	if !opts.Immutable {
		t.Error("Should be immutable")
	}
}

func TestIsValidSnapshotName(t *testing.T) {
	// Valid names
	validNames := []string{
		"snapshot1",
		"backup_2024",
		"prod-backup",
		"important.snapshot",
	}

	for _, name := range validNames {
		if !isValidSnapshotName(name) {
			t.Errorf("Name '%s' should be valid", name)
		}
	}

	// Invalid names
	invalidNames := []string{
		"123snapshot",   // starts with number
		"",              // empty
		"snapshot@name", // contains @
	}

	for _, name := range invalidNames {
		if isValidSnapshotName(name) {
			t.Errorf("Name '%s' should be invalid", name)
		}
	}
}

func TestZFSManagerCreation(t *testing.T) {
	mgr, err := NewZFSManager("", nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Check default policy
	if mgr.GetPolicy() == nil {
		t.Error("Should have default policy")
	}

	// Set new policy
	newPolicy := &ImmutablePolicy{
		DefaultLockType:      LockTypeHard,
		DefaultRetentionDays: 90,
	}
	mgr.SetPolicy(newPolicy)

	if mgr.GetPolicy().DefaultLockType != LockTypeHard {
		t.Error("Policy should be updated")
	}
}

func TestImmutableOperations(t *testing.T) {
	// 创建管理器（不检查 ZFS 可用性）
	mgr := &ZFSManager{
		policy:             DefaultImmutablePolicy(),
		snapshots:          make(map[string]*SnapshotInfo),
		immutableSnapshots: make(map[string]*SnapshotInfo),
		available:          true, // 模拟可用
	}

	fullName := "pool/dataset@test-immutable"
	now := time.Now()

	// 创建快照信息
	snap := &SnapshotInfo{
		FullName:     fullName,
		Dataset:      "pool/dataset",
		Name:         "test-immutable",
		CreationTime: now,
	}

	mgr.snapshots[fullName] = snap

	// 测试设置不可变（直接调用内部方法）
	err := mgr.setImmutable(fullName, LockTypeHard, nil)
	if err != nil {
		t.Fatalf("Failed to set immutable: %v", err)
	}

	// 检查不可变状态
	immutableSnaps := mgr.ListImmutableSnapshots()
	if len(immutableSnaps) != 1 {
		t.Error("Should have 1 immutable snapshot")
	}

	// 测试释放不可变（应该失败，因为需要审批）
	err = mgr.ReleaseImmutable(ctx, fullName, "")
	if err == nil {
		t.Error("Should fail to release without approver")
	}

	// 添加审批者后重试
	mgr.policy.Approvers = []string{"admin"}
	err = mgr.ReleaseImmutable(ctx, fullName, "admin")
	if err != nil {
		t.Logf("Release returned: %v", err)
	}
}

func TestLockTypes(t *testing.T) {
	lockTypes := []LockType{
		LockTypeNone,
		LockTypeSoft,
		LockTypeHard,
		LockTypeTimed,
		LockTypePermanent,
	}

	for _, lt := range lockTypes {
		if lt == "" {
			t.Error("Lock type should not be empty string")
		}
	}
}

func TestHoldInfo(t *testing.T) {
	hold := HoldInfo{
		Name:      "hold1",
		Tag:       "backup",
		CreatedAt: time.Now(),
		Immutable: true,
	}

	if hold.Tag != "backup" {
		t.Error("Hold tag mismatch")
	}
}

func TestSnapshotInfo(t *testing.T) {
	now := time.Now()
	snap := SnapshotInfo{
		Name:          "test",
		FullName:      "pool/dataset@test",
		Dataset:       "pool/dataset",
		CreationTime:  now,
		Used:          1024,
		Referenced:    2048,
		Immutable:     true,
		LockType:      LockTypeSoft,
	}

	if snap.Name != "test" {
		t.Error("Name mismatch")
	}
	if !snap.Immutable {
		t.Error("Should be immutable")
	}
}

func TestPoolInfo(t *testing.T) {
	pool := PoolInfo{
		Name:      "tank",
		State:     "ONLINE",
		Size:      1024 * 1024 * 1024 * 1024, // 1TB
		Allocated: 500 * 1024 * 1024 * 1024,   // 500GB
		Free:      524 * 1024 * 1024 * 1024,   // 524GB
		ReadOnly:  false,
	}

	if pool.State != "ONLINE" {
		t.Error("Pool state mismatch")
	}
	if pool.ReadOnly {
		t.Error("Pool should not be read-only")
	}
}

func TestDatasetInfo(t *testing.T) {
	ds := DatasetInfo{
		Name:        "tank/data",
		Type:        "filesystem",
		Mounted:     true,
		Mountpoint:  "/mnt/data",
		Compression: "lz4",
		Used:        1024 * 1024 * 1024,
		Avail:       500 * 1024 * 1024 * 1024,
	}

	if ds.Name != "tank/data" {
		t.Error("Dataset name mismatch")
	}
	if !ds.Mounted {
		t.Error("Dataset should be mounted")
	}
}

func TestVerifyFailActions(t *testing.T) {
	actions := []VerifyFailAction{
		VerifyFailWarn,
		VerifyFailAlert,
		VerifyFailLock,
		VerifyFailNone,
	}

	for _, action := range actions {
		if action == "" {
			t.Error("Action should not be empty")
		}
	}
}

func TestZFSManagerWithoutZFS(t *testing.T) {
	// 创建管理器，模拟 ZFS 不可用
	mgr := &ZFSManager{
		policy:             DefaultImmutablePolicy(),
		snapshots:          make(map[string]*SnapshotInfo),
		immutableSnapshots: make(map[string]*SnapshotInfo),
		available:          false,
	}

	ctx := context.Background()

	// 这些操作应该返回 ErrZFSNotAvailable
	_, err := mgr.ListPools(ctx)
	if err != ErrZFSNotAvailable {
		t.Error("Should return ErrZFSNotAvailable")
	}

	_, err = mgr.ListDatasets(ctx, "tank")
	if err != ErrZFSNotAvailable {
		t.Error("Should return ErrZFSNotAvailable")
	}

	_, err = mgr.ListSnapshots(ctx, "tank/data")
	if err != ErrZFSNotAvailable {
		t.Error("Should return ErrZFSNotAvailable")
	}
}

func BenchmarkImmutableCheck(b *testing.B) {
	mgr := &ZFSManager{
		policy:             DefaultImmutablePolicy(),
		snapshots:          make(map[string]*SnapshotInfo),
		immutableSnapshots: make(map[string]*SnapshotInfo),
	}

	snap := &SnapshotInfo{
		FullName: "pool/dataset@test",
		Immutable: true,
	}
	mgr.snapshots["pool/dataset@test"] = snap
	mgr.immutableSnapshots["pool/dataset@test"] = snap

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ListImmutableSnapshots()
	}
}