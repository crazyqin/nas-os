// Package zfsenhanced ZFS增强管理模块 - 单元测试
package zfsenhanced

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.poolMgr == nil {
		t.Error("poolMgr is nil")
	}
	if mgr.integrityChecker == nil {
		t.Error("integrityChecker is nil")
	}
	if mgr.perfMonitor == nil {
		t.Error("perfMonitor is nil")
	}
	if mgr.scrubPolicies == nil {
		t.Error("scrubPolicies map is nil")
	}
	if mgr.snapshotPolicies == nil {
		t.Error("snapshotPolicies map is nil")
	}
	if mgr.repairQueue == nil {
		t.Error("repairQueue is nil")
	}
}

func TestDefaultTemplates(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())
	templates := mgr.ListSnapshotTemplates()
	if len(templates) != 3 {
		t.Errorf("expected 3 default templates, got %d", len(templates))
	}

	templateNames := make(map[string]bool)
	for _, tmpl := range templates {
		templateNames[tmpl.Name] = true
	}
	for _, expected := range []string{"hourly", "daily", "weekly"} {
		if !templateNames[expected] {
			t.Errorf("missing default template: %s", expected)
		}
	}
}

func TestScrubPolicyCRUD(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	// Create
	policy := &ScrubSchedulePolicy{
		Name:          "test-scrub",
		PoolName:      "tank",
		Strategy:      ScrubStrategyTimed,
		IntervalDays:  14,
		PreferredHour: 2,
		Priority:      ScrubPriorityNormal,
		Enabled:       true,
	}

	created, err := mgr.CreateScrubPolicy(policy)
	if err != nil {
		t.Fatalf("CreateScrubPolicy failed: %v", err)
	}
	if created.ID == "" {
		t.Error("created policy has empty ID")
	}
	if created.PoolName != "tank" {
		t.Errorf("expected pool_name=tank, got %s", created.PoolName)
	}
	if created.NextScrub == nil {
		t.Error("NextScrub should be set for timed strategy")
	}

	// List
	policies := mgr.ListScrubPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	// Get
	got, err := mgr.GetScrubPolicy(created.ID)
	if err != nil {
		t.Fatalf("GetScrubPolicy failed: %v", err)
	}
	if got.Name != "test-scrub" {
		t.Errorf("expected name=test-scrub, got %s", got.Name)
	}

	// Update
	updated, err := mgr.UpdateScrubPolicy(created.ID, &ScrubSchedulePolicy{
		Name:         "updated-scrub",
		IntervalDays: 7,
		Enabled:      false,
	})
	if err != nil {
		t.Fatalf("UpdateScrubPolicy failed: %v", err)
	}
	if updated.Name != "updated-scrub" {
		t.Errorf("expected name=updated-scrub, got %s", updated.Name)
	}
	if updated.IntervalDays != 7 {
		t.Errorf("expected interval_days=7, got %d", updated.IntervalDays)
	}
	if updated.Enabled {
		t.Error("expected enabled=false")
	}

	// Delete
	if err := mgr.DeleteScrubPolicy(created.ID); err != nil {
		t.Fatalf("DeleteScrubPolicy failed: %v", err)
	}
	policies = mgr.ListScrubPolicies()
	if len(policies) != 0 {
		t.Errorf("expected 0 policies after delete, got %d", len(policies))
	}
}

func TestScrubPolicyValidation(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	// 空池名
	_, err := mgr.CreateScrubPolicy(&ScrubSchedulePolicy{Name: "test"})
	if err == nil {
		t.Error("expected error for empty pool name")
	}

	// 不存在的策略
	_, err = mgr.GetScrubPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}

	// 删除不存在的策略
	err = mgr.DeleteScrubPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent policy")
	}
}

func TestScrubPolicyDefaults(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	policy, err := mgr.CreateScrubPolicy(&ScrubSchedulePolicy{PoolName: "tank"})
	if err != nil {
		t.Fatalf("CreateScrubPolicy failed: %v", err)
	}

	if policy.IntervalDays != 14 {
		t.Errorf("default interval_days should be 14, got %d", policy.IntervalDays)
	}
	if policy.Priority != ScrubPriorityNormal {
		t.Errorf("default priority should be %d, got %d", ScrubPriorityNormal, policy.Priority)
	}
}

