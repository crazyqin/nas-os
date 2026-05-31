package storagetiering

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================
// 类型测试
// ============================================================

func TestTierStringAndParse(t *testing.T) {
	tests := []struct {
		tier Tier
		str  string
	}{
		{TierSSD, "ssd"},
		{TierHDD, "hdd"},
		{TierCold, "cold"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.str, tt.tier.String())
		assert.Equal(t, tt.tier, ParseTier(tt.str))
	}
	// 默认值
	assert.Equal(t, TierHDD, ParseTier("unknown"))
}

func TestTemperatureString(t *testing.T) {
	assert.Equal(t, "hot", TempHot.String())
	assert.Equal(t, "warm", TempWarm.String())
	assert.Equal(t, "cold", TempCold.String())
}

func TestMigrationStateString(t *testing.T) {
	assert.Equal(t, "pending", StatePending.String())
	assert.Equal(t, "running", StateRunning.String())
	assert.Equal(t, "paused", StatePaused.String())
	assert.Equal(t, "completed", StateCompleted.String())
	assert.Equal(t, "failed", StateFailed.String())
	assert.Equal(t, "cancelled", StateCancelled.String())
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Len(t, cfg.Tiers, 3)
	assert.Equal(t, TierSSD, cfg.Tiers[0].Tier)
	assert.Equal(t, TierHDD, cfg.Tiers[1].Tier)
	assert.Equal(t, TierCold, cfg.Tiers[2].Tier)
	assert.Greater(t, cfg.Thresholds.HotMinScore, cfg.Thresholds.WarmMinScore)
	assert.Greater(t, cfg.Migrator.MaxConcurrent, 0)
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	assert.NoError(t, cfg.Validate())

	// 测试无效配置：空层级
	bad := cfg
	bad.Tiers = nil
	assert.Error(t, bad.Validate())

	// 测试无效配置：阈值倒置
	bad2 := cfg
	bad2.Thresholds.HotMinScore = 10
	bad2.Thresholds.WarmMinScore = 50
	assert.Error(t, bad2.Validate())

	// 测试无效配置：容量阈值倒置
	bad3 := cfg
	bad3.Policy.CapacityHighPct = 0.5
	bad3.Policy.CapacityLowPct = 0.8
	assert.Error(t, bad3.Validate())

	// 测试无效配置：并发数为0
	bad4 := cfg
	bad4.Migrator.MaxConcurrent = 0
	assert.Error(t, bad4.Validate())
}

// ============================================================
// 分析器测试
// ============================================================

