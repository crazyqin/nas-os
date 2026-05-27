package cloudsync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) (*CloudSyncManager, *Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mgr := NewCloudSyncManager(zap.NewNop())
	handler := NewHandler(mgr, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return mgr, handler, router
}

// ============================================================
// 类型验证测试
// ============================================================

func TestCloudBackendIsValid(t *testing.T) {
	tests := []struct {
		backend CloudBackend
		valid   bool
	}{
		{BackendS3, true},
		{BackendAzure, true},
		{BackendGCS, true},
		{BackendOSS, true},
		{BackendMinIO, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.valid, tt.backend.IsValid(), "backend: %s", tt.backend)
	}
}

func TestSyncModeIsValid(t *testing.T) {
	tests := []struct {
		mode  SyncMode
		valid bool
	}{
		{SyncModeBidirectional, true},
		{SyncModeUploadOnly, true},
		{SyncModeDownloadOnly, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.valid, tt.mode.IsValid(), "mode: %s", tt.mode)
	}
}

func TestConflictPolicyIsValid(t *testing.T) {
	tests := []struct {
		policy ConflictPolicy
		valid  bool
	}{
		{ConflictLocalFirst, true},
		{ConflictRemoteFirst, true},
		{ConflictKeepBoth, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.valid, tt.policy.IsValid(), "policy: %s", tt.policy)
	}
}

func TestConnectionConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ConnectionConfig
		wantErr bool
	}{
		{
			name: "valid S3 config",
			config: ConnectionConfig{
				Name:      "test",
				Backend:   BackendS3,
				Bucket:    "my-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: ConnectionConfig{
				Backend:   BackendS3,
				Bucket:    "my-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "invalid backend",
			config: ConnectionConfig{
				Name:      "test",
				Backend:   "invalid",
				Bucket:    "my-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "MinIO without endpoint",
			config: ConnectionConfig{
				Name:      "test",
				Backend:   BackendMinIO,
				Bucket:    "my-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "Azure without account name",
			config: ConnectionConfig{
				Name:      "test",
				Backend:   BackendAzure,
				Bucket:    "my-container",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "GCS without project ID",
			config: ConnectionConfig{
				Name:      "test",
				Backend:   BackendGCS,
				Bucket:    "my-bucket",
				AccessKey: "key",
				SecretKey: "secret",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncTaskValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    SyncTask
		wantErr bool
	}{
		{
			name: "valid task",
			task: SyncTask{
				Name:           "test",
				ConnectionID:   "conn-1",
				LocalPath:      "/data",
				RemotePath:     "backup/",
				Mode:           SyncModeUploadOnly,
				ConflictPolicy: ConflictLocalFirst,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			task: SyncTask{
				ConnectionID:   "conn-1",
				LocalPath:      "/data",
				RemotePath:     "backup/",
				Mode:           SyncModeUploadOnly,
				ConflictPolicy: ConflictLocalFirst,
			},
			wantErr: true,
		},
		{
			name: "invalid mode",
			task: SyncTask{
				Name:           "test",
				ConnectionID:   "conn-1",
				LocalPath:      "/data",
				RemotePath:     "backup/",
				Mode:           "invalid",
				ConflictPolicy: ConflictLocalFirst,
			},
			wantErr: true,
		},
		{
			name: "cron schedule without expression",
			task: SyncTask{
				Name:           "test",
				ConnectionID:   "conn-1",
				LocalPath:      "/data",
				RemotePath:     "backup/",
				Mode:           SyncModeUploadOnly,
				ConflictPolicy: ConflictLocalFirst,
				Schedule:       SyncSchedule{Type: ScheduleCron},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultTransferConfig(t *testing.T) {
	cfg := DefaultTransferConfig()
	assert.Equal(t, 0, cfg.BandwidthLimit)
	assert.Equal(t, 4, cfg.ConcurrentTransfers)
	assert.False(t, cfg.EncryptionEnabled)
	assert.Equal(t, 8, cfg.BlockSizeMB)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 5, cfg.RetryDelaySec)
}

// ============================================================
// 连接管理测试
// ============================================================

func TestCreateConnection(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	req := CreateConnectionRequest{
		Name:      "Test S3",
		Backend:   BackendS3,
		Bucket:    "test-bucket",
		AccessKey: "key",
		SecretKey: "secret",
		Region:    "us-east-1",
	}

	conn, err := mgr.CreateConnection(req)
	require.NoError(t, err)
	assert.NotEmpty(t, conn.ID)
	assert.Equal(t, "Test S3", conn.Name)
	assert.Equal(t, BackendS3, conn.Backend)
}

func TestCreateConnectionInvalid(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	req := CreateConnectionRequest{
		Backend: BackendS3,
		Bucket:  "test-bucket",
	}

	_, err := mgr.CreateConnection(req)
	assert.Error(t, err)
}

func TestGetConnection(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	req := CreateConnectionRequest{
		Name:      "Test",
		Backend:   BackendS3,
		Bucket:    "bucket",
		AccessKey: "key",
		SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(req)

	found, err := mgr.GetConnection(conn.ID)
	require.NoError(t, err)
	assert.Equal(t, conn.ID, found.ID)
}

func TestGetConnectionNotFound(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	_, err := mgr.GetConnection("nonexistent")
	assert.Error(t, err)
}

func TestListConnections(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	mgr.CreateConnection(CreateConnectionRequest{
		Name: "Conn1", Backend: BackendS3, Bucket: "b1", AccessKey: "k", SecretKey: "s",
	})
	mgr.CreateConnection(CreateConnectionRequest{
		Name: "Conn2", Backend: BackendOSS, Bucket: "b2", AccessKey: "k", SecretKey: "s",
	})

	conns := mgr.ListConnections()
	assert.Len(t, conns, 2)
}

func TestUpdateConnection(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	req := CreateConnectionRequest{
		Name: "Original", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(req)

	newName := "Updated"
	updated, err := mgr.UpdateConnection(conn.ID, UpdateConnectionRequest{Name: newName})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
}

func TestDeleteConnection(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	req := CreateConnectionRequest{
		Name: "ToDelete", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(req)

	err := mgr.DeleteConnection(conn.ID)
	assert.NoError(t, err)

	_, err = mgr.GetConnection(conn.ID)
	assert.Error(t, err)
}

func TestDeleteConnectionWithTasks(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "HasTasks", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name:           "Task",
		ConnectionID:   conn.ID,
		LocalPath:      "/data",
		RemotePath:     "backup/",
		Mode:           SyncModeUploadOnly,
		ConflictPolicy: ConflictLocalFirst,
	}
	mgr.CreateTask(taskReq)

	err := mgr.DeleteConnection(conn.ID)
	assert.Error(t, err)
}

// ============================================================
// 任务管理测试
// ============================================================

func TestCreateTask(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name:           "Photo Backup",
		ConnectionID:   conn.ID,
		LocalPath:      "/volume1/photos",
		RemotePath:     "photos/",
		Mode:           SyncModeUploadOnly,
		ConflictPolicy: ConflictLocalFirst,
	}

	task, err := mgr.CreateTask(taskReq)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Photo Backup", task.Name)
	assert.Equal(t, StatusIdle, task.Status)
	assert.True(t, task.Enabled)
}

func TestCreateTaskWithInvalidConnection(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	taskReq := CreateTaskRequest{
		Name:           "Task",
		ConnectionID:   "nonexistent",
		LocalPath:      "/data",
		RemotePath:     "backup/",
		Mode:           SyncModeUploadOnly,
		ConflictPolicy: ConflictLocalFirst,
	}

	_, err := mgr.CreateTask(taskReq)
	assert.Error(t, err)
}

func TestCreateTaskDefaultConflictPolicy(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name:         "Task",
		ConnectionID: conn.ID,
		LocalPath:    "/data",
		RemotePath:   "backup/",
		Mode:         SyncModeUploadOnly,
	}

	task, err := mgr.CreateTask(taskReq)
	require.NoError(t, err)
	assert.Equal(t, ConflictLocalFirst, task.ConflictPolicy)
}

func TestGetTask(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	found, err := mgr.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, found.ID)
}

func TestListTasks(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	mgr.CreateTask(CreateTaskRequest{
		Name: "Task1", ConnectionID: conn.ID, LocalPath: "/data1", RemotePath: "b1/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	})
	mgr.CreateTask(CreateTaskRequest{
		Name: "Task2", ConnectionID: conn.ID, LocalPath: "/data2", RemotePath: "b2/",
		Mode: SyncModeDownloadOnly, ConflictPolicy: ConflictRemoteFirst,
	})

	tasks := mgr.ListTasks()
	assert.Len(t, tasks, 2)
}

func TestUpdateTask(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Original", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	newName := "Updated"
	updated, err := mgr.UpdateTask(task.ID, UpdateTaskRequest{Name: newName})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
}

func TestDeleteTask(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "ToDelete", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	err := mgr.DeleteTask(task.ID)
	assert.NoError(t, err)

	_, err = mgr.GetTask(task.ID)
	assert.Error(t, err)
}

// ============================================================
// 同步控制测试
// ============================================================

func TestStartSync(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	err := mgr.StartSync(task.ID)
	assert.NoError(t, err)

	// 等待同步开始
	status, _ := mgr.GetSyncStatus(task.ID)
	assert.Equal(t, StatusSyncing, status.Status)
}

func TestStartSyncDisabled(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	// 禁用任务
	enabled := false
	mgr.UpdateTask(task.ID, UpdateTaskRequest{Enabled: &enabled})

	err := mgr.StartSync(task.ID)
	assert.Error(t, err)
}

func TestPauseAndResumeSync(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	// 启动同步
	mgr.StartSync(task.ID)

	// 暂停
	err := mgr.PauseSync(task.ID)
	assert.NoError(t, err)

	status, _ := mgr.GetSyncStatus(task.ID)
	assert.Equal(t, StatusPaused, status.Status)

	// 恢复
	err = mgr.ResumeSync(task.ID)
	assert.NoError(t, err)
}

func TestStopSync(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	taskReq := CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	}
	task, _ := mgr.CreateTask(taskReq)

	mgr.StartSync(task.ID)

	err := mgr.StopSync(task.ID)
	assert.NoError(t, err)

	status, _ := mgr.GetSyncStatus(task.ID)
	assert.Equal(t, StatusIdle, status.Status)
	assert.Equal(t, 0.0, status.Progress)
}

// ============================================================
// 统计和日志测试
// ============================================================

func TestGetSyncStats(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	mgr.CreateTask(CreateTaskRequest{
		Name: "Task1", ConnectionID: conn.ID, LocalPath: "/d1", RemotePath: "b1/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	})
	mgr.CreateTask(CreateTaskRequest{
		Name: "Task2", ConnectionID: conn.ID, LocalPath: "/d2", RemotePath: "b2/",
		Mode: SyncModeDownloadOnly, ConflictPolicy: ConflictRemoteFirst,
	})

	stats := mgr.GetSyncStats()
	assert.Equal(t, int64(2), stats.TotalTasks)
}

func TestGetSyncLogs(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	// 手动添加日志
	mgr.addLog("task-1", "info", "test message", "/path/to/file")
	mgr.addLog("task-1", "error", "error message", "")
	mgr.addLog("task-2", "info", "other task", "")

	logs := mgr.GetSyncLogs("", 0)
	assert.Len(t, logs, 3)

	logs = mgr.GetSyncLogs("task-1", 0)
	assert.Len(t, logs, 2)

	logs = mgr.GetSyncLogs("", 1)
	assert.Len(t, logs, 1)
}

func TestGetStorageUsage(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	connReq := CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "bucket", AccessKey: "key", SecretKey: "secret",
	}
	conn, _ := mgr.CreateConnection(connReq)

	usage, err := mgr.GetStorageUsage(conn.ID)
	require.NoError(t, err)
	assert.Equal(t, conn.ID, usage.ConnectionID)
	assert.Equal(t, BackendS3, usage.Backend)
	assert.Greater(t, usage.TotalBytes, int64(0))
	assert.Greater(t, usage.UsedBytes, int64(0))
	assert.Equal(t, usage.TotalBytes-usage.UsedBytes, usage.FreeBytes)
}

// ============================================================
// Mock 数据测试
// ============================================================

func TestLoadMockData(t *testing.T) {
	mgr, _, _ := setupTestManager(t)

	mgr.LoadMockData()

	conns := mgr.ListConnections()
	assert.Len(t, conns, 3)

	tasks := mgr.ListTasks()
	assert.Len(t, tasks, 4)

	// 验证连接类型
	connMap := make(map[string]*ConnectionConfig)
	for _, c := range conns {
		connMap[c.ID] = c
	}
	assert.Equal(t, BackendS3, connMap["conn-s3-001"].Backend)
	assert.Equal(t, BackendOSS, connMap["conn-oss-002"].Backend)
	assert.Equal(t, BackendMinIO, connMap["conn-minio-003"].Backend)

	// 验证任务状态
	taskMap := make(map[string]*SyncTask)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}
	assert.Equal(t, StatusIdle, taskMap["task-001"].Status)
	assert.Equal(t, StatusPaused, taskMap["task-004"].Status)
}

// ============================================================
// HTTP Handler 测试
// ============================================================

func TestHandlerCreateConnection(t *testing.T) {
	_, _, router := setupTestManager(t)

	body := `{
		"name": "Test S3",
		"backend": "s3",
		"bucket": "my-bucket",
		"access_key": "key",
		"secret_key": "secret",
		"region": "us-east-1"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudsync/connections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var conn ConnectionConfig
	err := json.Unmarshal(w.Body.Bytes(), &conn)
	require.NoError(t, err)
	assert.Equal(t, "Test S3", conn.Name)
}

func TestHandlerListConnections(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	mgr.CreateConnection(CreateConnectionRequest{
		Name: "Conn1", Backend: BackendS3, Bucket: "b1", AccessKey: "k", SecretKey: "s",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudsync/connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Connections []*ConnectionConfig `json:"connections"`
		Total       int                 `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 1, resp.Total)
}

func TestHandlerCreateTask(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	conn, _ := mgr.CreateConnection(CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "b", AccessKey: "k", SecretKey: "s",
	})

	body := `{
		"name": "Photo Backup",
		"connection_id": "` + conn.ID + `",
		"local_path": "/volume1/photos",
		"remote_path": "photos/",
		"mode": "upload",
		"conflict_policy": "local_first"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudsync/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var task SyncTask
	json.Unmarshal(w.Body.Bytes(), &task)
	assert.Equal(t, "Photo Backup", task.Name)
	assert.Equal(t, SyncModeUploadOnly, task.Mode)
}

func TestHandlerStartSync(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	conn, _ := mgr.CreateConnection(CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "b", AccessKey: "k", SecretKey: "s",
	})
	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudsync/tasks/"+task.ID+"/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlerPauseSync(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	conn, _ := mgr.CreateConnection(CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "b", AccessKey: "k", SecretKey: "s",
	})
	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	})
	mgr.StartSync(task.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudsync/tasks/"+task.ID+"/pause", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlerGetSyncStats(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	conn, _ := mgr.CreateConnection(CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "b", AccessKey: "k", SecretKey: "s",
	})
	mgr.CreateTask(CreateTaskRequest{
		Name: "Task", ConnectionID: conn.ID, LocalPath: "/data", RemotePath: "backup/",
		Mode: SyncModeUploadOnly, ConflictPolicy: ConflictLocalFirst,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudsync/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]interface{})
	totalTasks, _ := data["total_tasks"].(float64)
	assert.Equal(t, float64(1), totalTasks)
}

func TestHandlerGetStorageUsage(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	conn, _ := mgr.CreateConnection(CreateConnectionRequest{
		Name: "Test", Backend: BackendS3, Bucket: "b", AccessKey: "k", SecretKey: "s",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudsync/connections/"+conn.ID+"/usage", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var usage StorageUsage
	json.Unmarshal(w.Body.Bytes(), &usage)
	assert.Equal(t, conn.ID, usage.ConnectionID)
}

func TestHandlerLoadMockData(t *testing.T) {
	_, _, router := setupTestManager(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloudsync/mock", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证数据已加载
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/cloudsync/connections", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var resp struct {
		Total int `json:"total"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.Equal(t, 3, resp.Total)
}

func TestHandlerDeleteConnectionNotFound(t *testing.T) {
	_, _, router := setupTestManager(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cloudsync/connections/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlerDeleteTaskNotFound(t *testing.T) {
	_, _, router := setupTestManager(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cloudsync/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlerGetSyncLogs(t *testing.T) {
	mgr, _, router := setupTestManager(t)

	mgr.addLog("task-1", "info", "test", "")
	mgr.addLog("task-1", "error", "fail", "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudsync/logs?task_id=task-1&limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Logs  []SyncLog `json:"logs"`
		Total int       `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Len(t, resp.Logs, 2)
}
