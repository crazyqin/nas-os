package hybridflash

import (
	"testing"
	"time"
)

func TestNewTieringEngine(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()

	engine := NewTieringEngine(config, heatConfig)

	if engine == nil {
		t.Fatal("NewTieringEngine 返回 nil")
	}

	if engine.config.Enabled != config.Enabled {
		t.Errorf("期望 Enabled=%v, 实际 %v", config.Enabled, engine.config.Enabled)
	}

	if engine.heatConfig.WindowSize != heatConfig.WindowSize {
		t.Errorf("期望 WindowSize=%v, 实际 %v", heatConfig.WindowSize, engine.heatConfig.WindowSize)
	}
}

func TestDefaultTieringConfig(t *testing.T) {
	config := DefaultTieringConfig()

	if config.CheckInterval != "5m" {
		t.Errorf("期望 CheckInterval=5m, 实际 %v", config.CheckInterval)
	}

	if config.MaxConcurrentMigrates != 4 {
		t.Errorf("期望 MaxConcurrentMigrates=4, 实际 %d", config.MaxConcurrentMigrates)
	}

	if config.SSDCapacityThreshold != 0.85 {
		t.Errorf("期望 SSDCapacityThreshold=0.85, 实际 %f", config.SSDCapacityThreshold)
	}
}

func TestDefaultHeatTrackingConfig(t *testing.T) {
	config := DefaultHeatTrackingConfig()

	if config.WindowSize != "1h" {
		t.Errorf("期望 WindowSize=1h, 实际 %v", config.WindowSize)
	}

	if config.DecayFactor != 0.95 {
		t.Errorf("期望 DecayFactor=0.95, 实际 %f", config.DecayFactor)
	}

	if config.MaxTrackedBlocks != 100000 {
		t.Errorf("期望 MaxTrackedBlocks=100000, 实际 %d", config.MaxTrackedBlocks)
	}
}

func TestRecordBlockAccess(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 记录块访问
	engine.RecordBlockAccess("block-1", "/data/file1.dat", 0, 4096, AccessPatternRandom)

	block, err := engine.GetBlockHeatInfo("block-1")
	if err != nil {
		t.Fatalf("获取块信息失败: %v", err)
	}

	if block.BlockID != "block-1" {
		t.Errorf("期望 BlockID=block-1, 实际 %s", block.BlockID)
	}

	if block.AccessCount != 1 {
		t.Errorf("期望 AccessCount=1, 实际 %d", block.AccessCount)
	}

	if block.Size != 4096 {
		t.Errorf("期望 Size=4096, 实际 %d", block.Size)
	}
}

func TestHeatLevelCalculation(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 记录多次访问以达到热数据阈值
	for i := 0; i < 100; i++ {
		engine.RecordBlockAccess("block-hot", "/data/hot.dat", 0, 4096, AccessPatternRandom)
	}

	block, _ := engine.GetBlockHeatInfo("block-hot")
	block.AccessTime = time.Now() // 确保最近访问

	heatLevel := engine.calculateHeatLevel(block, time.Now())
	if heatLevel != HeatLevelHot {
		t.Errorf("期望热度级别=hot, 实际 %s", heatLevel)
	}
}

func TestTieringEngineGetStatus(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	status := engine.GetStatus()

	if status == nil {
		t.Fatal("GetStatus 返回 nil")
	}

	if status.Enabled != config.Enabled {
		t.Errorf("期望 Enabled=%v, 实际 %v", config.Enabled, status.Enabled)
	}
}

func TestGenerateEfficiencyReport(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 添加一些测试数据
	for i := 0; i < 50; i++ {
		engine.RecordBlockAccess("block-hot-1", "/data/hot1.dat", 0, 4096, AccessPatternRandom)
	}
	for i := 0; i < 10; i++ {
		engine.RecordBlockAccess("block-warm-1", "/data/warm1.dat", 0, 4096, AccessPatternRandom)
	}
	engine.RecordBlockAccess("block-cold-1", "/data/cold1.dat", 0, 4096, AccessPatternRandom)

	report := engine.GenerateEfficiencyReport("daily")

	if report == nil {
		t.Fatal("GenerateEfficiencyReport 返回 nil")
	}

	if report.Period != "daily" {
		t.Errorf("期望 Period=daily, 实际 %s", report.Period)
	}

	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt 为零值")
	}
}

