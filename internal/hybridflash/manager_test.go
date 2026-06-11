// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理测试.
package hybridflash

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	if manager == nil {
		t.Fatal("NewManager 返回 nil")
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

	if manager.logger == nil {
		t.Error("logger 未初始化")
	}
}

func TestNewManagerNilLogger(t *testing.T) {
	// nil logger 应使用 nop logger
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("NewManager(nil) 返回 nil")
	}
	if manager.logger == nil {
		t.Error("nil logger 应使用 nop logger")
	}
}

func TestCreatePool(t *testing.T) {
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

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
	manager := NewManager(zap.NewNop())

	// 获取不存在的任务
	_, err := manager.GetRebalanceTask("non-existent")
	if err == nil {
		t.Error("获取不存在的任务应该返回错误")
	}
}

func TestGetStatus(t *testing.T) {
	manager := NewManager(zap.NewNop())

	status := manager.GetStatus()
	if status == nil {
		t.Fatal("GetStatus 返回 nil")
	}

	if !status.Enabled {
		t.Error("期望 Enabled=true")
	}
}

func TestGetCapacitySuggestion(t *testing.T) {
	manager := NewManager(zap.NewNop())

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

	// 记录一些热数据
	for i := 0; i < 100; i++ {
		manager.RecordAccess(pool.PoolID, "/test/hot.dat", 0, 4096, AccessPatternRandom)
	}

	suggestion, err := manager.GetCapacitySuggestion(pool.PoolID)
	if err != nil {
		t.Fatalf("获取容量建议失败: %v", err)
	}

	if suggestion == nil {
		t.Fatal("容量建议不应为 nil")
	}

	if suggestion.FlashRatio < 0 || suggestion.FlashRatio > 1 {
		t.Errorf("Flash 比例应在 0-1 之间，实际 %f", suggestion.FlashRatio)
	}

	if suggestion.Reason == "" {
		t.Error("建议理由不应为空")
	}

	// 测试不存在的池
	_, err = manager.GetCapacitySuggestion("non-existent")
	if err == nil {
		t.Error("不存在的池应返回错误")
	}
}

func TestGetPerTierMetrics(t *testing.T) {
	manager := NewManager(zap.NewNop())

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

	// 记录访问
	manager.RecordAccess(pool.PoolID, "/test/data.dat", 0, 4096, AccessPatternRandom)

	iops, throughput, latency, err := manager.GetPerTierMetrics(pool.PoolID)
	if err != nil {
		t.Fatalf("获取分层指标失败: %v", err)
	}

	if iops == nil {
		t.Error("IOPS 统计不应为 nil")
	}
	if throughput == nil {
		t.Error("吞吐统计不应为 nil")
	}
	if latency == nil {
		t.Error("延迟统计不应为 nil")
	}

	// 测试不存在的池
	_, _, _, err = manager.GetPerTierMetrics("non-existent")
	if err == nil {
		t.Error("不存在的池应返回错误")
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewManager(zap.NewNop())

	config := &HybridPoolConfig{
		PoolName:     "concurrent-pool",
		FlashDevices: []string{"/dev/nvme0n1"},
		HDDDevices:   []string{"/dev/sda"},
		FlashRole:    FlashRoleData,
		TierPolicy:   DefaultTierPolicy(),
	}

	pool, err := manager.CreatePool(config)
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	// 并发读写
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				manager.RecordAccess(pool.PoolID, "/test/file.dat", 0, 1024, AccessPatternRandom)
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据一致性
	pools := manager.ListPools()
	if len(pools) != 1 {
		t.Errorf("期望 1 个池，实际 %d", len(pools))
	}
}

// ========== ML Tiering Engine 测试 ==========

func TestNewMLTieringEngine(t *testing.T) {
	engine := NewMLTieringEngine(nil)

	if engine == nil {
		t.Fatal("NewMLTieringEngine 返回 nil")
	}

	if !engine.config.Enabled {
		t.Error("期望 Enabled=true")
	}

	if engine.model == nil {
		t.Error("model 未初始化")
	}

	if engine.featureStore == nil {
		t.Error("featureStore 未初始化")
	}
}

func TestMLTieringEnginePredict(t *testing.T) {
	engine := NewMLTieringEngine(DefaultMLTieringConfig())

	// 更新特征
	now := time.Now()
	for i := 0; i < 50; i++ {
		engine.UpdateFeatures("block-1", now.Add(-time.Duration(i)*time.Minute), 4096, AccessPatternRandom, true)
	}

	// 预测
	result := engine.Predict("block-1")

	if result == nil {
		t.Fatal("Predict 返回 nil")
	}

	if result.BlockID != "block-1" {
		t.Errorf("期望 BlockID=block-1, 实际 %s", result.BlockID)
	}

	if result.HotProbability < 0 || result.HotProbability > 1 {
		t.Errorf("HotProbability 应在 0-1 之间, 实际 %f", result.HotProbability)
	}

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence 应在 0-1 之间, 实际 %f", result.Confidence)
	}
}

