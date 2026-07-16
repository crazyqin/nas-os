package crossplatformsync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================
// 设备管理测试
// ============================================================

func TestCreateDevice(t *testing.T) {
	m := NewManager("", zap.NewNop())

	req := CreateDeviceRequest{
		Name:     "Test NAS",
		Address:  "192.168.1.100",
		Port:     8443,
		APIKey:   "test-key",
		Platform: "linux",
	}

	device, err := m.CreateDevice(req)
	require.NoError(t, err)
	assert.NotEmpty(t, device.ID)
	assert.Equal(t, "Test NAS", device.Name)
	assert.Equal(t, "192.168.1.100", device.Address)
	assert.Equal(t, 8443, device.Port)
	assert.Equal(t, DeviceStatusOffline, device.Status)
}

func TestCreateDeviceValidation(t *testing.T) {
	m := NewManager("", zap.NewNop())

	tests := []struct {
		name    string
		req     CreateDeviceRequest
		wantErr string
	}{
		{
			name:    "missing name",
			req:     CreateDeviceRequest{Address: "192.168.1.1", Port: 8443},
			wantErr: "device name is required",
		},
		{
			name:    "missing address",
			req:     CreateDeviceRequest{Name: "Test", Port: 8443},
			wantErr: "device address is required",
		},
		{
			name:    "invalid port",
			req:     CreateDeviceRequest{Name: "Test", Address: "192.168.1.1", Port: 0},
			wantErr: "invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.CreateDevice(tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestGetDevice(t *testing.T) {
	m := NewManager("", zap.NewNop())

	req := CreateDeviceRequest{Name: "Test", Address: "192.168.1.1", Port: 8443}
	created, err := m.CreateDevice(req)
	require.NoError(t, err)

	found, err := m.GetDevice(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	_, err = m.GetDevice("nonexistent")
	assert.Error(t, err)
}

func TestListDevices(t *testing.T) {
	m := NewManager("", zap.NewNop())

	m.CreateDevice(CreateDeviceRequest{Name: "NAS1", Address: "192.168.1.1", Port: 8443})
	m.CreateDevice(CreateDeviceRequest{Name: "NAS2", Address: "192.168.1.2", Port: 8443})

	devices := m.ListDevices()
	assert.Len(t, devices, 2)
}

func TestUpdateDevice(t *testing.T) {
	m := NewManager("", zap.NewNop())

	device, _ := m.CreateDevice(CreateDeviceRequest{Name: "Old Name", Address: "192.168.1.1", Port: 8443})

	newPort := 9443
	updated, err := m.UpdateDevice(device.ID, UpdateDeviceRequest{
		Name: "New Name",
		Port: &newPort,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, 9443, updated.Port)
}

func TestDeleteDevice(t *testing.T) {
	m := NewManager("", zap.NewNop())

	device, _ := m.CreateDevice(CreateDeviceRequest{Name: "Test", Address: "192.168.1.1", Port: 8443})

	err := m.DeleteDevice(device.ID)
	assert.NoError(t, err)

	_, err = m.GetDevice(device.ID)
	assert.Error(t, err)
}

func TestDeleteDeviceInUse(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "Source", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "Target", Address: "192.168.1.2", Port: 8443})

	m.CreateSyncTask(CreateSyncTaskRequest{
		Name:           "Test Task",
		SourceDeviceID: d1.ID,
		TargetDeviceID: d2.ID,
		SourcePath:     "/data",
		TargetPath:     "/data",
		Mode:           SyncModeBidirectional,
	})

	err := m.DeleteDevice(d1.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "in use")
}

func TestTestDeviceConnection(t *testing.T) {
	m := NewManager("", zap.NewNop())

	device, _ := m.CreateDevice(CreateDeviceRequest{Name: "Test", Address: "192.168.1.1", Port: 8443})

	result, err := m.TestDeviceConnection(device.ID)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.Latency, int64(0))
}

// ============================================================
// 同步任务管理测试
// ============================================================

func TestCreateSyncTask(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "Source", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "Target", Address: "192.168.1.2", Port: 8443})

	req := CreateSyncTaskRequest{
		Name:           "Test Sync",
		SourceDeviceID: d1.ID,
		TargetDeviceID: d2.ID,
		SourcePath:     "/volume1/data",
		TargetPath:     "/volume1/data",
		Mode:           SyncModeBidirectional,
	}

	task, err := m.CreateSyncTask(req)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Test Sync", task.Name)
	assert.Equal(t, SyncModeBidirectional, task.Mode)
	assert.Equal(t, TaskStatusIdle, task.Status)
	assert.True(t, task.Enabled)
}

