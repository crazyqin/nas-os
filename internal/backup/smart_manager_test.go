// Package backup 智能备份管理器测试
// Version: v2.50.0 - 单元测试
package backup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

func setupTestV2(t *testing.T) (*SmartManagerV2, string, func()) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	require.NoError(t, err)

	// 创建测试配置路径
	configPath := filepath.Join(tempDir, "backup-config.json")

	// 预创建配置文件，使用临时目录
	config := DefaultSmartBackupConfigV2()
	config.BackupPath = filepath.Join(tempDir, "backups")
	config.TempPath = filepath.Join(tempDir, "temp")
	config.IndexDBPath = filepath.Join(tempDir, "index.db")

	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	// 创建测试源目录
	sourceDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// 创建测试文件
	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	// 创建管理器
	logger := zap.NewNop()
	manager, err := NewSmartManagerV2(configPath, logger)
	require.NoError(t, err)

	cleanup := func() {
		manager.Close()
		os.RemoveAll(tempDir)
	}

	return manager, sourceDir, cleanup
}

// ============================================================================
// SmartManagerV2 测试
// ============================================================================

func TestNewSmartManagerV2(t *testing.T) {
	t.Run("创建管理器成功", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "backup-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		configPath := filepath.Join(tempDir, "config.json")

		// 预创建配置文件
		config := DefaultSmartBackupConfigV2()
		config.BackupPath = filepath.Join(tempDir, "backups")
		config.TempPath = filepath.Join(tempDir, "temp")
		config.IndexDBPath = filepath.Join(tempDir, "index.db")
		configData, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile(configPath, configData, 0644)

		logger := zap.NewNop()
		manager, err := NewSmartManagerV2(configPath, logger)
		require.NoError(t, err)
		require.NotNil(t, manager)
		defer manager.Close()

		assert.NotNil(t, manager.config)
		assert.NotNil(t, manager.jobs)
		assert.NotNil(t, manager.activeJobs)
		assert.NotNil(t, manager.backupIndex)
	})

	t.Run("使用默认配置", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "backup-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		configPath := filepath.Join(tempDir, "config.json")

		// 预创建配置文件
		config := DefaultSmartBackupConfigV2()
		config.BackupPath = filepath.Join(tempDir, "backups")
		config.TempPath = filepath.Join(tempDir, "temp")
		config.IndexDBPath = filepath.Join(tempDir, "index.db")
		configData, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile(configPath, configData, 0644)

		logger := zap.NewNop()
		manager, err := NewSmartManagerV2(configPath, logger)
		require.NoError(t, err)
		defer manager.Close()

		// 验证默认配置
		assert.True(t, manager.config.Compression.Enabled)
		assert.Equal(t, "gzip", manager.config.Compression.Algorithm)
		assert.True(t, manager.config.Incremental.Enabled)
		assert.True(t, manager.config.Versioning.Enabled)
		assert.True(t, manager.config.Cleanup.Enabled)
	})
}

func TestCreateBackupJobV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	t.Run("创建作业成功", func(t *testing.T) {
		job := &SmartBackupJobV2{
			Name:     "test-backup",
			Source:   sourceDir,
			Enabled:  true,
			Priority: 5,
		}

		err := manager.CreateJob(job)
		assert.NoError(t, err)
		assert.NotEmpty(t, job.ID)
		assert.False(t, job.CreatedAt.IsZero())
	})

	t.Run("名称不能为空", func(t *testing.T) {
		job := &SmartBackupJobV2{
			Source: sourceDir,
		}

		err := manager.CreateJob(job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "名称不能为空")
	})

	t.Run("源路径不能为空", func(t *testing.T) {
		job := &SmartBackupJobV2{
			Name: "test-backup",
		}

		err := manager.CreateJob(job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "源路径不能为空")
	})

	t.Run("源路径必须存在", func(t *testing.T) {
		job := &SmartBackupJobV2{
			Name:   "test-backup",
			Source: "/nonexistent/path",
		}

		err := manager.CreateJob(job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "源路径无效")
	})
}

func TestListJobsV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	// 创建多个作业
	for i := 0; i < 3; i++ {
		job := &SmartBackupJobV2{
			Name:   "test-backup-" + string(rune('A'+i)),
			Source: sourceDir,
		}
		require.NoError(t, manager.CreateJob(job))
	}

	jobs := manager.ListJobs()
	assert.Len(t, jobs, 3)
}