func TestMLTieringEngineNoData(t *testing.T) {
	engine := NewMLTieringEngine(DefaultMLTieringConfig())

	// 无数据预测
	result := engine.Predict("non-existent")

	if result.HotProbability != 0.0 {
		t.Errorf("无数据时 HotProbability 应为 0, 实际 %f", result.HotProbability)
	}

	if result.Confidence != 0.0 {
		t.Errorf("无数据时 Confidence 应为 0, 实际 %f", result.Confidence)
	}
}

func TestMLTieringEngineTrain(t *testing.T) {
	engine := NewMLTieringEngine(DefaultMLTieringConfig())

	// 生成训练样本
	samples := make([]*TrainingSample, 200)
	for i := 0; i < 200; i++ {
		label := 0.0
		if i%2 == 0 {
			label = 1.0
		}
		samples[i] = &TrainingSample{
			Features: []float64{
				float64(i) / 200.0,
				float64(i) / 100.0,
				0.5,
				1.0,
				float64(i) / 50.0,
			},
			Label:     label,
			Timestamp: time.Now(),
			BlockID:   fmt.Sprintf("block-%d", i),
		}
	}

	// 训练
	err := engine.Train(samples)
	if err != nil {
		t.Fatalf("训练失败: %v", err)
	}

	// 检查模型更新
	model := engine.GetModel()
	if model.Version == 0 {
		t.Error("模型版本应大于 0")
	}

	if model.TrainedAt.IsZero() {
		t.Error("TrainedAt 不应为零值")
	}
}

func TestMLTieringEngineCollectSample(t *testing.T) {
	engine := NewMLTieringEngine(DefaultMLTieringConfig())

	// 先更新特征
	now := time.Now()
	for i := 0; i < 20; i++ {
		engine.UpdateFeatures("block-1", now.Add(-time.Duration(i)*time.Minute), 4096, AccessPatternRandom, true)
	}

	// 收集训练样本
	engine.CollectTrainingSample("block-1", true)

	if engine.GetTrainingDataSize() != 1 {
		t.Errorf("期望训练数据大小=1, 实际 %d", engine.GetTrainingDataSize())
	}
}

func TestMLTieringEngineStats(t *testing.T) {
	engine := NewMLTieringEngine(DefaultMLTieringConfig())

	stats := engine.GetStats()

	if stats == nil {
		t.Fatal("GetStats 返回 nil")
	}

	if enabled, ok := stats["enabled"]; !ok || !enabled.(bool) {
		t.Error("期望 enabled=true")
	}
}

// ========== Smart Data Placer 测试 ==========

func TestNewSmartDataPlacer(t *testing.T) {
	logger := zap.NewNop()
	mlEngine := NewMLTieringEngine(DefaultMLTieringConfig())
	placer := NewSmartDataPlacer(logger, mlEngine, nil)

	if placer == nil {
		t.Fatal("NewSmartDataPlacer 返回 nil")
	}

	if !placer.config.Enabled {
		t.Error("期望 Enabled=true")
	}
}

func TestSmartDataPlacerAnalyze(t *testing.T) {
	logger := zap.NewNop()
	mlEngine := NewMLTieringEngine(DefaultMLTieringConfig())
	placer := NewSmartDataPlacer(logger, mlEngine, DefaultPlacerConfig())

	// 创建测试块
	blocks := []*BlockAccessRecord{
		{
			BlockID:     "block-1",
			PoolID:      "pool-1",
			CurrentTier: FlashTypeHDD,
			HeatLevel:   HeatLevelHot,
			AccessCount: 100,
			Size:        4096,
		},
		{
			BlockID:     "block-2",
			PoolID:      "pool-1",
			CurrentTier: FlashTypeNVMe,
			HeatLevel:   HeatLevelCold,
			AccessCount: 1,
			Size:        1024 * 1024,
		},
	}

	// 更新 ML 特征
	now := time.Now()
	for i := 0; i < 50; i++ {
		mlEngine.UpdateFeatures("block-1", now.Add(-time.Duration(i)*time.Minute), 4096, AccessPatternRandom, true)
	}

	decisions := placer.AnalyzeAndPlace("pool-1", blocks)

	if decisions == nil {
		t.Fatal("AnalyzeAndPlace 返回 nil")
	}

	t.Logf("生成了 %d 个放置决策", len(decisions))
}

