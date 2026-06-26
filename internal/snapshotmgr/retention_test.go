package snapshotmgr

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestClassifyPeriod(t *testing.T) {
	ts := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	buckets := classifyPeriod(ts)

	if buckets["minutely"].period != "2025-06-15T14:30" {
		t.Errorf("expected minutely period '2025-06-15T14:30', got %q", buckets["minutely"].period)
	}
	if buckets["hourly"].period != "2025-06-15T14" {
		t.Errorf("expected hourly period '2025-06-15T14', got %q", buckets["hourly"].period)
	}
	if buckets["daily"].period != "2025-06-15" {
		t.Errorf("expected daily period '2025-06-15', got %q", buckets["daily"].period)
	}
	if buckets["monthly"].period != "2025-06" {
		t.Errorf("expected monthly period '2025-06', got %q", buckets["monthly"].period)
	}
	if buckets["yearly"].period != "2025" {
		t.Errorf("expected yearly period '2025', got %q", buckets["yearly"].period)
	}
}

func TestRetentionPolicyCreation(t *testing.T) {
	store := NewPolicyStore(zap.NewNop())

	policy := &RetentionPolicy{
		Name:        "test-policy",
		Description: "test retention policy",
		Enabled:     true,
		TargetScope: "global",
		Hourly:      24,
		Daily:       7,
		Weekly:      4,
		Monthly:     12,
	}

	created, err := store.Create(policy)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Name != "test-policy" {
		t.Errorf("expected name 'test-policy', got %q", created.Name)
	}
	if created.Hourly != 24 {
		t.Errorf("expected hourly 24, got %d", created.Hourly)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestRetentionPolicyList(t *testing.T) {
	store := NewPolicyStore(zap.NewNop())

	// 创建多个策略
	store.Create(&RetentionPolicy{Name: "global-policy", TargetScope: "global", Daily: 7})
	store.Create(&RetentionPolicy{Name: "pool-policy", TargetScope: "pool", TargetRef: "tank", Daily: 14})
	store.Create(&RetentionPolicy{Name: "another-global", TargetScope: "global", Weekly: 4})

	// 列出全部
	all := store.List("", "")
	if len(all) != 3 {
		t.Errorf("expected 3 policies, got %d", len(all))
	}

	// 按scope过滤
	global := store.List("global", "")
	if len(global) != 2 {
		t.Errorf("expected 2 global policies, got %d", len(global))
	}

	// 按ref过滤
	poolRef := store.List("pool", "tank")
	if len(poolRef) != 1 {
		t.Errorf("expected 1 pool policy, got %d", len(poolRef))
	}
}

func TestRetentionPolicyUpdate(t *testing.T) {
	store := NewPolicyStore(zap.NewNop())

	created, _ := store.Create(&RetentionPolicy{
		Name:    "original",
		Enabled: true,
		Hourly:  12,
		Daily:   7,
	})

	updated, err := store.Update(created.ID, &RetentionPolicy{
		Name:    "updated",
		Enabled: false,
		Hourly:  48,
		Daily:   7, // explicitly carry over
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", updated.Name)
	}
	if updated.Hourly != 48 {
		t.Errorf("expected hourly 48, got %d", updated.Hourly)
	}
}

func TestRetentionPolicyDelete(t *testing.T) {
	store := NewPolicyStore(zap.NewNop())

	created, _ := store.Create(&RetentionPolicy{Name: "to-delete"})

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPolicySchedulerSelectForDeletion(t *testing.T) {
	m := setupTestManager(t)
	policy := &RetentionPolicy{
		ID:          "test-policy",
		Name:        "test",
		Enabled:     true,
		TargetScope: "global",
		Daily:       2, // 每天保留2份
		Hourly:      4, // 每小时保留4份
	}

	ps := NewPolicyScheduler(zap.NewNop(), m, policy)

	// 创建多个同一小时的快照
	var snapshots []Snapshot
	baseTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		snapshots = append(snapshots, Snapshot{
			ID:        fmt.Sprintf("snap-%d", i),
			CreatedAt: baseTime.Add(time.Duration(i) * 10 * time.Minute),
		})
	}

	toDelete := ps.selectForDeletion(snapshots)

	// 每小时保留4份，6个快照在同一小时内，应删除2个
	if len(toDelete) != 2 {
		t.Errorf("expected 2 snapshots to delete, got %d (toDelete=%v)", len(toDelete), toDelete)
	}
}

func TestQuotaManager(t *testing.T) {
	m := setupTestManager(t)
	qm := NewQuotaManager(zap.NewNop(), m, 20, 1000)

	// 20% of 1000 = 200 bytes limit
	if !qm.CheckQuota(100) {
		t.Error("expected quota check to pass for 100 bytes")
	}
	if !qm.CheckQuota(200) {
		t.Error("expected quota check to pass exactly at limit")
	}
	if qm.CheckQuota(201) {
		t.Error("expected quota check to fail for 201 bytes (over limit)")
	}

	// Update quota
	qm.SetQuota(50) // 50% = 500 bytes limit
	if !qm.CheckQuota(500) {
		t.Error("expected quota check to pass for 500 bytes with 50% limit")
	}
}

func TestQuotaManagerStatus(t *testing.T) {
	m := setupTestManager(t)
	qm := NewQuotaManager(zap.NewNop(), m, 20, 10000)

	status := qm.GetStatus()
	if status["max_percent"] != 20.0 {
		t.Errorf("expected max_percent 20, got %v", status["max_percent"])
	}
	if status["quota_limit_bytes"] != int64(2000) {
		t.Errorf("expected quota_limit_bytes 2000, got %v", status["quota_limit_bytes"])
	}
}