func TestGetJobV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	job := &SmartBackupJobV2{
		Name:   "test-backup",
		Source: sourceDir,
	}
	require.NoError(t, manager.CreateJob(job))

	t.Run("获取存在的作业", func(t *testing.T) {
		retrieved, err := manager.GetJob(job.ID)
		assert.NoError(t, err)
		assert.Equal(t, job.Name, retrieved.Name)
	})

	t.Run("获取不存在的作业", func(t *testing.T) {
		_, err := manager.GetJob("nonexistent-id")
		assert.Error(t, err)
	})
}

func TestUpdateJobV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	job := &SmartBackupJobV2{
		Name:   "test-backup",
		Source: sourceDir,
	}
	require.NoError(t, manager.CreateJob(job))

	updatedJob := &SmartBackupJobV2{
		Name:   "updated-backup",
		Source: sourceDir,
	}

	err := manager.UpdateJob(job.ID, updatedJob)
	assert.NoError(t, err)

	retrieved, _ := manager.GetJob(job.ID)
	assert.Equal(t, "updated-backup", retrieved.Name)
}

func TestDeleteJobV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	job := &SmartBackupJobV2{
		Name:   "test-backup",
		Source: sourceDir,
	}
	require.NoError(t, manager.CreateJob(job))

	err := manager.DeleteJob(job.ID)
	assert.NoError(t, err)

	_, err = manager.GetJob(job.ID)
	assert.Error(t, err)
}

func TestGetStatsV2(t *testing.T) {
	manager, _, cleanup := setupTestV2(t)
	defer cleanup()

	stats := manager.GetStats()
	require.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalBackups, 0)
}

func TestHealthCheckV2(t *testing.T) {
	manager, _, cleanup := setupTestV2(t)
	defer cleanup()

	result, err := manager.HealthCheck()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "healthy", result.Status)
	assert.NotNil(t, result.Details)
}

// ============================================================================
// BackupScheduler 测试
// ============================================================================

func TestNewBackupScheduler(t *testing.T) {
	_, _, cleanup := setupTestV2(t)
	defer cleanup()

	t.Run("创建调度器成功", func(t *testing.T) {
		config := DefaultSchedulerConfig()
		logger := zap.NewNop()

		scheduler := NewBackupScheduler(config, nil, logger)
		require.NotNil(t, scheduler)
		assert.Equal(t, config.MaxConcurrent, scheduler.config.MaxConcurrent)
	})

	t.Run("使用默认配置", func(t *testing.T) {
		logger := zap.NewNop()
		scheduler := NewBackupScheduler(nil, nil, logger)
		require.NotNil(t, scheduler)
		assert.Equal(t, 3, scheduler.config.MaxConcurrent)
	})
}

func TestSchedulerLifecycle(t *testing.T) {
	_, _, cleanup := setupTestV2(t)
	defer cleanup()

	config := DefaultSchedulerConfig()
	logger := zap.NewNop()
	scheduler := NewBackupScheduler(config, nil, logger)

	t.Run("启动调度器", func(t *testing.T) {
		err := scheduler.Start()
		assert.NoError(t, err)
	})

	t.Run("重复启动", func(t *testing.T) {
		err := scheduler.Start()
		assert.Error(t, err)
	})

	t.Run("停止调度器", func(t *testing.T) {
		err := scheduler.Stop()
		assert.NoError(t, err)
	})
}

func TestPriorityQueue(t *testing.T) {
	t.Run("入队和出队", func(t *testing.T) {
		queue := NewPriorityQueue(10)

		job1 := &ScheduledJob{ID: "job1", Priority: 5}
		job2 := &ScheduledJob{ID: "job2", Priority: 10}
		job3 := &ScheduledJob{ID: "job3", Priority: 3}

		queue.Push(job1)
		queue.Push(job2)
		queue.Push(job3)

		assert.Equal(t, 3, queue.Len())

		// 应该按优先级出队（高优先级先出）
		item := queue.Pop()
		assert.Equal(t, "job2", item.Job.ID)

		item = queue.Pop()
		assert.Equal(t, "job1", item.Job.ID)

		item = queue.Pop()
		assert.Equal(t, "job3", item.Job.ID)
	})

	t.Run("空队列", func(t *testing.T) {
		queue := NewPriorityQueue(10)
		assert.Equal(t, 0, queue.Len())

		item := queue.Pop()
		assert.Nil(t, item)
	})

	t.Run("清空队列", func(t *testing.T) {
		queue := NewPriorityQueue(10)
		queue.Push(&ScheduledJob{ID: "job1"})
		queue.Push(&ScheduledJob{ID: "job2"})

		queue.Clear()
		assert.Equal(t, 0, queue.Len())
	})
}