func TestSmartDataPlacerStats(t *testing.T) {
	logger := zap.NewNop()
	mlEngine := NewMLTieringEngine(DefaultMLTieringConfig())
	placer := NewSmartDataPlacer(logger, mlEngine, DefaultPlacerConfig())

	stats := placer.GetStats()

	if stats == nil {
		t.Fatal("GetStats 返回 nil")
	}

	if stats.TotalPlacements != 0 {
		t.Errorf("期望 TotalPlacements=0, 实际 %d", stats.TotalPlacements)
	}
}

func TestSmartDataPlacerActivePlacements(t *testing.T) {
	logger := zap.NewNop()
	mlEngine := NewMLTieringEngine(DefaultMLTieringConfig())
	placer := NewSmartDataPlacer(logger, mlEngine, DefaultPlacerConfig())

	placements := placer.GetActivePlacements()

	if len(placements) != 0 {
		t.Errorf("期望 0 个活跃放置, 实际 %d", len(placements))
	}
}

// ========== SLOG Manager 测试 ==========

func TestNewSLOGManager(t *testing.T) {
	logger := zap.NewNop()
	manager := NewSLOGManager(logger, nil)

	if manager == nil {
		t.Fatal("NewSLOGManager 返回 nil")
	}

	if !manager.config.Enabled {
		t.Error("期望 Enabled=true")
	}
}

func TestSLOGManagerRegisterDevice(t *testing.T) {
	logger := zap.NewNop()
	manager := NewSLOGManager(logger, DefaultSLOGConfig())

	device := &SLOGDevice{
		ID:       "slog-1",
		Name:     "NVMe SLOG",
		Path:     "/dev/nvme0n1",
		Type:     FlashTypeNVMe,
		Capacity: 100 * 1024 * 1024 * 1024, // 100GB
		Role:     FlashRoleSLOG,
		Health:   100.0,
	}

	err := manager.RegisterDevice(device)
	if err != nil {
		t.Fatalf("注册设备失败: %v", err)
	}

	devices := manager.GetDevices()
	if len(devices) != 1 {
		t.Errorf("期望 1 个设备, 实际 %d", len(devices))
	}

	// 重复注册应该失败
	err = manager.RegisterDevice(device)
	if err == nil {
		t.Error("重复注册应该返回错误")
	}
}

func TestSLOGManagerUnregisterDevice(t *testing.T) {
	logger := zap.NewNop()
	manager := NewSLOGManager(logger, DefaultSLOGConfig())

	device := &SLOGDevice{
		ID:       "slog-1",
		Name:     "NVMe SLOG",
		Path:     "/dev/nvme0n1",
		Type:     FlashTypeNVMe,
		Capacity: 100 * 1024 * 1024 * 1024,
		Role:     FlashRoleSLOG,
		Health:   100.0,
	}

	manager.RegisterDevice(device)

	err := manager.UnregisterDevice("slog-1")
	if err != nil {
		t.Fatalf("注销设备失败: %v", err)
	}

	devices := manager.GetDevices()
	if len(devices) != 0 {
		t.Errorf("期望 0 个设备, 实际 %d", len(devices))
	}

	// 注销不存在的设备应该失败
	err = manager.UnregisterDevice("non-existent")
	if err == nil {
		t.Error("注销不存在的设备应该返回错误")
	}
}

func TestSLOGManagerWrite(t *testing.T) {
	logger := zap.NewNop()
	manager := NewSLOGManager(logger, DefaultSLOGConfig())

	device := &SLOGDevice{
		ID:       "slog-1",
		Name:     "NVMe SLOG",
		Path:     "/dev/nvme0n1",
		Type:     FlashTypeNVMe,
		Capacity: 100 * 1024 * 1024 * 1024,
		Role:     FlashRoleSLOG,
		Health:   100.0,
	}

	manager.RegisterDevice(device)

	write, err := manager.Write("pool-1", 0, 4096, []byte("test data"), "standard")
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if write == nil {
		t.Fatal("Write 返回 nil")
	}

	if write.PoolID != "pool-1" {
		t.Errorf("期望 PoolID=pool-1, 实际 %s", write.PoolID)
	}

	stats := manager.GetStats()
	if stats.TotalWrites != 1 {
		t.Errorf("期望 TotalWrites=1, 实际 %d", stats.TotalWrites)
	}
}

