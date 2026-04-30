// Package active Active Backup 单元测试
package active

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestEnv(t *testing.T) (string, *BackupManager, *Engine, *RestoreManager, *DashboardHandler) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "backup.json")
	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	logger := zap.NewNop()
	mgr, err := NewBackupManager(configPath, logger)
	if err != nil {
		t.Fatalf("NewBackupManager failed: %v", err)
	}

	config := DefaultEngineConfig()
	config.StoragePath = storagePath
	engine, err := NewEngine(mgr, config, logger)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	restore := NewRestoreManager(mgr, logger)
	dashboard := NewDashboardHandler(engine, mgr, restore, logger)

	return tmpDir, mgr, engine, restore, dashboard
}

// ==================== Engine 测试 ====================

func TestNewEngine(t *testing.T) {
	_, _, engine, _, _ := setupTestEnv(t)

	if engine.GetState() != EngineStateIdle {
		t.Errorf("expected idle state, got %s", engine.GetState())
	}
}

func TestEngineStartStop(t *testing.T) {
	_, _, engine, _, _ := setupTestEnv(t)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if engine.GetState() != EngineStateRunning {
		t.Errorf("expected running state, got %s", engine.GetState())
	}

	// 双重启动应失败
	if err := engine.Start(ctx); err == nil {
		t.Error("double start should fail")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if engine.GetState() != EngineStateIdle {
		t.Errorf("expected idle state after stop, got %s", engine.GetState())
	}
}

func TestEngineSubmitTask(t *testing.T) {
	tmpDir, mgr, engine, _, _ := setupTestEnv(t)

	// 设置存储路径为临时目录
	storagePath := filepath.Join(tmpDir, "backup_storage")
	os.MkdirAll(storagePath, 0755)
	mgr.config.StoragePath = storagePath

	ctx := context.Background()
	engine.Start(ctx)
	defer engine.Stop()

	// 创建任务
	job, err := mgr.CreateJob(ctx, &BackupJob{
		Name: "test-backup",
		Source: BackupSource{
			Paths: []string{"/tmp/test"},
		},
		Destination: BackupDestination{
			Type: "local",
			Path: "/tmp/dest",
		},
		Policy: BackupPolicy{
			Type: BackupTypeFull,
		},
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// 提交任务
	taskRun, err := engine.SubmitTask(ctx, job.ID, BackupTypeFull)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if taskRun.Status != BackupStatusRunning {
		t.Errorf("expected running status, got %s", taskRun.Status)
	}

	// 等待完成
	time.Sleep(100 * time.Millisecond)

	// 获取任务运行记录
	run, err := engine.GetTaskRun(taskRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun failed: %v", err)
	}

	if run.Status != BackupStatusCompleted && run.Status != BackupStatusRunning {
		t.Logf("task run status: %s, error: %s", run.Status, run.Error)
	}
}

func TestEngineGetStats(t *testing.T) {
	_, _, engine, _, _ := setupTestEnv(t)

	stats := engine.GetStats()
	if stats.State != string(EngineStateIdle) {
		t.Errorf("expected idle, got %s", stats.State)
	}
	if stats.MaxConcurrent != 4 {
		t.Errorf("expected max concurrent 4, got %d", stats.MaxConcurrent)
	}
}

func TestEngineListTaskRuns(t *testing.T) {
	_, _, engine, _, _ := setupTestEnv(t)

	runs := engine.ListTaskRuns("")
	if runs == nil {
		t.Fatal("ListTaskRuns returned nil")
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 task runs, got %d", len(runs))
	}
}

// ==================== Agent 测试 ====================

func TestAgentRegistryRegister(t *testing.T) {
	logger := zap.NewNop()
	registry := NewAgentRegistry(logger)

	agent, err := registry.Register(&RegisterPayload{
		Hostname:     "test-host",
		Platform:     PlatformLinux,
		OSVersion:    "Ubuntu 22.04",
		AgentVersion: "1.0.0",
		Capabilities: []string{"file_backup"},
		Labels:       map[string]string{"env": "test"},
	}, nil, "192.168.1.100")

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if agent.Hostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got %s", agent.Hostname)
	}
	if agent.Status != AgentStatusOnline {
		t.Errorf("expected online status, got %s", agent.Status)
	}

	// 列出代理
	agents := registry.ListAgents("")
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	// 获取代理
	fetched, err := registry.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if fetched.ID != agent.ID {
		t.Errorf("agent ID mismatch")
	}

	// 代理计数
	if registry.Count() != 1 {
		t.Errorf("expected count 1, got %d", registry.Count())
	}
}

func TestAgentRegistryHeartbeat(t *testing.T) {
	logger := zap.NewNop()
	registry := NewAgentRegistry(logger)

	agent, _ := registry.Register(&RegisterPayload{
		Hostname: "hb-host",
		Platform: PlatformWindows,
	}, nil, "10.0.0.1")

	err := registry.Heartbeat(agent.ID, &HeartbeatPayload{
		Status:     AgentStatusBusy,
		ActiveJobs: 2,
		CPUUsage:   45.0,
	})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	fetched, _ := registry.GetAgent(agent.ID)
	if fetched.Status != AgentStatusBusy {
		t.Errorf("expected busy status, got %s", fetched.Status)
	}
	if fetched.ActiveJobs != 2 {
		t.Errorf("expected 2 active jobs, got %d", fetched.ActiveJobs)
	}
}

func TestAgentRegistryUnregister(t *testing.T) {
	logger := zap.NewNop()
	registry := NewAgentRegistry(logger)

	agent, _ := registry.Register(&RegisterPayload{
		Hostname: "unreg-host",
		Platform: PlatformMac,
	}, nil, "10.0.0.2")

	registry.Unregister(agent.ID)

	_, err := registry.GetAgent(agent.ID)
	// Agent still exists but status changed to offline
	if err != nil {
		t.Fatalf("GetAgent should not fail for unregistered: %v", err)
	}
	fetched, _ := registry.GetAgent(agent.ID)
	if fetched.Status != AgentStatusOffline {
		t.Errorf("expected offline status, got %s", fetched.Status)
	}
}

func TestAgentRegistryCleanupStale(t *testing.T) {
	logger := zap.NewNop()
	registry := NewAgentRegistry(logger)

	agent, _ := registry.Register(&RegisterPayload{
		Hostname: "stale-host",
		Platform: PlatformLinux,
	}, nil, "10.0.0.3")

	// 修改 LastSeen 为过去时间
	registry.mu.Lock()
	agent.LastSeen = time.Now().Add(-10 * time.Minute)
	registry.mu.Unlock()

	removed := registry.CleanupStale(5 * time.Minute)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	fetched, _ := registry.GetAgent(agent.ID)
	if fetched.Status != AgentStatusOffline {
		t.Errorf("expected offline, got %s", fetched.Status)
	}
}

func TestAgentHTTPHandler(t *testing.T) {
	logger := zap.NewNop()
	registry := NewAgentRegistry(logger)
	handler := NewAgentHandler(registry, logger)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 注册一个代理
	registry.Register(&RegisterPayload{
		Hostname: "api-host",
		Platform: PlatformLinux,
	}, nil, "10.0.0.4")

	// 列出代理 (路由在 /api/v1/agents 下)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agents", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	total := int(resp["total"].(float64))
	if total != 1 {
		t.Errorf("expected 1 agent, got %d", total)
	}
}

// ==================== Dedup 测试 ====================

func TestCDCEngineBasic(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(16, 1024, logger) // 小块方便测试

	// 第一次去重
	data := []byte("Hello, World! This is a test of CDC deduplication engine.")
	result1, err := engine.DeduplicateBytes(data)
	if err != nil {
		t.Fatalf("DeduplicateBytes failed: %v", err)
	}

	if result1.TotalChunks == 0 {
		t.Error("expected at least 1 chunk")
	}
	if result1.UniqueChunks == 0 {
		t.Error("expected at least 1 unique chunk")
	}

	// 第二次相同数据 - 应检测到重复
	result2, err := engine.DeduplicateBytes(data)
	if err != nil {
		t.Fatalf("DeduplicateBytes failed: %v", err)
	}

	if result2.DupChunks == 0 {
		t.Error("expected duplicate chunks on second pass")
	}

	// 检查统计
	stats := engine.GetStats()
	if stats.TotalChunks == 0 {
		t.Error("expected non-zero total chunks")
	}
	t.Logf("Stats: total=%d unique=%d dup=%d ratio=%.2f",
		stats.TotalChunks, stats.UniqueChunks, stats.DupChunks, stats.DedupRatio)
}

func TestCDCEngineLargeData(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(1024, 4096, logger)

	// 生成 100KB 数据
	data := make([]byte, 100*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	result, err := engine.DeduplicateBytes(data)
	if err != nil {
		t.Fatalf("DeduplicateBytes failed: %v", err)
	}

	t.Logf("Chunks: total=%d unique=%d size=%d", result.TotalChunks, result.UniqueChunks, result.TotalBytes)
	if result.TotalBytes != int64(len(data)) {
		t.Errorf("total bytes mismatch: got %d, want %d", result.TotalBytes, len(data))
	}
}

func TestCDCEngineFile(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(64, 4096, logger)

	// 创建临时文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	data := []byte(strings.Repeat("Hello CDC dedup test data! ", 100))
	os.WriteFile(testFile, data, 0644)

	result, err := engine.DeduplicateFile(testFile)
	if err != nil {
		t.Fatalf("DeduplicateFile failed: %v", err)
	}
	if result.TotalChunks == 0 {
		t.Error("expected chunks from file")
	}
}

func TestCDCEngineSaveLoadIndex(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(16, 1024, logger)

	// 添加一些数据
	data := []byte("persistent data for index test")
	engine.DeduplicateBytes(data)

	// 保存索引
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "cdc_index.json")

	if err := engine.SaveIndex(indexPath); err != nil {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	// 加载到新引擎
	engine2 := NewCDCEngine(16, 1024, logger)
	if err := engine2.LoadIndex(indexPath); err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// 验证数据已加载
	stats := engine2.GetStats()
	if stats.TotalChunks == 0 {
		t.Error("expected loaded chunks")
	}
}

func TestCDCEngineClear(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(16, 1024, logger)

	engine.DeduplicateBytes([]byte("some data"))
	engine.Clear()

	stats := engine.GetStats()
	if stats.TotalChunks != 0 {
		t.Errorf("expected 0 after clear, got %d", stats.TotalChunks)
	}
}

func TestCDCEngineHasChunk(t *testing.T) {
	logger := zap.NewNop()
	engine := NewCDCEngine(16, 1024, logger)

	data := []byte("check chunk existence")
	checksum := engine.ComputeChecksum(data)

	if engine.HasChunk(checksum) {
		t.Error("should not have chunk before dedup")
	}

	engine.DeduplicateBytes(data)

	if !engine.HasChunk(checksum) {
		t.Error("should have chunk after dedup")
	}
}

// ==================== Schedule 测试 ====================

func TestScheduleManagerTimeWindow(t *testing.T) {
	logger := zap.NewNop()
	sm, err := NewScheduleManager(logger)
	if err != nil {
		t.Fatalf("NewScheduleManager failed: %v", err)
	}

	now := time.Now()

	// 无时间窗口 - 应始终允许
	schedule := ScheduleConfig{Enabled: true}
	if !sm.IsWithinTimeWindow(schedule, now) {
		t.Error("empty time window should always allow")
	}

	// 当前时间窗口
	currentHour := now.Format("15:04")
	prevHour := now.Add(-1 * time.Hour).Format("15:04")
	nextHour := now.Add(1 * time.Hour).Format("15:04")

	schedule.StartTime = prevHour
	schedule.EndTime = nextHour
	if !sm.IsWithinTimeWindow(schedule, now) {
		t.Error("should be within time window")
	}

	// 不在窗口内
	schedule.StartTime = nextHour
	schedule.EndTime = currentHour
	if sm.IsWithinTimeWindow(schedule, now) {
		t.Error("should not be within time window")
	}
}

func TestScheduleManagerCalculateNextRun(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewScheduleManager(logger)

	// 无 cron 表达式
	schedule := ScheduleConfig{}
	next := sm.CalculateNextRun(schedule)
	if next.Before(time.Now()) {
		t.Error("next run should be in the future")
	}

	// 带 cron 表达式
	schedule.Cron = "0 3 * * *" // 每天 03:00
	next = sm.CalculateNextRun(schedule)
	if next.Before(time.Now()) {
		t.Error("next run should be in the future")
	}
}

func TestScheduleManagerRetentionPolicy(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewScheduleManager(logger)

	snapshots := []*BackupSnapshot{
		{ID: "snap-1", CreatedAt: time.Now().Add(-10 * 24 * time.Hour)},
		{ID: "snap-2", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)},
		{ID: "snap-3", CreatedAt: time.Now().Add(-3 * 24 * time.Hour)},
		{ID: "snap-4", CreatedAt: time.Now().Add(-1 * 24 * time.Hour)},
		{ID: "snap-5", CreatedAt: time.Now()},
	}

	// 按数量保留 3 个
	policy := RetentionPolicy{MaxCount: 3}
	toDelete := sm.ApplyRetentionPolicy(policy, snapshots)
	if len(toDelete) < 2 {
		t.Errorf("expected at least 2 to delete, got %d", len(toDelete))
	}

	// 按天数保留
	policy = RetentionPolicy{MaxDays: 4}
	toDelete = sm.ApplyRetentionPolicy(policy, snapshots)
	if len(toDelete) < 1 {
		t.Errorf("expected at least 1 to delete for 4 day retention, got %d", len(toDelete))
	}
}

func TestScheduleManagerBandwidth(t *testing.T) {
	logger := zap.NewNop()
	sm, _ := NewScheduleManager(logger)

	now := time.Now()

	// 不限速
	limit := BandwidthLimit{MaxMBps: 0}
	bw := sm.CalculateBandwidth(limit, now)
	if bw != 0 {
		t.Errorf("expected 0 (unlimited), got %d", bw)
	}

	// 无限速计划
	limit = BandwidthLimit{MaxMBps: 100}
	bw = sm.CalculateBandwidth(limit, now)
	if bw != 100 {
		t.Errorf("expected 100, got %d", bw)
	}
}

// ==================== Restore 测试 ====================

func TestRestoreManagerCreateTasks(t *testing.T) {
	_, mgr, _, restore, _ := setupTestEnv(t)

	ctx := context.Background()

	// 创建任务和快照
	job, _ := mgr.CreateJob(ctx, &BackupJob{
		Name:   "restore-test",
		Source: BackupSource{Paths: []string{"/tmp/test"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/dest"},
		Policy: BackupPolicy{Type: BackupTypeFull},
	})

	// 手动添加快照
	mgr.mu.Lock()
	snap := &BackupSnapshot{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		BackupType: BackupTypeFull,
		Size:      1024,
		FileCount: 10,
		Path:      t.TempDir(),
		CreatedAt: time.Now(),
	}
	mgr.snapshots[snap.ID] = snap
	job.Snapshots = append(job.Snapshots, snap.ID)
	mgr.mu.Unlock()

	// 创建单文件恢复
	task, err := restore.CreateSingleFileRestore(ctx, job.ID, snap.ID, []string{"file1.txt", "file2.txt"}, t.TempDir(), RestoreExecOptions{})
	if err != nil {
		t.Fatalf("CreateSingleFileRestore failed: %v", err)
	}
	if task.Mode != RestoreModeSingleFile {
		t.Errorf("expected single_file mode, got %s", task.Mode)
	}
	if len(task.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(task.Files))
	}

	// 创建整机恢复
	task2, err := restore.CreateFullRestore(ctx, job.ID, snap.ID, t.TempDir(), RestoreExecOptions{})
	if err != nil {
		t.Fatalf("CreateFullRestore failed: %v", err)
	}
	if task2.Mode != RestoreModeFullImage {
		t.Errorf("expected full_image mode, got %s", task2.Mode)
	}
}

func TestRestoreManagerListRestorePoints(t *testing.T) {
	_, mgr, _, restore, _ := setupTestEnv(t)

	ctx := context.Background()
	job, _ := mgr.CreateJob(ctx, &BackupJob{
		Name:        "rp-test",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})

	mgr.mu.Lock()
	snap := &BackupSnapshot{
		ID:         uuid.New().String(),
		JobID:      job.ID,
		BackupType: BackupTypeFull,
		Size:       2048,
		FileCount:  5,
		Path:       t.TempDir(),
		CreatedAt:  time.Now(),
	}
	mgr.snapshots[snap.ID] = snap
	job.Snapshots = append(job.Snapshots, snap.ID)
	mgr.mu.Unlock()

	points := restore.ListRestorePoints(job.ID)
	if len(points) != 1 {
		t.Errorf("expected 1 restore point, got %d", len(points))
	}
	if points[0].SnapshotID != snap.ID {
		t.Error("restore point snapshot ID mismatch")
	}
}

func TestRestoreManagerGetTask(t *testing.T) {
	_, mgr, _, restore, _ := setupTestEnv(t)

	ctx := context.Background()
	job, _ := mgr.CreateJob(ctx, &BackupJob{
		Name:        "get-task-test",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})

	mgr.mu.Lock()
	snap := &BackupSnapshot{
		ID:         uuid.New().String(),
		JobID:      job.ID,
		BackupType: BackupTypeFull,
		Size:       512,
		FileCount:  3,
		Path:       t.TempDir(),
		CreatedAt:  time.Now(),
	}
	mgr.snapshots[snap.ID] = snap
	job.Snapshots = append(job.Snapshots, snap.ID)
	mgr.mu.Unlock()

	task, _ := restore.CreateSingleFileRestore(ctx, job.ID, snap.ID, []string{"a.txt"}, t.TempDir(), RestoreExecOptions{})

	fetched, err := restore.GetRestoreTask(task.ID)
	if err != nil {
		t.Fatalf("GetRestoreTask failed: %v", err)
	}
	if fetched.ID != task.ID {
		t.Error("task ID mismatch")
	}

	// 不存在的任务
	_, err = restore.GetRestoreTask("non-existent")
	if err == nil {
		t.Error("should error for non-existent task")
	}
}

// ==================== Dashboard 测试 ====================

func TestDashboardSummary(t *testing.T) {
	_, mgr, _, _, dashboard := setupTestEnv(t)

	ctx := context.Background()
	mgr.CreateJob(ctx, &BackupJob{
		Name:        "dash-1",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d1"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})
	mgr.CreateJob(ctx, &BackupJob{
		Name:        "dash-2",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d2"},
		Policy:      BackupPolicy{Type: BackupTypeIncremental},
	})

	summary := dashboard.buildSummary()
	if summary.TotalJobs != 2 {
		t.Errorf("expected 2 jobs, got %d", summary.TotalJobs)
	}
	if summary.EngineState != string(EngineStateIdle) {
		t.Errorf("expected idle, got %s", summary.EngineState)
	}
	if summary.JobsByType == nil {
		t.Error("JobsByType should not be nil")
	}
}

func TestDashboardStorageTrend(t *testing.T) {
	_, _, _, _, dashboard := setupTestEnv(t)

	trend := dashboard.buildStorageTrend()
	if trend == nil {
		t.Fatal("trend should not be nil")
	}
	// 空趋势
	if len(trend.Points) != 0 {
		t.Errorf("expected 0 trend points, got %d", len(trend.Points))
	}
}

func TestDashboardAPIEndpoints(t *testing.T) {
	_, mgr, engine, restore, dashboard := setupTestEnv(t)

	ctx := context.Background()
	mgr.CreateJob(ctx, &BackupJob{
		Name:        "api-test",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/api"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})

	handler := NewHandler(engine, mgr, restore, dashboard, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	tests := []struct {
		method string
		path   string
		code   int
	}{
		{"GET", "/api/v1/backup/active/jobs", 200},
		{"GET", "/api/v1/backup/active/engine/status", 200},
		{"GET", "/api/v1/backup/active/engine/stats", 200},
		{"GET", "/api/v1/backup/active/snapshots", 200},
		{"GET", "/api/v1/backup/active/restore-points", 200},
		{"GET", "/api/v1/backup/active/restore/tasks", 200},
		{"GET", "/api/v1/backup/active/agents", 200},
		{"GET", "/api/v1/backup/active/dashboard/summary", 200},
		{"GET", "/api/v1/backup/active/dashboard/storage-trend", 200},
		{"GET", "/api/v1/backup/active/dashboard/restore-points", 200},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		router.ServeHTTP(w, req)
		if w.Code != tt.code {
			t.Errorf("%s %s: expected %d, got %d", tt.method, tt.path, tt.code, w.Code)
		}
	}
}

func TestAPIJobCRUD(t *testing.T) {
	_, mgr, engine, restore, dashboard := setupTestEnv(t)

	handler := NewHandler(engine, mgr, restore, dashboard, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 创建任务
	jobJSON := `{
		"name": "api-crud-test",
		"source": {"paths": ["/tmp/test"]},
		"destination": {"type": "local", "path": "/tmp/dest"},
		"policy": {"type": "full"}
	}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/backup/active/jobs", bytes.NewBufferString(jobJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create job: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	jobID := created["id"].(string)

	// 获取任务
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/backup/active/jobs/"+jobID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("get job: expected 200, got %d", w.Code)
	}

	// 删除任务
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/backup/active/jobs/"+jobID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("delete job: expected 200, got %d", w.Code)
	}

	// 404 after delete
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/backup/active/jobs/"+jobID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("get deleted job: expected 404, got %d", w.Code)
	}
}

func TestAPIRunJob(t *testing.T) {
	_, mgr, engine, restore, dashboard := setupTestEnv(t)
	engine.Start(context.Background())
	defer engine.Stop()

	handler := NewHandler(engine, mgr, restore, dashboard, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先创建任务
	jobJSON := `{"name":"run-test","source":{"paths":["/tmp/test"]},"destination":{"type":"local","path":"/tmp/dest"},"policy":{"type":"full"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/backup/active/jobs", bytes.NewBufferString(jobJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	jobID := created["id"].(string)

	// 运行任务
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/backup/active/jobs/"+jobID+"/run", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("run job: expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== BackupManager 测试 ====================

func TestBackupManagerCreateJob(t *testing.T) {
	_, mgr, _, _, _ := setupTestEnv(t)
	ctx := context.Background()

	job, err := mgr.CreateJob(ctx, &BackupJob{
		Name:        "mgr-test",
		Source:      BackupSource{Paths: []string{"/tmp/test"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/dest"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if job.ID == "" {
		t.Error("job ID should not be empty")
	}
	if job.Status != BackupStatusPending {
		t.Errorf("expected pending, got %s", job.Status)
	}

	// 空名称应失败
	_, err = mgr.CreateJob(ctx, &BackupJob{})
	if err == nil {
		t.Error("empty name should fail")
	}

	// 空路径应失败
	_, err = mgr.CreateJob(ctx, &BackupJob{Name: "no-paths"})
	if err == nil {
		t.Error("empty paths should fail")
	}
}

func TestBackupManagerGetAndList(t *testing.T) {
	_, mgr, _, _, _ := setupTestEnv(t)
	ctx := context.Background()

	mgr.CreateJob(ctx, &BackupJob{
		Name:        "list-test-1",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})
	mgr.CreateJob(ctx, &BackupJob{
		Name:        "list-test-2",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d"},
		Policy:      BackupPolicy{Type: BackupTypeIncremental},
	})

	jobs := mgr.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	// 不存在的任务
	_, err := mgr.GetJob("non-existent")
	if err == nil {
		t.Error("should error for non-existent job")
	}
}

func TestBackupManagerRunBackup(t *testing.T) {
	tmpDir, mgr, _, _, _ := setupTestEnv(t)
	ctx := context.Background()

	// 设置存储路径为临时目录
	storagePath := filepath.Join(tmpDir, "backup_storage")
	os.MkdirAll(storagePath, 0755)
	mgr.config.StoragePath = storagePath

	job, _ := mgr.CreateJob(ctx, &BackupJob{
		Name:        "run-test",
		Source:      BackupSource{Paths: []string{"/tmp/test"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/dest"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})

	result, err := mgr.RunBackup(ctx, job.ID)
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}
	if result.BackupType != BackupTypeFull {
		t.Errorf("expected full, got %s", result.BackupType)
	}
	if result.SnapshotID == "" {
		t.Error("snapshot ID should not be empty")
	}

	// 不存在的任务
	_, err = mgr.RunBackup(ctx, "non-existent")
	if err == nil {
		t.Error("should error for non-existent job")
	}
}

func TestBackupManagerDeleteJob(t *testing.T) {
	_, mgr, _, _, _ := setupTestEnv(t)
	ctx := context.Background()

	job, _ := mgr.CreateJob(ctx, &BackupJob{
		Name:        "delete-test",
		Source:      BackupSource{Paths: []string{"/tmp"}},
		Destination: BackupDestination{Type: "local", Path: "/tmp/d"},
		Policy:      BackupPolicy{Type: BackupTypeFull},
	})

	err := mgr.DeleteJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}

	// 重复删除应失败
	err = mgr.DeleteJob(ctx, job.ID)
	if err == nil {
		t.Error("double delete should fail")
	}
}

func TestBackupManagerListSnapshots(t *testing.T) {
	_, mgr, _, _, _ := setupTestEnv(t)

	snaps := mgr.ListSnapshots("")
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestBackupManagerGetSnapshot(t *testing.T) {
	_, mgr, _, _, _ := setupTestEnv(t)

	_, err := mgr.GetSnapshot("non-existent")
	if err == nil {
		t.Error("should error for non-existent snapshot")
	}
}

// ==================== BackupManager Close 测试 ====================

func TestBackupManagerClose(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "close-test.json")
	logger := zap.NewNop()

	mgr, err := NewBackupManager(configPath, logger)
	if err != nil {
		t.Fatalf("NewBackupManager failed: %v", err)
	}

	if err := mgr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证配置已保存
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file should exist after Close")
	}
}
