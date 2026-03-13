package cloudsync

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Manager 扩展测试 ====================

func TestManager_ListSyncTasks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 创建提供商
	provider, err := m.CreateProvider(ProviderConfig{
		Name:      "test",
		Type:      ProviderAWSS3,
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "bucket",
	})
	require.NoError(t, err)

	// 创建多个任务
	for i := 0; i < 3; i++ {
		_, err = m.CreateSyncTask(SyncTask{
			Name:       "task-" + string(rune('A'+i)),
			ProviderID: provider.ID,
			LocalPath:  "/tmp/test",
			RemotePath: "/backup",
		})
		require.NoError(t, err)
	}

	tasks := m.ListSyncTasks()
	assert.Len(t, tasks, 3)
}

func TestManager_UpdateSyncTask(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test",
		Type:      ProviderAWSS3,
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "bucket",
	})

	task, err := m.CreateSyncTask(SyncTask{
		Name:       "test-task",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/old",
		RemotePath: "/backup",
	})
	require.NoError(t, err)

	// 更新任务
	err = m.UpdateSyncTask(task.ID, SyncTask{
		Name:       "updated-task",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/new",
		RemotePath: "/backup/new",
		Direction:  SyncDirectionUpload,
	})
	require.NoError(t, err)

	// 验证更新
	updated, err := m.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-task", updated.Name)
	assert.Equal(t, "/tmp/new", updated.LocalPath)
}

func TestManager_GetAllSyncStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test",
		Type:      ProviderAWSS3,
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "bucket",
	})

	task, _ := m.CreateSyncTask(SyncTask{
		Name:       "test-task",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	})

	statuses := m.GetAllSyncStatuses()
	assert.Contains(t, statuses, task.ID)
}

func TestManager_ValidateProviderConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name: "valid S3 config",
			config: ProviderConfig{
				Type:      ProviderAWSS3,
				AccessKey: "key",
				SecretKey: "secret",
				Bucket:    "bucket",
			},
			wantErr: false,
		},
		{
			name: "missing AccessKey",
			config: ProviderConfig{
				Type:      ProviderAWSS3,
				SecretKey: "secret",
				Bucket:    "bucket",
			},
			wantErr: true,
		},
		{
			name: "missing SecretKey",
			config: ProviderConfig{
				Type:      ProviderAWSS3,
				AccessKey: "key",
				Bucket:    "bucket",
			},
			wantErr: true,
		},
		{
			name: "missing Bucket",
			config: ProviderConfig{
				Type:      ProviderAWSS3,
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "WebDAV missing endpoint",
			config: ProviderConfig{
				Type: ProviderWebDAV,
			},
			wantErr: true,
		},
		{
			name: "WebDAV valid",
			config: ProviderConfig{
				Type:     ProviderWebDAV,
				Endpoint: "https://webdav.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Name = "test"
			_, err := m.CreateProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_ScheduleTask(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	provider, _ := m.CreateProvider(ProviderConfig{
		Name:      "test",
		Type:      ProviderAWSS3,
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "bucket",
	})

	// 创建定时任务 (使用 6 字段 cron 表达式: 秒 分 时 日 月 周)
	task, err := m.CreateSyncTask(SyncTask{
		Name:         "scheduled-task",
		ProviderID:   provider.ID,
		LocalPath:    "/tmp/test",
		RemotePath:   "/backup",
		ScheduleType: ScheduleTypeCron,
		ScheduleExpr: "0 0 * * * *", // 每小时
	})
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
}

// ==================== 调度器扩展测试 ====================

func TestScheduler_AddCronTask(t *testing.T) {
	scheduler := NewScheduler()
	go scheduler.Run()
	defer scheduler.Stop()

	// 使用 6 字段 cron 表达式
	err := scheduler.AddCronTask("cron-task", "0 0 * * * *", func() {})
	require.NoError(t, err)

	tasks := scheduler.ListTasks()
	assert.Contains(t, tasks, "cron-task")
}

func TestScheduler_RemoveTask(t *testing.T) {
	scheduler := NewScheduler()
	go scheduler.Run()
	defer scheduler.Stop()

	err := scheduler.AddCronTask("task-to-remove", "0 0 * * * *", func() {})
	require.NoError(t, err)

	scheduler.RemoveTask("task-to-remove")

	tasks := scheduler.ListTasks()
	assert.NotContains(t, tasks, "task-to-remove")
}

// ==================== 类型测试 ====================

func TestSyncTask_Fields(t *testing.T) {
	task := SyncTask{
		ID:               "task-123",
		Name:             "test-sync",
		ProviderID:       "provider-456",
		LocalPath:        "/data/files",
		RemotePath:       "/backup/files",
		Direction:        SyncDirectionBidirect,
		Mode:             SyncModeSync,
		ScheduleType:     ScheduleTypeCron,
		ScheduleExpr:     "0 2 * * *",
		ConflictStrategy: ConflictStrategyNewer,
		ExcludePatterns:  []string{"*.tmp", "*.log"},
		IncludePatterns:  []string{"*.txt", "*.pdf"},
		DeleteRemote:     true,
		DeleteLocal:      false,
		ChecksumVerify:   true,
		PreserveModTime:  true,
		MaxFileSize:      100 * 1024 * 1024,
		Enabled:          true,
		Status:           TaskStatusIdle,
	}

	assert.Equal(t, "task-123", task.ID)
	assert.Equal(t, SyncDirectionBidirect, task.Direction)
	assert.True(t, task.ChecksumVerify)
	assert.True(t, task.Enabled)
}

func TestSyncError(t *testing.T) {
	syncErr := SyncError{
		Time:   time.Now(),
		Path:   "/data/file.txt",
		Action: "upload",
		Error:  "connection refused",
	}

	assert.Equal(t, "upload", syncErr.Action)
	assert.Contains(t, syncErr.Error, "connection")
}

func TestConflictInfo_Fields(t *testing.T) {
	conflict := ConflictInfo{
		Path:          "/data/file.txt",
		LocalSize:     1024,
		LocalHash:     "abc123",
		RemoteSize:    2048,
		RemoteHash:    "def456",
	}

	assert.Equal(t, int64(1024), conflict.LocalSize)
	assert.Equal(t, int64(2048), conflict.RemoteSize)
	assert.NotEqual(t, conflict.LocalHash, conflict.RemoteHash)
}