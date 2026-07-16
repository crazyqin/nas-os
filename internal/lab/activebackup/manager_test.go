// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"path/filepath"
	"testing"
	"time"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr, err := NewManager("")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_WithConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "activebackup-config.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

// ========== Agent 管理测试 ==========

func TestRegisterAgent(t *testing.T) {
	mgr, _ := NewManager("")

	req := AgentRegistrationRequest{
		Name:        "test-server",
		Hostname:    "test-server.local",
		IP:          "192.168.1.100",
		Platform:    PlatformLinux,
		OSVersion:   "Ubuntu 22.04",
		MACAddress:  "00:11:22:33:44:55",
		Fingerprint: "abc123def456",
		Memory:      8 << 30,
	}

	agent, err := mgr.RegisterAgent(req)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	if agent.ID == "" {
		t.Error("agent ID should not be empty")
	}
	if agent.Name != "test-server" {
		t.Errorf("expected name test-server, got %s", agent.Name)
	}
	if agent.Status != AgentStatusOnline {
		t.Errorf("expected status online, got %s", agent.Status)
	}
	if agent.Token == "" {
		t.Error("agent token should not be empty")
	}
}

func TestRegisterAgent_Duplicate(t *testing.T) {
	mgr, _ := NewManager("")

	req := AgentRegistrationRequest{
		Name:        "test-server",
		Hostname:    "test-server.local",
		IP:          "192.168.1.100",
		Platform:    PlatformLinux,
		MACAddress:  "00:11:22:33:44:55",
		Fingerprint: "abc123def456",
	}

	_, err := mgr.RegisterAgent(req)
	if err != nil {
		t.Fatalf("first RegisterAgent failed: %v", err)
	}

	_, err = mgr.RegisterAgent(req)
	if err != ErrAgentExists {
		t.Errorf("expected ErrAgentExists, got %v", err)
	}
}

func TestGetAgent(t *testing.T) {
	mgr, _ := NewManager("")

	req := AgentRegistrationRequest{
		Name:        "test-server",
		Hostname:    "test-server.local",
		IP:          "192.168.1.100",
		Platform:    PlatformLinux,
		MACAddress:  "00:11:22:33:44:55",
		Fingerprint: "abc123def456",
	}

	agent, _ := mgr.RegisterAgent(req)

	retrieved, err := mgr.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if retrieved.ID != agent.ID {
		t.Error("agent ID mismatch")
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	mgr, _ := NewManager("")

	_, err := mgr.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestListAgents(t *testing.T) {
	mgr, _ := NewManager("")

	// 注册两个 Agent
	mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "agent1", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})
	mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "agent2", Hostname: "h2", IP: "10.0.0.2",
		Platform: PlatformWindows, MACAddress: "aa:bb:cc:dd:ee:02", Fingerprint: "fp2",
	})

	agents := mgr.ListAgents()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestDeleteAgent(t *testing.T) {
	mgr, _ := NewManager("")

	req := AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	}

	agent, _ := mgr.RegisterAgent(req)

	err := mgr.DeleteAgent(agent.ID)
	if err != nil {
		t.Fatalf("DeleteAgent failed: %v", err)
	}

	_, err = mgr.GetAgent(agent.ID)
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound after delete, got %v", err)
	}
}

