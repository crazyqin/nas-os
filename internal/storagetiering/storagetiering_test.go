package storagetiering

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// Helper
// ============================================================================

func newTestManager() *Manager {
	return NewManager(nil, nil)
}

func validFile() *FileMetadata {
	return &FileMetadata{
		Path:          "/data/test.txt",
		SizeBytes:     1024 * 1024, // 1MB
		CurrentTier:   TierLevelWarm,
		AccessPattern: AccessPatternReadWrite,
		AccessCount:   5,
		LastAccessAt:  time.Now().Add(-24 * time.Hour),
		CreatedAt:     time.Now().Add(-7 * 24 * time.Hour),
	}
}

func hotFile() *FileMetadata {
	return &FileMetadata{
		Path:          "/data/hot.txt",
		SizeBytes:     512 * 1024,
		CurrentTier:   TierLevelHot,
		AccessPattern: AccessPatternReadWrite,
		AccessCount:   100,
		LastAccessAt:  time.Now(),
		CreatedAt:     time.Now().Add(-1 * 24 * time.Hour),
	}
}

func coldFile() *FileMetadata {
	return &FileMetadata{
		Path:          "/data/cold.txt",
		SizeBytes:     10 * 1024 * 1024 * 1024, // 10GB
		CurrentTier:   TierLevelCold,
		AccessPattern: AccessPatternWriteOnce,
		AccessCount:   1,
		LastAccessAt:  time.Now().Add(-90 * 24 * time.Hour),
		CreatedAt:     time.Now().Add(-180 * 24 * time.Hour),
	}
}

// ============================================================================
// 构造函数
// ============================================================================

func TestNewManager_NilConfig(t *testing.T) {
	m := NewManager(nil, nil)
	require.NotNil(t, m)
	assert.NotNil(t, m.config)
	assert.NotNil(t, m.analyzer)
	assert.NotNil(t, m.scheduler)
}

func TestNewManager_CustomConfig(t *testing.T) {
	cfg := &StorageTieringConfig{
		Enabled:            true,
		AnalysisIntervalMs: 30000,
		MigrationBatchSize: 50,
		HeatThresholdHot:   80,
		HeatThresholdWarm:  50,
		HeatThresholdCold:  20,
	}
	m := NewManager(nil, cfg)
	assert.Equal(t, int64(30000), m.config.AnalysisIntervalMs)
	assert.Equal(t, 50, m.config.MigrationBatchSize)
	assert.Equal(t, 80.0, m.config.HeatThresholdHot)
}

func TestNewManager_WithLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	m := NewManager(logger, nil)
	require.NotNil(t, m)
	assert.NotNil(t, m.logger)
}

// ============================================================================
// DefaultStorageTieringConfig
// ============================================================================

func TestDefaultStorageTieringConfig(t *testing.T) {
	cfg := DefaultStorageTieringConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, int64(60000), cfg.AnalysisIntervalMs)
	assert.Equal(t, 100, cfg.MigrationBatchSize)
	assert.Equal(t, 5, cfg.MaxConcurrentMigrations)
	assert.Equal(t, 30, cfg.HeatDecayDays)
	assert.Equal(t, 70.0, cfg.HeatThresholdHot)
	assert.Equal(t, 40.0, cfg.HeatThresholdWarm)
	assert.Equal(t, 10.0, cfg.HeatThresholdCold)
	assert.True(t, cfg.AutoTieringEnabled)
	assert.Equal(t, 100, cfg.MigrationBandwidthMBps)
	assert.Equal(t, int64(1024), cfg.MinFileSizeBytes)
}

// ============================================================================
// Default Pools
// ============================================================================

func TestListPools_Default(t *testing.T) {
	m := newTestManager()
	pools := m.ListPools()
	assert.Len(t, pools, 3) // SSD, HDD, Cloud
}

func TestGetPool_Success(t *testing.T) {
	m := newTestManager()
	pool, err := m.GetPool("pool-ssd")
	require.NoError(t, err)
	assert.Equal(t, "SSD热存储池", pool.Name)
	assert.Equal(t, TierLevelHot, pool.Tier)
}

