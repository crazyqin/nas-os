package tiering

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_NewManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	manager := NewManager(configPath, config)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.tiers)
	assert.NotNil(t, manager.policies)
	assert.NotNil(t, manager.tracker)
	assert.NotNil(t, manager.migrator)
}

func TestManager_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false // 禁用自动策略引擎

	manager := NewManager(configPath, config)
	err := manager.Initialize()
	require.NoError(t, err)

	defer manager.Stop()

	// 验证默认存储层已初始化
	tiers := manager.ListTiers()
	assert.Len(t, tiers, 3)
}

func TestManager_TierOperations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	manager := NewManager(configPath, config)
	err := manager.Initialize()
	require.NoError(t, err)
	defer manager.Stop()

	t.Run("GetTier", func(t *testing.T) {
		tier, err := manager.GetTier(TierTypeSSD)
		require.NoError(t, err)
		assert.Equal(t, TierTypeSSD, tier.Type)
		assert.Equal(t, "SSD 缓存层", tier.Name)
	})

	t.Run("GetTier_NotFound", func(t *testing.T) {
		_, err := manager.GetTier(TierType("unknown"))
		assert.Error(t, err)
	})

	t.Run("ListTiers", func(t *testing.T) {
		tiers := manager.ListTiers()
		assert.Len(t, tiers, 3)
	})

	t.Run("UpdateTier", func(t *testing.T) {
		newConfig := TierConfig{
			Name:      "Updated SSD",
			Path:      "/mnt/ssd-new",
			Priority:  150,
			Enabled:   true,
			Threshold: 85,
		}

		err := manager.UpdateTier(TierTypeSSD, newConfig)
		require.NoError(t, err)

		tier, err := manager.GetTier(TierTypeSSD)
		require.NoError(t, err)
		assert.Equal(t, "Updated SSD", tier.Name)
		assert.Equal(t, "/mnt/ssd-new", tier.Path)
	})
}

func TestManager_PolicyOperations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	manager := NewManager(configPath, config)
	err := manager.Initialize()
	require.NoError(t, err)
	defer manager.Stop()

	t.Run("CreatePolicy", func(t *testing.T) {
		policy := Policy{
			Name:        "hot-to-cold",
			Description: "Move cold data to HDD",
			Enabled:     true,
			SourceTier:  TierTypeSSD,
			TargetTier:  TierTypeHDD,
			Action:      PolicyActionMove,
		}

		created, err := manager.CreatePolicy(policy)
		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, PolicyStatusEnabled, created.Status)
	})

	t.Run("CreatePolicy_InvalidSourceTier", func(t *testing.T) {
		policy := Policy{
			Name:       "invalid-policy",
			Enabled:    true,
			SourceTier: TierType("unknown"),
			TargetTier: TierTypeHDD,
		}

		_, err := manager.CreatePolicy(policy)
		assert.Error(t, err)
	})

	t.Run("GetPolicy", func(t *testing.T) {
		// 先创建一个策略
		policy := Policy{
			Name:       "test-policy",
			Enabled:    true,
			SourceTier: TierTypeSSD,
			TargetTier: TierTypeHDD,
		}
		created, _ := manager.CreatePolicy(policy)

		found, err := manager.GetPolicy(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("ListPolicies", func(t *testing.T) {
		policies := manager.ListPolicies()
		assert.GreaterOrEqual(t, len(policies), 1)
	})

	t.Run("UpdatePolicy", func(t *testing.T) {
		policy := Policy{
			Name:       "update-test",
			Enabled:    true,
			SourceTier: TierTypeSSD,
			TargetTier: TierTypeHDD,
		}
		created, _ := manager.CreatePolicy(policy)

		updated := Policy{
			Name:        "updated-policy",
			Description: "Updated description",
			Enabled:     false,
			SourceTier:  TierTypeSSD,
			TargetTier:  TierTypeHDD,
		}

		err := manager.UpdatePolicy(created.ID, updated)
		require.NoError(t, err)

		found, _ := manager.GetPolicy(created.ID)
		assert.Equal(t, "updated-policy", found.Name)
		assert.Equal(t, PolicyStatusDisabled, found.Status)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		policy := Policy{
			Name:       "delete-test",
			Enabled:    true,
			SourceTier: TierTypeSSD,
			TargetTier: TierTypeHDD,
		}
		created, _ := manager.CreatePolicy(policy)

		err := manager.DeletePolicy(created.ID)
		require.NoError(t, err)

		_, err = manager.GetPolicy(created.ID)
		assert.Error(t, err)
	})
}

func TestManager_Migrate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	manager := NewManager(configPath, config)

	// 更新存储层路径为测试目录
	manager.mu.Lock()
	manager.tiers[TierTypeSSD] = &TierConfig{
		Type:     TierTypeSSD,
		Name:     "SSD",
		Path:     tmpDir,
		Enabled:  true,
		Priority: 100,
	}
	manager.tiers[TierTypeHDD] = &TierConfig{
		Type:     TierTypeHDD,
		Name:     "HDD",
		Path:     filepath.Join(tmpDir, "hdd"),
		Enabled:  true,
		Priority: 50,
	}
	manager.mu.Unlock()

	// 创建 HDD 目录
	err = os.MkdirAll(filepath.Join(tmpDir, "hdd"), 0755)
	require.NoError(t, err)

	err = manager.Initialize()
	require.NoError(t, err)
	defer manager.Stop()

	t.Run("Migrate_InvalidSourceTier", func(t *testing.T) {
		req := MigrateRequest{
			Paths:      []string{testFile},
			SourceTier: TierType("unknown"),
			TargetTier: TierTypeHDD,
		}
		_, err := manager.Migrate(req)
		assert.Error(t, err)
	})

	t.Run("Migrate_EmptyPaths", func(t *testing.T) {
		req := MigrateRequest{
			Paths:      []string{},
			SourceTier: TierTypeSSD,
			TargetTier: TierTypeHDD,
		}
		_, err := manager.Migrate(req)
		assert.Error(t, err)
	})
}

func TestManager_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	manager := NewManager(configPath, config)
	err := manager.Initialize()
	require.NoError(t, err)
	defer manager.Stop()

	status := manager.GetStatus()

	assert.NotNil(t, status)
	assert.False(t, status.Enabled) // 我们禁用了自动分层
	assert.Len(t, status.Tiers, 3)
}

