package smartrebalance

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.pools == nil {
		t.Error("pools map not initialized")
	}
	if m.jobs == nil {
		t.Error("jobs map not initialized")
	}
	if m.policies == nil {
		t.Error("policies map not initialized")
	}
}

func TestRegisterPool(t *testing.T) {
	m := NewManager()
	pool := &StoragePool{
		ID:     "pool-1",
		Name:   "Main Pool",
		Status: PoolStatusHealthy,
		Disks: []DiskInfo{
			{ID: "disk-1", Path: "/dev/sda", Tier: TierSSD, TotalBytes: 1024 * 1024 * 1024 * 500, UsedBytes: 1024 * 1024 * 1024 * 200, Utilization: 0.4},
			{ID: "disk-2", Path: "/dev/sdb", Tier: TierHDD, TotalBytes: 1024 * 1024 * 1024 * 1000, UsedBytes: 1024 * 1024 * 1024 * 800, Utilization: 0.8},
		},
		TotalBytes: 1024 * 1024 * 1024 * 1500,
		UsedBytes:  1024 * 1024 * 1024 * 1000,
	}

	m.RegisterPool(pool)

	got, err := m.GetPool("pool-1")
	if err != nil {
		t.Fatalf("GetPool failed: %v", err)
	}
	if got.ID != "pool-1" {
		t.Errorf("expected pool ID pool-1, got %s", got.ID)
	}
}

func TestGetPoolNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetPool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestListPools(t *testing.T) {
	m := NewManager()
	m.RegisterPool(&StoragePool{ID: "pool-1", Name: "Pool 1"})
	m.RegisterPool(&StoragePool{ID: "pool-2", Name: "Pool 2"})

	pools := m.ListPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}

func TestCreatePolicy(t *testing.T) {
	m := NewManager()
	policy := &RebalancePolicy{
		ID:       "policy-1",
		Name:     "Default Policy",
		Enabled:  true,
		Strategy: StrategyHybrid,
		Threshold: 0.2,
	}

	if err := m.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	got, err := m.GetPolicy("policy-1")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "Default Policy" {
		t.Errorf("expected policy name Default Policy, got %s", got.Name)
	}
}

func TestCreatePolicyNoID(t *testing.T) {
	m := NewManager()
	policy := &RebalancePolicy{Name: "No ID"}
	if err := m.CreatePolicy(policy); err == nil {
		t.Error("expected error for policy without ID")
	}
}

func TestTriggerRebalance(t *testing.T) {
	m := NewManager()
	pool := &StoragePool{
		ID:   "pool-1",
		Name: "Test Pool",
		Disks: []DiskInfo{
			{ID: "disk-1", Path: "/dev/sda", Tier: TierSSD, TotalBytes: 500, UsedBytes: 400, Utilization: 0.8},
			{ID: "disk-2", Path: "/dev/sdb", Tier: TierSSD, TotalBytes: 500, UsedBytes: 100, Utilization: 0.2},
		},
		TotalBytes: 1000,
		UsedBytes:  500,
	}
	pool.Utilization = 0.5
	m.RegisterPool(pool)

	job, err := m.TriggerRebalance("pool-1", StrategyCapacity)
	if err != nil {
		t.Fatalf("TriggerRebalance failed: %v", err)
	}

	if job.PoolID != "pool-1" {
		t.Errorf("expected pool ID pool-1, got %s", job.PoolID)
	}
	if job.Strategy != StrategyCapacity {
		t.Errorf("expected strategy capacity, got %s", job.Strategy)
	}

	// 等待任务完成
	time.Sleep(2 * time.Second)

	updated, _ := m.GetJob(job.ID)
	if updated.Status != JobStatusCompleted {
		t.Errorf("expected job status completed, got %s", updated.Status)
	}
}

func TestTriggerRebalanceNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.TriggerRebalance("nonexistent", StrategyCapacity)
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestAnalyzePool(t *testing.T) {
	m := NewManager()
	pool := &StoragePool{
		ID:   "pool-1",
		Name: "Test Pool",
		Disks: []DiskInfo{
			{ID: "disk-1", Path: "/dev/sda", Tier: TierSSD, TotalBytes: 500, UsedBytes: 400, Utilization: 0.8, HealthScore: 0.9},
			{ID: "disk-2", Path: "/dev/sdb", Tier: TierHDD, TotalBytes: 1000, UsedBytes: 200, Utilization: 0.2, HealthScore: 0.95},
		},
		TotalBytes: 1500,
		UsedBytes:  600,
	}
	pool.Utilization = 0.4
	m.RegisterPool(pool)

	analysis, err := m.AnalyzePool("pool-1")
	if err != nil {
		t.Fatalf("AnalyzePool failed: %v", err)
	}

	if analysis.PoolID != "pool-1" {
		t.Errorf("expected pool ID pool-1, got %s", analysis.PoolID)
	}
	if len(analysis.DiskDetails) != 2 {
		t.Errorf("expected 2 disk details, got %d", len(analysis.DiskDetails))
	}
	if analysis.Imbalance < 0 {
		t.Error("imbalance should be non-negative")
	}
}

func TestGetMetrics(t *testing.T) {
	m := NewManager()
	metrics := m.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
	if metrics.PoolsMonitored != 0 {
		t.Errorf("expected 0 pools monitored, got %d", metrics.PoolsMonitored)
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	m.Stop()
}

func TestCalculateImbalance(t *testing.T) {
	// 均衡的池
	pool := &StoragePool{
		Disks: []DiskInfo{
			{Utilization: 0.5},
			{Utilization: 0.5},
			{Utilization: 0.5},
		},
	}
	imb := calculateImbalance(pool)
	if imb > 0.01 {
		t.Errorf("expected near-zero imbalance for balanced pool, got %f", imb)
	}

	// 不均衡的池
	pool2 := &StoragePool{
		Disks: []DiskInfo{
			{Utilization: 0.9},
			{Utilization: 0.1},
		},
	}
	imb2 := calculateImbalance(pool2)
	if imb2 < 0.3 {
		t.Errorf("expected high imbalance for unbalanced pool, got %f", imb2)
	}

	// 单盘
	pool3 := &StoragePool{
		Disks: []DiskInfo{{Utilization: 0.5}},
	}
	imb3 := calculateImbalance(pool3)
	if imb3 != 0 {
		t.Errorf("expected 0 imbalance for single disk, got %f", imb3)
	}
}

func TestFindRebalanceTargets(t *testing.T) {
	pool := &StoragePool{
		Disks: []DiskInfo{
			{ID: "disk-1", Utilization: 0.9, ReadIOPS: 1000, WriteIOPS: 500},
			{ID: "disk-2", Utilization: 0.3, ReadIOPS: 200, WriteIOPS: 100},
			{ID: "disk-3", Utilization: 0.6, ReadIOPS: 500, WriteIOPS: 300},
		},
	}

	source, target := findRebalanceTargets(pool, StrategyCapacity)
	if source == nil || target == nil {
		t.Fatal("expected valid source and target")
	}
	if source.ID != "disk-1" {
		t.Errorf("expected source disk-1, got %s", source.ID)
	}
	if target.ID != "disk-2" {
		t.Errorf("expected target disk-2, got %s", target.ID)
	}
}