func TestGetPool_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetPool("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrPoolNotFound, err)
}

func TestListPoolsByTier(t *testing.T) {
	m := newTestManager()
	pools := m.ListPoolsByTier(TierLevelHot)
	assert.Len(t, pools, 1)
	assert.Equal(t, "pool-ssd", pools[0].ID)
}

func TestAddPool_Success(t *testing.T) {
	m := newTestManager()
	pool := &StoragePool{
		Name:          "NVMe池",
		Type:          StoragePoolSSD,
		Tier:          TierLevelHot,
		CapacityBytes: 1024 * 1024 * 1024 * 1024,
	}

	added, err := m.AddPool(pool)
	require.NoError(t, err)
	assert.NotEmpty(t, added.ID)
	assert.True(t, added.IsActive)
}

// ============================================================================
// Default Rules
// ============================================================================

func TestListRules_Default(t *testing.T) {
	m := newTestManager()
	rules := m.ListRules()
	assert.Len(t, rules, 4) // hot, warm, cold, large file
}

func TestGetRule_Success(t *testing.T) {
	m := newTestManager()
	rule, err := m.GetRule("rule-hot-access")
	require.NoError(t, err)
	assert.Equal(t, "高频访问热数据规则", rule.Name)
	assert.Equal(t, TierLevelHot, rule.TargetTier)
}

func TestGetRule_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetRule("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrRuleNotFound, err)
}

func TestAddRule_Success(t *testing.T) {
	m := newTestManager()
	rule := &TieringRule{
		Name:       "自定义规则",
		Priority:   10,
		MinAgeDays: 60,
		TargetTier: TierLevelCold,
	}

	added, err := m.AddRule(rule)
	require.NoError(t, err)
	assert.NotEmpty(t, added.ID)
	assert.True(t, added.IsActive)
}

// ============================================================================
// RegisterFile
// ============================================================================

func TestRegisterFile_Success(t *testing.T) {
	m := newTestManager()
	file := validFile()

	registered, err := m.RegisterFile(file)
	require.NoError(t, err)
	assert.NotEmpty(t, registered.ID)
	assert.Equal(t, TierLevelWarm, registered.CurrentTier)
	assert.True(t, registered.HeatScore >= 0)
}

func TestRegisterFile_AutoGenerateID(t *testing.T) {
	m := newTestManager()
	file := validFile()
	assert.Empty(t, file.ID)

	registered, err := m.RegisterFile(file)
	require.NoError(t, err)
	assert.NotEmpty(t, registered.ID)
}

func TestRegisterFile_DisabledTiering(t *testing.T) {
	m := newTestManager()
	m.config.Enabled = false
	file := validFile()

	_, err := m.RegisterFile(file)
	assert.Error(t, err)
	assert.Equal(t, ErrStorageTieringDisabled, err)
}

func TestRegisterFile_DefaultValues(t *testing.T) {
	m := newTestManager()
	file := &FileMetadata{
		Path:      "/data/minimal.txt",
		SizeBytes: 100,
	}

	registered, err := m.RegisterFile(file)
	require.NoError(t, err)
	assert.Equal(t, TierLevelWarm, registered.CurrentTier)
	assert.Equal(t, AccessPatternReadWrite, registered.AccessPattern)
}

// ============================================================================
// GetFile / ListFiles
// ============================================================================

func TestGetFile_Success(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())

	got, err := m.GetFile(file.ID)
	require.NoError(t, err)
	assert.Equal(t, file.ID, got.ID)
	assert.Equal(t, "/data/test.txt", got.Path)
}

func TestGetFile_NotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetFile("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestListFiles_Empty(t *testing.T) {
	m := newTestManager()
	files := m.ListFiles()
	assert.Empty(t, files)
}

func TestListFiles_Multiple(t *testing.T) {
	m := newTestManager()
	m.RegisterFile(validFile())
	m.RegisterFile(hotFile())
	m.RegisterFile(coldFile())

	files := m.ListFiles()
	assert.Len(t, files, 3)
}

