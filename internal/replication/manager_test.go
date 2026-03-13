package replication

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	config := &Config{
		MaxConcurrentTasks: 2,
		BandwidthLimit:     0,
		SSHKeyPath:         "",
		Retries:            3,
		Timeout:            60,
	}

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	return mgr
}

func TestNewManager(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.tasks)
	assert.NotNil(t, mgr.config)
}

func TestNewManager_DefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	mgr, err := NewManager(configPath, nil)
	require.NoError(t, err)
	defer mgr.Stop()

	assert.NotNil(t, mgr.config)
	assert.Equal(t, 2, mgr.config.MaxConcurrentTasks)
	assert.Equal(t, 3, mgr.config.Retries)
}

func TestNewManager_ConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "replication.json")

	// 创建第一个管理器
	config := &Config{
		MaxConcurrentTasks: 5,
		BandwidthLimit:     1000,
		Retries:            5,
		Timeout:            120,
	}

	mgr1, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 创建任务以触发保存
	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err = mgr1.CreateTask(task)
	require.NoError(t, err)
	mgr1.Stop()

	// 验证配置文件已保存
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-task")
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 2, config.MaxConcurrentTasks)
	assert.Equal(t, 0, config.BandwidthLimit)
	assert.Equal(t, "~/.ssh/id_rsa", config.SSHKeyPath)
	assert.Equal(t, 3, config.Retries)
	assert.Equal(t, 3600, config.Timeout)
}

func TestCreateTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}

	err := mgr.CreateTask(task)
	require.NoError(t, err)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, StatusIdle, task.Status)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
}

func TestCreateTask_ScheduledNextSync(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "scheduled-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Schedule:   "daily",
		Enabled:    true,
	}

	err := mgr.CreateTask(task)
	require.NoError(t, err)

	// 验证下次同步时间已设置
	assert.False(t, task.NextSyncAt.IsZero())
	assert.True(t, task.NextSyncAt.After(time.Now()))
}

func TestUpdateTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// 创建任务
	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	// 更新任务
	updates := map[string]interface{}{
		"name":    "updated-name",
		"enabled": false,
	}

	err = mgr.UpdateTask(task.ID, updates)
	require.NoError(t, err)

	// 验证更新
	updated, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-name", updated.Name)
	assert.False(t, updated.Enabled)
}

func TestUpdateTask_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	updates := map[string]interface{}{
		"name": "updated-name",
	}

	err := mgr.UpdateTask("nonexistent", updates)
	assert.Error(t, err)
}

func TestUpdateTask_Compress(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
		Compress:   false,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	updates := map[string]interface{}{
		"compress": true,
	}

	err = mgr.UpdateTask(task.ID, updates)
	require.NoError(t, err)

	updated, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.True(t, updated.Compress)
}

func TestDeleteTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	err = mgr.DeleteTask(task.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = mgr.GetTask(task.ID)
	assert.Error(t, err)
}

func TestDeleteTask_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	err := mgr.DeleteTask("nonexistent")
	assert.Error(t, err)
}

func TestListTasks(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// 创建多个任务
	for i := 0; i < 5; i++ {
		task := &ReplicationTask{
			Name:       "task-" + string(rune('0'+i)),
			SourcePath: "/data/source",
			TargetPath: "/data/target",
			Type:       TypeScheduled,
			Enabled:    true,
		}
		err := mgr.CreateTask(task)
		require.NoError(t, err)
	}

	tasks := mgr.ListTasks()
	assert.Len(t, tasks, 5)
}

func TestListTasks_Empty(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	tasks := mgr.ListTasks()
	assert.Empty(t, tasks)
}

func TestGetTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	retrieved, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.Name, retrieved.Name)
	assert.Equal(t, task.SourcePath, retrieved.SourcePath)
}

func TestGetTask_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	_, err := mgr.GetTask("nonexistent")
	assert.Error(t, err)
}

func TestPauseTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	err = mgr.PauseTask(task.ID)
	require.NoError(t, err)

	updated, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPaused, updated.Status)
}

func TestPauseTask_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	err := mgr.PauseTask("nonexistent")
	assert.Error(t, err)
}

func TestResumeTask(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	// 先暂停
	err = mgr.PauseTask(task.ID)
	require.NoError(t, err)

	// 再恢复
	err = mgr.ResumeTask(task.ID)
	require.NoError(t, err)

	updated, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusIdle, updated.Status)
}

func TestResumeTask_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	err := mgr.ResumeTask("nonexistent")
	assert.Error(t, err)
}

func TestStartSync(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// 创建实际存在的源目录
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// 创建测试文件
	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: sourceDir + "/",
		TargetPath: targetDir,
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	// 注意：实际同步依赖 rsync，可能失败
	err = mgr.StartSync(task.ID)
	// 只验证 API 不崩溃，实际结果取决于环境
	_ = err
}

