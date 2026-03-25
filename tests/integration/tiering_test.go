// Package integration 提供 NAS-OS 集成测试
// 存储分层模块集成测试
package integration

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"nas-os/internal/tiering"
)

// MockTieringManager 模拟分层管理器.
type MockTieringManager struct {
	tiers    map[tiering.TierType]*tiering.TierConfig
	policies map[string]*tiering.Policy
	tasks    map[string]*tiering.MigrateTask
	stats    *tiering.AccessStats
	mu       sync.RWMutex
}

// NewMockTieringManager 创建模拟分层管理器.
func NewMockTieringManager() *MockTieringManager {
	m := &MockTieringManager{
		tiers:    make(map[tiering.TierType]*tiering.TierConfig),
		policies: make(map[string]*tiering.Policy),
		tasks:    make(map[string]*tiering.MigrateTask),
		stats:    &tiering.AccessStats{ByTier: make(map[tiering.TierType]*tiering.TierStats)},
	}

	// 初始化默认存储层
	m.tiers[tiering.TierTypeSSD] = &tiering.TierConfig{
		Type:      tiering.TierTypeSSD,
		Name:      "SSD Cache",
		Path:      "/mnt/ssd",
		Capacity:  500000000000,
		Used:      100000000000,
		Threshold: 80,
		Priority:  100,
		Enabled:   true,
	}

	m.tiers[tiering.TierTypeHDD] = &tiering.TierConfig{
		Type:      tiering.TierTypeHDD,
		Name:      "HDD Storage",
		Path:      "/mnt/hdd",
		Capacity:  4000000000000,
		Used:      2000000000000,
		Threshold: 85,
		Priority:  50,
		Enabled:   true,
	}

	m.tiers[tiering.TierTypeCloud] = &tiering.TierConfig{
		Type:      tiering.TierTypeCloud,
		Name:      "Cloud Archive",
		Path:      "/mnt/cloud",
		Capacity:  10000000000000,
		Used:      100000000000,
		Threshold: 90,
		Priority:  10,
		Enabled:   true,
	}

	return m
}

// GetTier 获取存储层配置.
func (m *MockTieringManager) GetTier(tierType tiering.TierType) (*tiering.TierConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tiers[tierType], nil
}

// ListTiers 列出所有存储层.
func (m *MockTieringManager) ListTiers() ([]*tiering.TierConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tiers := make([]*tiering.TierConfig, 0, len(m.tiers))
	for _, t := range m.tiers {
		tiers = append(tiers, t)
	}
	// 按优先级降序排序（优先级高的在前）
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].Priority > tiers[j].Priority
	})
	return tiers, nil
}