func TestDeleteAgent_NotFound(t *testing.T) {
	mgr, _ := NewManager("")

	err := mgr.DeleteAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestProcessHeartbeat(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	err := mgr.ProcessHeartbeat(AgentHeartbeatRequest{
		AgentID: agent.ID,
		Status:  AgentStatusOnline,
		Disks: []DiskInfo{
			{Device: "/dev/sda1", Size: 500 << 30, Used: 200 << 30, Free: 300 << 30, FileSystem: "ext4", MountPoint: "/"},
		},
	})
	if err != nil {
		t.Fatalf("ProcessHeartbeat failed: %v", err)
	}

	// 验证磁盘信息已更新
	updated, _ := mgr.GetAgent(agent.ID)
	if len(updated.Disks) != 1 {
		t.Errorf("expected 1 disk, got %d", len(updated.Disks))
	}
}

func TestCheckAgentOnline(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	// 刚注册的 Agent 应该在线
	online, err := mgr.CheckAgentOnline(agent.ID)
	if err != nil {
		t.Fatalf("CheckAgentOnline failed: %v", err)
	}
	if !online {
		t.Error("agent should be online right after registration")
	}
}

// ========== 备份任务测试 ==========

func TestCreateTask(t *testing.T) {
	mgr, _ := NewManager("")

	// 先注册 Agent
	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	req := CreateTaskRequest{
		Name:          "daily-backup",
		Description:   "每日全量备份",
		AgentID:       agent.ID,
		BackupType:    BackupTypeFull,
		ScheduleType:  ScheduleTypeScheduled,
		Schedule:      "0 2 * * *",
		Enabled:       true,
		RetentionDays: 30,
	}

	task, err := mgr.CreateTask(req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.Name != "daily-backup" {
		t.Errorf("expected name daily-backup, got %s", task.Name)
	}
	if task.Status != TaskStatusIdle {
		t.Errorf("expected status idle, got %s", task.Status)
	}
	if task.Compression != CompressionLZ4 {
		t.Errorf("expected default compression lz4, got %s", task.Compression)
	}
}

func TestCreateTask_AgentNotFound(t *testing.T) {
	mgr, _ := NewManager("")

	req := CreateTaskRequest{
		Name:       "daily-backup",
		AgentID:    "nonexistent",
		BackupType: BackupTypeFull,
	}

	_, err := mgr.CreateTask(req)
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestGetTask(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	retrieved, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if retrieved.ID != task.ID {
		t.Error("task ID mismatch")
	}
}

func TestListTasks(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	mgr.CreateTask(CreateTaskRequest{Name: "task1", AgentID: agent.ID, BackupType: BackupTypeFull})
	mgr.CreateTask(CreateTaskRequest{Name: "task2", AgentID: agent.ID, BackupType: BackupTypeIncremental})

	tasks := mgr.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "old-name", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	newName := "new-name"
	updated, err := mgr.UpdateTask(task.ID, UpdateTaskRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("expected name new-name, got %s", updated.Name)
	}
}

func TestDeleteTask(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	err := mgr.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	_, err = mgr.GetTask(task.ID)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound after delete, got %v", err)
	}
}

func TestRunTask(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	running, err := mgr.RunTask(task.ID)
	if err != nil {
		t.Fatalf("RunTask failed: %v", err)
	}
	if running.Status != TaskStatusRunning {
		t.Errorf("expected status running, got %s", running.Status)
	}
	if running.Progress != 0 {
		t.Errorf("expected progress 0, got %f", running.Progress)
	}
}

func TestRunTask_AlreadyRunning(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	_, _ = mgr.RunTask(task.ID)
	_, err := mgr.RunTask(task.ID)
	if err != ErrTaskRunning {
		t.Errorf("expected ErrTaskRunning, got %v", err)
	}
}

func TestCancelTask(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	_, _ = mgr.RunTask(task.ID)

	err := mgr.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	cancelled, _ := mgr.GetTask(task.ID)
	if cancelled.Status != TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}
}

func TestCancelTask_NotRunning(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	err := mgr.CancelTask(task.ID)
	if err != ErrTaskNotRunning {
		t.Errorf("expected ErrTaskNotRunning, got %v", err)
	}
}

// ========== 恢复点测试 ==========

func TestCreateAndGetRestorePoint(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	point := &RestorePoint{
		ID:             "rp-001",
		TaskID:         task.ID,
		TaskName:       task.Name,
		AgentID:        agent.ID,
		AgentName:      agent.Name,
		Type:           RestorePointFull,
		Size:           50 << 30,
		CompressedSize: 30 << 30,
		StoragePath:    "/backup/pool/rp-001",
		CreatedAt:      time.Now(),
		Volumes:        []string{"C:", "D:"},
		Checksum:       "sha256:abcdef1234567890",
	}

	mgr.CreateRestorePoint(point)

	retrieved, err := mgr.GetRestorePoint("rp-001")
	if err != nil {
		t.Fatalf("GetRestorePoint failed: %v", err)
	}
	if retrieved.Size != 50<<30 {
		t.Errorf("expected size 50GB, got %d", retrieved.Size)
	}
}

func TestListRestorePoints(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-001", TaskID: task.ID, AgentID: agent.ID,
		Type: RestorePointFull, Size: 50 << 30, CreatedAt: time.Now(),
	})
	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-002", TaskID: task.ID, AgentID: agent.ID,
		Type: RestorePointIncremental, Size: 10 << 30, CreatedAt: time.Now(),
	})

	points := mgr.ListRestorePoints("")
	if len(points) != 2 {
		t.Errorf("expected 2 restore points, got %d", len(points))
	}

	// 按任务过滤
	filtered := mgr.ListRestorePoints(task.ID)
	if len(filtered) != 2 {
		t.Errorf("expected 2 restore points for task, got %d", len(filtered))
	}
}