func TestBackupWindow(t *testing.T) {
	_, _, cleanup := setupTestV2(t)
	defer cleanup()

	config := DefaultSchedulerConfig()
	config.WindowEnabled = true
	config.WindowStartHour = 22
	config.WindowEndHour = 6

	logger := zap.NewNop()
	scheduler := NewBackupScheduler(config, nil, logger)

	t.Run("获取备份窗口", func(t *testing.T) {
		window := scheduler.GetWindow()
		require.NotNil(t, window)
		assert.Equal(t, 22, window.StartHour)
		assert.Equal(t, 6, window.EndHour)
	})

	t.Run("设置备份窗口", func(t *testing.T) {
		scheduler.SetWindow(20, 8)

		window := scheduler.GetWindow()
		require.NotNil(t, window)
		assert.Equal(t, 20, window.StartHour)
		assert.Equal(t, 8, window.EndHour)
	})

	t.Run("禁用备份窗口", func(t *testing.T) {
		scheduler.DisableWindow()

		window := scheduler.GetWindow()
		assert.Nil(t, window)
	})
}

func TestSchedulerStats(t *testing.T) {
	_, _, cleanup := setupTestV2(t)
	defer cleanup()

	config := DefaultSchedulerConfig()
	logger := zap.NewNop()
	scheduler := NewBackupScheduler(config, nil, logger)

	stats := scheduler.GetStats()
	require.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.QueueLength, 0)
	assert.GreaterOrEqual(t, stats.RunningJobs, 0)
}

// ============================================================================
// 配置测试
// ============================================================================

func TestCompressionConfigV2(t *testing.T) {
	config := DefaultSmartBackupConfigV2()

	assert.True(t, config.Compression.Enabled)
	assert.Equal(t, "gzip", config.Compression.Algorithm)
	assert.Equal(t, 6, config.Compression.Level)
}

func TestVersioningConfigV2(t *testing.T) {
	config := DefaultSmartBackupConfigV2()

	assert.True(t, config.Versioning.Enabled)
	assert.Equal(t, 100, config.Versioning.MaxVersions)
	assert.Equal(t, 7, config.Versioning.KeepDaily)
	assert.Equal(t, 4, config.Versioning.KeepWeekly)
	assert.Equal(t, 12, config.Versioning.KeepMonthly)
}

func TestCleanupConfigV2(t *testing.T) {
	config := DefaultSmartBackupConfigV2()

	assert.True(t, config.Cleanup.Enabled)
	assert.Equal(t, 90, config.Cleanup.MaxAge)
	assert.Greater(t, config.Cleanup.MinFreeSpace, int64(0))
}

// ============================================================================
// 并发测试
// ============================================================================

func TestConcurrentJobCreationV2(t *testing.T) {
	manager, sourceDir, cleanup := setupTestV2(t)
	defer cleanup()

	const numJobs = 10
	done := make(chan error, numJobs)

	for i := 0; i < numJobs; i++ {
		go func(idx int) {
			job := &SmartBackupJobV2{
				Name:   "concurrent-job-" + string(rune('0'+idx)),
				Source: sourceDir,
			}
			done <- manager.CreateJob(job)
		}(i)
	}

	for i := 0; i < numJobs; i++ {
		err := <-done
		assert.NoError(t, err)
	}

	jobs := manager.ListJobs()
	assert.Len(t, jobs, numJobs)
}

// ============================================================================
// 基准测试
// ============================================================================

func BenchmarkPriorityQueuePushV2(b *testing.B) {
	queue := NewPriorityQueue(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Push(&ScheduledJob{
			ID:       "job",
			Priority: i % 10,
		})
	}
}

func BenchmarkPriorityQueuePopV2(b *testing.B) {
	queue := NewPriorityQueue(10000)

	// 预填充
	for i := 0; i < 1000; i++ {
		queue.Push(&ScheduledJob{ID: "job", Priority: i % 10})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if queue.Len() == 0 {
			queue.Push(&ScheduledJob{ID: "job", Priority: 5})
		}
		queue.Pop()
	}
}

// 需要导入 context.
var _ = context.Background
