// Package hybridpoolmgr 混合存储池管理器测试
package hybridpoolmgr

import (
	"testing"

	"go.uber.org/zap"
)

// newTestManager 创建测试用管理器.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	mgr, err := NewManager(logger, t.TempDir())
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	return mgr
}

// newTestPool 创建测试用池.
func newTestPool(t *testing.T, mgr *Manager, name string) *HybridPool {
	t.Helper()
	pool, err := mgr.CreatePool(&CreatePoolRequest{
		Name:        name,
		Description: "测试池",
		NVMEDevices: []string{"/dev/nvme0n1"},
		SSDDevices:  []string{"/dev/sda"},
		HDDDevices:  []string{"/dev/sdb", "/dev/sdc"},
	})
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}
	return pool
}

// TestNewManager 测试创建管理器.
func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 正常创建
	mgr, err := NewManager(logger, t.TempDir())
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	if mgr == nil {
		t.Fatal("管理器不应为 nil")
	}
	if len(mgr.pools) != 0 {
		t.Errorf("期望 0 个池，实际为 %d", len(mgr.pools))
	}
}

// TestNewManager_NilLogger 测试空 logger.
func TestNewManager_NilLogger(t *testing.T) {
	_, err := NewManager(nil, t.TempDir())
	if err == nil {
		t.Error("期望返回错误")
	}
}

// TestNewManager_EmptyMountBase 测试空挂载路径.
func TestNewManager_EmptyMountBase(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr, err := NewManager(logger, "")
	if err != nil {
		// 在非 root 环境下，默认路径 /mnt/hybrid 可能无法创建，这是预期行为
		t.Logf("创建管理器失败（可能是权限问题）: %v", err)
		return
	}
	if mgr.mountBase != "/mnt/hybrid" {
		t.Errorf("期望默认挂载路径 '/mnt/hybrid'，实际为 '%s'", mgr.mountBase)
	}
}

// TestCreatePool 测试创建池.
func TestCreatePool(t *testing.T) {
	mgr := newTestManager(t)

	pool, err := mgr.CreatePool(&CreatePoolRequest{
		Name:        "test-pool",
		Description: "测试混合池",
		NVMEDevices: []string{"/dev/nvme0n1"},
		SSDDevices:  []string{"/dev/sda"},
		HDDDevices:  []string{"/dev/sdb"},
	})
	if err != nil {
		t.Fatalf("创建池失败: %v", err)
	}

	if pool.Name != "test-pool" {
		t.Errorf("期望 Name 为 'test-pool'，实际为 '%s'", pool.Name)
	}
	if len(pool.NVMEDevices) != 1 {
		t.Errorf("期望 1 个 NVMe 设备，实际为 %d", len(pool.NVMEDevices))
	}
	if len(pool.SSDDevices) != 1 {
		t.Errorf("期望 1 个 SSD 设备，实际为 %d", len(pool.SSDDevices))
	}
	if len(pool.HDDDevices) != 1 {
		t.Errorf("期望 1 个 HDD 设备，实际为 %d", len(pool.HDDDevices))
	}
	if pool.Status != PoolStatusOnline {
		t.Errorf("期望状态为 '%s'，实际为 '%s'", PoolStatusOnline, pool.Status)
	}
	if !pool.Healthy {
		t.Error("期望池为健康状态")
	}
	if pool.TotalBytes == 0 {
		t.Error("总容量不应为 0")
	}
}

// TestCreatePool_Duplicate 测试重复创建池.
func TestCreatePool_Duplicate(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.CreatePool(&CreatePoolRequest{
		Name:       "test-pool",
		HDDDevices: []string{"/dev/sdb"},
	})
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}

	_, err = mgr.CreatePool(&CreatePoolRequest{
		Name:       "test-pool",
		HDDDevices: []string{"/dev/sdc"},
	})
	if err == nil {
		t.Error("期望重复创建返回错误")
	}
}

// TestCreatePool_NoHDD 测试无 HDD 设备.
func TestCreatePool_NoHDD(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.CreatePool(&CreatePoolRequest{
		Name:        "test-pool",
		NVMEDevices: []string{"/dev/nvme0n1"},
	})
	if err == nil {
		t.Error("期望无 HDD 设备时返回错误")
	}
}

// TestGetPool 测试获取池.
func TestGetPool(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	pool, err := mgr.GetPool("test-pool")
	if err != nil {
		t.Fatalf("获取池失败: %v", err)
	}
	if pool.Name != "test-pool" {
		t.Errorf("期望 Name 为 'test-pool'，实际为 '%s'", pool.Name)
	}
}