func TestDeleteRestorePoint(t *testing.T) {
	mgr, _ := NewManager("")

	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-001", TaskID: "t1", AgentID: "a1",
		Type: RestorePointFull, Size: 50 << 30, CreatedAt: time.Now(),
	})

	err := mgr.DeleteRestorePoint("rp-001")
	if err != nil {
		t.Fatalf("DeleteRestorePoint failed: %v", err)
	}

	_, err = mgr.GetRestorePoint("rp-001")
	if err != ErrRestorePointNotFound {
		t.Errorf("expected ErrRestorePointNotFound, got %v", err)
	}
}

func TestGetRestorePointChain(t *testing.T) {
	mgr, _ := NewManager("")

	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-001", TaskID: "t1", AgentID: "a1",
		Type: RestorePointFull, Size: 50 << 30, CreatedAt: time.Now(),
	})
	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-002", TaskID: "t1", AgentID: "a1",
		Type: RestorePointIncremental, Size: 10 << 30,
		ParentID: "rp-001", CreatedAt: time.Now(),
	})
	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-003", TaskID: "t1", AgentID: "a1",
		Type: RestorePointIncremental, Size: 5 << 30,
		ParentID: "rp-002", CreatedAt: time.Now(),
	})

	chain := mgr.GetRestorePointChain("rp-003")
	if len(chain) != 3 {
		t.Errorf("expected chain length 3, got %d", len(chain))
	}
	if chain[0].ID != "rp-001" {
		t.Errorf("expected first in chain to be rp-001, got %s", chain[0].ID)
	}
}

func TestBrowseRestorePoint(t *testing.T) {
	mgr, _ := NewManager("")

	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-001", TaskID: "t1", AgentID: "a1",
		Type: RestorePointFull, Size: 50 << 30, CreatedAt: time.Now(),
	})

	// 浏览根目录
	items, err := mgr.BrowseRestorePoint("rp-001", "")
	if err != nil {
		t.Fatalf("BrowseRestorePoint failed: %v", err)
	}
	if len(items) == 0 {
		t.Error("expected at least one browse item")
	}

	// 浏览子目录
	items, err = mgr.BrowseRestorePoint("rp-001", "/C:")
	if err != nil {
		t.Fatalf("BrowseRestorePoint with path failed: %v", err)
	}
	if len(items) == 0 {
		t.Error("expected at least one browse item")
	}
}

// ========== 恢复任务测试 ==========

func TestCreateRestoreJob(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	mgr.CreateRestorePoint(&RestorePoint{
		ID: "rp-001", TaskID: "t1", AgentID: agent.ID, AgentName: agent.Name,
		Type: RestorePointFull, Size: 50 << 30, CreatedAt: time.Now(),
	})

	req := RestoreRequest{
		RestorePointID: "rp-001",
		RestoreType:    RestoreTypeFull,
		TargetDisk:     "/dev/sda",
	}

	job, err := mgr.CreateRestoreJob(req)
	if err != nil {
		t.Fatalf("CreateRestoreJob failed: %v", err)
	}
	if job.Status != TaskStatusRunning {
		t.Errorf("expected status running, got %s", job.Status)
	}
	if job.AgentID != agent.ID {
		t.Error("agent ID mismatch")
	}
}

func TestCreateRestoreJob_RestorePointNotFound(t *testing.T) {
	mgr, _ := NewManager("")

	req := RestoreRequest{
		RestorePointID: "nonexistent",
		RestoreType:    RestoreTypeFull,
	}

	_, err := mgr.CreateRestoreJob(req)
	if err != ErrRestorePointNotFound {
		t.Errorf("expected ErrRestorePointNotFound, got %v", err)
	}
}

// ========== 存储池测试 ==========

func TestCreateAndGetStoragePool(t *testing.T) {
	mgr, _ := NewManager("")

	pool := mgr.CreateStoragePool("primary", "/backup/pool", 1<<40)
	if pool.ID == "" {
		t.Error("pool ID should not be empty")
	}
	if pool.TotalBytes != 1<<40 {
		t.Errorf("expected 1TB, got %d", pool.TotalBytes)
	}

	retrieved, err := mgr.GetStoragePool(pool.ID)
	if err != nil {
		t.Fatalf("GetStoragePool failed: %v", err)
	}
	if retrieved.Name != "primary" {
		t.Errorf("expected name primary, got %s", retrieved.Name)
	}
}