func TestListFilesByTier(t *testing.T) {
	m := newTestManager()
	m.RegisterFile(validFile())
	m.RegisterFile(hotFile())
	m.RegisterFile(coldFile())

	hotFiles := m.ListFilesByTier(TierLevelHot)
	assert.Len(t, hotFiles, 1)
	assert.Equal(t, "/data/hot.txt", hotFiles[0].Path)
}

func TestListFilesByHeatLevel(t *testing.T) {
	m := newTestManager()
	now := time.Now()
	
	// Register a file with high access count and recent access to ensure it's hot
	hot := &FileMetadata{
		Path:          "/data/hot.txt",
		SizeBytes:     512 * 1024,
		CurrentTier:   TierLevelHot,
		AccessPattern: AccessPatternReadWrite,
		AccessCount:   1000,
		LastAccessAt:  now,
		LastModifiedAt: now,
		CreatedAt:     now.Add(-1 * 24 * time.Hour),
	}
	m.RegisterFile(hot)

	hotFiles := m.ListFilesByHeatLevel(HeatLevelHot)
	assert.Len(t, hotFiles, 1)
}

// ============================================================================
// RecordAccess
// ============================================================================

func TestRecordAccess_Success(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	originalCount := file.AccessCount

	err := m.RecordAccess(file.ID)
	require.NoError(t, err)

	updated, _ := m.GetFile(file.ID)
	assert.Equal(t, originalCount+1, updated.AccessCount)
}

func TestRecordAccess_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.RecordAccess("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestRecordAccess_UpdatesHeatScore(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	originalScore := file.HeatScore

	// Record multiple accesses
	for i := 0; i < 10; i++ {
		m.RecordAccess(file.ID)
	}

	updated, _ := m.GetFile(file.ID)
	assert.True(t, updated.HeatScore >= originalScore)
}

// ============================================================================
// PinFile / UnpinFile
// ============================================================================

func TestPinFile_Success(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	assert.False(t, file.IsPinned)

	err := m.PinFile(file.ID)
	require.NoError(t, err)

	updated, _ := m.GetFile(file.ID)
	assert.True(t, updated.IsPinned)
}

func TestUnpinFile_Success(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	m.PinFile(file.ID)

	err := m.UnpinFile(file.ID)
	require.NoError(t, err)

	updated, _ := m.GetFile(file.ID)
	assert.False(t, updated.IsPinned)
}

func TestPinFile_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.PinFile("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

// ============================================================================
// UpdateFileSize
// ============================================================================

func TestUpdateFileSize_Success(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())

	err := m.UpdateFileSize(file.ID, 2*1024*1024)
	require.NoError(t, err)

	updated, _ := m.GetFile(file.ID)
	assert.Equal(t, int64(2*1024*1024), updated.SizeBytes)
}

