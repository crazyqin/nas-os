// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
package hybridflash

import (
	"testing"
	"time"
)

func TestNewHybridFlashManager(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	if manager == nil {
		t.Fatal("NewHybridFlashManager 返回 nil")
	}

	if manager.pools == nil {
		t.Error("pools 未初始化")
	}

	if manager.configs == nil {
		t.Error("configs 未初始化")
	}

	if manager.rebalanceTasks == nil {
		t.Error("rebalanceTasks 未初始化")
	}
}

func TestCreatePool(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		HDDDevices:   []string{"/dev/sda", "/dev/sdb"},
		FlashRole:    FlashRoleData,
		FlashType:    FlashTypeNVMe,
		Compression:  true,
		Dedup:        false,
		Sync:         "standard",
		RecordSize:   128 * 1024,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	if pool.PoolName != "test-pool" {
		t.Errorf("期望池名 test-pool，实际 %s", pool.PoolName)
	}

	if pool.State != PoolStateOnline {
		t.Errorf("期望状态 online，实际 %s", pool.State)
	}

	if pool.FlashRole != FlashRoleData {
		t.Errorf("期望 Flash 角色 data，实际 %s", pool.FlashRole)
	}

	if len(pool.FlashDevices) != 2 {
		t.Errorf("期望 2 个 Flash 设备，实际 %d", len(pool.FlashDevices))
	}

	if len(pool.HDDDevices) != 2 {
		t.Errorf("期望 2 个 HDD 设备，实际 %d", len(pool.HDDDevices))
	}

	if pool.TierPolicy == nil {
		t.Error("TierPolicy 不应为 nil")
	}
}

func TestCreatePoolValidation(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	// 测试空池名
	config := &HybridPoolConfig{
		PoolName:     "",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
	}
	_, err := manager.CreatePool(config)
	if err == nil {
		t.Error("空池名应该返回错误")
	}

	// 测试无 Flash 设备
	config = &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
	}
	_, err = manager.CreatePool(config)
	if err == nil {
		t.Error("无 Flash 设备应该返回错误")
	}

	// 测试无 HDD 设备
	config = &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{},
		FlashRole:    FlashRoleData,
	}
	_, err = manager.CreatePool(config)
	if err == nil {
		t.Error("无 HDD 设备应该返回错误")
	}

	// 测试无效 Flash 角色
	config = &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRole("invalid"),
	}
	_, err = manager.CreatePool(config)
	if err == nil {
		t.Error("无效 Flash 角色应该返回错误")
	}
}

func TestCreateDuplicatePool(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	_, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("第一次创建池失败: %v", err)
	}

	// 创建同名池应该失败
	_, err = manager.CreatePool(config)
	if err == nil {
		t.Error("创建同名池应该返回错误")
	}
}

func TestGetPool(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	created, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 获取存在的池
	pool, err := manager.GetPool(created.PoolID)
	if err != nil {
		t.Fatalf("获取池失败: %v", err)
	}

	if pool.PoolID != created.PoolID {
		t.Errorf("期望池ID %s，实际 %s", created.PoolID, pool.PoolID)
	}

	// 获取不存在的池
	_, err = manager.GetPool("non-existent")
	if err == nil {
		t.Error("获取不存在的池应该返回错误")
	}
}