func TestListStoragePools(t *testing.T) {
	mgr, _ := NewManager("")

	mgr.CreateStoragePool("pool1", "/backup/pool1", 500<<30)
	mgr.CreateStoragePool("pool2", "/backup/pool2", 1<<40)

	pools := mgr.ListStoragePools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	mgr, _ := NewManager("")

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	mgr.CreateTask(CreateTaskRequest{
		Name: "task1", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	stats := mgr.GetStats()
	if stats.TotalAgents != 1 {
		t.Errorf("expected 1 agent, got %d", stats.TotalAgents)
	}
	if stats.OnlineAgents != 1 {
		t.Errorf("expected 1 online agent, got %d", stats.OnlineAgents)
	}
	if stats.TotalTasks != 1 {
		t.Errorf("expected 1 task, got %d", stats.TotalTasks)
	}
}

func TestGetStorageUsage(t *testing.T) {
	mgr, _ := NewManager("")

	mgr.CreateStoragePool("pool1", "/backup/pool1", 500<<30)

	usage := mgr.GetStorageUsage()
	if usage.TotalBytes != 500<<30 {
		t.Errorf("expected 500GB, got %d", usage.TotalBytes)
	}
	if len(usage.Pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(usage.Pools))
	}
}

// ========== 调度器测试 ==========

func TestScheduler_StartStop(t *testing.T) {
	mgr, _ := NewManager("")
	scheduler := NewScheduler(mgr)

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}

	scheduler.Start()
	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// 重复启动不报错
	scheduler.Start()

	scheduler.Stop()
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop()")
	}
}

func TestScheduler_GetStats(t *testing.T) {
	mgr, _ := NewManager("")
	scheduler := NewScheduler(mgr)

	agent, _ := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	mgr.CreateTask(CreateTaskRequest{
		Name: "scheduled-task", AgentID: agent.ID, BackupType: BackupTypeFull,
		ScheduleType: ScheduleTypeScheduled, Schedule: "0 2 * * *",
	})
	mgr.CreateTask(CreateTaskRequest{
		Name: "manual-task", AgentID: agent.ID, BackupType: BackupTypeIncremental,
		ScheduleType: ScheduleTypeManual,
	})

	stats := scheduler.GetStats()
	if stats.ScheduledTasks != 1 {
		t.Errorf("expected 1 scheduled task, got %d", stats.ScheduledTasks)
	}
	if stats.ManualTasks != 1 {
		t.Errorf("expected 1 manual task, got %d", stats.ManualTasks)
	}
}

// ========== Agent Manager 测试 ==========

func TestAgentManager_StartStop(t *testing.T) {
	mgr, _ := NewManager("")
	am := NewAgentManager(mgr)

	if am.IsRunning() {
		t.Error("agent manager should not be running initially")
	}

	am.Start()
	if !am.IsRunning() {
		t.Error("agent manager should be running after Start()")
	}

	am.Stop()
	if am.IsRunning() {
		t.Error("agent manager should not be running after Stop()")
	}
}

func TestAgentManager_Connections(t *testing.T) {
	mgr, _ := NewManager("")
	am := NewAgentManager(mgr)

	am.RegisterConnection("agent-1", "192.168.1.100:8080")

	conn := am.GetConnection("agent-1")
	if conn == nil {
		t.Fatal("connection should not be nil")
	}
	if conn.RemoteAddr != "192.168.1.100:8080" {
		t.Errorf("expected addr 192.168.1.100:8080, got %s", conn.RemoteAddr)
	}

	conns := am.ListConnections()
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}

	am.UnregisterConnection("agent-1")
	conns = am.ListConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections after unregister, got %d", len(conns))
	}
}

// ========== Restore Manager 测试 ==========

func TestRestoreManager_StartStop(t *testing.T) {
	mgr, _ := NewManager("")
	rm := NewRestoreManager(mgr)

	if rm.IsRunning() {
		t.Error("restore manager should not be running initially")
	}

	rm.Start()
	if !rm.IsRunning() {
		t.Error("restore manager should be running after Start()")
	}

	rm.Stop()
	if rm.IsRunning() {
		t.Error("restore manager should not be running after Stop()")
	}
}

func TestRestoreManager_StartRestore(t *testing.T) {
	mgr, _ := NewManager("")
	rm := NewRestoreManager(mgr)

	job := &RestoreJob{
		ID:             "job-001",
		RestorePointID: "rp-001",
		AgentID:        "agent-001",
		RestoreType:    RestoreTypeFull,
		Status:         TaskStatusRunning,
		StartedAt:      time.Now(),
	}

	state := rm.StartRestore(job)
	if state.Stage != RestoreStageInit {
		t.Errorf("expected stage init, got %s", state.Stage)
	}

	rm.UpdateStage("job-001", RestoreStageDownloading, "正在下载数据")
	updated := rm.GetJobState("job-001")
	if updated.Stage != RestoreStageDownloading {
		t.Errorf("expected stage downloading, got %s", updated.Stage)
	}
}