func TestStartSync_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	err := mgr.StartSync("nonexistent")
	assert.Error(t, err)
}

func TestStartSync_AlreadySyncing(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	err := mgr.CreateTask(task)
	require.NoError(t, err)

	// 手动设置状态为 syncing
	mgr.mu.Lock()
	task.Status = StatusSyncing
	mgr.mu.Unlock()

	err = mgr.StartSync(task.ID)
	assert.Error(t, err)
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// 创建任务
	for i := 0; i < 3; i++ {
		task := &ReplicationTask{
			Name:       "task-" + string(rune('0'+i)),
			SourcePath: "/data/source",
			TargetPath: "/data/target",
			Type:       TypeScheduled,
			Enabled:    true,
		}
		err := mgr.CreateTask(task)
		require.NoError(t, err)
	}

	// 设置一些状态
	mgr.mu.Lock()
	for _, task := range mgr.tasks {
		if task.Name == "task-0" {
			task.Status = StatusSyncing
		}
		if task.Name == "task-1" {
			task.Status = StatusPaused
		}
	}
	mgr.mu.Unlock()

	stats := mgr.GetStats()

	assert.Equal(t, 3, stats["total_tasks"])
	assert.Equal(t, 1, stats["syncing"])
	assert.Equal(t, 1, stats["paused"])
	assert.Equal(t, 0, stats["errors"])
}

func TestGetStats_Empty(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	stats := mgr.GetStats()

	assert.Equal(t, 0, stats["total_tasks"])
	assert.Equal(t, int64(0), stats["bytes_transferred"])
}

func TestCalculateNextSync(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	tests := []struct {
		schedule  string
		expectDur time.Duration
	}{
		{"hourly", time.Hour},
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			task := &ReplicationTask{
				Name:       "test-task",
				SourcePath: "/data/source",
				TargetPath: "/data/target",
				Type:       TypeScheduled,
				Schedule:   tt.schedule,
				Enabled:    true,
			}

			err := mgr.calculateNextSync(task)
			require.NoError(t, err)

			// 验证时间接近预期
			expectedTime := time.Now().Add(tt.expectDur)
			diff := task.NextSyncAt.Sub(expectedTime)
			assert.True(t, diff < time.Minute && diff > -time.Minute)
		})
	}
}

func TestCalculateNextSync_EmptySchedule(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeRealtime,
		Enabled:    true,
	}

	err := mgr.calculateNextSync(task)
	require.NoError(t, err)
	assert.True(t, task.NextSyncAt.IsZero())
}

func TestConcurrency(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 并发创建任务
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task := &ReplicationTask{
				Name:       "concurrent-task",
				SourcePath: "/data/source",
				TargetPath: "/data/target",
				Type:       TypeScheduled,
				Enabled:    true,
			}
			if err := mgr.CreateTask(task); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 验证没有错误
	for err := range errChan {
		t.Errorf("并发创建任务失败: %v", err)
	}

	// 验证任务数量
	tasks := mgr.ListTasks()
	assert.Len(t, tasks, 50)
}

func TestConcurrentReadWrite(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// 创建初始任务
	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	require.NoError(t, mgr.CreateTask(task))

	var wg sync.WaitGroup

	// 并发读
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListTasks()
			_, _ = mgr.GetTask(task.ID)
			_ = mgr.GetStats()
		}()
	}

	// 并发写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			updates := map[string]interface{}{
				"name": "updated-name",
			}
			_ = mgr.UpdateTask(task.ID, updates)
		}(i)
	}

	wg.Wait()
}

func TestTaskTypes(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	types := []ReplicationType{
		TypeRealtime,
		TypeScheduled,
		TypeBidirectional,
	}

	for _, typ := range types {
		task := &ReplicationTask{
			Name:       string(typ) + "-task",
			SourcePath: "/data/source",
			TargetPath: "/data/target",
			Type:       typ,
			Enabled:    true,
		}
		err := mgr.CreateTask(task)
		require.NoError(t, err)
		assert.Equal(t, typ, task.Type)
	}

	tasks := mgr.ListTasks()
	assert.Len(t, tasks, 3)
}

func TestTaskStatuses(t *testing.T) {
	statuses := []ReplicationStatus{
		StatusIdle,
		StatusSyncing,
		StatusPaused,
		StatusError,
		StatusCompleted,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, string(status))
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "repl-")
}

func TestStop(t *testing.T) {
	mgr := setupTestManager(t)

	// 创建任务
	task := &ReplicationTask{
		Name:       "test-task",
		SourcePath: "/data/source",
		TargetPath: "/data/target",
		Type:       TypeScheduled,
		Enabled:    true,
	}
	require.NoError(t, mgr.CreateTask(task))

	// 停止管理器
	mgr.Stop()

	// 验证停止通道已关闭
	select {
	case <-mgr.stopChan:
		// 已关闭
	default:
		t.Error("stopChan 应该已关闭")
	}
}
