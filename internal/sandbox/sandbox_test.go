// Package sandbox 提供安全沙箱隔离环境管理功能
package sandbox

import (
	"testing"
)

// ========== Manager 测试 ==========

func TestManager_Create(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	sandbox, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	if sandbox.ID == "" {
		t.Fatal("沙箱ID不能为空")
	}
	if sandbox.Config.Name != "test-sandbox" {
		t.Fatalf("期望名称为 'test-sandbox'，实际为 '%s'", sandbox.Config.Name)
	}
	if sandbox.Status != SandboxStatusCreated {
		t.Fatalf("期望状态为 'created'，实际为 '%s'", sandbox.Status)
	}
}

func TestManager_Create_InvalidConfig(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	req := &CreateSandboxRequest{
		Config: nil,
	}

	_, err := manager.Create(req)
	if err != ErrInvalidConfig {
		t.Fatalf("期望错误 ErrInvalidConfig，实际为 %v", err)
	}
}

func TestManager_Create_DuplicateName(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	_, err := manager.Create(req)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}

	_, err = manager.Create(req)
	if err != ErrSandboxAlreadyExists {
		t.Fatalf("期望错误 ErrSandboxAlreadyExists，实际为 %v", err)
	}
}

func TestManager_Get(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	got, err := manager.Get(created.ID)
	if err != nil {
		t.Fatalf("获取沙箱失败: %v", err)
	}

	if got.ID != created.ID {
		t.Fatalf("期望ID为 '%s'，实际为 '%s'", created.ID, got.ID)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	_, err := manager.Get("non-existent")
	if err != ErrSandboxNotFound {
		t.Fatalf("期望错误 ErrSandboxNotFound，实际为 %v", err)
	}
}

func TestManager_List(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	// 创建多个沙箱
	for i := 0; i < 3; i++ {
		config := DefaultSandboxConfig("test-sandbox-" + string(rune('a'+i)))
		req := &CreateSandboxRequest{
			Config: config,
		}
		_, err := manager.Create(req)
		if err != nil {
			t.Fatalf("创建沙箱 %d 失败: %v", i, err)
		}
	}

	list := manager.List()
	if len(list) != 3 {
		t.Fatalf("期望 3 个沙箱，实际为 %d", len(list))
	}
}

func TestManager_Update(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	newDesc := "更新后的描述"
	updateReq := &UpdateSandboxRequest{
		Description: newDesc,
	}

	updated, err := manager.Update(created.ID, updateReq)
	if err != nil {
		t.Fatalf("更新沙箱失败: %v", err)
	}

	if updated.Config.Description != newDesc {
		t.Fatalf("期望描述为 '%s'，实际为 '%s'", newDesc, updated.Config.Description)
	}
}

func TestManager_Update_Running(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动沙箱
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	// 尝试更新运行中的沙箱
	updateReq := &UpdateSandboxRequest{
		Description: "new desc",
	}

	_, err = manager.Update(created.ID, updateReq)
	if err != ErrSandboxRunning {
		t.Fatalf("期望错误 ErrSandboxRunning，实际为 %v", err)
	}
}

func TestManager_Delete(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	err = manager.Delete(created.ID)
	if err != nil {
		t.Fatalf("删除沙箱失败: %v", err)
	}

	// 验证已删除
	_, err = manager.Get(created.ID)
	if err != ErrSandboxNotFound {
		t.Fatalf("期望错误 ErrSandboxNotFound，实际为 %v", err)
	}
}

func TestManager_Delete_Running(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动沙箱
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	// 尝试删除运行中的沙箱
	err = manager.Delete(created.ID)
	if err != ErrSandboxRunning {
		t.Fatalf("期望错误 ErrSandboxRunning，实际为 %v", err)
	}
}

func TestManager_StartStop(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	sandbox, _ := manager.Get(created.ID)
	if sandbox.Status != SandboxStatusRunning {
		t.Fatalf("期望状态为 'running'，实际为 '%s'", sandbox.Status)
	}
	if sandbox.PID == 0 {
		t.Fatal("PID不能为0")
	}

	// 停止
	err = manager.Stop(created.ID)
	if err != nil {
		t.Fatalf("停止沙箱失败: %v", err)
	}

	sandbox, _ = manager.Get(created.ID)
	if sandbox.Status != SandboxStatusStopped {
		t.Fatalf("期望状态为 'stopped'，实际为 '%s'", sandbox.Status)
	}
}

func TestManager_Start_AlreadyRunning(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	// 再次启动
	err = manager.Start(created.ID)
	if err != ErrSandboxRunning {
		t.Fatalf("期望错误 ErrSandboxRunning，实际为 %v", err)
	}
}

func TestManager_Stop_NotRunning(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 尝试停止未运行的沙箱
	err = manager.Stop(created.ID)
	if err != ErrSandboxStopped {
		t.Fatalf("期望错误 ErrSandboxStopped，实际为 %v", err)
	}
}

func TestManager_PauseResume(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	// 暂停
	err = manager.Pause(created.ID)
	if err != nil {
		t.Fatalf("暂停沙箱失败: %v", err)
	}

	sandbox, _ := manager.Get(created.ID)
	if sandbox.Status != SandboxStatusPaused {
		t.Fatalf("期望状态为 'paused'，实际为 '%s'", sandbox.Status)
	}

	// 恢复
	err = manager.Resume(created.ID)
	if err != nil {
		t.Fatalf("恢复沙箱失败: %v", err)
	}

	sandbox, _ = manager.Get(created.ID)
	if sandbox.Status != SandboxStatusRunning {
		t.Fatalf("期望状态为 'running'，实际为 '%s'", sandbox.Status)
	}
}

func TestManager_GetResourceUsage(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := DefaultSandboxConfig("test-sandbox")
	req := &CreateSandboxRequest{
		Config: config,
	}

	created, err := manager.Create(req)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	// 启动
	err = manager.Start(created.ID)
	if err != nil {
		t.Fatalf("启动沙箱失败: %v", err)
	}

	usage, err := manager.GetResourceUsage(created.ID)
	if err != nil {
		t.Fatalf("获取资源使用失败: %v", err)
	}

	if usage == nil {
		t.Fatal("资源使用不能为nil")
	}
}

func TestManager_GetStats(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	// 创建多个沙箱
	for i := 0; i < 3; i++ {
		config := DefaultSandboxConfig("test-sandbox-" + string(rune('a'+i)))
		req := &CreateSandboxRequest{
			Config: config,
		}
		_, err := manager.Create(req)
		if err != nil {
			t.Fatalf("创建沙箱 %d 失败: %v", i, err)
		}
	}

	stats := manager.GetStats()
	if stats.TotalSandbox != 3 {
		t.Fatalf("期望 3 个沙箱，实际为 %d", stats.TotalSandbox)
	}
}

// ========== SnapshotManager 测试 ==========

func TestSnapshotManager_Create(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	req := &CreateSnapshotRequest{
		Name:        "test-snapshot",
		Description: "测试快照",
		Type:        SnapshotTypeFull,
	}

	snapshot, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	if snapshot.ID == "" {
		t.Fatal("快照ID不能为空")
	}
	if snapshot.Name != "test-snapshot" {
		t.Fatalf("期望名称为 'test-snapshot'，实际为 '%s'", snapshot.Name)
	}
	if snapshot.SandboxID != "sandbox-1" {
		t.Fatalf("期望沙箱ID为 'sandbox-1'，实际为 '%s'", snapshot.SandboxID)
	}
}

func TestSnapshotManager_Create_Incremental(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 创建全量快照
	req1 := &CreateSnapshotRequest{
		Name: "full-snapshot",
		Type: SnapshotTypeFull,
	}
	_, err := sm.Create("sandbox-1", req1)
	if err != nil {
		t.Fatalf("创建全量快照失败: %v", err)
	}

	// 创建增量快照
	req2 := &CreateSnapshotRequest{
		Name: "incremental-snapshot",
		Type: SnapshotTypeIncremental,
	}
	snapshot2, err := sm.Create("sandbox-1", req2)
	if err != nil {
		t.Fatalf("创建增量快照失败: %v", err)
	}

	if snapshot2.ParentID == "" {
		t.Fatal("增量快照应该有父快照ID")
	}
}

func TestSnapshotManager_Create_DuplicateName(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	req := &CreateSnapshotRequest{
		Name: "test-snapshot",
		Type: SnapshotTypeFull,
	}

	_, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}

	_, err = sm.Create("sandbox-1", req)
	if err != ErrSnapshotAlreadyExists {
		t.Fatalf("期望错误 ErrSnapshotAlreadyExists，实际为 %v", err)
	}
}