func TestCreateSyncTaskValidation(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "Source", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "Target", Address: "192.168.1.2", Port: 8443})

	tests := []struct {
		name    string
		req     CreateSyncTaskRequest
		wantErr string
	}{
		{
			name:    "missing name",
			req:     CreateSyncTaskRequest{SourceDeviceID: d1.ID, TargetDeviceID: d2.ID, SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional},
			wantErr: "task name is required",
		},
		{
			name:    "same source and target",
			req:     CreateSyncTaskRequest{Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d1.ID, SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional},
			wantErr: "source and target device must be different",
		},
		{
			name:    "invalid mode",
			req:     CreateSyncTaskRequest{Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID, SourcePath: "/a", TargetPath: "/b", Mode: "invalid"},
			wantErr: "invalid sync mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.CreateSyncTask(tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCreateSyncTaskDeviceNotFound(t *testing.T) {
	m := NewManager("", zap.NewNop())

	_, err := m.CreateSyncTask(CreateSyncTaskRequest{
		Name:           "Test",
		SourceDeviceID: "nonexistent",
		TargetDeviceID: "also-nonexistent",
		SourcePath:     "/a",
		TargetPath:     "/b",
		Mode:           SyncModeBidirectional,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSyncTask(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	found, err := m.GetSyncTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, found.ID)
}

func TestListSyncTasks(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})

	m.CreateSyncTask(CreateSyncTaskRequest{Name: "Task1", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID, SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional})
	m.CreateSyncTask(CreateSyncTaskRequest{Name: "Task2", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID, SourcePath: "/c", TargetPath: "/d", Mode: SyncModeMirror})

	tasks := m.ListSyncTasks()
	assert.Len(t, tasks, 2)
}

func TestUpdateSyncTask(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Old Name", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	newMode := SyncModeMirror
	updated, err := m.UpdateSyncTask(task.ID, UpdateSyncTaskRequest{
		Name: "New Name",
		Mode: &newMode,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, SyncModeMirror, updated.Mode)
}

func TestDeleteSyncTask(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	err := m.DeleteSyncTask(task.ID)
	assert.NoError(t, err)

	_, err = m.GetSyncTask(task.ID)
	assert.Error(t, err)
}

// ============================================================
// 同步控制测试
// ============================================================

func TestStartSync(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})
	m.UpdateDeviceStatus(d1.ID, DeviceStatusOnline)
	m.UpdateDeviceStatus(d2.ID, DeviceStatusOnline)

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	err := m.StartSync(task.ID)
	assert.NoError(t, err)

	// 验证状态变为 syncing
	status, _ := m.GetSyncStatus(task.ID)
	assert.Equal(t, TaskStatusSyncing, status.Status)

	// 等待同步完成
	time.Sleep(4 * time.Second)
}

func TestStartSyncDisabledTask(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})
	m.UpdateDeviceStatus(d1.ID, DeviceStatusOnline)
	m.UpdateDeviceStatus(d2.ID, DeviceStatusOnline)

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	falseVal := false
	m.UpdateSyncTask(task.ID, UpdateSyncTaskRequest{Enabled: &falseVal})

	err := m.StartSync(task.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestStartSyncOfflineDevice(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})
	// d2 保持 offline

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	err := m.StartSync(task.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "offline")
}