// CreatePolicy 创建分层策略.
func (m *MockTieringManager) CreatePolicy(policy *tiering.Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取策略.
func (m *MockTieringManager) GetPolicy(id string) (*tiering.Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policies[id], nil
}

// ListPolicies 列出所有策略.
func (m *MockTieringManager) ListPolicies() ([]*tiering.Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policies := make([]*tiering.Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies, nil
}

// Migrate 执行迁移.
func (m *MockTieringManager) Migrate(ctx context.Context, req *tiering.MigrateRequest) (*tiering.MigrateTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &tiering.MigrateTask{
		ID:          "task-" + time.Now().Format("20060102150405"),
		Status:      tiering.MigrateStatusCompleted,
		CreatedAt:   time.Now(),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		SourceTier:  req.SourceTier,
		TargetTier:  req.TargetTier,
		Action:      req.Action,
		TotalFiles:  int64(len(req.Paths)),
		TotalBytes:  1000000000,
	}

	m.tasks[task.ID] = task
	return task, nil
}

// GetStats 获取统计.
func (m *MockTieringManager) GetStats() (*tiering.AccessStats, error) {
	return m.stats, nil
}

// ========== 存储分层集成测试 ==========

// TestTiering_TierConfigurations 测试存储层配置.
func TestTiering_TierConfigurations(t *testing.T) {
	manager := NewMockTieringManager()

	tests := []struct {
		name     string
		tierType tiering.TierType
		wantName string
		wantPath string
		enabled  bool
		priority int
	}{
		{
			name:     "SSD Cache Tier",
			tierType: tiering.TierTypeSSD,
			wantName: "SSD Cache",
			wantPath: "/mnt/ssd",
			enabled:  true,
			priority: 100,
		},
		{
			name:     "HDD Storage Tier",
			tierType: tiering.TierTypeHDD,
			wantName: "HDD Storage",
			wantPath: "/mnt/hdd",
			enabled:  true,
			priority: 50,
		},
		{
			name:     "Cloud Archive Tier",
			tierType: tiering.TierTypeCloud,
			wantName: "Cloud Archive",
			wantPath: "/mnt/cloud",
			enabled:  true,
			priority: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := manager.GetTier(tt.tierType)
			if err != nil {
				t.Fatalf("GetTier failed: %v", err)
			}

			if tier.Name != tt.wantName {
				t.Errorf("Expected name=%s, got %s", tt.wantName, tier.Name)
			}

			if tier.Path != tt.wantPath {
				t.Errorf("Expected path=%s, got %s", tt.wantPath, tier.Path)
			}

			if tier.Enabled != tt.enabled {
				t.Errorf("Expected enabled=%v, got %v", tt.enabled, tier.Enabled)
			}

			if tier.Priority != tt.priority {
				t.Errorf("Expected priority=%d, got %d", tt.priority, tier.Priority)
			}
		})
	}
}

// TestTiering_ListTiers 测试列出所有存储层.
func TestTiering_ListTiers(t *testing.T) {
	manager := NewMockTieringManager()

	tiers, err := manager.ListTiers()
	if err != nil {
		t.Fatalf("ListTiers failed: %v", err)
	}

	if len(tiers) != 3 {
		t.Errorf("Expected 3 tiers, got %d", len(tiers))
	}

	// 验证优先级排序
	for i := 1; i < len(tiers); i++ {
		if tiers[i].Priority > tiers[i-1].Priority {
			t.Errorf("Tiers not sorted by priority")
		}
	}
}

// TestTiering_PolicyLifecycle 测试策略生命周期.
func TestTiering_PolicyLifecycle(t *testing.T) {
	manager := NewMockTieringManager()

	// 创建策略
	policy := &tiering.Policy{
		ID:             "policy-001",
		Name:           "Hot to Cold",
		Description:    "Move hot data to cold storage after 30 days",
		Enabled:        true,
		Status:         tiering.PolicyStatusEnabled,
		SourceTier:     tiering.TierTypeSSD,
		TargetTier:     tiering.TierTypeHDD,
		Action:         tiering.PolicyActionMove,
		MinAccessCount: 10,
		MaxAccessAge:   720 * time.Hour,
		ScheduleType:   tiering.ScheduleTypeInterval,
	}

	err := manager.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 获取策略
	got, err := manager.GetPolicy("policy-001")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}

	if got.Name != policy.Name {
		t.Errorf("Expected name=%s, got %s", policy.Name, got.Name)
	}

	if got.SourceTier != tiering.TierTypeSSD {
		t.Errorf("Expected source tier=ssd, got %s", got.SourceTier)
	}

	if got.TargetTier != tiering.TierTypeHDD {
		t.Errorf("Expected target tier=hdd, got %s", got.TargetTier)
	}

	// 列出策略
	policies, err := manager.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies failed: %v", err)
	}

	if len(policies) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(policies))
	}
}

