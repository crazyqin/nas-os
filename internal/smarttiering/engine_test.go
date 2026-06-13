package smarttiering

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTieringEngine(t *testing.T) {
	engine := NewTieringEngine(nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.tiers)
	assert.NotNil(t, engine.policies)
	assert.NotNil(t, engine.files)
	assert.NotNil(t, engine.accessLog)
	assert.NotNil(t, engine.migrationQ)
	assert.NotNil(t, engine.stats)
	assert.NotNil(t, engine.heatAnalyzer)
}

func TestTieringEngine_StartStop(t *testing.T) {
	engine := NewTieringEngine(nil)

	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.running)

	err = engine.Start()
	require.NoError(t, err)

	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.running)

	err = engine.Stop()
	require.NoError(t, err)
}

func TestTieringEngine_RegisterTier(t *testing.T) {
	engine := NewTieringEngine(nil)

	tier := &StorageTier{
		ID:            "tier-1",
		Name:          "Hot Storage",
		Type:          TierTypeHot,
		Performance:   "high",
		TotalCapacity: 1024 * 1024 * 1024, // 1GB
		AvailableSpace: 1024 * 1024 * 1024,
	}

	err := engine.RegisterTier(tier)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.stats.TotalTiers)

	// 注册无ID层
	invalid := &StorageTier{Name: "Invalid"}
	err = engine.RegisterTier(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidTierID, err)
}

