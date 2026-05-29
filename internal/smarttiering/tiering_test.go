package smarttiering

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHeatScoreCalculation(t *testing.T) {
	config := DefaultPredictorConfig()
	logger := zap.NewNop()
	predictor := NewPredictor(config, logger)

	// 测试文件1：刚访问过的热文件
	meta1 := FileMetadata{
		Path:        "/data/hot_file.dat",
		Size:        1024 * 1024 * 10, // 10MB
		CurrentTier: TierHot,
		AccessedAt:  time.Now(),
		CreatedAt:   time.Now().AddDate(0, 0, -5),
		AccessCount: 100,
		ReadCount:   80,
		WriteCount:  20,
	}
	predictor.RegisterFile(meta1)

	// 添加访问记录
	for i := 0; i < 10; i++ {
		predictor.RecordAccess(AccessRecord{
			Path:      "/data/hot_file.dat",
			OpType:    "read",
			Size:      1024 * 1024,
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	// 测试文件2：很久没访问的冷文件
	meta2 := FileMetadata{
		Path:        "/archive/cold_file.dat",
		Size:        1024 * 1024 * 100, // 100MB
		CurrentTier: TierCold,
		AccessedAt:  time.Now().AddDate(0, 0, -20),
		CreatedAt:   time.Now().AddDate(0, -6, 0),
		AccessCount: 5,
		ReadCount:   5,
		WriteCount:  0,
	}
	predictor.RegisterFile(meta2)

	// 更新热度评分
	count, err := predictor.UpdateHeatScores(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// 验证热文件热度高
	heat1, exists1 := predictor.GetFileHeat("/data/hot_file.dat")
	assert.True(t, exists1)
	assert.Greater(t, heat1, 60.0, "Hot file should have heat score > 60")

	// 验证冷文件热度低
	heat2, exists2 := predictor.GetFileHeat("/archive/cold_file.dat")
	assert.True(t, exists2)
	assert.Less(t, heat2, 40.0, "Cold file should have heat score < 40")

	// 验证热文件分数高于冷文件
	assert.Greater(t, heat1, heat2, "Hot file should have higher heat than cold file")
}

func TestTierPrediction(t *testing.T) {
	config := DefaultPredictorConfig()
	thresholds := DefaultMigratorConfig()
	logger := zap.NewNop()
	predictor := NewPredictor(config, logger)

	// 注册不同热度的文件
	files := []struct {
		path      string
		heatScore float64
		expected  StorageTier
	}{
		{"/hot.dat", 85.0, TierHot},
		{"/warm.dat", 55.0, TierWarm},
		{"/cold.dat", 25.0, TierCold},
		{"/archive.dat", 5.0, TierArchive},
	}

	for _, f := range files {
		meta := FileMetadata{
			Path:       f.path,
			Size:       1024 * 1024,
			HeatScore:  f.heatScore,
			AccessedAt: time.Now(),
		}
		predictor.RegisterFile(meta)
	}

	// 验证层级预测
	for _, f := range files {
		tier, score := predictor.PredictTier(f.path, thresholds)
		assert.Equal(t, f.expected, tier, "File %s with heat %.1f should be in %s tier", f.path, f.heatScore, f.expected)
		assert.Greater(t, score, 0.0)
	}
}

func TestMigratorQueue(t *testing.T) {
	config := DefaultMigratorConfig()
	config.DryRun = true
	config.BatchSize = 10
	config.MaxConcurrent = 2

	logger := zap.NewNop()
	predictor := NewPredictor(DefaultPredictorConfig(), logger)
	migrator := NewMigrator(config, predictor, logger)

	ctx := context.Background()
	err := migrator.Start(ctx)
	require.NoError(t, err)
	defer migrator.Stop()

	// 验证迁移器启动
	assert.True(t, migrator.IsRunning())
	assert.Equal(t, 0, migrator.GetQueueSize())

	// 强制迁移
	err = migrator.ForceMigrate(ctx, "/test.dat", TierHot, TierCold, 1024*1024)
	assert.NoError(t, err)
}

func TestFileRegistration(t *testing.T) {
	config := DefaultPredictorConfig()
	logger := zap.NewNop()
	predictor := NewPredictor(config, logger)

	// 注册文件
	meta := FileMetadata{
		Path:        "/data/test.dat",
		Size:        1024 * 50,
		CurrentTier: TierWarm,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
	}
	predictor.RegisterFile(meta)

	// 验证文件存在
	files := predictor.GetAllFiles()
	assert.Contains(t, files, "/data/test.dat")
	assert.Equal(t, TierWarm, files["/data/test.dat"].CurrentTier)

	// 获取指定层级的文件
	warmFiles := predictor.GetFilesByTier(TierWarm)
	assert.Len(t, warmFiles, 1)
	assert.Equal(t, "/data/test.dat", warmFiles[0].Path)
}

func TestMigratorEvents(t *testing.T) {
	config := DefaultMigratorConfig()
	config.DryRun = true

	logger := zap.NewNop()
	predictor := NewPredictor(DefaultPredictorConfig(), logger)
	migrator := NewMigrator(config, predictor, logger)

	// 记录事件
	event := MigrationEvent{
		ID:        "test-001",
		FilePath:  "/test.dat",
		FromTier:  TierHot,
		ToTier:    TierCold,
		FileSize:  1024 * 1024,
		Reason:    "test migration",
		Status:    "completed",
		StartedAt: time.Now(),
	}

	migrator.recordEvent(event)

	// 获取事件
	events := migrator.GetMigrationEvents(10)
	assert.Len(t, events, 1)
	assert.Equal(t, "test-001", events[0].ID)
	assert.Equal(t, "completed", events[0].Status)
}

func TestTierStrings(t *testing.T) {
	tests := []struct {
		tier     StorageTier
		expected string
	}{
		{TierHot, "hot"},
		{TierWarm, "warm"},
		{TierCold, "cold"},
		{TierArchive, "archive"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.tier.String())
	}

	// 测试ParseTier
	assert.Equal(t, TierHot, ParseTier("hot"))
	assert.Equal(t, TierCold, ParseTier("unknown"))
}

func TestDefaultConfigs(t *testing.T) {
	// 验证默认配置有效
	config := DefaultConfig()
	assert.True(t, config.Predictor.Enabled)
	assert.True(t, config.Migrator.Enabled)
	assert.Equal(t, 100, config.Migrator.BatchSize)
	assert.Equal(t, 4, config.Migrator.MaxConcurrent)

	// 验证阈值
	assert.Equal(t, 70.0, config.Migrator.HotThreshold)
	assert.Equal(t, 40.0, config.Migrator.WarmThreshold)
	assert.Equal(t, 15.0, config.Migrator.ColdThreshold)

	// 验证权重总和
	weights := config.Predictor.WeightRecency +
		config.Predictor.WeightFrequency +
		config.Predictor.WeightSize +
		config.Predictor.WeightPattern
	assert.InDelta(t, 1.0, weights, 0.01, "Weights should sum to ~1.0")
}

func TestManagerLifecycle(t *testing.T) {
	config := DefaultConfig()
	logger, _ := zap.NewDevelopment()
	manager := NewManager(config, logger)

	ctx := context.Background()

	// 启动
	err := manager.Start(ctx)
	require.NoError(t, err)
	assert.True(t, manager.IsRunning())

	// 重复启动应失败
	err = manager.Start(ctx)
	assert.Error(t, err)

	// 注册文件并记录访问
	meta := FileMetadata{
		Path:        "/test.dat",
		Size:        1024 * 1024,
		CurrentTier: TierHot,
		AccessedAt:  time.Now(),
		CreatedAt:   time.Now(),
	}
	manager.RegisterFile(meta)

	manager.RecordAccess(AccessRecord{
		Path:      "/test.dat",
		OpType:    "read",
		Size:      1024,
		Timestamp: time.Now(),
	})

	// 获取热度
	heat, exists := manager.GetFileHeat("/test.dat")
	assert.True(t, exists)
	assert.GreaterOrEqual(t, heat, 0.0)

	// 获取配置
	cfg := manager.GetConfig()
	assert.NotNil(t, cfg)

	// 停止
	manager.Stop()
	assert.False(t, manager.IsRunning())
}