// TestTiering_Migration 测试数据迁移.
func TestTiering_Migration(t *testing.T) {
	manager := NewMockTieringManager()
	ctx := context.Background()

	req := &tiering.MigrateRequest{
		Paths:      []string{"/data/file1.txt", "/data/file2.txt"},
		SourceTier: tiering.TierTypeSSD,
		TargetTier: tiering.TierTypeHDD,
		Action:     tiering.PolicyActionMove,
		DryRun:     false,
		Preserve:   false,
	}

	task, err := manager.Migrate(ctx, req)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if task.Status != tiering.MigrateStatusCompleted {
		t.Errorf("Expected status=completed, got %s", task.Status)
	}

	if task.TotalFiles != 2 {
		t.Errorf("Expected 2 files, got %d", task.TotalFiles)
	}

	if task.SourceTier != tiering.TierTypeSSD {
		t.Errorf("Expected source tier=ssd, got %s", task.SourceTier)
	}

	if task.TargetTier != tiering.TierTypeHDD {
		t.Errorf("Expected target tier=hdd, got %s", task.TargetTier)
	}
}

// TestTiering_PolicyActions 测试策略动作类型.
func TestTiering_PolicyActions(t *testing.T) {
	actions := []tiering.PolicyAction{
		tiering.PolicyActionMove,
		tiering.PolicyActionCopy,
		tiering.PolicyActionArchive,
		tiering.PolicyActionDelete,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			manager := NewMockTieringManager()
			ctx := context.Background()

			req := &tiering.MigrateRequest{
				Paths:      []string{"/data/test.txt"},
				SourceTier: tiering.TierTypeSSD,
				TargetTier: tiering.TierTypeHDD,
				Action:     action,
			}

			task, err := manager.Migrate(ctx, req)
			if err != nil {
				t.Fatalf("Migrate with action %s failed: %v", action, err)
			}

			if task.Action != action {
				t.Errorf("Expected action=%s, got %s", action, task.Action)
			}
		})
	}
}

// TestTiering_AccessFrequency 测试访问频率分类.
func TestTiering_AccessFrequency(t *testing.T) {
	frequencies := []tiering.AccessFrequency{
		tiering.AccessFrequencyHot,
		tiering.AccessFrequencyWarm,
		tiering.AccessFrequencyCold,
	}

	expectedThresholds := map[tiering.AccessFrequency]int64{
		tiering.AccessFrequencyHot:  100,
		tiering.AccessFrequencyWarm: 10,
		tiering.AccessFrequencyCold: 0,
	}

	for _, freq := range frequencies {
		t.Run(string(freq), func(t *testing.T) {
			// 验证访问频率类型存在
			if freq == "" {
				t.Error("Access frequency should not be empty")
			}

			// 验证阈值配置
			config := tiering.DefaultPolicyEngineConfig()
			switch freq {
			case tiering.AccessFrequencyHot:
				if config.HotThreshold != expectedThresholds[freq] {
					t.Errorf("Hot threshold mismatch")
				}
			case tiering.AccessFrequencyWarm:
				if config.WarmThreshold != expectedThresholds[freq] {
					t.Errorf("Warm threshold mismatch")
				}
			}
		})
	}
}

// TestTiering_TierStats 测试存储层统计.
func TestTiering_TierStats(t *testing.T) {
	manager := NewMockTieringManager()

	stats, err := manager.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.ByTier == nil {
		t.Error("ByTier map should not be nil")
	}
}

// TestTiering_ConcurrentAccess 测试并发访问.
func TestTiering_ConcurrentAccess(t *testing.T) {
	manager := NewMockTieringManager()
	done := make(chan bool, 20)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = manager.ListTiers()
			done <- true
		}()
	}

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(i int) {
			_ = manager.CreatePolicy(&tiering.Policy{
				ID:     "concurrent-" + time.Now().Format("150405.999"),
				Name:   "Concurrent Policy",
				Status: tiering.PolicyStatusEnabled,
			})
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestTiering_MigrateStatus 测试迁移状态.
func TestTiering_MigrateStatus(t *testing.T) {
	statuses := []tiering.MigrateStatus{
		tiering.MigrateStatusPending,
		tiering.MigrateStatusRunning,
		tiering.MigrateStatusCompleted,
		tiering.MigrateStatusFailed,
		tiering.MigrateStatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			task := &tiering.MigrateTask{
				ID:     "test-task",
				Status: status,
			}

			if task.Status != status {
				t.Errorf("Expected status=%s, got %s", status, task.Status)
			}
		})
	}
}