func TestSnapshotLifecycleCRUD(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	policy := &SnapshotLifecyclePolicy{
		Name:            "test-snap",
		PoolName:        "tank",
		Datasets:        []string{"tank/data"},
		IntervalMinutes: 60,
		RetentionCount:  10,
		Prefix:          "test-",
		Enabled:         true,
		AutoCreate:      true,
		AutoDestroy:     true,
	}

	created, err := mgr.CreateSnapshotLifecycle(policy)
	if err != nil {
		t.Fatalf("CreateSnapshotLifecycle failed: %v", err)
	}
	if created.ID == "" {
		t.Error("created policy has empty ID")
	}
	if created.NextRun == nil {
		t.Error("NextRun should be set")
	}

	// List
	policies := mgr.ListSnapshotLifecycles()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}

	// Get
	got, err := mgr.GetSnapshotLifecycle(created.ID)
	if err != nil {
		t.Fatalf("GetSnapshotLifecycle failed: %v", err)
	}
	if got.Name != "test-snap" {
		t.Errorf("expected name=test-snap, got %s", got.Name)
	}

	// Update
	updated, err := mgr.UpdateSnapshotLifecycle(created.ID, &SnapshotLifecyclePolicy{
		RetentionCount: 20,
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("UpdateSnapshotLifecycle failed: %v", err)
	}
	if updated.RetentionCount != 20 {
		t.Errorf("expected retention_count=20, got %d", updated.RetentionCount)
	}

	// Delete
	if err := mgr.DeleteSnapshotLifecycle(created.ID); err != nil {
		t.Fatalf("DeleteSnapshotLifecycle failed: %v", err)
	}
	policies = mgr.ListSnapshotLifecycles()
	if len(policies) != 0 {
		t.Errorf("expected 0 policies after delete, got %d", len(policies))
	}
}

func TestSnapshotLifecycleWithTemplate(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	policy := &SnapshotLifecyclePolicy{
		Name:         "templated-snap",
		PoolName:     "tank",
		Datasets:     []string{"tank/data"},
		TemplateName: "daily",
		Enabled:      true,
	}

	created, err := mgr.CreateSnapshotLifecycle(policy)
	if err != nil {
		t.Fatalf("CreateSnapshotLifecycle failed: %v", err)
	}

	if created.RetentionCount != 30 {
		t.Errorf("expected retention_count=30 from template, got %d", created.RetentionCount)
	}
	if created.RetentionDays != 30 {
		t.Errorf("expected retention_days=30 from template, got %d", created.RetentionDays)
	}
	if !created.Recursive {
		t.Error("expected recursive=true from daily template")
	}
}

func TestSnapshotLifecycleValidation(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	// 空池名
	_, err := mgr.CreateSnapshotLifecycle(&SnapshotLifecyclePolicy{Datasets: []string{"tank/data"}})
	if err == nil {
		t.Error("expected error for empty pool name")
	}

	// 空数据集
	_, err = mgr.CreateSnapshotLifecycle(&SnapshotLifecyclePolicy{PoolName: "tank"})
	if err == nil {
		t.Error("expected error for empty datasets")
	}
}

func TestRepairQueuePriority(t *testing.T) {
	q := &RepairQueue{tasks: make([]*AutoRepairTask, 0)}

	low := &AutoRepairTask{ID: "low", Priority: ScrubPriorityLow}
	high := &AutoRepairTask{ID: "high", Priority: ScrubPriorityHigh}
	normal := &AutoRepairTask{ID: "normal", Priority: ScrubPriorityNormal}

	q.Push(low)
	q.Push(high)
	q.Push(normal)

	if q.Len() != 3 {
		t.Errorf("expected queue length 3, got %d", q.Len())
	}

	// 高优先级应该先出
	first := q.Pop()
	if first.ID != "high" {
		t.Errorf("expected high priority first, got %s", first.ID)
	}

	second := q.Pop()
	if second.ID != "normal" {
		t.Errorf("expected normal priority second, got %s", second.ID)
	}

	third := q.Pop()
	if third.ID != "low" {
		t.Errorf("expected low priority third, got %s", third.ID)
	}

	// 空队列
	if q.Pop() != nil {
		t.Error("expected nil from empty queue")
	}
}

func TestGetRepairTasks(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	tasks := mgr.GetRepairTasks("")
	if len(tasks) != 0 {
		t.Errorf("expected 0 repair tasks initially, got %d", len(tasks))
	}
}