// ========== 持久化测试 ==========

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "activebackup-config.json")

	// 创建管理器和数据
	mgr1, _ := NewManager(configPath)

	agent, _ := mgr1.RegisterAgent(AgentRegistrationRequest{
		Name: "test-server", Hostname: "h1", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
	})

	task, _ := mgr1.CreateTask(CreateTaskRequest{
		Name: "daily-backup", AgentID: agent.ID, BackupType: BackupTypeFull,
	})

	// 重新加载
	mgr2, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("NewManager (reload) failed: %v", err)
	}

	// 验证 Agent 持久化
	loadedAgent, err := mgr2.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent after reload failed: %v", err)
	}
	if loadedAgent.Name != "test-server" {
		t.Errorf("agent name mismatch after reload")
	}

	// 验证任务持久化
	loadedTask, err := mgr2.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask after reload failed: %v", err)
	}
	if loadedTask.Name != "daily-backup" {
		t.Errorf("task name mismatch after reload")
	}
}

// ========== 完整流程测试 ==========

func TestFullBackupFlow(t *testing.T) {
	mgr, _ := NewManager("")

	// 1. 注册 Agent
	agent, err := mgr.RegisterAgent(AgentRegistrationRequest{
		Name: "production-server", Hostname: "prod.local", IP: "10.0.0.1",
		Platform: PlatformLinux, MACAddress: "aa:bb:cc:dd:ee:01", Fingerprint: "fp1",
		OSVersion: "Ubuntu 22.04", Memory: 16 << 30,
	})
	if err != nil {
		t.Fatalf("step 1 - RegisterAgent: %v", err)
	}

	// 2. 创建备份任务
	task, err := mgr.CreateTask(CreateTaskRequest{
		Name:          "full-backup",
		Description:   "每日全量备份",
		AgentID:       agent.ID,
		BackupType:    BackupTypeFull,
		ScheduleType:  ScheduleTypeScheduled,
		Schedule:      "0 2 * * *",
		Enabled:       true,
		RetentionDays: 30,
		MaxVersions:   10,
	})
	if err != nil {
		t.Fatalf("step 2 - CreateTask: %v", err)
	}

	// 3. 手动执行备份
	running, err := mgr.RunTask(task.ID)
	if err != nil {
		t.Fatalf("step 3 - RunTask: %v", err)
	}
	if running.Status != TaskStatusRunning {
		t.Fatalf("step 3 - expected running, got %s", running.Status)
	}

	// 4. 创建恢复点（模拟备份完成）
	mgr.CreateRestorePoint(&RestorePoint{
		ID:             "rp-001",
		TaskID:         task.ID,
		TaskName:       task.Name,
		AgentID:        agent.ID,
		AgentName:      agent.Name,
		Type:           RestorePointFull,
		Size:           100 << 30,
		CompressedSize: 60 << 30,
		StoragePath:    "/backup/pool/rp-001",
		CreatedAt:      time.Now(),
		Volumes:        []string{"/dev/sda1"},
		Checksum:       "sha256:abcdef",
	})

	// 5. 完成任务
	mgr.CompleteTask(task.ID, true, "rp-001", "")

	completed, _ := mgr.GetTask(task.ID)
	if completed.Status != TaskStatusSuccess {
		t.Errorf("step 5 - expected success, got %s", completed.Status)
	}

	// 6. 验证统计
	stats := mgr.GetStats()
	if stats.TotalAgents != 1 {
		t.Errorf("step 6 - expected 1 agent, got %d", stats.TotalAgents)
	}
	if stats.TotalRestorePoints != 1 {
		t.Errorf("step 6 - expected 1 restore point, got %d", stats.TotalRestorePoints)
	}
	if stats.SuccessRate != 100 {
		t.Errorf("step 6 - expected 100%% success rate, got %f", stats.SuccessRate)
	}

	// 7. 整机恢复
	restoreJob, err := mgr.CreateRestoreJob(RestoreRequest{
		RestorePointID: "rp-001",
		RestoreType:    RestoreTypeFull,
		TargetDisk:     "/dev/sda",
	})
	if err != nil {
		t.Fatalf("step 7 - CreateRestoreJob: %v", err)
	}
	if restoreJob.Status != TaskStatusRunning {
		t.Errorf("step 7 - expected running, got %s", restoreJob.Status)
	}
}