func TestSnapshotManager_Get(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	req := &CreateSnapshotRequest{
		Name: "test-snapshot",
		Type: SnapshotTypeFull,
	}

	created, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	got, err := sm.Get(created.ID)
	if err != nil {
		t.Fatalf("获取快照失败: %v", err)
	}

	if got.ID != created.ID {
		t.Fatalf("期望ID为 '%s'，实际为 '%s'", created.ID, got.ID)
	}
}

func TestSnapshotManager_Get_NotFound(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	_, err := sm.Get("non-existent")
	if err != ErrSnapshotNotFound {
		t.Fatalf("期望错误 ErrSnapshotNotFound，实际为 %v", err)
	}
}

func TestSnapshotManager_ListBySandbox(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 创建不同沙箱的快照
	for i := 0; i < 3; i++ {
		req := &CreateSnapshotRequest{
			Name: "snapshot-" + string(rune('a'+i)),
			Type: SnapshotTypeFull,
		}
		_, err := sm.Create("sandbox-1", req)
		if err != nil {
			t.Fatalf("创建快照 %d 失败: %v", i, err)
		}
	}

	req := &CreateSnapshotRequest{
		Name: "snapshot-other",
		Type: SnapshotTypeFull,
	}
	_, err := sm.Create("sandbox-2", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	list := sm.ListBySandbox("sandbox-1")
	if len(list) != 3 {
		t.Fatalf("期望 3 个快照，实际为 %d", len(list))
	}
}

func TestSnapshotManager_Delete(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	req := &CreateSnapshotRequest{
		Name: "test-snapshot",
		Type: SnapshotTypeFull,
	}

	created, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	err = sm.Delete(created.ID)
	if err != nil {
		t.Fatalf("删除快照失败: %v", err)
	}

	// 验证已删除
	_, err = sm.Get(created.ID)
	if err != ErrSnapshotNotFound {
		t.Fatalf("期望错误 ErrSnapshotNotFound，实际为 %v", err)
	}
}

func TestSnapshotManager_Delete_WithDependency(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 创建全量快照
	req1 := &CreateSnapshotRequest{
		Name: "full-snapshot",
		Type: SnapshotTypeFull,
	}
	full, err := sm.Create("sandbox-1", req1)
	if err != nil {
		t.Fatalf("创建全量快照失败: %v", err)
	}

	// 创建增量快照
	req2 := &CreateSnapshotRequest{
		Name: "incremental-snapshot",
		Type: SnapshotTypeIncremental,
	}
	_, err = sm.Create("sandbox-1", req2)
	if err != nil {
		t.Fatalf("创建增量快照失败: %v", err)
	}

	// 尝试删除全量快照（应该失败，因为有依赖）
	err = sm.Delete(full.ID)
	if err == nil {
		t.Fatal("期望删除失败，但成功了")
	}
}

func TestSnapshotManager_DeleteBySandbox(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 创建多个快照
	for i := 0; i < 3; i++ {
		req := &CreateSnapshotRequest{
			Name: "snapshot-" + string(rune('a'+i)),
			Type: SnapshotTypeFull,
		}
		_, err := sm.Create("sandbox-1", req)
		if err != nil {
			t.Fatalf("创建快照 %d 失败: %v", i, err)
		}
	}

	err := sm.DeleteBySandbox("sandbox-1")
	if err != nil {
		t.Fatalf("删除沙箱快照失败: %v", err)
	}

	list := sm.ListBySandbox("sandbox-1")
	if len(list) != 0 {
		t.Fatalf("期望 0 个快照，实际为 %d", len(list))
	}
}

func TestSnapshotManager_Restore(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	req := &CreateSnapshotRequest{
		Name: "test-snapshot",
		Type: SnapshotTypeFull,
	}

	created, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	err = sm.Restore(created.ID, "/tmp/restore-target")
	if err != nil {
		t.Fatalf("恢复快照失败: %v", err)
	}
}

func TestSnapshotManager_Restore_NotFound(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	err := sm.Restore("non-existent", "/tmp/restore-target")
	if err != ErrSnapshotNotFound {
		t.Fatalf("期望错误 ErrSnapshotNotFound，实际为 %v", err)
	}
}

func TestSnapshotManager_GetStats(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 创建快照
	for i := 0; i < 3; i++ {
		req := &CreateSnapshotRequest{
			Name: "snapshot-" + string(rune('a'+i)),
			Type: SnapshotTypeFull,
		}
		_, err := sm.Create("sandbox-1", req)
		if err != nil {
			t.Fatalf("创建快照 %d 失败: %v", i, err)
		}
	}

	stats := sm.GetStats()
	if stats.TotalCount != 3 {
		t.Fatalf("期望 3 个快照，实际为 %d", stats.TotalCount)
	}
	if stats.BySandbox["sandbox-1"] != 3 {
		t.Fatalf("期望沙箱 sandbox-1 有 3 个快照，实际为 %d", stats.BySandbox["sandbox-1"])
	}
}

func TestSnapshotManager_Count(t *testing.T) {
	sm := NewSnapshotManager("/tmp/snapshot-test")

	// 初始应该为 0
	if sm.Count() != 0 {
		t.Fatalf("期望 0 个快照，实际为 %d", sm.Count())
	}

	// 创建快照
	req := &CreateSnapshotRequest{
		Name: "test-snapshot",
		Type: SnapshotTypeFull,
	}
	_, err := sm.Create("sandbox-1", req)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	if sm.Count() != 1 {
		t.Fatalf("期望 1 个快照，实际为 %d", sm.Count())
	}
}

// ========== Isolator 测试 ==========

func TestIsolator_SetupIsolation(t *testing.T) {
	isolator := NewIsolator()

	sandbox := &Sandbox{
		ID:       "test-sandbox",
		RootPath: "/tmp/test-sandbox",
		Config: &SandboxConfig{
			Name:           "test",
			IsolationLevel: IsolationLevelStandard,
			Filesystem: &FilesystemIsolation{
				ReadOnly:    false,
				TmpFSSizeMB: 100,
			},
			Network: &NetworkIsolation{
				Enabled:       true,
				Mode:          NetworkModeBridge,
				AllowOutbound: true,
			},
		},
	}

	err := isolator.SetupIsolation(sandbox)
	if err != nil {
		t.Fatalf("设置隔离失败: %v", err)
	}
}

func TestIsolator_StartStop(t *testing.T) {
	isolator := NewIsolator()

	sandbox := &Sandbox{
		ID:       "test-sandbox",
		RootPath: "/tmp/test-sandbox",
		Config: &SandboxConfig{
			Name: "test",
		},
	}

	// 启动
	err := isolator.StartProcess(sandbox)
	if err != nil {
		t.Fatalf("启动进程失败: %v", err)
	}

	if sandbox.PID == 0 {
		t.Fatal("PID不能为0")
	}

	// 停止
	err = isolator.StopProcess(sandbox)
	if err != nil {
		t.Fatalf("停止进程失败: %v", err)
	}

	if sandbox.PID != 0 {
		t.Fatal("停止后PID应该为0")
	}
}

func TestIsolator_PauseResume(t *testing.T) {
	isolator := NewIsolator()

	sandbox := &Sandbox{
		ID:       "test-sandbox",
		RootPath: "/tmp/test-sandbox",
		Config: &SandboxConfig{
			Name: "test",
		},
	}

	// 启动
	err := isolator.StartProcess(sandbox)
	if err != nil {
		t.Fatalf("启动进程失败: %v", err)
	}

	// 暂停
	err = isolator.PauseProcess(sandbox)
	if err != nil {
		t.Fatalf("暂停进程失败: %v", err)
	}

	// 恢复
	err = isolator.ResumeProcess(sandbox)
	if err != nil {
		t.Fatalf("恢复进程失败: %v", err)
	}

	// 停止
	err = isolator.StopProcess(sandbox)
	if err != nil {
		t.Fatalf("停止进程失败: %v", err)
	}
}

func TestIsolator_GetResourceUsage(t *testing.T) {
	isolator := NewIsolator()

	sandbox := &Sandbox{
		ID:       "test-sandbox",
		RootPath: "/tmp/test-sandbox",
		Config: &SandboxConfig{
			Name: "test",
		},
	}

	// 启动
	err := isolator.StartProcess(sandbox)
	if err != nil {
		t.Fatalf("启动进程失败: %v", err)
	}

	// 获取资源使用
	usage, err := isolator.GetResourceUsage(sandbox)
	if err != nil {
		t.Fatalf("获取资源使用失败: %v", err)
	}

	if usage == nil {
		t.Fatal("资源使用不能为nil")
	}

	// 停止
	err = isolator.StopProcess(sandbox)
	if err != nil {
		t.Fatalf("停止进程失败: %v", err)
	}
}

func TestIsolator_ApplyResourceLimit(t *testing.T) {
	isolator := NewIsolator()

	sandbox := &Sandbox{
		ID:       "test-sandbox",
		RootPath: "/tmp/test-sandbox",
		Config: &SandboxConfig{
			Name: "test",
			ResourceLimit: &ResourceLimit{
				CPUCores:             2.0,
				CPUShares:            2048,
				MemoryMB:             1024,
				MemorySwapMB:         -1,
				DiskIOMBps:           200,
				DiskIOPS:             2000,
				NetworkBandwidthMbps: 100,
				PIDsLimit:            512,
				OpenFilesLimit:       2048,
			},
			Network: &NetworkIsolation{
				Enabled: true,
				Mode:    NetworkModeBridge,
			},
		},
	}

	err := isolator.ApplyResourceLimit(sandbox)
	if err != nil {
		t.Fatalf("应用资源限制失败: %v", err)
	}
}

// ========== Types 测试 ==========

func TestDefaultResourceLimit(t *testing.T) {
	limit := DefaultResourceLimit()

	if limit.CPUCores != 1.0 {
		t.Fatalf("期望CPU核心数为 1.0，实际为 %f", limit.CPUCores)
	}
	if limit.MemoryMB != 512 {
		t.Fatalf("期望内存为 512MB，实际为 %d", limit.MemoryMB)
	}
	if limit.PIDsLimit != 256 {
		t.Fatalf("期望进程数限制为 256，实际为 %d", limit.PIDsLimit)
	}
}

func TestDefaultSandboxConfig(t *testing.T) {
	config := DefaultSandboxConfig("test")

	if config.Name != "test" {
		t.Fatalf("期望名称为 'test'，实际为 '%s'", config.Name)
	}
	if config.IsolationLevel != IsolationLevelStandard {
		t.Fatalf("期望隔离级别为 'standard'，实际为 '%s'", config.IsolationLevel)
	}
	if config.ResourceLimit == nil {
		t.Fatal("资源限制不能为nil")
	}
	if config.Network == nil {
		t.Fatal("网络配置不能为nil")
	}
	if config.Filesystem == nil {
		t.Fatal("文件系统配置不能为nil")
	}
}

// ========== Validation 测试 ==========

func TestManager_validateConfig_EmptyName(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	config := &SandboxConfig{
		Name: "",
	}

	err := manager.validateConfig(config)
	if err == nil {
		t.Fatal("期望验证失败，但成功了")
	}
}

func TestManager_validateResourceLimit_Negative(t *testing.T) {
	manager := NewManager("/tmp/sandbox-test")

	tests := []struct {
		name  string
		limit *ResourceLimit
	}{
		{
			name:  "负CPU核心数",
			limit: &ResourceLimit{CPUCores: -1},
		},
		{
			name:  "负内存",
			limit: &ResourceLimit{MemoryMB: -1},
		},
		{
			name:  "负磁盘IO",
			limit: &ResourceLimit{DiskIOMBps: -1},
		},
		{
			name:  "负网络带宽",
			limit: &ResourceLimit{NetworkBandwidthMbps: -1},
		},
		{
			name:  "负进程数",
			limit: &ResourceLimit{PIDsLimit: -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateResourceLimit(tt.limit)
			if err == nil {
				t.Fatal("期望验证失败，但成功了")
			}
		})
	}
}