func TestGetHotBlocks(t *testing.T) {
	config := DefaultTieringConfig()
	config.HotThreshold = 50 // 降低阈值以便测试
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 创建热块
	for i := 0; i < 5; i++ {
		blockID := "hot-block-" + string(rune('a'+i))
		for j := 0; j < 60; j++ {
			engine.RecordBlockAccess(blockID, "/data/"+blockID+".dat", 0, 4096, AccessPatternRandom)
		}
	}

	// 创建冷块
	engine.RecordBlockAccess("cold-block", "/data/cold.dat", 0, 4096, AccessPatternRandom)

	hotBlocks := engine.GetHotBlocks(10)
	if len(hotBlocks) == 0 {
		t.Error("期望有热块，实际为空")
	}
}

func TestGetColdBlocks(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 创建冷块
	engine.RecordBlockAccess("cold-block-1", "/data/cold1.dat", 0, 4096, AccessPatternRandom)
	engine.RecordBlockAccess("cold-block-2", "/data/cold2.dat", 0, 4096, AccessPatternRandom)

	// 设置访问时间为很久以前（超过7天才变cold）
	engine.blockTracker.mu.Lock()
	if block, ok := engine.blockTracker.blocks["cold-block-1"]; ok {
		block.AccessTime = time.Now().Add(-30 * 24 * time.Hour) // 30 天前
		block.AccessCount = 1
		block.HeatLevel = engine.calculateHeatLevel(block, time.Now())
	}
	engine.blockTracker.mu.Unlock()

	coldBlocks := engine.GetColdBlocks(10)
	if len(coldBlocks) == 0 {
		t.Error("期望有冷块，实际为空")
	}
}

func TestPoolRegistration(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	pool := &HybridPool{
		ID:   "test-pool",
		Name: "测试池",
		State: PoolStateOnline,
	}

	engine.RegisterPool(pool)

	retrieved, err := engine.GetPool("test-pool")
	if err != nil {
		t.Fatalf("获取池失败: %v", err)
	}

	if retrieved.ID != "test-pool" {
		t.Errorf("期望 ID=test-pool, 实际 %s", retrieved.ID)
	}

	engine.UnregisterPool("test-pool")

	_, err = engine.GetPool("test-pool")
	if err == nil {
		t.Error("期望 GetPool 返回错误，实际未返回")
	}
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	newConfig := TieringConfig{
		Enabled:               true,
		CheckInterval:         "10m",
		MaxConcurrentMigrates: 8,
		SSDCapacityThreshold:  0.9,
	}

	engine.UpdateConfig(newConfig)

	if engine.config.CheckInterval != "10m" {
		t.Errorf("期望 CheckInterval=10m, 实际 %v", engine.config.CheckInterval)
	}

	if engine.config.MaxConcurrentMigrates != 8 {
		t.Errorf("期望 MaxConcurrentMigrates=8, 实际 %d", engine.config.MaxConcurrentMigrates)
	}
}

func TestBlockAccessTracking(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 记录多次访问
	for i := 0; i < 50; i++ {
		engine.RecordBlockAccess("block-1", "/data/file.dat", 0, 4096, AccessPatternRandom)
	}

	block, err := engine.GetBlockHeatInfo("block-1")
	if err != nil {
		t.Fatalf("获取块信息失败: %v", err)
	}

	if block.AccessCount != 50 {
		t.Errorf("期望 AccessCount=50, 实际 %d", block.AccessCount)
	}

	if block.ReadBytes != 50*4096 {
		t.Errorf("期望 ReadBytes=%d, 实际 %d", 50*4096, block.ReadBytes)
	}
}