func TestGetScrubJobsEmpty(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	jobs := mgr.GetScrubJobs("")
	if len(jobs) != 0 {
		t.Errorf("expected 0 scrub jobs initially, got %d", len(jobs))
	}
}

func TestGetMetricsHistoryEmpty(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	history := mgr.GetMetricsHistory("nonexistent", 10)
	if history != nil {
		t.Errorf("expected nil history for nonexistent pool, got %v", history)
	}
}

func TestGetPoolAnalysisNotFound(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	_, err := mgr.GetPoolAnalysis("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool analysis")
	}
}

func TestHealthScore(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	tests := []struct {
		name     string
		pool     *PoolInfo
		minScore float64
		maxScore float64
	}{
		{
			name:     "healthy pool",
			pool:     &PoolInfo{Status: PoolStatusOnline, UsedPercent: 50, Fragmentation: 10},
			minScore: 95,
			maxScore: 100,
		},
		{
			name:     "degraded pool",
			pool:     &PoolInfo{Status: PoolStatusDegraded, UsedPercent: 50, Fragmentation: 10},
			minScore: 60,
			maxScore: 75,
		},
		{
			name:     "full pool",
			pool:     &PoolInfo{Status: PoolStatusOnline, UsedPercent: 95, Fragmentation: 10},
			minScore: 75,
			maxScore: 85,
		},
		{
			name:     "fragmented pool",
			pool:     &PoolInfo{Status: PoolStatusOnline, UsedPercent: 50, Fragmentation: 55},
			minScore: 85,
			maxScore: 95,
		},
		{
			name:     "faulted pool",
			pool:     &PoolInfo{Status: PoolStatusFaulted, UsedPercent: 95, Fragmentation: 60, ReadErrors: 200},
			minScore: 0,
			maxScore: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := mgr.calculateHealthScore(tt.pool)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("health score %.1f not in range [%.1f, %.1f]", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestDiskHealthPercent(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	// Healthy disk
	healthy := mgr.calculateDiskHealthPercent(&SMARTInfo{Temperature: 35})
	if healthy < 95 || healthy > 100 {
		t.Errorf("healthy disk score %.1f not in range [95, 100]", healthy)
	}

	// Hot disk
	hot := mgr.calculateDiskHealthPercent(&SMARTInfo{Temperature: 60})
	if hot > 90 {
		t.Errorf("hot disk score %.1f should be < 90", hot)
	}

	// Disk with reallocated sectors
	reallocated := mgr.calculateDiskHealthPercent(&SMARTInfo{Temperature: 35, ReallocatedSectors: 10})
	if reallocated > 85 {
		t.Errorf("reallocated disk score %.1f should be < 85", reallocated)
	}
}

func TestPoolRecommendations(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	pool := &PoolInfo{
		Name:           "tank",
		Status:         PoolStatusOnline,
		UsedPercent:    90,
		Fragmentation:  35,
		ReadErrors:     5,
		WriteErrors:    3,
		ChecksumErrors: 2,
	}
	analysis := &PoolAnalysis{DailyGrowthBytes: 0}

	recs := mgr.generatePoolRecommendations(pool, analysis)
	if len(recs) < 3 {
		t.Errorf("expected at least 3 recommendations, got %d", len(recs))
	}
}

func TestAlertProxy(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	// 初始无告警
	alerts := mgr.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts initially, got %d", len(alerts))
	}

	// 获取默认告警配置
	config := mgr.GetAlertConfig()
	if config.CapacityWarningPercent != 80 {
		t.Errorf("expected capacity_warning_percent=80, got %.0f", config.CapacityWarningPercent)
	}

	// 更新告警配置
	newConfig := DefaultAlertConfig()
	newConfig.CapacityWarningPercent = 75
	mgr.UpdateAlertConfig(newConfig)

	config = mgr.GetAlertConfig()
	if config.CapacityWarningPercent != 75 {
		t.Errorf("expected capacity_warning_percent=75, got %.0f", config.CapacityWarningPercent)
	}
}

func TestScubScheduleStrategyConstants(t *testing.T) {
	if ScrubStrategyManual != "manual" {
		t.Error("ScrubStrategyManual should be 'manual'")
	}
	if ScrubStrategyTimed != "timed" {
		t.Error("ScrubStrategyTimed should be 'timed'")
	}
	if ScrubStrategySmart != "smart" {
		t.Error("ScrubStrategySmart should be 'smart'")
	}
}

func TestScrubPriorityOrder(t *testing.T) {
	if ScrubPriorityLow >= ScrubPriorityNormal {
		t.Error("ScrubPriorityLow should be less than ScrubPriorityNormal")
	}
	if ScrubPriorityNormal >= ScrubPriorityHigh {
		t.Error("ScrubPriorityNormal should be less than ScrubPriorityHigh")
	}
}

func TestCreateScrubPolicySmartStrategy(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	policy, err := mgr.CreateScrubPolicy(&ScrubSchedulePolicy{
		PoolName:    "tank",
		Strategy:    ScrubStrategySmart,
		IOThreshold: 200,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateScrubPolicy failed: %v", err)
	}

	if policy.Strategy != ScrubStrategySmart {
		t.Errorf("expected strategy=smart, got %s", policy.Strategy)
	}
	if policy.IOThreshold != 200 {
		t.Errorf("expected io_threshold=200, got %d", policy.IOThreshold)
	}
	// 智能策略不应设置 NextScrub
	if policy.NextScrub != nil {
		t.Error("smart strategy should not set NextScrub at creation")
	}
}

func TestSnapshotLifecycleDefaults(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())

	policy, err := mgr.CreateSnapshotLifecycle(&SnapshotLifecyclePolicy{
		PoolName: "tank",
		Datasets: []string{"tank/data"},
	})
	if err != nil {
		t.Fatalf("CreateSnapshotLifecycle failed: %v", err)
	}

	if policy.RetentionCount != 10 {
		t.Errorf("default retention_count=10, got %d", policy.RetentionCount)
	}
	if policy.IntervalMinutes != 60 {
		t.Errorf("default interval_minutes=60, got %d", policy.IntervalMinutes)
	}
	if policy.Prefix != "auto-" {
		t.Errorf("default prefix='auto-', got '%s'", policy.Prefix)
	}
}

func TestCheckAndRunScheduledScrubs(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())
	ctx := context.Background()

	// 无策略时应该返回空
	triggered, err := mgr.CheckAndRunScheduledScrubs(ctx)
	if err != nil {
		t.Fatalf("CheckAndRunScheduledScrubs failed: %v", err)
	}
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered, got %d", len(triggered))
	}
}

func TestCheckAndRunSnapshotLifecycles(t *testing.T) {
	mgr := NewManager(DefaultAlertConfig())
	ctx := context.Background()

	triggered, err := mgr.CheckAndRunSnapshotLifecycles(ctx)
	if err != nil {
		t.Fatalf("CheckAndRunSnapshotLifecycles failed: %v", err)
	}
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered, got %d", len(triggered))
	}
}