func TestManager_TaskOperations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	manager := NewManager(configPath, config)
	err := manager.Initialize()
	require.NoError(t, err)
	defer manager.Stop()

	t.Run("ListTasks", func(t *testing.T) {
		tasks := manager.ListTasks(10)
		assert.NotNil(t, tasks)
	})

	t.Run("GetTask_NotFound", func(t *testing.T) {
		_, err := manager.GetTask("non-existent")
		assert.Error(t, err)
	})
}

func TestAccessTracker(t *testing.T) {
	config := DefaultPolicyEngineConfig()
	tracker := NewAccessTracker(config)

	err := tracker.Start()
	require.NoError(t, err)
	defer tracker.Stop()

	t.Run("RecordAccess", func(t *testing.T) {
		err := tracker.RecordAccess("/test/file.txt", TierTypeSSD, 1024, 512)
		require.NoError(t, err)

		record, err := tracker.GetRecord("/test/file.txt")
		require.NoError(t, err)
		assert.Equal(t, int64(1), record.AccessCount)
		assert.Equal(t, int64(1024), record.ReadBytes)
		assert.Equal(t, int64(512), record.WriteBytes)
	})

	t.Run("RecordFileRead", func(t *testing.T) {
		err := tracker.RecordFileRead("/test/read.txt", TierTypeSSD, 2048)
		require.NoError(t, err)

		record, err := tracker.GetRecord("/test/read.txt")
		require.NoError(t, err)
		assert.Equal(t, int64(2048), record.ReadBytes)
	})

	t.Run("RecordFileWrite", func(t *testing.T) {
		err := tracker.RecordFileWrite("/test/write.txt", TierTypeHDD, 4096)
		require.NoError(t, err)

		record, err := tracker.GetRecord("/test/write.txt")
		require.NoError(t, err)
		assert.Equal(t, int64(4096), record.WriteBytes)
	})

	t.Run("GetRecordsByTier", func(t *testing.T) {
		tracker.RecordAccess("/test/ssd1.txt", TierTypeSSD, 100, 0)
		tracker.RecordAccess("/test/ssd2.txt", TierTypeSSD, 100, 0)
		tracker.RecordAccess("/test/hdd1.txt", TierTypeHDD, 100, 0)

		ssdRecords := tracker.GetRecordsByTier(TierTypeSSD)
		assert.GreaterOrEqual(t, len(ssdRecords), 2)

		hddRecords := tracker.GetRecordsByTier(TierTypeHDD)
		assert.GreaterOrEqual(t, len(hddRecords), 1)
	})

	t.Run("UpdateFileTier", func(t *testing.T) {
		tracker.RecordAccess("/test/move.txt", TierTypeSSD, 100, 0)

		err := tracker.UpdateFileTier("/test/move.txt", TierTypeHDD)
		require.NoError(t, err)

		record, _ := tracker.GetRecord("/test/move.txt")
		assert.Equal(t, TierTypeHDD, record.CurrentTier)
	})

	t.Run("RemoveRecord", func(t *testing.T) {
		tracker.RecordAccess("/test/remove.txt", TierTypeSSD, 100, 0)

		err := tracker.RemoveRecord("/test/remove.txt")
		require.NoError(t, err)

		_, err = tracker.GetRecord("/test/remove.txt")
		assert.Error(t, err)
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := tracker.GetStats()
		assert.NotNil(t, stats)
		assert.GreaterOrEqual(t, stats.TotalFiles, int64(1))
	})
}