func TestAnalyzerHeatScore(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewAnalyzer(config.Analyzer, config.Policy, zap.NewNop())

	now := time.Now()

	// 注册热文件
	hotFile := FileEntry{
		Path:        "/data/database.db",
		Size:        1024 * 1024 * 10,
		CurrentTier: TierSSD,
		AccessedAt:  now,
		CreatedAt:   now.AddDate(0, 0, -5),
		AccessCount: 200,
		ReadCount:   150,
		WriteCount:  50,
	}
	analyzer.RegisterFile(hotFile)

	// 添加访问记录
	for i := 0; i < 10; i++ {
		analyzer.RecordAccess(AccessRecord{
			Path:      "/data/database.db",
			OpType:    "read",
			Size:      1024 * 1024,
			Timestamp: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	// 注册冷文件
	coldFile := FileEntry{
		Path:        "/archive/old_backup.tar.gz",
		Size:        1024 * 1024 * 500,
		CurrentTier: TierHDD,
		AccessedAt:  now.AddDate(0, 0, -60),
		CreatedAt:   now.AddDate(0, -6, 0),
		AccessCount: 2,
		ReadCount:   2,
		WriteCount:  0,
	}
	analyzer.RegisterFile(coldFile)

	// 注册温文件
	warmFile := FileEntry{
		Path:        "/data/report.pdf",
		Size:        1024 * 1024 * 5,
		CurrentTier: TierHDD,
		AccessedAt:  now.AddDate(0, 0, -3),
		CreatedAt:   now.AddDate(0, 0, -10),
		AccessCount: 30,
		ReadCount:   25,
		WriteCount:  5,
	}
	analyzer.RegisterFile(warmFile)

	// 执行分析
	tasks, err := analyzer.Analyze(context.Background())
	require.NoError(t, err)

	// 验证热文件热度高
	heat1, ok := analyzer.GetFileHeat("/data/database.db")
	assert.True(t, ok)
	assert.Greater(t, heat1, 50.0, "DB file should have high heat score")

	// 验证冷文件热度低
	heat2, ok := analyzer.GetFileHeat("/archive/old_backup.tar.gz")
	assert.True(t, ok)
	assert.Less(t, heat2, 30.0, "Old backup should have low heat score")

	// 验证热文件分数高于冷文件
	assert.Greater(t, heat1, heat2, "Hot file should have higher heat than cold file")

	// 验证分析产生了迁移候选
	assert.NotNil(t, tasks)
}

func TestAnalyzerTemperatureClassification(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewAnalyzer(config.Analyzer, config.Policy, zap.NewNop())

	// 测试温度分类
	assert.Equal(t, TempHot, analyzer.classifyTemperature(80.0))
	assert.Equal(t, TempWarm, analyzer.classifyTemperature(50.0))
	assert.Equal(t, TempCold, analyzer.classifyTemperature(10.0))
	assert.Equal(t, TempHot, analyzer.classifyTemperature(70.0))  // 边界值
	assert.Equal(t, TempWarm, analyzer.classifyTemperature(30.0)) // 边界值
}

func TestAnalyzerFileRegistration(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig().Analyzer, DefaultConfig().Policy, zap.NewNop())

	entry := FileEntry{
		Path:       "/test/file.txt",
		Size:       1024,
		CurrentTier: TierHDD,
	}
	analyzer.RegisterFile(entry)

	got, ok := analyzer.GetFileEntry("/test/file.txt")
	assert.True(t, ok)
	assert.Equal(t, "/test/file.txt", got.Path)
	assert.Equal(t, int64(1024), got.Size)

	_, ok = analyzer.GetFileEntry("/nonexistent")
	assert.False(t, ok)
}

func TestAnalyzerHitRate(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig().Analyzer, DefaultConfig().Policy, zap.NewNop())

	assert.Equal(t, float64(0), analyzer.HitRate())

	analyzer.RecordHit()
	analyzer.RecordHit()
	analyzer.RecordMiss()

	assert.InDelta(t, 2.0/3.0, analyzer.HitRate(), 0.001)
}

func TestAnalyzerRecordAccess(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig().Analyzer, DefaultConfig().Policy, zap.NewNop())
	now := time.Now()

	// 记录访问会自动注册文件
	analyzer.RecordAccess(AccessRecord{
		Path:      "/auto/registered.txt",
		OpType:    "read",
		Size:      2048,
		Timestamp: now,
	})

	entry, ok := analyzer.GetFileEntry("/auto/registered.txt")
	assert.True(t, ok)
	assert.Equal(t, int64(1), entry.AccessCount)
	assert.Equal(t, int64(1), entry.ReadCount)
	assert.Equal(t, int64(0), entry.WriteCount)
	assert.Equal(t, int64(2048), entry.Size)
}

func TestAnalyzerContextCancellation(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig().Analyzer, DefaultConfig().Policy, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := analyzer.Analyze(ctx)
	assert.Error(t, err)
}

func TestAnalyzerFileCount(t *testing.T) {
	analyzer := NewAnalyzer(DefaultConfig().Analyzer, DefaultConfig().Policy, zap.NewNop())
	assert.Equal(t, 0, analyzer.FileCount())

	analyzer.RegisterFile(FileEntry{Path: "/a.txt", Size: 100, CurrentTier: TierHDD})
	analyzer.RegisterFile(FileEntry{Path: "/b.txt", Size: 200, CurrentTier: TierSSD})
	assert.Equal(t, 2, analyzer.FileCount())
}

// ============================================================
// 策略测试
// ============================================================

func TestPolicyRecommendTier(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	// 高热度 → SSD
	assert.Equal(t, TierSSD, policy.RecommendTier(85.0, "/data/hot.db", 1024*1024))

	// 中热度 → HDD
	assert.Equal(t, TierHDD, policy.RecommendTier(50.0, "/data/warm.pdf", 1024*1024))

	// 低热度 → Cold
	assert.Equal(t, TierCold, policy.RecommendTier(10.0, "/archive/old.tar", 1024*1024))
}