func TestTieringEngine_UnregisterTier(t *testing.T) {
	engine := NewTieringEngine(nil)

	tier := &StorageTier{
		ID:            "tier-1",
		Name:          "Hot Storage",
		Type:          TierTypeHot,
		TotalCapacity: 1024 * 1024 * 1024,
		AvailableSpace: 1024 * 1024 * 1024,
	}
	engine.RegisterTier(tier)

	err := engine.UnregisterTier("tier-1")
	require.NoError(t, err)
	assert.Equal(t, 0, engine.stats.TotalTiers)

	// 注销锁定层
	tier2 := &StorageTier{
		ID:     "tier-2",
		Name:   "Locked",
	}
	engine.RegisterTier(tier2)
	tier2.Status = TierStatusLocked // 设置为锁定状态
	err = engine.UnregisterTier("tier-2")
	assert.Error(t, err)
	assert.Equal(t, ErrTierLocked, err)

	// 注销不存在的层
	err = engine.UnregisterTier("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTierNotFound, err)
}

func TestTieringEngine_GetTier(t *testing.T) {
	engine := NewTieringEngine(nil)

	tier := &StorageTier{
		ID:   "tier-1",
		Name: "Hot Storage",
		Type: TierTypeHot,
	}
	engine.RegisterTier(tier)

	result, err := engine.GetTier("tier-1")
	require.NoError(t, err)
	assert.Equal(t, "Hot Storage", result.Name)

	_, err = engine.GetTier("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTierNotFound, err)
}

func TestTieringEngine_ListTiers(t *testing.T) {
	engine := NewTieringEngine(nil)

	tiers := []*StorageTier{
		{ID: "tier-1", Name: "Hot", Type: TierTypeHot},
		{ID: "tier-2", Name: "Warm", Type: TierTypeWarm},
		{ID: "tier-3", Name: "Cold", Type: TierTypeCold},
	}

	for _, tier := range tiers {
		engine.RegisterTier(tier)
	}

	result := engine.ListTiers()
	assert.Len(t, result, 3)
}

func TestTieringEngine_CreatePolicy(t *testing.T) {
	engine := NewTieringEngine(nil)

	policy := &TieringPolicy{
		ID:           "policy-1",
		Name:         "Hot to Warm",
		SourceTierID: "tier-hot",
		TargetTierID: "tier-warm",
		IsActive:     true,
		Priority:     1,
	}

	err := engine.CreatePolicy(policy)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.stats.TotalPolicies)

	// 创建无ID策略
	invalid := &TieringPolicy{Name: "Invalid"}
	err = engine.CreatePolicy(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidPolicyID, err)
}

func TestTieringEngine_UpdatePolicy(t *testing.T) {
	engine := NewTieringEngine(nil)

	policy := &TieringPolicy{
		ID:   "policy-1",
		Name: "Original",
	}
	engine.CreatePolicy(policy)

	policy.Name = "Updated"
	err := engine.UpdatePolicy(policy)
	require.NoError(t, err)

	updated, _ := engine.GetPolicy("policy-1")
	assert.Equal(t, "Updated", updated.Name)

	// 更新不存在的策略
	invalid := &TieringPolicy{ID: "non-existent"}
	err = engine.UpdatePolicy(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrPolicyNotFound, err)
}

func TestTieringEngine_DeletePolicy(t *testing.T) {
	engine := NewTieringEngine(nil)

	policy := &TieringPolicy{
		ID:   "policy-1",
		Name: "Test",
	}
	engine.CreatePolicy(policy)

	err := engine.DeletePolicy("policy-1")
	require.NoError(t, err)
	assert.Equal(t, 0, engine.stats.TotalPolicies)

	// 删除不存在的策略
	err = engine.DeletePolicy("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPolicyNotFound, err)
}

func TestTieringEngine_GetPolicy(t *testing.T) {
	engine := NewTieringEngine(nil)

	policy := &TieringPolicy{
		ID:   "policy-1",
		Name: "Test Policy",
	}
	engine.CreatePolicy(policy)

	result, err := engine.GetPolicy("policy-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", result.Name)

	_, err = engine.GetPolicy("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrPolicyNotFound, err)
}

func TestTieringEngine_ListPolicies(t *testing.T) {
	engine := NewTieringEngine(nil)

	policies := []*TieringPolicy{
		{ID: "policy-1", Name: "Policy 1"},
		{ID: "policy-2", Name: "Policy 2"},
	}

	for _, policy := range policies {
		engine.CreatePolicy(policy)
	}

	result := engine.ListPolicies()
	assert.Len(t, result, 2)
}

func TestTieringEngine_RegisterFile(t *testing.T) {
	engine := NewTieringEngine(nil)

	file := &FileMetadata{
		ID:            "file-1",
		Name:          "test.txt",
		Size:          1024,
		CurrentTierID: "tier-1",
		LastAccessed:  time.Now(),
		LastModified:  time.Now(),
	}

	err := engine.RegisterFile(file)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.stats.TotalFiles)

	// 注册无ID文件
	invalid := &FileMetadata{Name: "Invalid"}
	err = engine.RegisterFile(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFileID, err)
}

func TestTieringEngine_UnregisterFile(t *testing.T) {
	engine := NewTieringEngine(nil)

	file := &FileMetadata{
		ID:           "file-1",
		Name:         "test.txt",
		CurrentTierID: "tier-1",
	}
	engine.RegisterFile(file)

	err := engine.UnregisterFile("file-1")
	require.NoError(t, err)
	assert.Equal(t, 0, engine.stats.TotalFiles)

	// 注销不存在的文件
	err = engine.UnregisterFile("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestTieringEngine_GetFile(t *testing.T) {
	engine := NewTieringEngine(nil)

	file := &FileMetadata{
		ID:   "file-1",
		Name: "test.txt",
	}
	engine.RegisterFile(file)

	result, err := engine.GetFile("file-1")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", result.Name)

	_, err = engine.GetFile("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestTieringEngine_RecordAccess(t *testing.T) {
	engine := NewTieringEngine(nil)

	file := &FileMetadata{
		ID:           "file-1",
		Name:         "test.txt",
		CurrentTierID: "tier-1",
		LastAccessed: time.Now().Add(-24 * time.Hour),
		AccessCount:  0,
	}
	engine.RegisterFile(file)

	err := engine.RecordAccess("file-1")
	require.NoError(t, err)

	updated, _ := engine.GetFile("file-1")
	assert.Equal(t, int64(1), updated.AccessCount)
	assert.True(t, updated.HeatScore > 0)

	// 访问不存在的文件
	err = engine.RecordAccess("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestTieringEngine_HeatScore(t *testing.T) {
	engine := NewTieringEngine(nil)

	// 热文件
	hotFile := &FileMetadata{
		ID:           "file-hot",
		Size:         1024,
		LastAccessed: time.Now(),
		AccessCount:  100,
	}

	// 冷文件
	coldFile := &FileMetadata{
		ID:           "file-cold",
		Size:         1024 * 1024 * 100,
		LastAccessed: time.Now().Add(-30 * 24 * time.Hour),
		AccessCount:  1,
	}

	engine.RegisterFile(hotFile)
	engine.RegisterFile(coldFile)

	hotScore := hotFile.HeatScore
	coldScore := coldFile.HeatScore

	assert.Greater(t, hotScore, coldScore)
}

func TestTieringEngine_GetHotFiles(t *testing.T) {
	engine := NewTieringEngine(nil)

	files := []*FileMetadata{
		{ID: "file-1", Size: 1024, LastAccessed: time.Now(), AccessCount: 100},
		{ID: "file-2", Size: 1024, LastAccessed: time.Now().Add(-24 * time.Hour), AccessCount: 10},
		{ID: "file-3", Size: 1024, LastAccessed: time.Now().Add(-48 * time.Hour), AccessCount: 1},
	}

	for _, file := range files {
		engine.RegisterFile(file)
	}

	hotFiles := engine.GetHotFiles(2)
	assert.Len(t, hotFiles, 2)
	assert.Equal(t, "file-1", hotFiles[0].ID)
}

func TestTieringEngine_GetFilesByTier(t *testing.T) {
	engine := NewTieringEngine(nil)

	files := []*FileMetadata{
		{ID: "file-1", CurrentTierID: "tier-1"},
		{ID: "file-2", CurrentTierID: "tier-1"},
		{ID: "file-3", CurrentTierID: "tier-2"},
	}

	for _, file := range files {
		engine.RegisterFile(file)
	}

	tier1Files := engine.GetFilesByTier("tier-1")
	assert.Len(t, tier1Files, 2)

	tier2Files := engine.GetFilesByTier("tier-2")
	assert.Len(t, tier2Files, 1)
}

func TestTieringEngine_AnalyzeAndMigrate(t *testing.T) {
	engine := NewTieringEngine(nil)
	engine.Start()

	// 注册存储层
	hotTier := &StorageTier{
		ID:            "tier-hot",
		Name:          "Hot",
		Type:          TierTypeHot,
		TotalCapacity: 1024 * 1024 * 1024,
		AvailableSpace: 1024 * 1024 * 1024,
	}
	warmTier := &StorageTier{
		ID:            "tier-warm",
		Name:          "Warm",
		Type:          TierTypeWarm,
		TotalCapacity: 1024 * 1024 * 1024 * 10,
		AvailableSpace: 1024 * 1024 * 1024 * 10,
	}
	engine.RegisterTier(hotTier)
	engine.RegisterTier(warmTier)

	// 创建策略
	policy := &TieringPolicy{
		ID:           "policy-1",
		Name:         "Hot to Warm",
		SourceTierID: "tier-hot",
		TargetTierID: "tier-warm",
		IsActive:     true,
		Priority:     1,
		Conditions: []Condition{
			{
				Type:     ConditionLastAccess,
				Operator: ">",
				Value:    7.0, // 7天未访问
			},
		},
	}
	engine.CreatePolicy(policy)

	// 注册冷文件
	coldFile := &FileMetadata{
		ID:           "file-cold",
		Name:         "old.txt",
		Size:         1024,
		CurrentTierID: "tier-hot",
		LastAccessed: time.Now().Add(-10 * 24 * time.Hour),
	}
	engine.RegisterFile(coldFile)

	// 分析并迁移
	tasks, err := engine.AnalyzeAndMigrate()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "file-cold", tasks[0].FileID)
}

func TestTieringEngine_ExecuteMigration(t *testing.T) {
	engine := NewTieringEngine(nil)
	engine.Start()

	// 注册存储层
	hotTier := &StorageTier{
		ID:            "tier-hot",
		Name:          "Hot",
		Type:          TierTypeHot,
		TotalCapacity: 1024 * 1024 * 1024,
		AvailableSpace: 1024 * 1024 * 1024,
		UsedCapacity:   1024,
		FileCount:      1,
	}
	warmTier := &StorageTier{
		ID:            "tier-warm",
		Name:          "Warm",
		Type:          TierTypeWarm,
		TotalCapacity: 1024 * 1024 * 1024 * 10,
		AvailableSpace: 1024 * 1024 * 1024 * 10,
	}
	engine.RegisterTier(hotTier)
	engine.RegisterTier(warmTier)

	// 注册文件
	file := &FileMetadata{
		ID:            "file-1",
		Name:          "test.txt",
		Size:          1024,
		CurrentTierID: "tier-hot",
	}
	engine.RegisterFile(file)

	// 创建迁移任务
	task := &MigrationTask{
		ID:           "task-1",
		FileID:       "file-1",
		SourceTierID: "tier-hot",
		TargetTierID: "tier-warm",
		Status:       TaskStatusPending,
	}
	engine.migrationQ = append(engine.migrationQ, task)

	// 执行迁移
	err := engine.ExecuteMigration("task-1")
	require.NoError(t, err)

	// 验证迁移结果
	updatedFile, _ := engine.GetFile("file-1")
	assert.Equal(t, "tier-warm", updatedFile.CurrentTierID)

	updatedHotTier, _ := engine.GetTier("tier-hot")
	assert.Equal(t, int64(0), updatedHotTier.UsedCapacity)
	assert.Equal(t, 0, updatedHotTier.FileCount)

	updatedWarmTier, _ := engine.GetTier("tier-warm")
	assert.Equal(t, int64(1024), updatedWarmTier.UsedCapacity)
	assert.Equal(t, 1, updatedWarmTier.FileCount)
}

func TestTieringEngine_GetMigrationQueue(t *testing.T) {
	engine := NewTieringEngine(nil)

	tasks := []*MigrationTask{
		{ID: "task-1", FileID: "file-1"},
		{ID: "task-2", FileID: "file-2"},
	}

	engine.migrationQ = tasks

	queue := engine.GetMigrationQueue()
	assert.Len(t, queue, 2)
}

func TestTieringEngine_GetStats(t *testing.T) {
	engine := NewTieringEngine(nil)

	tiers := []*StorageTier{
		{ID: "tier-1", Name: "Hot", Type: TierTypeHot, UsedCapacity: 100, FileCount: 10},
		{ID: "tier-2", Name: "Warm", Type: TierTypeWarm, UsedCapacity: 200, FileCount: 20},
	}

	for _, tier := range tiers {
		engine.RegisterTier(tier)
	}

	policies := []*TieringPolicy{
		{ID: "policy-1", Name: "Policy 1"},
		{ID: "policy-2", Name: "Policy 2"},
	}

	for _, policy := range policies {
		engine.CreatePolicy(policy)
	}

	files := []*FileMetadata{
		{ID: "file-1", CurrentTierID: "tier-1", HeatScore: 0.8},
		{ID: "file-2", CurrentTierID: "tier-2", HeatScore: 0.3},
	}

	for _, file := range files {
		engine.RegisterFile(file)
	}

	stats := engine.GetStats()
	assert.Equal(t, 2, stats.TotalTiers)
	assert.Equal(t, 2, stats.TotalPolicies)
	assert.Equal(t, 2, stats.TotalFiles)
	assert.Len(t, stats.TierUsage, 2)
	assert.Len(t, stats.TierFiles, 2)
	assert.Len(t, stats.PolicyStats, 2)
	assert.Greater(t, stats.AvgHeatScore, 0.0)
}

func TestTieringEngine_DefaultConfig(t *testing.T) {
	config := DefaultTierConfig()
	assert.True(t, config.EnableAutoTiering)
	assert.Equal(t, 24*time.Hour, config.AnalysisInterval)
	assert.Equal(t, 100, config.MigrationBatchSize)
	assert.Equal(t, 7, config.HeatThresholdDays)
	assert.Equal(t, 30, config.WarmThresholdDays)
	assert.Equal(t, 90, config.ColdThresholdDays)
	assert.Equal(t, 3, config.MinAccessCount)
	assert.True(t, config.EnableCompression)
	assert.True(t, config.EnableDeduplication)
}

func TestTieringEngine_TierTypes(t *testing.T) {
	engine := NewTieringEngine(nil)

	tiers := []*StorageTier{
		{ID: "tier-1", Name: "Hot", Type: TierTypeHot},
		{ID: "tier-2", Name: "Warm", Type: TierTypeWarm},
		{ID: "tier-3", Name: "Cold", Type: TierTypeCold},
		{ID: "tier-4", Name: "Archive", Type: TierTypeArchive},
	}

	for _, tier := range tiers {
		engine.RegisterTier(tier)
	}

	result := engine.ListTiers()
	assert.Len(t, result, 4)
}

func TestTieringEngine_MultipleAccess(t *testing.T) {
	engine := NewTieringEngine(nil)

	file := &FileMetadata{
		ID:           "file-1",
		Name:         "test.txt",
		CurrentTierID: "tier-1",
		LastAccessed: time.Now(),
		AccessCount:  0,
	}
	engine.RegisterFile(file)

	// 多次访问
	for i := 0; i < 10; i++ {
		engine.RecordAccess("file-1")
	}

	updated, _ := engine.GetFile("file-1")
	assert.Equal(t, int64(10), updated.AccessCount)
	assert.Greater(t, updated.HeatScore, 0.5)
}

func TestTieringEngine_PinnedFile(t *testing.T) {
	engine := NewTieringEngine(nil)
	engine.Start()

	// 注册存储层
	hotTier := &StorageTier{
		ID:            "tier-hot",
		Name:          "Hot",
		TotalCapacity: 1024 * 1024 * 1024,
		AvailableSpace: 1024 * 1024 * 1024,
	}
	warmTier := &StorageTier{
		ID:            "tier-warm",
		Name:          "Warm",
		TotalCapacity: 1024 * 1024 * 1024 * 10,
		AvailableSpace: 1024 * 1024 * 1024 * 10,
	}
	engine.RegisterTier(hotTier)
	engine.RegisterTier(warmTier)

	// 创建策略
	policy := &TieringPolicy{
		ID:           "policy-1",
		Name:         "Hot to Warm",
		SourceTierID: "tier-hot",
		TargetTierID: "tier-warm",
		IsActive:     true,
		Conditions: []Condition{
			{
				Type:     ConditionLastAccess,
				Operator: ">",
				Value:    7.0,
			},
		},
	}
	engine.CreatePolicy(policy)

	// 注册固定文件
	pinnedFile := &FileMetadata{
		ID:            "file-pinned",
		Name:          "pinned.txt",
		Size:          1024,
		CurrentTierID: "tier-hot",
		LastAccessed:  time.Now().Add(-10 * 24 * time.Hour),
		IsPinned:      true,
	}
	engine.RegisterFile(pinnedFile)

	// 分析并迁移 - 固定文件不应被迁移
	tasks, err := engine.AnalyzeAndMigrate()
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestTieringEngine_DisabledAutoTiering(t *testing.T) {
	config := DefaultTierConfig()
	config.EnableAutoTiering = false
	engine := NewTieringEngine(config)

	tasks, err := engine.AnalyzeAndMigrate()
	require.NoError(t, err)
	assert.Nil(t, tasks)
}

func TestTieringEngine_ConcurrentAccess(t *testing.T) {
	engine := NewTieringEngine(nil)
	engine.Start()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			file := &FileMetadata{
				ID:            "file-" + string(rune(id+'0')),
				Name:          "test.txt",
				CurrentTierID: "tier-1",
				LastAccessed:  time.Now(),
			}
			engine.RegisterFile(file)
			engine.RecordAccess(file.ID)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := engine.GetStats()
	assert.Equal(t, 10, stats.TotalFiles)
}