// TestGetPool_NotFound 测试获取不存在的池.
func TestGetPool_NotFound(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.GetPool("non-existent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

// TestListPools 测试列出池.
func TestListPools(t *testing.T) {
	mgr := newTestManager(t)

	pools := mgr.ListPools()
	if len(pools) != 0 {
		t.Errorf("期望 0 个池，实际为 %d", len(pools))
	}

	newTestPool(t, mgr, "pool-1")
	newTestPool(t, mgr, "pool-2")

	pools = mgr.ListPools()
	if len(pools) != 2 {
		t.Errorf("期望 2 个池，实际为 %d", len(pools))
	}
}

// TestDeletePool 测试删除池.
func TestDeletePool(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	err := mgr.DeletePool("test-pool", false)
	if err != nil {
		t.Fatalf("删除池失败: %v", err)
	}

	pools := mgr.ListPools()
	if len(pools) != 0 {
		t.Errorf("期望 0 个池，实际为 %d", len(pools))
	}
}

// TestDeletePool_NotFound 测试删除不存在的池.
func TestDeletePool_NotFound(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.DeletePool("non-existent", false)
	if err == nil {
		t.Error("期望返回错误")
	}
}

// TestAddDevice 测试添加设备.
func TestAddDevice(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	err := mgr.AddDevice("test-pool", &AddDeviceRequest{
		DevicePath: "/dev/sdd",
		Tier:       TierHDD,
	})
	if err != nil {
		t.Fatalf("添加设备失败: %v", err)
	}

	pool, _ := mgr.GetPool("test-pool")
	if len(pool.HDDDevices) != 3 {
		t.Errorf("期望 3 个 HDD 设备，实际为 %d", len(pool.HDDDevices))
	}
}

// TestAddDevice_InvalidTier 测试无效层级.
func TestAddDevice_InvalidTier(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	err := mgr.AddDevice("test-pool", &AddDeviceRequest{
		DevicePath: "/dev/sdd",
		Tier:       "invalid",
	})
	if err == nil {
		t.Error("期望返回错误")
	}
}

// TestRemoveDevice 测试移除设备.
func TestRemoveDevice(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	err := mgr.RemoveDevice("test-pool", "/dev/sdb")
	if err != nil {
		t.Fatalf("移除设备失败: %v", err)
	}

	pool, _ := mgr.GetPool("test-pool")
	if len(pool.HDDDevices) != 1 {
		t.Errorf("期望 1 个 HDD 设备，实际为 %d", len(pool.HDDDevices))
	}
}

// TestRemoveDevice_NotFound 测试移除不存在的设备.
func TestRemoveDevice_NotFound(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	err := mgr.RemoveDevice("test-pool", "/dev/non-existent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

// TestRecordIO 测试记录 IO.
func TestRecordIO(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	// 记录读 IO
	mgr.RecordIO("test-pool", "block-001", "/data/file.dat", TierSSD, 4096, true, 100.5)

	// 记录写 IO
	mgr.RecordIO("test-pool", "block-001", "/data/file.dat", TierSSD, 4096, false, 200.3)

	// 检查块热度
	heat, err := mgr.GetBlockHeat("test-pool", "block-001")
	if err != nil {
		t.Fatalf("获取块热度失败: %v", err)
	}
	if heat.ReadCount != 1 {
		t.Errorf("期望 ReadCount 为 1，实际为 %d", heat.ReadCount)
	}
	if heat.WriteCount != 1 {
		t.Errorf("期望 WriteCount 为 1，实际为 %d", heat.WriteCount)
	}
}

// TestAnalyzeHeat 测试热度分析.
func TestAnalyzeHeat(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	// 添加多个块的 IO 记录
	for i := 0; i < 20; i++ {
		blockID := "block-" + string(rune('A'+i))
		readCount := 20 - i // 递减的读次数
		for j := 0; j < readCount; j++ {
			mgr.RecordIO("test-pool", blockID, "/data/file.dat", TierSSD, 4096, true, 100)
		}
	}

	result, err := mgr.AnalyzeHeat("test-pool", 5)
	if err != nil {
		t.Fatalf("热度分析失败: %v", err)
	}

	if result.PoolName != "test-pool" {
		t.Errorf("期望 PoolName 为 'test-pool'，实际为 '%s'", result.PoolName)
	}
	if result.TotalBlocks != 20 {
		t.Errorf("期望 TotalBlocks 为 20，实际为 %d", result.TotalBlocks)
	}
	if len(result.TopHotBlocks) != 5 {
		t.Errorf("期望 TopHotBlocks 为 5，实际为 %d", len(result.TopHotBlocks))
	}
}

// TestAnalyzeHeat_Empty 测试空池热度分析.
func TestAnalyzeHeat_Empty(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	result, err := mgr.AnalyzeHeat("test-pool", 10)
	if err != nil {
		t.Fatalf("热度分析失败: %v", err)
	}
	if result.TotalBlocks != 0 {
		t.Errorf("期望 TotalBlocks 为 0，实际为 %d", result.TotalBlocks)
	}
}

// TestRunTiering 测试自动分层.
func TestRunTiering(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	// 添加一些冷数据块
	for i := 0; i < 5; i++ {
		blockID := "cold-block-" + string(rune('A'+i))
		mgr.RecordIO("test-pool", blockID, "/data/old.dat", TierNVMe, 4096, true, 100)
	}

	result, err := mgr.RunTiering("test-pool")
	if err != nil {
		t.Fatalf("执行分层失败: %v", err)
	}

	if result.PoolName != "test-pool" {
		t.Errorf("期望 PoolName 为 'test-pool'，实际为 '%s'", result.PoolName)
	}
	if result.StartTime.IsZero() {
		t.Error("StartTime 不应为零值")
	}
	if result.EndTime.IsZero() {
		t.Error("EndTime 不应为零值")
	}
}

// TestRunTiering_Disabled 测试禁用分层.
func TestRunTiering_Disabled(t *testing.T) {
	mgr := newTestManager(t)
	pool, _ := mgr.CreatePool(&CreatePoolRequest{
		Name:       "test-pool",
		HDDDevices: []string{"/dev/sdb"},
		Tiering: &TieringConfig{
			Enabled: false,
		},
	})

	_, err := mgr.RunTiering(pool.Name)
	if err == nil {
		t.Error("期望禁用分层时返回错误")
	}
}

// TestRunRebalance 测试重平衡.
func TestRunRebalance(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	result, err := mgr.RunRebalance("test-pool")
	if err != nil {
		t.Fatalf("执行重平衡失败: %v", err)
	}

	if result.PoolName != "test-pool" {
		t.Errorf("期望 PoolName 为 'test-pool'，实际为 '%s'", result.PoolName)
	}
	if result.BeforeBalance == nil {
		t.Error("BeforeBalance 不应为 nil")
	}
	if result.AfterBalance == nil {
		t.Error("AfterBalance 不应为 nil")
	}
}

// TestCheckHealth 测试健康检查.
func TestCheckHealth(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	health, err := mgr.CheckHealth("test-pool")
	if err != nil {
		t.Fatalf("健康检查失败: %v", err)
	}

	if health.PoolName != "test-pool" {
		t.Errorf("期望 PoolName 为 'test-pool'，实际为 '%s'", health.PoolName)
	}
	if !health.Healthy {
		t.Error("期望池为健康状态")
	}
	if len(health.DeviceHealth) != 4 { // 1 NVMe + 1 SSD + 2 HDD
		t.Errorf("期望 4 个设备健康记录，实际为 %d", len(health.DeviceHealth))
	}
	if health.TierBalance == nil {
		t.Error("TierBalance 不应为 nil")
	}
}

// TestAlerts 测试告警.
func TestAlerts(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	// 添加告警
	mgr.AddAlert("test-pool", "/dev/sda", "设备温度过高", AlertLevelWarning)
	mgr.AddAlert("test-pool", "/dev/sdb", "设备即将故障", AlertLevelCritical)

	// 获取未解决告警
	alerts := mgr.GetAlerts("test-pool", false)
	if len(alerts) != 2 {
		t.Errorf("期望 2 个未解决告警，实际为 %d", len(alerts))
	}

	// 解决告警
	err := mgr.ResolveAlert("test-pool", alerts[0].ID)
	if err != nil {
		t.Fatalf("解决告警失败: %v", err)
	}

	// 检查未解决告警数量
	alerts = mgr.GetAlerts("test-pool", false)
	if len(alerts) != 1 {
		t.Errorf("期望 1 个未解决告警，实际为 %d", len(alerts))
	}

	// 检查已解决告警
	resolved := mgr.GetAlerts("test-pool", true)
	if len(resolved) != 1 {
		t.Errorf("期望 1 个已解决告警，实际为 %d", len(resolved))
	}
}

// TestUpdateTieringConfig 测试更新分层配置.
func TestUpdateTieringConfig(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	newConfig := TieringConfig{
		Enabled:         false,
		HotThreshold:    2000,
		WarmThreshold:   200,
		ColdAgeDays:     60,
		PromotePolicy:   "aggressive",
		DemotePolicy:    "moderate",
		MaxPromoteMBps:  1000,
		MaxDemoteMBps:   500,
		TieringWindow:   "00:00-04:00",
		ScanIntervalMin: 30,
	}

	err := mgr.UpdateTieringConfig("test-pool", newConfig)
	if err != nil {
		t.Fatalf("更新分层配置失败: %v", err)
	}

	pool, _ := mgr.GetPool("test-pool")
	if pool.TieringConfig.HotThreshold != 2000 {
		t.Errorf("期望 HotThreshold 为 2000，实际为 %f", pool.TieringConfig.HotThreshold)
	}
	if pool.TieringConfig.ColdAgeDays != 60 {
		t.Errorf("期望 ColdAgeDays 为 60，实际为 %d", pool.TieringConfig.ColdAgeDays)
	}
}

// TestUpdateRebalancePolicy 测试更新重平衡策略.
func TestUpdateRebalancePolicy(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	newPolicy := RebalancePolicy{
		Enabled:          false,
		ThresholdPercent: 20.0,
		MaxMigrateMBps:   500,
		MinFreePercent:   15.0,
		ScheduleCron:     "0 4 * * 1",
	}

	err := mgr.UpdateRebalancePolicy("test-pool", newPolicy)
	if err != nil {
		t.Fatalf("更新重平衡策略失败: %v", err)
	}

	pool, _ := mgr.GetPool("test-pool")
	if pool.RebalancePolicy.ThresholdPercent != 20.0 {
		t.Errorf("期望 ThresholdPercent 为 20.0，实际为 %f", pool.RebalancePolicy.ThresholdPercent)
	}
}

// TestGetPoolIOStats 测试获取 IO 统计.
func TestGetPoolIOStats(t *testing.T) {
	mgr := newTestManager(t)
	newTestPool(t, mgr, "test-pool")

	stats, err := mgr.GetPoolIOStats("test-pool")
	if err != nil {
		t.Fatalf("获取 IO 统计失败: %v", err)
	}
	if stats == nil {
		t.Fatal("IO 统计不应为 nil")
	}
	if stats.NVMeStats == nil {
		t.Error("NVMeStats 不应为 nil")
	}
	if stats.SSDStats == nil {
		t.Error("SSDStats 不应为 nil")
	}
	if stats.HDDStats == nil {
		t.Error("HDDStats 不应为 nil")
	}
}

// TestHelperFunctions 测试辅助函数.
func TestHelperFunctions(t *testing.T) {
	// 测试 nextHigherTier
	if nextHigherTier(TierHDD) != TierSSD {
		t.Error("HDD 上一层应为 SSD")
	}
	if nextHigherTier(TierSSD) != TierNVMe {
		t.Error("SSD 上一层应为 NVMe")
	}
	if nextHigherTier(TierNVMe) != TierNVMe {
		t.Error("NVMe 上一层应为 NVMe")
	}

	// 测试 nextLowerTier
	if nextLowerTier(TierNVMe) != TierSSD {
		t.Error("NVMe 下一层应为 SSD")
	}
	if nextLowerTier(TierSSD) != TierHDD {
		t.Error("SSD 下一层应为 HDD")
	}
	if nextLowerTier(TierHDD) != TierHDD {
		t.Error("HDD 下一层应为 HDD")
	}

	// 测试 percentUsed
	if percentUsed(500, 1000) != 50.0 {
		t.Errorf("期望 50.0%%，实际为 %f", percentUsed(500, 1000))
	}
	if percentUsed(0, 0) != 0 {
		t.Errorf("期望 0%%，实际为 %f", percentUsed(0, 0))
	}

	// 测试 tierUsage
	devices := []*StorageDevice{
		{UsedBytes: 100, TotalBytes: 1000},
		{UsedBytes: 200, TotalBytes: 2000},
	}
	used, total := tierUsage(devices)
	if used != 300 {
		t.Errorf("期望 used 为 300，实际为 %d", used)
	}
	if total != 3000 {
		t.Errorf("期望 total 为 3000，实际为 %d", total)
	}
}