func TestPauseResumeSync(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})
	m.UpdateDeviceStatus(d1.ID, DeviceStatusOnline)
	m.UpdateDeviceStatus(d2.ID, DeviceStatusOnline)

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	// 启动
	err := m.StartSync(task.ID)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// 暂停
	err = m.PauseSync(task.ID)
	assert.NoError(t, err)

	status, _ := m.GetSyncStatus(task.ID)
	assert.Equal(t, TaskStatusPaused, status.Status)

	// 恢复
	err = m.ResumeSync(task.ID)
	assert.NoError(t, err)

	status, _ = m.GetSyncStatus(task.ID)
	assert.Equal(t, TaskStatusSyncing, status.Status)

	// 停止
	time.Sleep(200 * time.Millisecond)
	err = m.StopSync(task.ID)
	assert.NoError(t, err)

	status, _ = m.GetSyncStatus(task.ID)
	assert.Equal(t, TaskStatusIdle, status.Status)
}

func TestStopSync(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})
	m.UpdateDeviceStatus(d1.ID, DeviceStatusOnline)
	m.UpdateDeviceStatus(d2.ID, DeviceStatusOnline)

	task, _ := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})

	m.StartSync(task.ID)
	time.Sleep(200 * time.Millisecond)

	err := m.StopSync(task.ID)
	assert.NoError(t, err)

	status, _ := m.GetSyncStatus(task.ID)
	assert.Equal(t, TaskStatusIdle, status.Status)
	assert.Equal(t, float64(0), status.Progress)
}

// ============================================================
// 冲突管理测试
// ============================================================

func TestGetConflicts(t *testing.T) {
	m := NewManager("", zap.NewNop())

	// 加载 mock 数据获取预设冲突
	m.LoadMockData()

	conflicts := m.GetConflicts("sync-003")
	assert.Len(t, conflicts, 3)
	assert.False(t, conflicts[0].Resolved)
}

func TestResolveConflict(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	err := m.ResolveConflict("sync-003", "conflict-001", ConflictStrategyNewer)
	assert.NoError(t, err)

	conflicts := m.GetConflicts("sync-003")
	assert.True(t, conflicts[0].Resolved)
	assert.Equal(t, string(ConflictStrategyNewer), conflicts[0].Resolution)
}

func TestResolveConflictNotFound(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	err := m.ResolveConflict("sync-003", "nonexistent", ConflictStrategyNewer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveAllConflicts(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	err := m.ResolveAllConflicts("sync-003", ConflictStrategySource)
	assert.NoError(t, err)

	conflicts := m.GetConflicts("sync-003")
	for _, c := range conflicts {
		assert.True(t, c.Resolved)
		assert.Equal(t, string(ConflictStrategySource), c.Resolution)
	}
}

func TestResolveAllConflictsNoConflicts(t *testing.T) {
	m := NewManager("", zap.NewNop())

	err := m.ResolveAllConflicts("nonexistent", ConflictStrategyNewer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no conflicts")
}

// ============================================================
// 统计和日志测试
// ============================================================

func TestGetSyncStats(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	stats := m.GetSyncStats()
	assert.Equal(t, int64(3), stats.TotalDevices)
	assert.Equal(t, int64(2), stats.OnlineDevices)
	assert.Equal(t, int64(3), stats.TotalTasks)
	assert.Greater(t, stats.TotalFiles, int64(0))
}

func TestGetSyncLogs(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	logs := m.GetSyncLogs("", 0)
	assert.Greater(t, len(logs), 0)

	// 按 task 过滤
	logs = m.GetSyncLogs("sync-001", 0)
	for _, l := range logs {
		assert.Equal(t, "sync-001", l.TaskID)
	}

	// 限制数量
	logs = m.GetSyncLogs("", 2)
	assert.LessOrEqual(t, len(logs), 2)
}

func TestGetAllSyncStatuses(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	statuses := m.GetAllSyncStatuses()
	assert.Len(t, statuses, 3)
}

// ============================================================
// Mock 数据测试
// ============================================================

func TestLoadMockData(t *testing.T) {
	m := NewManager("", zap.NewNop())
	m.LoadMockData()

	devices := m.ListDevices()
	assert.Len(t, devices, 3)

	tasks := m.ListSyncTasks()
	assert.Len(t, tasks, 3)

	stats := m.GetSyncStats()
	assert.Equal(t, int64(3), stats.TotalDevices)
	assert.Equal(t, int64(3), stats.TotalTasks)
}

// ============================================================
// 同步模式和冲突策略测试
// ============================================================

func TestSyncModeValidation(t *testing.T) {
	tests := []struct {
		mode    SyncMode
		isValid bool
	}{
		{SyncModeBidirectional, true},
		{SyncModeMirror, true},
		{SyncModeOneWay, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.isValid, tt.mode.IsValid(), "mode: %s", tt.mode)
	}
}

func TestConflictStrategyValidation(t *testing.T) {
	tests := []struct {
		strategy ConflictStrategy
		isValid  bool
	}{
		{ConflictStrategySource, true},
		{ConflictStrategyTarget, true},
		{ConflictStrategyNewer, true},
		{ConflictStrategyLarger, true},
		{ConflictStrategyKeepBoth, true},
		{ConflictStrategyManual, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.isValid, tt.strategy.IsValid(), "strategy: %s", tt.strategy)
	}
}

func TestDeviceStatus(t *testing.T) {
	m := NewManager("", zap.NewNop())

	device, _ := m.CreateDevice(CreateDeviceRequest{Name: "Test", Address: "192.168.1.1", Port: 8443})
	assert.Equal(t, DeviceStatusOffline, device.Status)

	m.UpdateDeviceStatus(device.ID, DeviceStatusOnline)
	updated, _ := m.GetDevice(device.ID)
	assert.Equal(t, DeviceStatusOnline, updated.Status)
	assert.NotNil(t, updated.LastSeen)
}

// ============================================================
// 边界情况测试
// ============================================================

func TestGetDeviceNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	_, err := m.GetDevice("nonexistent")
	assert.Error(t, err)
}

func TestGetSyncTaskNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	_, err := m.GetSyncTask("nonexistent")
	assert.Error(t, err)
}

func TestUpdateDeviceNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	_, err := m.UpdateDevice("nonexistent", UpdateDeviceRequest{Name: "Test"})
	assert.Error(t, err)
}

func TestDeleteDeviceNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.DeleteDevice("nonexistent")
	assert.Error(t, err)
}

func TestUpdateSyncTaskNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	_, err := m.UpdateSyncTask("nonexistent", UpdateSyncTaskRequest{Name: "Test"})
	assert.Error(t, err)
}

func TestDeleteSyncTaskNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.DeleteSyncTask("nonexistent")
	assert.Error(t, err)
}

func TestStartSyncNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.StartSync("nonexistent")
	assert.Error(t, err)
}

func TestPauseSyncNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.PauseSync("nonexistent")
	assert.Error(t, err)
}

func TestResumeSyncNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.ResumeSync("nonexistent")
	assert.Error(t, err)
}

func TestStopSyncNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	err := m.StopSync("nonexistent")
	assert.Error(t, err)
}

func TestTestDeviceConnectionNonExistent(t *testing.T) {
	m := NewManager("", zap.NewNop())
	_, err := m.TestDeviceConnection("nonexistent")
	assert.Error(t, err)
}

// ============================================================
// 默认值测试
// ============================================================

func TestCreateSyncTaskDefaults(t *testing.T) {
	m := NewManager("", zap.NewNop())

	d1, _ := m.CreateDevice(CreateDeviceRequest{Name: "S", Address: "192.168.1.1", Port: 8443})
	d2, _ := m.CreateDevice(CreateDeviceRequest{Name: "T", Address: "192.168.1.2", Port: 8443})

	task, err := m.CreateSyncTask(CreateSyncTaskRequest{
		Name: "Test", SourceDeviceID: d1.ID, TargetDeviceID: d2.ID,
		SourcePath: "/a", TargetPath: "/b", Mode: SyncModeBidirectional,
	})
	require.NoError(t, err)

	assert.Equal(t, ConflictStrategyNewer, task.ConflictStrategy) // 默认策略
	assert.Equal(t, "manual", task.ScheduleType)                  // 默认调度
	assert.True(t, task.PreserveModTime)                          // 默认保留修改时间
	assert.True(t, task.PreservePerms)                            // 默认保留权限
	assert.True(t, task.ChecksumVerify)                           // 默认校验和验证
	assert.True(t, task.CompressTransfer)                         // 默认压缩传输
	assert.Equal(t, 4, task.Concurrent)                           // 默认并发数
	assert.False(t, task.DeleteExtraneous)                        // 默认不删除多余文件
}