func TestSLOGManagerHealth(t *testing.T) {
	logger := zap.NewNop()
	manager := NewSLOGManager(logger, DefaultSLOGConfig())

	device := &SLOGDevice{
		ID:       "slog-1",
		Name:     "NVMe SLOG",
		Path:     "/dev/nvme0n1",
		Type:     FlashTypeNVMe,
		Capacity: 100 * 1024 * 1024 * 1024,
		Role:     FlashRoleSLOG,
		Health:   100.0,
	}

	manager.RegisterDevice(device)

	health := manager.CheckHealth()

	if health == nil {
		t.Fatal("CheckHealth 返回 nil")
	}

	if status, ok := health["status"]; !ok || status != "healthy" {
		t.Errorf("期望 status=healthy, 实际 %v", health["status"])
	}
}

// ========== Metadata Optimizer 测试 ==========

func TestNewMetadataOptimizer(t *testing.T) {
	logger := zap.NewNop()
	optimizer := NewMetadataOptimizer(logger, nil)

	if optimizer == nil {
		t.Fatal("NewMetadataOptimizer 返回 nil")
	}

	if !optimizer.config.Enabled {
		t.Error("期望 Enabled=true")
	}
}

func TestMetadataOptimizerShouldUseFlash(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultMetadataConfig()
	optimizer := NewMetadataOptimizer(logger, config)

	// 小文件应该使用 flash
	if !optimizer.ShouldUseFlash("/test/small.dat", 1024, false) {
		t.Error("小文件应该使用 flash")
	}

	// 大文件不应该使用 flash
	if optimizer.ShouldUseFlash("/test/large.dat", 10*1024*1024, false) {
		t.Error("大文件不应该使用 flash")
	}

	// 元数据应该使用 flash
	if !optimizer.ShouldUseFlash("/test/metadata", 1024, true) {
		t.Error("元数据应该使用 flash")
	}
}

func TestMetadataOptimizerRecordAccess(t *testing.T) {
	logger := zap.NewNop()
	optimizer := NewMetadataOptimizer(logger, DefaultMetadataConfig())

	optimizer.RecordAccess("/test/file1.dat", 1024, false)
	optimizer.RecordAccess("/test/file2.dat", 2048, true)

	stats := optimizer.GetStats()

	if stats.TotalEntries != 2 {
		t.Errorf("期望 TotalEntries=2, 实际 %d", stats.TotalEntries)
	}

	if stats.SmallFileCount != 2 {
		t.Errorf("期望 SmallFileCount=2, 实际 %d", stats.SmallFileCount)
	}

	if stats.MetadataFileCount != 1 {
		t.Errorf("期望 MetadataFileCount=1, 实际 %d", stats.MetadataFileCount)
	}
}

func TestMetadataOptimizerRecommendation(t *testing.T) {
	logger := zap.NewNop()
	optimizer := NewMetadataOptimizer(logger, DefaultMetadataConfig())

	// 小文件推荐 NVMe
	tier := optimizer.GetRecommendation("/test/small.dat", 1024, false)
	if tier != FlashTypeNVMe {
		t.Errorf("期望 NVMe, 实际 %s", tier)
	}

	// 大文件推荐 HDD
	tier = optimizer.GetRecommendation("/test/large.dat", 10*1024*1024, false)
	if tier != FlashTypeHDD {
		t.Errorf("期望 HDD, 实际 %s", tier)
	}
}

// ========== Cost Analyzer 测试 ==========

func TestNewCostAnalyzer(t *testing.T) {
	logger := zap.NewNop()
	analyzer := NewCostAnalyzer(logger, nil)

	if analyzer == nil {
		t.Fatal("NewCostAnalyzer 返回 nil")
	}

	if !analyzer.config.Enabled {
		t.Error("期望 Enabled=true")
	}
}

func TestCostAnalyzerAnalyzeSchemes(t *testing.T) {
	logger := zap.NewNop()
	analyzer := NewCostAnalyzer(logger, DefaultCostConfig())

	results := analyzer.AnalyzeTieringSchemes(100.0, 0.2)

	if results == nil {
		t.Fatal("AnalyzeTieringSchemes 返回 nil")
	}

	if len(results) != 5 {
		t.Errorf("期望 5 个方案, 实际 %d", len(results))
	}

	// 验证每个方案都有成本
	for _, r := range results {
		if r.TotalCost <= 0 {
			t.Errorf("方案 %s 的总成本应大于 0", r.Scenario)
		}
		if r.Performance == nil {
			t.Errorf("方案 %s 的性能估算不应为 nil", r.Scenario)
		}
	}

	// 检查是否有推荐方案
	foundRecommended := false
	for _, r := range results {
		for _, rec := range r.Recommendations {
			if rec == "★ 推荐方案: 性价比最高" {
				foundRecommended = true
			}
		}
	}

	if !foundRecommended {
		t.Error("应有推荐方案")
	}
}

