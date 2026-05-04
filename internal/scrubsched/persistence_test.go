// Package scrubsched 提供智能Scrub调度功能
// persistence_test.go - 持久化测试
package scrubsched

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPersistSaveAndLoad 测试保存和加载.
func TestPersistSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := setupTestManager()

	// 创建策略
	_, err := mgr.CreatePolicy(CreatePolicyRequest{
		Name:      "持久化测试策略",
		PoolID:    "pool1",
		Schedule:  ScheduleWeekly,
		WeekDay:   0,
		Hour:      3,
		Minute:    0,
		Enabled:   true,
		AvoidPeak: true,
		PeakWindows: []PeakWindow{
			{DayOfWeek: -1, StartHour: 9, StartMin: 0, EndHour: 18, EndMin: 0},
		},
	})
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	// 触发Scrub并取消（产生历史记录）
	_ = mgr.TriggerScrub("pool1")
	time.Sleep(10 * time.Millisecond)
	_ = mgr.CancelScrub("pool1")

	// 保存
	cfg := DefaultPersistConfig(tmpDir)
	cfg.AutoSave = false
	persister := NewPersister(cfg, mgr)

	if err := persister.Save(); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	// 验证文件存在
	dataFile := filepath.Join(tmpDir, "scrubsched.json")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		t.Fatal("持久化文件应存在")
	}

	// 创建新的管理器并加载
	mgr2 := setupTestManager()
	persister2 := NewPersister(cfg, mgr2)

	if err := persister2.Load(); err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	// 验证策略已恢复
	policies := mgr2.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("策略数量应为1，实际: %d", len(policies))
	}
	if policies[0].Name != "持久化测试策略" {
		t.Errorf("策略名称不匹配: %s", policies[0].Name)
	}
	if !policies[0].AvoidPeak {
		t.Error("避峰配置应已恢复")
	}
	if policies[0].NextRun == nil {
		t.Error("NextRun应已重新计算")
	}

	// 验证历史已恢复
	records := mgr2.GetHistory("pool1")
	if len(records) != 1 {
		t.Fatalf("历史记录应为1条，实际: %d", len(records))
	}
	if records[0].State != StateCancelled {
		t.Errorf("历史记录状态应为cancelled，实际: %s", records[0].State)
	}
}

// TestPersistLoadNonExistent 测试加载不存在的文件.
func TestPersistLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := setupTestManager()
	cfg := DefaultPersistConfig(tmpDir)
	cfg.AutoSave = false
	persister := NewPersister(cfg, mgr)

	// 加载不存在的文件不应报错
	if err := persister.Load(); err != nil {
		t.Errorf("加载不存在的文件不应报错: %v", err)
	}
}

// TestPersistAutoSave 测试自动保存.
func TestPersistAutoSave(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := setupTestManager()

	cfg := DefaultPersistConfig(tmpDir)
	cfg.AutoSave = true
	cfg.SaveInterval = 100 * time.Millisecond // 快速保存用于测试

	persister := NewPersister(cfg, mgr)
	persister.Start()

	// 创建策略触发变更
	_, _ = mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "自动保存测试",
		PoolID:   "pool1",
		Schedule: ScheduleWeekly,
		WeekDay:  1,
		Hour:     2,
		Minute:   0,
		Enabled:  true,
	})

	// 等待自动保存
	time.Sleep(250 * time.Millisecond)
	persister.Stop()

	// 验证文件已生成
	dataFile := filepath.Join(tmpDir, "scrubsched.json")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		t.Fatal("自动保存应已生成文件")
	}
}

// TestPersistStartStop 测试启动和停止.
func TestPersistStartStop(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := setupTestManager()
	cfg := DefaultPersistConfig(tmpDir)
	cfg.SaveInterval = 50 * time.Millisecond

	persister := NewPersister(cfg, mgr)

	// 重复启动不应 panic
	persister.Start()
	persister.Start()

	// 停止后再次停止不应 panic
	persister.Stop()
	persister.Stop()
}

// TestPersistDataIntegrity 测试数据完整性.
func TestPersistDataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := setupTestManager()

	// 创建多个策略
	for i := 0; i < 5; i++ {
		_, _ = mgr.CreatePolicy(CreatePolicyRequest{
			Name:     "策略" + string(rune('A'+i)),
			PoolID:   "pool1",
			Schedule: ScheduleWeekly,
			WeekDay:  i,
			Hour:     i + 1,
			Minute:   0,
			Enabled:  true,
		})
	}

	cfg := DefaultPersistConfig(tmpDir)
	cfg.AutoSave = false
	persister := NewPersister(cfg, mgr)
	_ = persister.Save()

	// 重新加载
	mgr2 := setupTestManager()
	persister2 := NewPersister(cfg, mgr2)
	_ = persister2.Load()

	if len(mgr2.ListPolicies()) != 5 {
		t.Errorf("应恢复5个策略，实际: %d", len(mgr2.ListPolicies()))
	}
}