func TestCapacityTrend(t *testing.T) {
	capTrend := CapacityTrend{
		Timestamp:     time.Now(),
		TotalBytes:    1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:     500 * 1024 * 1024 * 1024,  // 500GB
		FreeBytes:     524 * 1024 * 1024 * 1024,  // 524GB
		UsedPercent:   48.8,
		GrowthRateDay: 1024 * 1024 * 1024, // 1GB/day
		DaysUntilFull: 524,
	}

	if capTrend.UsedPercent > 100 || capTrend.UsedPercent < 0 {
		t.Error("UsedPercent out of range")
	}
	if capTrend.DaysUntilFull < 0 {
		t.Error("DaysUntilFull should be positive")
	}
}

func TestRealtimeMetrics(t *testing.T) {
	metrics := RealtimeMetrics{
		PoolName:      "tank",
		Timestamp:     time.Now(),
		ReadIOPS:      1000,
		WriteIOPS:     500,
		ReadMBps:      200.5,
		WriteMBps:     100.3,
		ReadLatency:   0.5,
		WriteLatency:  1.2,
		HealthScore:   95.0,
		UsedPercent:   65.0,
		Fragmentation: 15.0,
	}

	if metrics.HealthScore < 0 || metrics.HealthScore > 100 {
		t.Error("HealthScore out of range")
	}
	if metrics.ReadIOPS < 0 || metrics.WriteIOPS < 0 {
		t.Error("IOPS should be non-negative")
	}
}