func TestUpdateFileSize_NotFound(t *testing.T) {
	m := newTestManager()
	err := m.UpdateFileSize("nonexistent", 100)
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

// ============================================================================
// Analyzer
// ============================================================================

func TestAnalyzer_CalculateHeatScore(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	file := &FileMetadata{
		AccessCount:    100,
		LastAccessAt:   now,
		LastModifiedAt: now,
	}

	score := analyzer.CalculateHeatScore(file, now)
	assert.True(t, score > 50, "Active file should have high heat score")
}

func TestAnalyzer_CalculateHeatScore_ZeroAccess(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	file := &FileMetadata{
		AccessCount: 0,
	}

	score := analyzer.CalculateHeatScore(file, now)
	assert.Equal(t, 0.0, score)
}

func TestAnalyzer_ClassifyHeatLevel(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	assert.Equal(t, HeatLevelHot, analyzer.ClassifyHeatLevel(80))
	assert.Equal(t, HeatLevelWarm, analyzer.ClassifyHeatLevel(50))
	assert.Equal(t, HeatLevelCold, analyzer.ClassifyHeatLevel(20))
	assert.Equal(t, HeatLevelFrozen, analyzer.ClassifyHeatLevel(5))
}

func TestAnalyzer_AnalyzeFile(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	file := &FileMetadata{
		AccessCount:    50,
		LastAccessAt:   now.Add(-24 * time.Hour),
		LastModifiedAt: now.Add(-7 * 24 * time.Hour),
	}

	result := analyzer.AnalyzeFile(file, now)
	assert.True(t, result.HeatScore > 0)
	assert.NotEmpty(t, result.HeatLevel)
}

func TestAnalyzer_GetHeatDistribution(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	files := []*FileMetadata{
		{HeatLevel: HeatLevelHot},
		{HeatLevel: HeatLevelWarm},
		{HeatLevel: HeatLevelCold},
	}

	dist := analyzer.GetHeatDistribution(files)
	assert.Equal(t, int64(1), dist[HeatLevelHot])
	assert.Equal(t, int64(1), dist[HeatLevelWarm])
	assert.Equal(t, int64(1), dist[HeatLevelCold])
}

func TestAnalyzer_CalculateAverageHeatScore(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	files := []*FileMetadata{
		{HeatScore: 80},
		{HeatScore: 60},
		{HeatScore: 40},
	}

	avg := analyzer.CalculateAverageHeatScore(files)
	assert.Equal(t, 60.0, avg)
}

func TestAnalyzer_CalculateTierFromHeat(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	assert.Equal(t, TierLevelHot, analyzer.CalculateTierFromHeat(HeatLevelHot))
	assert.Equal(t, TierLevelWarm, analyzer.CalculateTierFromHeat(HeatLevelWarm))
	assert.Equal(t, TierLevelCold, analyzer.CalculateTierFromHeat(HeatLevelCold))
	assert.Equal(t, TierLevelCold, analyzer.CalculateTierFromHeat(HeatLevelFrozen))
}

// ============================================================================
// Scheduler
// ============================================================================

func TestScheduler_StartStop(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)

	scheduler.Start()
	assert.True(t, scheduler.IsRunning())

	scheduler.Stop()
	assert.False(t, scheduler.IsRunning())
}

func TestScheduler_ScheduleMigration(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())

	task, err := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, StatePending, task.State)
	assert.Equal(t, TierSSD, task.ToTier)
}

func TestScheduler_ScheduleMigration_AlreadyInTier(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())

	_, err := m.GetScheduler().ScheduleMigration(file, TierLevelWarm, "test-rule")
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyInTier, err)
}

func TestScheduler_ScheduleMigration_PinnedFile(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	m.PinFile(file.ID)

	_, err := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")
	assert.Error(t, err)
	assert.Equal(t, ErrFilePinned, err)
}

func TestScheduler_GetTask(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	task, _ := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")

	got, err := m.GetScheduler().GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)
}

func TestScheduler_GetTask_NotFound(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	_, err := scheduler.GetTask("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestScheduler_ListTasks(t *testing.T) {
	m := newTestManager()
	file1, _ := m.RegisterFile(validFile())
	file2, _ := m.RegisterFile(coldFile())

	m.GetScheduler().ScheduleMigration(file1, TierLevelHot, "rule-1")
	m.GetScheduler().ScheduleMigration(file2, TierLevelWarm, "rule-2")

	tasks := m.GetScheduler().ListTasks()
	assert.Len(t, tasks, 2)
}

func TestScheduler_CancelTask(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())
	task, _ := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")

	err := m.GetScheduler().CancelTask(task.ID)
	require.NoError(t, err)

	got, _ := m.GetScheduler().GetTask(task.ID)
	assert.Equal(t, StateCancelled, got.State)
}