func TestHeatThresholds(t *testing.T) {
	config := DefaultTieringConfig()
	heatConfig := DefaultHeatTrackingConfig()
	engine := NewTieringEngine(config, heatConfig)

	// 测试不同访问次数对应的热度级别
	testCases := []struct {
		accessCount int64
		expected    DataHeatLevel
	}{
		{200, HeatLevelHot},
		{50, HeatLevelHot},
		{20, HeatLevelHot},  // >= 10 且最近访问，应为 hot
		{5, HeatLevelWarm},  // < 10 且最近访问，应为 warm
		{1, HeatLevelWarm},  // < 10 且最近访问，应为 warm
	}

	for _, tc := range testCases {
		block := &BlockAccessRecord{
			BlockID:     "test-block",
			AccessCount: tc.accessCount,
			AccessTime:  time.Now(),
		}

		level := engine.calculateHeatLevel(block, time.Now())
		if level != tc.expected {
			t.Errorf("访问次数 %d: 期望热度=%s, 实际=%s", tc.accessCount, tc.expected, level)
		}
	}
}

func TestMigrateTaskCreation(t *testing.T) {
	task := &MigrateTask{
		ID:         "test-task",
		Status:     MigrateStatusPending,
		SourceTier: FlashTypeHDD,
		TargetTier: FlashTypeSSD,
		BlockSize:  4096,
		TotalBlocks: 100,
		TotalBytes:  409600,
	}

	if task.ID != "test-task" {
		t.Errorf("期望 ID=test-task, 实际 %s", task.ID)
	}

	if task.Status != MigrateStatusPending {
		t.Errorf("期望 Status=pending, 实际 %s", task.Status)
	}

	if task.SourceTier != FlashTypeHDD {
		t.Errorf("期望 SourceTier=hdd, 实际 %s", task.SourceTier)
	}
}

func TestCachePolicyCreation(t *testing.T) {
	policy := &CachePolicy{
		ID:            "test-policy",
		Name:          "测试策略",
		Enabled:       true,
		CacheRole:     CacheRoleL2ARC,
		HeatLevel:     HeatLevelHot,
		AccessPattern: AccessPatternRandom,
		MinBlockSize:  4096,
		MaxBlockSize:  1048576,
		Priority:      100,
		PreferSSD:     true,
	}

	if policy.CacheRole != CacheRoleL2ARC {
		t.Errorf("期望 CacheRole=l2arc, 实际 %s", policy.CacheRole)
	}

	if policy.HeatLevel != HeatLevelHot {
		t.Errorf("期望 HeatLevel=hot, 实际 %s", policy.HeatLevel)
	}
}

func TestHybridPoolCreation(t *testing.T) {
	pool := &HybridPool{
		ID:   "test-pool",
		Name: "测试混合池",
		State: PoolStateOnline,
		FlashDevices: []*FlashDevice{
			{
				ID:        "ssd-1",
				Name:      "SSD 缓存盘",
				Type:      FlashTypeSSD,
				CacheRole: CacheRoleL2ARC,
				Capacity:  1024 * 1024 * 1024 * 500, // 500GB
			},
		},
		HDDDevices: []*HDDDevice{
			{
				ID:       "hdd-1",
				Name:     "HDD 存储盘",
				Capacity: 1024 * 1024 * 1024 * 1024 * 4, // 4TB
				RPM:      7200,
			},
		},
	}

	if len(pool.FlashDevices) != 1 {
		t.Errorf("期望 FlashDevices 数量=1, 实际 %d", len(pool.FlashDevices))
	}

	if len(pool.HDDDevices) != 1 {
		t.Errorf("期望 HDDDevices 数量=1, 实际 %d", len(pool.HDDDevices))
	}

	if pool.FlashDevices[0].CacheRole != CacheRoleL2ARC {
		t.Errorf("期望 CacheRole=l2arc, 实际 %s", pool.FlashDevices[0].CacheRole)
	}
}
