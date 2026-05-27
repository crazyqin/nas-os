package filesync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTest(t *testing.T) (*SyncManager, *Handler) {
	logger, _ := zap.NewDevelopment()
	manager := NewSyncManager(logger, "/tmp/filesync-test")
	handler := NewHandler(manager, logger)
	return manager, handler
}

func TestCreateTask(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{
		Name:       "文档同步",
		LocalPath:  "/home/user/docs",
		RemotePath: "/remote/docs",
		DeviceID:   "dev-1",
		Direction:  "bidirectional",
	}

	err := manager.CreateTask(nil, task)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "idle", task.Status)
	assert.True(t, task.Enabled)
}

func TestCreateDuplicateTask(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	err := manager.CreateTask(nil, task)
	require.NoError(t, err)

	err = manager.CreateTask(nil, task)
	assert.Error(t, err)
}

func TestGetTask(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	got, err := manager.GetTask(nil, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.Name, got.Name)
}

func TestGetTaskNotFound(t *testing.T) {
	manager, _ := setupTest(t)

	_, err := manager.GetTask(nil, "nonexistent")
	assert.Error(t, err)
}

func TestListTasks(t *testing.T) {
	manager, _ := setupTest(t)

	_ = manager.CreateTask(nil, &SyncTask{Name: "task1", LocalPath: "/a", RemotePath: "/b"})
	_ = manager.CreateTask(nil, &SyncTask{Name: "task2", LocalPath: "/c", RemotePath: "/d"})

	tasks := manager.ListTasks(nil)
	assert.Len(t, tasks, 2)
}

func TestDeleteTask(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	err := manager.DeleteTask(nil, task.ID)
	require.NoError(t, err)

	_, err = manager.GetTask(nil, task.ID)
	assert.Error(t, err)
}

func TestStartStopSync(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	err := manager.StartSync(nil, task.ID)
	require.NoError(t, err)

	updated, _ := manager.GetTask(nil, task.ID)
	assert.Equal(t, "syncing", updated.Status)

	err = manager.StopSync(nil, task.ID)
	require.NoError(t, err)

	updated, _ = manager.GetTask(nil, task.ID)
	assert.Equal(t, "paused", updated.Status)
}

func TestRecordSyncFile(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	file := &SyncFile{
		TaskID:     task.ID,
		Path:       "/docs/file.txt",
		Size:       1024,
		Checksum:   "abc123",
		SyncStatus: "synced",
	}

	err := manager.RecordSyncFile(nil, file)
	require.NoError(t, err)
	assert.NotEmpty(t, file.ID)
}

func TestReportConflict(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	conflict := &SyncConflict{
		TaskID:   task.ID,
		FilePath: "/docs/conflict.txt",
	}

	err := manager.ReportConflict(nil, conflict)
	require.NoError(t, err)
	assert.NotEmpty(t, conflict.ID)
	assert.Equal(t, "pending", conflict.Resolution)
}

func TestResolveConflict(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	conflict := &SyncConflict{
		TaskID:   task.ID,
		FilePath: "/docs/conflict.txt",
	}
	_ = manager.ReportConflict(nil, conflict)

	err := manager.ResolveConflict(nil, conflict.ID, "keep_local")
	require.NoError(t, err)
}

func TestResolveConflictNotFound(t *testing.T) {
	manager, _ := setupTest(t)

	err := manager.ResolveConflict(nil, "nonexistent", "keep_local")
	assert.Error(t, err)
}

func TestRegisterDevice(t *testing.T) {
	manager, _ := setupTest(t)

	device := &DeviceInfo{
		Name: "我的电脑",
		Type: "desktop",
		OS:   "Windows 11",
	}

	err := manager.RegisterDevice(nil, device)
	require.NoError(t, err)
	assert.NotEmpty(t, device.ID)
	assert.True(t, device.Online)
}

func TestListDevices(t *testing.T) {
	manager, _ := setupTest(t)

	_ = manager.RegisterDevice(nil, &DeviceInfo{Name: "dev1", Type: "desktop"})
	_ = manager.RegisterDevice(nil, &DeviceInfo{Name: "dev2", Type: "mobile"})

	devices := manager.ListDevices(nil)
	assert.Len(t, devices, 2)
}

func TestGetSyncStats(t *testing.T) {
	manager, _ := setupTest(t)

	task := &SyncTask{Name: "test", LocalPath: "/local", RemotePath: "/remote"}
	_ = manager.CreateTask(nil, task)

	_ = manager.RecordSyncFile(nil, &SyncFile{TaskID: task.ID, Path: "/a", Size: 100, SyncStatus: "synced"})
	_ = manager.RecordSyncFile(nil, &SyncFile{TaskID: task.ID, Path: "/b", Size: 200, SyncStatus: "pending"})

	stats := manager.GetSyncStats(nil, task.ID)
	assert.Equal(t, 2, stats.TotalFiles)
	assert.Equal(t, 1, stats.SyncedFiles)
	assert.Equal(t, 1, stats.PendingFiles)
	assert.Equal(t, int64(300), stats.TotalSize)
}

func TestAddVersion(t *testing.T) {
	manager, _ := setupTest(t)

	version := &SyncVersion{
		FileID:    "file-1",
		Version:   1,
		Size:      1024,
		Checksum:  "abc",
		CreatedBy: "user1",
	}

	err := manager.AddVersion(nil, version)
	require.NoError(t, err)
	assert.NotEmpty(t, version.ID)
}

func TestGetVersions(t *testing.T) {
	manager, _ := setupTest(t)

	_ = manager.AddVersion(nil, &SyncVersion{FileID: "file-1", Version: 1})
	_ = manager.AddVersion(nil, &SyncVersion{FileID: "file-1", Version: 2})

	versions := manager.GetVersions(nil, "file-1")
	assert.Len(t, versions, 2)
}