func TestPolicyFileTypeBoost(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	// .db 文件有 +20 加成，所以 55 分的文件实际 75 分 → 应该是 SSD
	tier := policy.RecommendTier(55.0, "/data/app.db", 1024*1024)
	assert.Equal(t, TierSSD, tier, ".db boost should push to SSD tier")

	// .tar 文件有 -10 惩罚，所以 40 分的文件实际 30 分 → 应该是 HDD
	tier2 := policy.RecommendTier(40.0, "/archive/data.tar", 1024*1024)
	assert.Equal(t, TierHDD, tier2, ".tar penalty should push down")
}

func TestPolicyCapacityThreshold(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	// 初始状态不触发驱逐
	assert.False(t, policy.NeedsEviction(TierSSD))

	// 设置使用率到 90%
	total := config.Tiers[0].TotalBytes // 500GB
	policy.UpdateTierUsage(TierSSD, int64(float64(total)*0.90))
	assert.True(t, policy.NeedsEviction(TierSSD))
	assert.False(t, policy.EvictionTargetReached(TierSSD))

	// 设置使用率到 65%
	policy.UpdateTierUsage(TierSSD, int64(float64(total)*0.65))
	assert.False(t, policy.NeedsEviction(TierSSD))
	assert.True(t, policy.EvictionTargetReached(TierSSD))
}

func TestPolicyShouldMigrate(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	// 当前 HDD，热度高 → 应该迁移到 SSD
	should, target := policy.ShouldMigrate(TierHDD, 85.0, "/data/hot.db", 1024*1024)
	assert.True(t, should)
	assert.Equal(t, TierSSD, target)

	// 当前 SSD，热度也高 → 不需要迁移
	should, _ = policy.ShouldMigrate(TierSSD, 85.0, "/data/hot.db", 1024*1024)
	assert.False(t, should)

	// 当前 HDD，热度低 → 应该迁移到 Cold
	should, target = policy.ShouldMigrate(TierHDD, 10.0, "/archive/old.tar", 1024*1024)
	assert.True(t, should)
	assert.Equal(t, TierCold, target)
}

func TestPolicyFreeSpace(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	// 初始全部可用
	assert.Equal(t, config.Tiers[0].TotalBytes, policy.FreeSpace(TierSSD))

	// 使用 100GB
	policy.UpdateTierUsage(TierSSD, 100*1024*1024*1024)
	assert.Equal(t, config.Tiers[0].TotalBytes-100*1024*1024*1024, policy.FreeSpace(TierSSD))
}

func TestPolicyCanFit(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	assert.True(t, policy.CanFit(TierSSD, 1024*1024*1024)) // 1GB

	// 填满
	policy.UpdateTierUsage(TierSSD, config.Tiers[0].TotalBytes)
	assert.False(t, policy.CanFit(TierSSD, 1))
}

func TestPolicyEvictionCandidates(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	files := map[string]*FileEntry{
		"/a.dat": {Path: "/a.dat", CurrentTier: TierSSD, HeatScore: 90},
		"/b.dat": {Path: "/b.dat", CurrentTier: TierSSD, HeatScore: 30},
		"/c.dat": {Path: "/c.dat", CurrentTier: TierSSD, HeatScore: 60},
		"/d.dat": {Path: "/d.dat", CurrentTier: TierHDD, HeatScore: 80},
	}

	candidates := policy.GetEvictionCandidates(TierSSD, files)
	assert.Len(t, candidates, 3)
	// 应该按热度从低到高
	assert.Equal(t, 30.0, candidates[0].HeatScore)
	assert.Equal(t, 60.0, candidates[1].HeatScore)
	assert.Equal(t, 90.0, candidates[2].HeatScore)
}

func TestPolicyUpdateConfig(t *testing.T) {
	config := DefaultConfig()
	policy := NewPolicy(config.Policy, config.Tiers, zap.NewNop())

	newConfig := config.Policy
	newConfig.Thresholds.HotMinScore = 80.0
	policy.UpdateConfig(newConfig)

	got := policy.GetConfig()
	assert.Equal(t, 80.0, got.Thresholds.HotMinScore)
}