func TestScheduler_CancelTask_NotFound(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	err := scheduler.CancelTask("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestScheduler_GetStats(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	stats := scheduler.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalMigrations)
}

// ============================================================================
// Rule Evaluation
// ============================================================================

func TestEvaluateRules_HotFile(t *testing.T) {
	m := newTestManager()

	// Register a file that should match hot rule
	file := &FileMetadata{
		Path:          "/data/frequent.txt",
		SizeBytes:     1024,
		CurrentTier:   TierLevelWarm,
		AccessPattern: AccessPatternReadWrite,
		AccessCount:   20, // > 10
		LastAccessAt:  time.Now(),
		CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
	}
	m.RegisterFile(file)

	tasks := m.EvaluateRules()
	assert.True(t, len(tasks) > 0, "Should have migration tasks")
}

func TestEvaluateRules_ColdFile(t *testing.T) {
	m := newTestManager()

	// Register a file that should match cold rule
	file := &FileMetadata{
		Path:          "/data/archive.dat",
		SizeBytes:     1024 * 1024,
		CurrentTier:   TierLevelWarm,
		AccessPattern: AccessPatternWriteOnce,
		AccessCount:   1,
		LastAccessAt:  time.Now().Add(-60 * 24 * time.Hour),
		CreatedAt:     time.Now().Add(-90 * 24 * time.Hour),
	}
	m.RegisterFile(file)

	tasks := m.EvaluateRules()
	assert.True(t, len(tasks) > 0, "Should have migration tasks")
}

func TestEvaluateRules_PinnedFileSkipped(t *testing.T) {
	m := newTestManager()

	file := &FileMetadata{
		Path:          "/data/pinned.txt",
		SizeBytes:     1024,
		CurrentTier:   TierLevelWarm,
		AccessCount:   20,
		LastAccessAt:  time.Now(),
		CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
	}
	registered, _ := m.RegisterFile(file)
	m.PinFile(registered.ID)

	tasks := m.EvaluateRules()
	assert.Len(t, tasks, 0, "Pinned files should not be migrated")
}

func TestEvaluateRules_DisabledAutoTiering(t *testing.T) {
	m := newTestManager()
	m.config.AutoTieringEnabled = false

	m.RegisterFile(validFile())

	tasks := m.EvaluateRules()
	assert.Nil(t, tasks)
}

// ============================================================================
// Analysis Report
// ============================================================================

func TestGetAnalysisReport(t *testing.T) {
	m := newTestManager()
	m.RegisterFile(hotFile())
	m.RegisterFile(validFile())
	m.RegisterFile(coldFile())

	report := m.GetAnalysisReport()
	require.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, int64(3), report.TotalFiles)
	assert.True(t, report.TotalBytes > 0)
	assert.Len(t, report.TierStats, 3)
	assert.NotNil(t, report.MigrationStats)
	assert.NotNil(t, report.HeatDistribution)
	assert.NotEmpty(t, report.Recommendations)
}

func TestGetAnalysisReport_Empty(t *testing.T) {
	m := newTestManager()
	report := m.GetAnalysisReport()

	assert.Equal(t, int64(0), report.TotalFiles)
	assert.Equal(t, int64(0), report.TotalBytes)
}

// ============================================================================
// GetConfig / UpdateConfig
// ============================================================================

func TestGetConfig_ReturnsCopy(t *testing.T) {
	m := newTestManager()
	cfg1 := m.GetConfig()
	cfg2 := m.GetConfig()

	cfg1.AnalysisIntervalMs = 99999
	assert.NotEqual(t, cfg1.AnalysisIntervalMs, cfg2.AnalysisIntervalMs)
}

func TestUpdateConfig_Success(t *testing.T) {
	m := newTestManager()
	newCfg := &StorageTieringConfig{
		Enabled:            true,
		AnalysisIntervalMs: 30000,
		HeatThresholdHot:   80,
	}
	m.UpdateConfig(newCfg)
	assert.Equal(t, int64(30000), m.config.AnalysisIntervalMs)
}

func TestUpdateConfig_NilIgnored(t *testing.T) {
	m := newTestManager()
	original := m.config.AnalysisIntervalMs
	m.UpdateConfig(nil)
	assert.Equal(t, original, m.config.AnalysisIntervalMs)
}

// ============================================================================
// Concurrent Access
// ============================================================================

func TestConcurrent_RegisterAndGet(t *testing.T) {
	m := newTestManager()
	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			file := &FileMetadata{
				Path:      "/data/file-" + string(rune('A'+idx%26)),
				SizeBytes: 1024,
			}
			m.RegisterFile(file)
		}(i)
	}

	wg.Wait()
	files := m.ListFiles()
	assert.Len(t, files, n)
}