func TestListPools(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	// 空列表
	pools := manager.ListPools()
	if len(pools) != 0 {
		t.Errorf("期望 0 个池，实际 %d", len(pools))
	}

	// 创建 2 个池
	config1 := &HybridPoolConfig{
		PoolName:     "pool-1",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	config2 := &HybridPoolConfig{
		PoolName:     "pool-2",
		FlashDevices: []string{"/dev/nvme2n1"},
		HDDDevices:   []string{"/dev/sdb"},
		FlashRole:    FlashRoleSLOG,
		TierPolicy:   DefaultTierPolicy(),
	}

	_, err := manager.CreatePool(config1)
	if err != nil {
		t.Fatalf("创建池1失败: %v", err)
	}

	_, err = manager.CreatePool(config2)
	if err != nil {
		t.Fatalf("创建池2失败: %v", err)
	}

	pools = manager.ListPools()
	if len(pools) != 2 {
		t.Errorf("期望 2 个池，实际 %d", len(pools))
	}
}

func TestUpdateTierPolicy(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 更新策略
	newPolicy := &TierPolicy{
		HotDataThreshold:   200,
		ColdDataAge:        "360h",
		MetadataPreference: "hdd",
		AutoTiering:        false,
		SmallFileThreshold: 2 * 1024 * 1024,
		RebalanceInterval:  "2h",
		MaxFlashUsage:      0.9,
		MinHotDataRatio:    0.2,
	}

	err = manager.UpdateTierPolicy(pool.PoolID, newPolicy)
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	// 验证更新
	updated, err := manager.GetPool(pool.PoolID)
	if err != nil {
		t.Fatalf("获取池失败: %v", err)
	}

	if updated.TierPolicy.HotDataThreshold != 200 {
		t.Errorf("期望热数据阈值 200，实际 %d", updated.TierPolicy.HotDataThreshold)
	}

	if updated.TierPolicy.ColdDataAge != "360h" {
		t.Errorf("期望冷数据时间 360h，实际 %s", updated.TierPolicy.ColdDataAge)
	}

	// 更新不存在的池
	err = manager.UpdateTierPolicy("non-existent", newPolicy)
	if err == nil {
		t.Error("更新不存在的池应该返回错误")
	}
}

func TestUpdateTierPolicyValidation(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 测试 nil 策略
	err = manager.UpdateTierPolicy(pool.PoolID, nil)
	if err == nil {
		t.Error("nil 策略应该返回错误")
	}

	// 测试负数阈值
	err = manager.UpdateTierPolicy(pool.PoolID, &TierPolicy{
		HotDataThreshold: -1,
		ColdDataAge:      "720h",
		MaxFlashUsage:    0.85,
	})
	if err == nil {
		t.Error("负数阈值应该返回错误")
	}

	// 测试无效的 Flash 使用率
	err = manager.UpdateTierPolicy(pool.PoolID, &TierPolicy{
		HotDataThreshold: 100,
		ColdDataAge:      "720h",
		MaxFlashUsage:    1.5,
	})
	if err == nil {
		t.Error("无效的 Flash 使用率应该返回错误")
	}

	// 测试无效的时间格式
	err = manager.UpdateTierPolicy(pool.PoolID, &TierPolicy{
		HotDataThreshold: 100,
		ColdDataAge:      "invalid",
		MaxFlashUsage:    0.85,
	})
	if err == nil {
		t.Error("无效的时间格式应该返回错误")
	}
}

func TestRebalance(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 记录一些访问
	manager.RecordAccess(pool.PoolID, "/test/file1.dat", 0, 1024, AccessPatternRandom)
	manager.RecordAccess(pool.PoolID, "/test/file2.dat", 0, 2048, AccessPatternSequential)

	// 触发重平衡
	req := &RebalanceRequest{
		Force:           false,
		TargetFlashUsed: 0.7,
		DryRun:          false,
	}

	result, err := manager.Rebalance(pool.PoolID, req)
	if err != nil {
		t.Fatalf("重平衡失败: %v", err)
	}

	if result.Status != "running" {
		t.Errorf("期望状态 running，实际 %s", result.Status)
	}

	// 等待重平衡完成
	time.Sleep(100 * time.Millisecond)

	// 检查任务状态
	task, err := manager.GetRebalanceTask(result.TaskID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}

	if task.Status != "completed" {
		t.Errorf("期望状态 completed，实际 %s", task.Status)
	}
}

func TestRebalanceDryRun(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	req := &RebalanceRequest{
		Force:           false,
		TargetFlashUsed: 0.7,
		DryRun:          true,
	}

	result, err := manager.Rebalance(pool.PoolID, req)
	if err != nil {
		t.Fatalf("重平衡失败: %v", err)
	}

	// 试运行应该立即完成
	time.Sleep(50 * time.Millisecond)

	task, err := manager.GetRebalanceTask(result.TaskID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}

	if task.Status != "dry_run_completed" {
		t.Errorf("期望状态 dry_run_completed，实际 %s", task.Status)
	}
}

func TestRecordAccess(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy: &TierPolicy{
			HotDataThreshold:   3,
			ColdDataAge:        "720h",
			MetadataPreference: "flash",
			AutoTiering:        true,
			SmallFileThreshold: 1024 * 1024,
			MaxFlashUsage:      0.85,
		},
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 记录多次访问
	for i := 0; i < 5; i++ {
		manager.RecordAccess(pool.PoolID, "/test/hotfile.dat", 0, 1024, AccessPatternRandom)
	}

	// 验证块被创建且热度正确
	blockID := "/test/hotfile.dat:0:1024"
	block, err := manager.blockTracker.getBlock(blockID)
	if err != nil {
		t.Fatalf("获取块失败: %v", err)
	}

	if block.AccessCount != 5 {
		t.Errorf("期望访问次数 5，实际 %d", block.AccessCount)
	}

	if block.HeatLevel != HeatLevelHot {
		t.Errorf("期望热度 hot，实际 %s", block.HeatLevel)
	}
}

func TestDeletePool(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	config := &HybridPoolConfig{
		PoolName:     "test-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 删除池
	err = manager.DeletePool(pool.PoolID)
	if err != nil {
		t.Fatalf("删除池失败: %v", err)
	}

	// 验证池已被删除
	_, err = manager.GetPool(pool.PoolID)
	if err == nil {
		t.Error("池应该已被删除")
	}

	// 删除不存在的池
	err = manager.DeletePool("non-existent")
	if err == nil {
		t.Error("删除不存在的池应该返回错误")
	}
}

func TestDefaultTierPolicy(t *testing.T) {
	policy := DefaultTierPolicy()

	if policy == nil {
		t.Fatal("DefaultTierPolicy 返回 nil")
	}

	if policy.HotDataThreshold != 100 {
		t.Errorf("期望热数据阈值 100，实际 %d", policy.HotDataThreshold)
	}

	if policy.ColdDataAge != "720h" {
		t.Errorf("期望冷数据时间 720h，实际 %s", policy.ColdDataAge)
	}

	if policy.MetadataPreference != "flash" {
		t.Errorf("期望元数据偏好 flash，实际 %s", policy.MetadataPreference)
	}

	if !policy.AutoTiering {
		t.Error("期望 AutoTiering 为 true")
	}

	if policy.SmallFileThreshold != 1024*1024 {
		t.Errorf("期望小文件阈值 1048576，实际 %d", policy.SmallFileThreshold)
	}

	if policy.MaxFlashUsage != 0.85 {
		t.Errorf("期望最大 Flash 使用率 0.85，实际 %f", policy.MaxFlashUsage)
	}
}

func TestGetRebalanceTask(t *testing.T) {
	engine := NewTieringEngine(DefaultTieringConfig(), DefaultHeatTrackingConfig())
	manager := NewHybridFlashManager(engine)

	// 获取不存在的任务
	_, err := manager.GetRebalanceTask("non-existent")
	if err == nil {
		t.Error("获取不存在的任务应该返回错误")
	}
}