func TestMigrator(t *testing.T) {
	config := DefaultPolicyEngineConfig()
	migrator := NewMigrator(config)
	migrator.Start()
	defer migrator.Stop()

	t.Run("EstimateMigration", func(t *testing.T) {
		files := []MigrateFile{
			{Path: "/test/file1.txt", Size: 1024},
			{Path: "/test/file2.txt", Size: 2048},
		}

		totalSize, fileCount := migrator.EstimateMigration(files, TierTypeHDD)
		assert.Equal(t, int64(3072), totalSize)
		assert.Equal(t, 2, fileCount)
	})
}

func TestTypes_TierType(t *testing.T) {
	tests := []struct {
		name     string
		tierType TierType
		expected string
	}{
		{"SSD", TierTypeSSD, "ssd"},
		{"HDD", TierTypeHDD, "hdd"},
		{"Cloud", TierTypeCloud, "cloud"},
		{"Memory", TierTypeMemory, "memory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, TierType(tt.expected), tt.tierType)
		})
	}
}

func TestTypes_PolicyAction(t *testing.T) {
	tests := []struct {
		name     string
		action   PolicyAction
		expected string
	}{
		{"Move", PolicyActionMove, "move"},
		{"Copy", PolicyActionCopy, "copy"},
		{"Archive", PolicyActionArchive, "archive"},
		{"Delete", PolicyActionDelete, "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, PolicyAction(tt.expected), tt.action)
		})
	}
}

func TestTypes_AccessFrequency(t *testing.T) {
	tests := []struct {
		name      string
		frequency AccessFrequency
		expected  string
	}{
		{"Hot", AccessFrequencyHot, "hot"},
		{"Warm", AccessFrequencyWarm, "warm"},
		{"Cold", AccessFrequencyCold, "cold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, AccessFrequency(tt.expected), tt.frequency)
		})
	}
}

func TestDefaultPolicyEngineConfig(t *testing.T) {
	config := DefaultPolicyEngineConfig()

	assert.Equal(t, 1*time.Hour, config.CheckInterval)
	assert.Equal(t, int64(100), config.HotThreshold)
	assert.Equal(t, int64(10), config.WarmThreshold)
	assert.Equal(t, 720, config.ColdAgeHours)
	assert.Equal(t, 5, config.MaxConcurrent)
	assert.True(t, config.EnableAutoTier)
}

func TestManager_ConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tiering.json")

	config := DefaultPolicyEngineConfig()
	config.EnableAutoTier = false

	// 创建第一个管理器并保存配置
	manager1 := NewManager(configPath, config)
	err := manager1.Initialize()
	require.NoError(t, err)

	// 创建策略
	policy := Policy{
		Name:       "persist-test",
		Enabled:    true,
		SourceTier: TierTypeSSD,
		TargetTier: TierTypeHDD,
	}
	created, err := manager1.CreatePolicy(policy)
	require.NoError(t, err)

	manager1.Stop()

	// 创建第二个管理器，验证配置已加载
	manager2 := NewManager(configPath, config)
	err = manager2.Initialize()
	require.NoError(t, err)
	defer manager2.Stop()

	// 验证策略已加载
	found, err := manager2.GetPolicy(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "persist-test", found.Name)
}