func TestConcurrent_RecordAccess(t *testing.T) {
	m := newTestManager()
	file, _ := m.RegisterFile(validFile())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RecordAccess(file.ID)
		}()
	}

	wg.Wait()

	updated, _ := m.GetFile(file.ID)
	assert.Equal(t, int64(100+5), updated.AccessCount) // 100 new + 5 original
}

func TestConcurrent_ScheduleMigration(t *testing.T) {
	m := newTestManager()

	var files []*FileMetadata
	for i := 0; i < 20; i++ {
		file := &FileMetadata{
			Path:        "/data/file-" + string(rune('A'+i)),
			SizeBytes:   1024 * 1024,
			CurrentTier: TierLevelWarm,
			AccessCount: 100, // High access to trigger hot rule
			LastAccessAt: time.Now(),
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		}
		registered, _ := m.RegisterFile(file)
		files = append(files, registered)
	}

	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		go func(f *FileMetadata) {
			defer wg.Done()
			m.GetScheduler().ScheduleMigration(f, TierLevelHot, "test-rule")
		}(file)
	}

	wg.Wait()
	tasks := m.GetScheduler().ListTasks()
	assert.Len(t, tasks, 20)
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestDefaultPools_HaveCorrectTiers(t *testing.T) {
	m := newTestManager()
	pools := m.ListPools()

	tierMap := map[TierLevel]bool{}
	for _, p := range pools {
		tierMap[p.Tier] = true
	}

	assert.True(t, tierMap[TierLevelHot])
	assert.True(t, tierMap[TierLevelWarm])
	assert.True(t, tierMap[TierLevelCold])
}

func TestDefaultRules_AreActive(t *testing.T) {
	m := newTestManager()
	for _, r := range m.ListRules() {
		assert.True(t, r.IsActive)
	}
}

func TestDefaultPools_AreActive(t *testing.T) {
	m := newTestManager()
	for _, p := range m.ListPools() {
		assert.True(t, p.IsActive)
	}
}

func TestFileHeatScoreRange(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	// Very active file
	activeFile := &FileMetadata{
		AccessCount:    10000,
		LastAccessAt:   now,
		LastModifiedAt: now,
	}
	score := analyzer.CalculateHeatScore(activeFile, now)
	assert.True(t, score >= 0 && score <= 100, "Heat score should be 0-100")
}

func TestScheduleMigration_DisabledTiering(t *testing.T) {
	m := newTestManager()
	m.config.Enabled = false
	file := validFile()

	_, err := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")
	assert.Error(t, err)
	assert.Equal(t, ErrStorageTieringDisabled, err)
}

func TestScheduleMigration_FileTooSmall(t *testing.T) {
	m := newTestManager()
	file := &FileMetadata{
		ID:          "tiny-file",
		Path:        "/data/tiny.txt",
		SizeBytes:   100, // Below MinFileSizeBytes (1024)
		CurrentTier: TierLevelWarm,
	}

	_, err := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")
	assert.Error(t, err)
	assert.Equal(t, ErrFileTooSmall, err)
}

func TestScheduleMigration_FileTooLarge(t *testing.T) {
	m := newTestManager()
	file := &FileMetadata{
		ID:          "huge-file",
		Path:        "/data/huge.dat",
		SizeBytes:   1024 * 1024 * 1024 * 100, // 100GB - exceeds MaxFileSizeBytes (10GB)
		CurrentTier: TierLevelWarm,
	}

	_, err := m.GetScheduler().ScheduleMigration(file, TierLevelHot, "test-rule")
	assert.Error(t, err)
	assert.Equal(t, ErrFileTooLarge, err)
}

func TestScheduler_IsRunningInitialState(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	assert.False(t, scheduler.IsRunning())
}

func TestScheduler_GetQueueLength(t *testing.T) {
	scheduler := NewScheduler(nil, nil, nil)
	assert.Equal(t, 0, scheduler.GetQueueLength())
}