// ============================================================
// 迁移器测试
// ============================================================

func TestMigratorStartStop(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)

	// 重复启动
	err = migrator.Start(ctx)
	assert.Error(t, err)

	migrator.Stop()

	// 重复停止不应 panic
	migrator.Stop()
}

func TestMigratorSubmitAndComplete(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	task := &MigrationTask{
		FilePath: "/data/test.dat",
		FromTier: TierHDD,
		ToTier:   TierSSD,
		FileSize: 1024 * 1024,
		Reason:   "test migration",
	}

	err = migrator.Submit(task)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)

	// 等待完成
	time.Sleep(200 * time.Millisecond)

	got, ok := migrator.GetTask(task.ID)
	require.True(t, ok)
	assert.Equal(t, StateCompleted, got.State)
	assert.Equal(t, 100.0, got.Progress)
	assert.NotEmpty(t, got.ChecksumSrc)
	assert.NotEmpty(t, got.ChecksumDst)

	// 验证历史
	history := migrator.GetHistory(10)
	assert.Len(t, history, 1)
	assert.Equal(t, task.ID, history[0].TaskID)
}

func TestMigratorBatch(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	var tasks []*MigrationTask
	for i := 0; i < 5; i++ {
		tasks = append(tasks, &MigrationTask{
			FilePath: "/data/batch_" + formatInt(int64(i)) + ".dat",
			FromTier: TierHDD,
			ToTier:   TierSSD,
			FileSize: 1024,
			Reason:   "batch test",
		})
	}

	submitted := migrator.SubmitBatch(tasks)
	assert.Equal(t, 5, submitted)

	// 等待完成
	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, int64(5), migrator.TotalMigrations())
}

func TestMigratorPauseResume(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	// 暂停
	err = migrator.Pause()
	require.NoError(t, err)

	// 重复暂停
	err = migrator.Pause()
	assert.Error(t, err)

	// 恢复
	err = migrator.Resume()
	require.NoError(t, err)

	// 重复恢复
	err = migrator.Resume()
	assert.Error(t, err)
}

func TestMigratorCancelTask(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	task := &MigrationTask{
		FilePath: "/data/cancel_test.dat",
		FromTier: TierHDD,
		ToTier:   TierSSD,
		FileSize: 1024,
	}

	err = migrator.Submit(task)
	require.NoError(t, err)

	// 等待任务完成（模拟模式很快）
	time.Sleep(100 * time.Millisecond)

	// 已完成的任务不能取消
	err = migrator.CancelTask(task.ID)
	assert.Error(t, err, "completed task should not be cancelable")

	// 不存在的任务
	err = migrator.CancelTask("nonexistent")
	assert.Error(t, err)
}

func TestMigratorActiveCount(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	assert.Equal(t, 0, migrator.ActiveCount())

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	assert.Equal(t, 0, migrator.ActiveCount())
}

func TestMigratorSubmitWhenStopped(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	// 未启动时提交应失败
	err := migrator.Submit(&MigrationTask{FilePath: "/test"})
	assert.Error(t, err)
}

func TestMigratorEventChannel(t *testing.T) {
	config := DefaultConfig()
	migrator := NewMigrator(config.Migrator, nil, zap.NewNop())

	ch := migrator.EventChannel()
	assert.NotNil(t, ch)
}

// ============================================================
// 引擎集成测试
// ============================================================

func TestEngineStartStop(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	err = engine.Start(ctx)
	require.NoError(t, err)

	// 重复启动
	err = engine.Start(ctx)
	assert.Error(t, err)

	engine.Stop()

	// 重复停止不应 panic
	engine.Stop()
}

func TestEngineInvalidConfig(t *testing.T) {
	config := DefaultConfig()
	config.Tiers = nil

	_, err := NewEngine(config, nil, zap.NewNop())
	assert.Error(t, err)
}