func TestCostAnalyzerEstimateSavings(t *testing.T) {
	logger := zap.NewNop()
	analyzer := NewCostAnalyzer(logger, DefaultCostConfig())

	current := &CostAnalysisResult{
		Scenario:   "全 HDD",
		TotalCost:  1000.0,
		Performance: &PerformanceEst{AvgLatency: 5.0},
	}

	optimal := &CostAnalysisResult{
		Scenario:   "NVMe + HDD 混合",
		TotalCost:  600.0,
		Performance: &PerformanceEst{AvgLatency: 1.0},
	}

	savings := analyzer.EstimateCostSavings(current, optimal)

	if savings == nil {
		t.Fatal("EstimateCostSavings 返回 nil")
	}

	if savings["savings"] != 400.0 {
		t.Errorf("期望节省 400, 实际 %v", savings["savings"])
	}

	if savings["savingsPercent"] != 40.0 {
		t.Errorf("期望节省 40%%, 实际 %v%%", savings["savingsPercent"])
	}
}

// ========== 集成测试 ==========

func TestIntegrationMLTieringWithPlacer(t *testing.T) {
	logger := zap.NewNop()
	mlEngine := NewMLTieringEngine(DefaultMLTieringConfig())
	placer := NewSmartDataPlacer(logger, mlEngine, DefaultPlacerConfig())

	// 模拟数据访问
	now := time.Now()
	for i := 0; i < 100; i++ {
		mlEngine.UpdateFeatures("hot-block", now.Add(-time.Duration(i)*time.Minute), 4096, AccessPatternRandom, true)
	}

	// 预测
	prediction := mlEngine.Predict("hot-block")
	if prediction == nil {
		t.Fatal("预测失败")
	}

	t.Logf("热数据概率: %.2f, 置信度: %.2f", prediction.HotProbability, prediction.Confidence)

	// 生成放置决策
	blocks := []*BlockAccessRecord{
		{
			BlockID:     "hot-block",
			CurrentTier: FlashTypeHDD,
			HeatLevel:   HeatLevelHot,
			AccessCount: 100,
			Size:        4096,
		},
	}

	decisions := placer.AnalyzeAndPlace("pool-1", blocks)
	t.Logf("生成了 %d 个放置决策", len(decisions))
}

func TestIntegrationSLOGWithMetadata(t *testing.T) {
	logger := zap.NewNop()

	slogManager := NewSLOGManager(logger, DefaultSLOGConfig())
	metadataOptimizer := NewMetadataOptimizer(logger, DefaultMetadataConfig())

	// 注册 SLOG 设备
	device := &SLOGDevice{
		ID:       "slog-1",
		Name:     "NVMe SLOG",
		Path:     "/dev/nvme0n1",
		Type:     FlashTypeNVMe,
		Capacity: 100 * 1024 * 1024 * 1024,
		Role:     FlashRoleSLOG,
		Health:   100.0,
	}

	slogManager.RegisterDevice(device)

	// 写入 SLOG
	_, err := slogManager.Write("pool-1", 0, 4096, []byte("test"), "standard")
	if err != nil {
		t.Fatalf("SLOG 写入失败: %v", err)
	}

	// 记录元数据访问
	metadataOptimizer.RecordAccess("/test/metadata", 512, true)

	// 检查推荐
	tier := metadataOptimizer.GetRecommendation("/test/metadata", 512, true)
	if tier != FlashTypeNVMe {
		t.Errorf("元数据应推荐 NVMe, 实际 %s", tier)
	}
}

func TestIntegrationCostAnalysis(t *testing.T) {
	logger := zap.NewNop()
	analyzer := NewCostAnalyzer(logger, DefaultCostConfig())

	// 分析不同容量下的方案
	capacities := []float64{10, 50, 100, 500}
	for _, cap := range capacities {
		results := analyzer.AnalyzeTieringSchemes(cap, 0.2)
		if len(results) == 0 {
			t.Errorf("容量 %.0f TB 无分析结果", cap)
		}

		t.Logf("容量 %.0f TB: 最优方案成本 $%.2f", cap, results[0].TotalCost)
	}
}