// TestTiering_ScheduleTypes 测试调度类型.
func TestTiering_ScheduleTypes(t *testing.T) {
	scheduleTypes := []tiering.ScheduleType{
		tiering.ScheduleTypeManual,
		tiering.ScheduleTypeInterval,
		tiering.ScheduleTypeCron,
	}

	for _, st := range scheduleTypes {
		t.Run(string(st), func(t *testing.T) {
			policy := &tiering.Policy{
				ID:           "schedule-test",
				Name:         "Schedule Test",
				ScheduleType: st,
			}

			if policy.ScheduleType != st {
				t.Errorf("Expected schedule type=%s, got %s", st, policy.ScheduleType)
			}
		})
	}
}

// TestTiering_DefaultConfig 测试默认配置.
func TestTiering_DefaultConfig(t *testing.T) {
	config := tiering.DefaultPolicyEngineConfig()

	if config.CheckInterval != 1*time.Hour {
		t.Errorf("Expected CheckInterval=1h, got %v", config.CheckInterval)
	}

	if config.HotThreshold != 100 {
		t.Errorf("Expected HotThreshold=100, got %d", config.HotThreshold)
	}

	if config.WarmThreshold != 10 {
		t.Errorf("Expected WarmThreshold=10, got %d", config.WarmThreshold)
	}

	if config.ColdAgeHours != 720 {
		t.Errorf("Expected ColdAgeHours=720, got %d", config.ColdAgeHours)
	}

	if config.MaxConcurrent != 5 {
		t.Errorf("Expected MaxConcurrent=5, got %d", config.MaxConcurrent)
	}

	if !config.EnableAutoTier {
		t.Error("Expected EnableAutoTier=true")
	}
}

// TestTiering_FileAccessRecord 测试文件访问记录.
func TestTiering_FileAccessRecord(t *testing.T) {
	now := time.Now()
	record := tiering.FileAccessRecord{
		Path:        "/data/document.pdf",
		Size:        1024000,
		ModTime:     now,
		AccessTime:  now,
		AccessCount: 50,
		ReadBytes:   5000000,
		WriteBytes:  1000000,
		CurrentTier: tiering.TierTypeSSD,
		Frequency:   tiering.AccessFrequencyHot,
	}

	if record.Path != "/data/document.pdf" {
		t.Errorf("Expected path=/data/document.pdf, got %s", record.Path)
	}

	if record.Frequency != tiering.AccessFrequencyHot {
		t.Errorf("Expected frequency=hot, got %s", record.Frequency)
	}

	if record.CurrentTier != tiering.TierTypeSSD {
		t.Errorf("Expected current tier=ssd, got %s", record.CurrentTier)
	}
}

// ========== 性能测试 ==========

// BenchmarkTiering_GetTier 性能测试：获取存储层.
func BenchmarkTiering_GetTier(b *testing.B) {
	manager := NewMockTieringManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = manager.GetTier(tiering.TierTypeSSD)
	}
}

// BenchmarkTiering_CreatePolicy 性能测试：创建策略.
func BenchmarkTiering_CreatePolicy(b *testing.B) {
	manager := NewMockTieringManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = manager.CreatePolicy(&tiering.Policy{
			ID:     "bench-" + time.Now().Format("150405.999"),
			Name:   "Benchmark Policy",
			Status: tiering.PolicyStatusEnabled,
		})
	}
}

// BenchmarkTiering_Migrate 性能测试：迁移操作.
func BenchmarkTiering_Migrate(b *testing.B) {
	manager := NewMockTieringManager()
	ctx := context.Background()

	req := &tiering.MigrateRequest{
		Paths:      []string{"/data/test.txt"},
		SourceTier: tiering.TierTypeSSD,
		TargetTier: tiering.TierTypeHDD,
		Action:     tiering.PolicyActionMove,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = manager.Migrate(ctx, req)
	}
}