func TestEngineRegisterAndRecordAccess(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// 注册文件
	engine.RegisterFile(FileEntry{
		Path:        "/data/test.dat",
		Size:        1024 * 1024,
		CurrentTier: TierHDD,
		AccessedAt:  time.Now(),
	})

	// 记录访问
	engine.RecordAccess(AccessRecord{
		Path:      "/data/test.dat",
		OpType:    "read",
		Size:      1024,
		Timestamp: time.Now(),
	})

	entry, ok := engine.Analyzer().GetFileEntry("/data/test.dat")
	assert.True(t, ok)
	assert.Equal(t, int64(1), entry.AccessCount)
}

func TestEngineRunAnalysis(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	now := time.Now()

	// 注册一个高热度文件在 HDD 上
	engine.RegisterFile(FileEntry{
		Path:        "/data/hot.db",
		Size:        1024 * 1024 * 10,
		CurrentTier: TierHDD,
		AccessedAt:  now,
		AccessCount: 200,
		ReadCount:   150,
		WriteCount:  50,
	})

	// 注册一个冷文件在 SSD 上
	engine.RegisterFile(FileEntry{
		Path:        "/archive/old.tar",
		Size:        1024 * 1024 * 100,
		CurrentTier: TierSSD,
		AccessedAt:  now.AddDate(0, -3, 0),
		AccessCount: 1,
		ReadCount:   1,
		WriteCount:  0,
	})

	// 运行分析
	count, err := engine.RunAnalysis(ctx)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "should generate migration tasks")
}

func TestEngineStats(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	stats := engine.Stats()
	assert.Len(t, stats.Tiers, 3)
	assert.Equal(t, int64(0), stats.TotalMigrations)
	assert.Equal(t, 0, stats.ActiveMigrations)
	assert.Equal(t, float64(0), stats.HitRate)
}

func TestEnginePauseResumeCancel(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// 暂停
	err = engine.PauseMigrations()
	require.NoError(t, err)

	// 恢复
	err = engine.ResumeMigrations()
	require.NoError(t, err)

	// 取消不存在的任务
	err = engine.CancelMigration("nonexistent")
	assert.Error(t, err)
}

func TestEngineSubsystemAccessors(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config, nil, zap.NewNop())
	require.NoError(t, err)

	assert.NotNil(t, engine.Migrator())
	assert.NotNil(t, engine.Analyzer())
	assert.NotNil(t, engine.Policy())
}

// ============================================================
// Checksum 测试
// ============================================================

func TestComputeChecksum(t *testing.T) {
	data1 := []byte("hello world")
	data2 := []byte("hello world")
	data3 := []byte("hello world!")

	cs1 := computeChecksum(data1)
	cs2 := computeChecksum(data2)
	cs3 := computeChecksum(data3)

	assert.Equal(t, cs1, cs2, "same data should produce same checksum")
	assert.NotEqual(t, cs1, cs3, "different data should produce different checksum")
	assert.Len(t, cs1, 8, "CRC32 hex should be 8 chars")
}

// ============================================================
// 辅助函数测试
// ============================================================

func TestFormatInt(t *testing.T) {
	assert.Equal(t, "0", formatInt(0))
	assert.Equal(t, "123", formatInt(123))
	assert.Equal(t, "-42", formatInt(-42))
	assert.Equal(t, "1000000", formatInt(1000000))
}

func TestFormatFloat(t *testing.T) {
	s := formatFloat(3.14, 2)
	assert.Equal(t, "3.14", s)

	s2 := formatFloat(10.0, 1)
	assert.Equal(t, "10.0", s2)
}

func TestInferContentType(t *testing.T) {
	assert.Equal(t, "text/plain", inferContentType("/path/file.txt"))
	assert.Equal(t, "application/json", inferContentType("/data/config.json"))
	assert.Equal(t, "video/mp4", inferContentType("/media/movie.mp4"))
	assert.Equal(t, "application/octet-stream", inferContentType("/data/unknown.xyz"))
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID("/data/test.dat", TierHDD, TierSSD)
	id2 := generateTaskID("/data/test.dat", TierHDD, TierSSD)
	assert.NotEqual(t, id1, id2, "IDs should be unique")
	assert.Contains(t, id1, "test.dat")
	assert.Contains(t, id1, "hdd->ssd")
}
