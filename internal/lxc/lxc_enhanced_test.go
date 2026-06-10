package lxc

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestManager creates a Manager with a temp storage directory for tests.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StoragePath = filepath.Join(tmpDir, "lxc")
	cfg.SnapshotPath = filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(cfg.StoragePath, 0755)
	os.MkdirAll(cfg.SnapshotPath, 0755)
	return NewManager(cfg)
}

// ===== 状态机测试 =====

func TestValidContainerTransition(t *testing.T) {
	tests := []struct {
		name string
		from ContainerStatus
		to   ContainerStatus
		want bool
	}{
		{"created->starting", StatusCreated, StatusStarting, true},
		{"created->stopped", StatusCreated, StatusStopped, true},
		{"starting->running", StatusStarting, StatusRunning, true},
		{"starting->error", StatusStarting, StatusError, true},
		{"running->stopping", StatusRunning, StatusStopping, true},
		{"running->paused", StatusRunning, StatusPaused, true},
		{"running->migrating", StatusRunning, StatusMigrating, true},
		{"running->error", StatusRunning, StatusError, true},
		{"stopping->stopped", StatusStopping, StatusStopped, true},
		{"stopped->starting", StatusStopped, StatusStarting, true},
		{"paused->running", StatusPaused, StatusRunning, true},
		{"error->starting", StatusError, StatusStarting, true},
		{"error->stopped", StatusError, StatusStopped, true},
		// 无效转换
		{"running->starting", StatusRunning, StatusStarting, false},
		{"stopped->running", StatusStopped, StatusRunning, false},
		{"starting->stopping", StatusStarting, StatusStopping, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidContainerTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("ValidContainerTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// ===== 资源限制验证测试 =====

func TestResourceLimitValidate(t *testing.T) {
	tests := []struct {
		name    string
		res     ResourceLimit
		wantErr bool
	}{
		{"valid", ResourceLimit{CPUCores: 2, MemoryMB: 512}, false},
		{"zero memory", ResourceLimit{CPUCores: 1, MemoryMB: 0}, true},
		{"negative cpu", ResourceLimit{CPUCores: -1, MemoryMB: 256}, true},
		{"cpu percent over 100", ResourceLimit{CPUCores: 1, MemoryMB: 256, CPUPercent: 101}, true},
		{"negative process max", ResourceLimit{CPUCores: 1, MemoryMB: 256, ProcessMax: -1}, true},
		{"io weight over 1000", ResourceLimit{CPUCores: 1, MemoryMB: 256, IOWeight: 1001}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.res.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ===== 模板管理测试 =====

func TestTemplateManagerBuiltin(t *testing.T) {
	tm := NewTemplateManager()

	// 应有内置模板
	if tm.Count() < 10 {
		t.Errorf("内置模板数量不足，got %d", tm.Count())
	}

	// 获取 ubuntu-24.04
	tpl, err := tm.Get("ubuntu-24.04")
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}
	if tpl.Distro != "ubuntu" {
		t.Errorf("期望 distro=ubuntu, got %s", tpl.Distro)
	}
	if !tpl.IsBuiltin {
		t.Error("内置模板应标记为 IsBuiltin")
	}
}

func TestTemplateManagerRegister(t *testing.T) {
	tm := NewTemplateManager()

	// 注册新模板
	err := tm.Register(&Template{
		Name:    "custom-1",
		Distro:  "custom",
		Version: "1.0",
	})
	if err != nil {
		t.Fatalf("注册模板失败: %v", err)
	}

	// 验证已注册
	if !tm.Exists("custom-1") {
		t.Error("模板应已存在")
	}

	// 重复注册应失败
	err = tm.Register(&Template{Name: "custom-1", Distro: "x"})
	if err == nil {
		t.Error("重复注册应返回错误")
	}

	// 空名称应失败
	err = tm.Register(&Template{})
	if err == nil {
		t.Error("空名称注册应返回错误")
	}

	// 空发行版应失败
	err = tm.Register(&Template{Name: "test"})
	if err == nil {
		t.Error("空发行版注册应返回错误")
	}
}

func TestTemplateManagerListByCategory(t *testing.T) {
	tm := NewTemplateManager()

	base := tm.ListByCategory(CategoryBase)
	if len(base) < 4 {
		t.Errorf("基础模板数量不足，got %d", len(base))
	}

	web := tm.ListByCategory(CategoryWeb)
	if len(web) < 1 {
		t.Errorf("Web 模板数量不足，got %d", len(web))
	}

	db := tm.ListByCategory(CategoryDatabase)
	if len(db) < 2 {
		t.Errorf("数据库模板数量不足，got %d", len(db))
	}
}

func TestTemplateManagerListByDistro(t *testing.T) {
	tm := NewTemplateManager()

	ubuntus := tm.ListByDistro("ubuntu")
	if len(ubuntus) < 2 {
		t.Errorf("ubuntu 模板数量不足，got %d", len(ubuntus))
	}

	alpines := tm.ListByDistro("alpine")
	if len(alpines) < 1 {
		t.Errorf("alpine 模板数量不足，got %d", len(alpines))
	}
}

func TestTemplateManagerDelete(t *testing.T) {
	tm := NewTemplateManager()

	// 注册自定义模板
	tm.Register(&Template{Name: "to-delete", Distro: "test", Version: "1.0"})

	// 删除自定义模板
	err := tm.Delete("to-delete")
	if err != nil {
		t.Fatalf("删除模板失败: %v", err)
	}
	if tm.Exists("to-delete") {
		t.Error("模板应已被删除")
	}

	// 删除内置模板应失败
	err = tm.Delete("ubuntu-24.04")
	if err == nil {
		t.Error("删除内置模板应返回错误")
	}

	// 删除不存在的模板
	err = tm.Delete("nonexistent")
	if err == nil {
		t.Error("删除不存在的模板应返回错误")
	}
}

func TestTemplateManagerUpdate(t *testing.T) {
	tm := NewTemplateManager()

	// 注册自定义模板
	tm.Register(&Template{Name: "to-update", Distro: "test", Version: "1.0", Description: "original"})

	// 更新自定义模板
	err := tm.Update("to-update", &Template{Distro: "test", Version: "2.0", Description: "updated"})
	if err != nil {
		t.Fatalf("更新模板失败: %v", err)
	}

	tpl, _ := tm.Get("to-update")
	if tpl.Description != "updated" {
		t.Errorf("期望 description=updated, got %s", tpl.Description)
	}

	// 更新内置模板应失败
	err = tm.Update("ubuntu-24.04", &Template{Distro: "x"})
	if err == nil {
		t.Error("更新内置模板应返回错误")
	}

	// 更新不存在的模板
	err = tm.Update("nonexistent", &Template{Distro: "x"})
	if err == nil {
		t.Error("更新不存在的模板应返回错误")
	}
}

func TestTemplateManagerGetDefaultResources(t *testing.T) {
	tm := NewTemplateManager()

	res, err := tm.GetDefaultResources("ubuntu-24.04")
	if err != nil {
		t.Fatalf("获取默认资源失败: %v", err)
	}
	if res.CPUCores != 1 {
		t.Errorf("期望 cpu=1, got %d", res.CPUCores)
	}
	if res.MemoryMB != 512 {
		t.Errorf("期望 memory=512, got %d", res.MemoryMB)
	}

	// 不存在的模板
	_, err = tm.GetDefaultResources("nonexistent")
	if err == nil {
		t.Error("获取不存在模板的资源应返回错误")
	}
}

// ===== Manager 生命周期测试 =====

func TestManagerCreateContainer(t *testing.T) {
	mgr := newTestManager(t)

	c, err := mgr.CreateContainer(CreateRequest{
		Name:      "test-ct",
		Template:  "ubuntu-24.04",
		Resources: ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 20},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	if c.Status != StatusCreated {
		t.Errorf("新容器应为 created, got %s", c.Status)
	}
	if c.Hostname != "test-ct" {
		t.Errorf("默认 hostname 应为容器名, got %s", c.Hostname)
	}
	if c.Resources.CPUCores != 2 {
		t.Errorf("期望 cpu=2, got %d", c.Resources.CPUCores)
	}
}

func TestManagerCreateContainerDefaults(t *testing.T) {
	mgr := newTestManager(t)

	c, err := mgr.CreateContainer(CreateRequest{
		Name:     "defaults",
		Template: "alpine-3.20",
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	// alpine-3.20 默认：CPU=1, Memory=128, Disk=2
	if c.Resources.CPUCores != 1 {
		t.Errorf("期望 cpu=1, got %d", c.Resources.CPUCores)
	}
	if c.Resources.MemoryMB != 128 {
		t.Errorf("期望 memory=128, got %d", c.Resources.MemoryMB)
	}
	if c.Resources.DiskGB != 2 {
		t.Errorf("期望 disk=2, got %d", c.Resources.DiskGB)
	}
}

func TestManagerCreateContainerValidation(t *testing.T) {
	mgr := newTestManager(t)

	// 无效模板
	_, err := mgr.CreateContainer(CreateRequest{Name: "bad", Template: "nonexistent"})
	if err == nil {
		t.Error("不存在的模板应返回错误")
	}

	// 重复名称
	mgr.CreateContainer(CreateRequest{Name: "dup", Template: "alpine-3.20"})
	_, err = mgr.CreateContainer(CreateRequest{Name: "dup", Template: "alpine-3.20"})
	if err == nil {
		t.Error("重复名称应返回错误")
	}
}

func TestManagerCreateContainerMaxLimit(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StoragePath = filepath.Join(tmpDir, "lxc")
	cfg.SnapshotPath = filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(cfg.StoragePath, 0755)
	os.MkdirAll(cfg.SnapshotPath, 0755)
	cfg.MaxContainers = 2
	mgr := NewManager(cfg)

	mgr.CreateContainer(CreateRequest{Name: "c1", Template: "alpine-3.20"})
	mgr.CreateContainer(CreateRequest{Name: "c2", Template: "alpine-3.20"})
	_, err := mgr.CreateContainer(CreateRequest{Name: "c3", Template: "alpine-3.20"})
	if err == nil {
		t.Error("超过最大容器数应返回错误")
	}
}

func TestManagerGetContainer(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{Name: "get-test", Template: "alpine-3.20"})

	got, err := mgr.GetContainer(c.ID)
	if err != nil {
		t.Fatalf("获取容器失败: %v", err)
	}
	if got.ID != c.ID {
		t.Errorf("ID 不匹配: %s vs %s", got.ID, c.ID)
	}

	// 不存在
	_, err = mgr.GetContainer("nonexistent")
	if err == nil {
		t.Error("获取不存在容器应返回错误")
	}
}

func TestManagerListContainers(t *testing.T) {
	mgr := newTestManager(t)

	if len(mgr.ListContainers()) != 0 {
		t.Error("初始容器列表应为空")
	}

	mgr.CreateContainer(CreateRequest{Name: "l1", Template: "alpine-3.20"})
	mgr.CreateContainer(CreateRequest{Name: "l2", Template: "alpine-3.20"})

	if len(mgr.ListContainers()) != 2 {
		t.Errorf("期望 2 个容器, got %d", len(mgr.ListContainers()))
	}
}

func TestManagerListByStatus(t *testing.T) {
	mgr := newTestManager(t)

	mgr.CreateContainer(CreateRequest{Name: "s1", Template: "alpine-3.20"})
	mgr.CreateContainer(CreateRequest{Name: "s2", Template: "alpine-3.20"})

	created := mgr.ListByStatus(StatusCreated)
	if len(created) != 2 {
		t.Errorf("期望 2 个 created 容器, got %d", len(created))
	}

	running := mgr.ListByStatus(StatusRunning)
	if len(running) != 0 {
		t.Errorf("期望 0 个 running 容器, got %d", len(running))
	}
}

func TestManagerDeleteContainer(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{Name: "del-test", Template: "alpine-3.20"})

	err := mgr.DeleteContainer(c.ID)
	if err != nil {
		t.Fatalf("删除容器失败: %v", err)
	}

	_, err = mgr.GetContainer(c.ID)
	if err == nil {
		t.Error("获取已删除容器应返回错误")
	}

	// 删除不存在
	err = mgr.DeleteContainer("nonexistent")
	if err == nil {
		t.Error("删除不存在容器应返回错误")
	}
}

func TestManagerUpdateResources(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:      "res-update",
		Template:  "alpine-3.20",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 256},
	})

	err := mgr.UpdateResources(c.ID, ResourceLimit{CPUCores: 4, MemoryMB: 2048})
	if err != nil {
		t.Fatalf("更新资源失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if got.Resources.CPUCores != 4 {
		t.Errorf("期望 cpu=4, got %d", got.Resources.CPUCores)
	}
	if got.Resources.MemoryMB != 2048 {
		t.Errorf("期望 memory=2048, got %d", got.Resources.MemoryMB)
	}

	// 无效资源
	err = mgr.UpdateResources(c.ID, ResourceLimit{CPUCores: 1, MemoryMB: 0})
	if err == nil {
		t.Error("内存为 0 应返回错误")
	}
}

func TestManagerContainerCount(t *testing.T) {
	mgr := newTestManager(t)

	if mgr.ContainerCount() != 0 {
		t.Error("初始容器数应为 0")
	}

	mgr.CreateContainer(CreateRequest{Name: "cnt1", Template: "alpine-3.20"})
	mgr.CreateContainer(CreateRequest{Name: "cnt2", Template: "alpine-3.20"})

	if mgr.ContainerCount() != 2 {
		t.Errorf("期望 2 个容器, got %d", mgr.ContainerCount())
	}
}

func TestManagerGetStatusSummary(t *testing.T) {
	mgr := newTestManager(t)
	mgr.CreateContainer(CreateRequest{Name: "sum1", Template: "alpine-3.20"})
	mgr.CreateContainer(CreateRequest{Name: "sum2", Template: "alpine-3.20"})

	summary := mgr.GetStatusSummary()
	if summary.TotalContainers != 2 {
		t.Errorf("期望 total=2, got %d", summary.TotalContainers)
	}
	if summary.Stopped != 2 {
		t.Errorf("期望 stopped=2, got %d", summary.Stopped)
	}
	if summary.MaxContainers != 100 {
		t.Errorf("期望 max=100, got %d", summary.MaxContainers)
	}
}

// ===== 快照测试 =====

func TestManagerCreateSnapshot(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "snap-test",
		Template: "alpine-3.20",
	})

	snap, err := mgr.CreateSnapshot(c.ID, "v1.0", "初始快照")
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}
	if snap.ContainerID != c.ID {
		t.Errorf("快照容器ID不匹配: %s vs %s", snap.ContainerID, c.ID)
	}
	if snap.Name != "v1.0" {
		t.Errorf("快照名不匹配: %s", snap.Name)
	}
}

func TestManagerListSnapshots(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "snap-list",
		Template: "alpine-3.20",
	})

	mgr.CreateSnapshot(c.ID, "snap1", "first")
	mgr.CreateSnapshot(c.ID, "snap2", "second")

	snaps, err := mgr.ListSnapshots(c.ID)
	if err != nil {
		t.Fatalf("列出快照失败: %v", err)
	}
	if len(snaps) != 2 {
		t.Errorf("期望 2 个快照, got %d", len(snaps))
	}
}

func TestManagerDeleteSnapshot(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "snap-del",
		Template: "alpine-3.20",
	})

	snap, _ := mgr.CreateSnapshot(c.ID, "to-del", "to delete")

	err := mgr.DeleteSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("删除快照失败: %v", err)
	}

	snaps, _ := mgr.ListSnapshots(c.ID)
	if len(snaps) != 0 {
		t.Errorf("期望 0 个快照, got %d", len(snaps))
	}

	// 删除不存在
	err = mgr.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Error("删除不存在快照应返回错误")
	}
}

func TestManagerSnapshotOnNonexistentContainer(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.CreateSnapshot("nonexistent", "snap", "desc")
	if err == nil {
		t.Error("对不存在容器创建快照应返回错误")
	}
}

// ===== 存储卷管理测试 =====

func TestManagerAddVolume(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "vol-test",
		Template: "alpine-3.20",
	})

	err := mgr.AddVolume(c.ID, VolumeMount{
		Source:      "/data",
		Destination: "/mnt/data",
	})
	if err != nil {
		t.Fatalf("添加卷失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if len(got.Volumes) != 1 {
		t.Errorf("期望 1 个卷, got %d", len(got.Volumes))
	}

	// 重复挂载点
	err = mgr.AddVolume(c.ID, VolumeMount{
		Source:      "/other",
		Destination: "/mnt/data",
	})
	if err == nil {
		t.Error("重复挂载点应返回错误")
	}

	// 空路径
	err = mgr.AddVolume(c.ID, VolumeMount{})
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

func TestManagerRemoveVolume(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "vol-remove",
		Template: "alpine-3.20",
	})

	mgr.AddVolume(c.ID, VolumeMount{Source: "/data", Destination: "/mnt/data"})

	err := mgr.RemoveVolume(c.ID, "/mnt/data")
	if err != nil {
		t.Fatalf("移除卷失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if len(got.Volumes) != 0 {
		t.Errorf("期望 0 个卷, got %d", len(got.Volumes))
	}

	// 不存在的挂载点
	err = mgr.RemoveVolume(c.ID, "/nonexistent")
	if err == nil {
		t.Error("移除不存在挂载点应返回错误")
	}
}

// ===== 高可用测试 =====

func TestManagerEnableHA(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "ha-test",
		Template: "alpine-3.20",
	})

	err := mgr.EnableHA(c.ID, HAConfig{
		FailoverNode: "node-2",
		AutoRestart:  true,
		MaxRestarts:  3,
	})
	if err != nil {
		t.Fatalf("启用HA失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if got.HAConfig == nil || !got.HAConfig.Enabled {
		t.Error("HA 应已启用")
	}
	if got.HAConfig.FailoverNode != "node-2" {
		t.Errorf("期望 failoverNode=node-2, got %s", got.HAConfig.FailoverNode)
	}
}

func TestManagerDisableHA(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "ha-disable",
		Template: "alpine-3.20",
	})

	mgr.EnableHA(c.ID, HAConfig{AutoRestart: true})
	mgr.DisableHA(c.ID)

	got, _ := mgr.GetContainer(c.ID)
	if got.HAConfig != nil {
		t.Error("HA 应已禁用")
	}
}

// ===== 网络管理测试 =====

func TestManagerAddPortMapping(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "port-test",
		Template: "alpine-3.20",
	})

	err := mgr.AddPortMapping(c.ID, PortMap{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
	})
	if err != nil {
		t.Fatalf("添加端口映射失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if len(got.Network.Ports) != 1 {
		t.Errorf("期望 1 个端口映射, got %d", len(got.Network.Ports))
	}

	// 重复端口
	err = mgr.AddPortMapping(c.ID, PortMap{
		HostPort:      8080,
		ContainerPort: 8080,
		Protocol:      "tcp",
	})
	if err == nil {
		t.Error("重复端口应返回错误")
	}
}

func TestManagerRemovePortMapping(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "port-remove",
		Template: "alpine-3.20",
	})

	mgr.AddPortMapping(c.ID, PortMap{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"})

	err := mgr.RemovePortMapping(c.ID, 8080, "tcp")
	if err != nil {
		t.Fatalf("移除端口映射失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if len(got.Network.Ports) != 0 {
		t.Errorf("期望 0 个端口映射, got %d", len(got.Network.Ports))
	}

	// 不存在
	err = mgr.RemovePortMapping(c.ID, 9999, "tcp")
	if err == nil {
		t.Error("移除不存在端口应返回错误")
	}
}

func TestManagerIsolateNetwork(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.CreateContainer(CreateRequest{
		Name:     "iso-test",
		Template: "alpine-3.20",
	})

	err := mgr.CreateIsolatedNetwork(c.ID)
	if err != nil {
		t.Fatalf("创建隔离网络失败: %v", err)
	}

	got, _ := mgr.GetContainer(c.ID)
	if !got.Network.Isolated {
		t.Error("网络隔离应已启用")
	}
	if got.Network.Mode != NetworkIsolated {
		t.Errorf("期望 mode=isolated, got %s", got.Network.Mode)
	}
}

// ===== 批量操作测试 =====

func TestManagerBatchOperations(t *testing.T) {
	mgr := newTestManager(t)
	c1, _ := mgr.CreateContainer(CreateRequest{Name: "b1", Template: "alpine-3.20"})
	c2, _ := mgr.CreateContainer(CreateRequest{Name: "b2", Template: "alpine-3.20"})

	ids := []string{c1.ID, c2.ID}

	// 批量停止（created 状态不允许停止，应有错误）
	result := mgr.BatchStop(ids)
	for _, err := range result {
		if err == nil {
			t.Error("created 状态批量停止应报错")
		}
	}

	// 批量删除（created 状态允许删除）
	result = mgr.BatchDelete(ids)
	for id, err := range result {
		if err != nil {
			t.Errorf("批量删除 %s 不应报错: %v", id, err)
		}
	}

	if mgr.ContainerCount() != 0 {
		t.Errorf("批量删除后期望 0 个容器, got %d", mgr.ContainerCount())
	}
}

// ===== 集成测试 =====

func TestManagerFullLifecycle(t *testing.T) {
	mgr := newTestManager(t)

	// 1. 创建容器
	c, err := mgr.CreateContainer(CreateRequest{
		Name:     "full-lifecycle",
		Template: "alpine-3.20",
		Resources: ResourceLimit{
			CPUCores:   2,
			MemoryMB:   512,
			DiskGB:     10,
			ProcessMax: 100,
		},
		Network: NetworkConfig{
			Mode:      NetworkBridge,
			BridgeName: "lxcbr0",
		},
		Volumes: []VolumeMount{
			{Source: "/data", Destination: "/mnt/data"},
		},
		Tags: map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	if c.Status != StatusCreated {
		t.Errorf("期望 status=created, got %s", c.Status)
	}

	// 2. 验证属性
	if c.Resources.CPUCores != 2 {
		t.Errorf("期望 cpu=2, got %d", c.Resources.CPUCores)
	}
	if len(c.Volumes) != 1 {
		t.Errorf("期望 1 个卷, got %d", len(c.Volumes))
	}
	if c.Tags["env"] != "test" {
		t.Errorf("期望 tag env=test, got %s", c.Tags["env"])
	}

	// 3. 添加快照
	snap, err := mgr.CreateSnapshot(c.ID, "initial", "初始快照")
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}
	if snap.Name != "initial" {
		t.Errorf("快照名应为 initial, got %s", snap.Name)
	}

	// 4. 添加端口映射
	err = mgr.AddPortMapping(c.ID, PortMap{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"})
	if err != nil {
		t.Fatalf("添加端口映射失败: %v", err)
	}

	// 5. 启用 HA
	err = mgr.EnableHA(c.ID, HAConfig{
		AutoRestart: true,
		MaxRestarts: 3,
	})
	if err != nil {
		t.Fatalf("启用HA失败: %v", err)
	}

	// 6. 验证最终状态
	final, _ := mgr.GetContainer(c.ID)
	if final.HAConfig == nil || !final.HAConfig.Enabled {
		t.Error("HA 应已启用")
	}
	if len(final.Snapshots) != 1 {
		t.Errorf("期望 1 个快照, got %d", len(final.Snapshots))
	}
	if len(final.Network.Ports) != 1 {
		t.Errorf("期望 1 个端口映射, got %d", len(final.Network.Ports))
	}

	// 7. 删除容器
	err = mgr.DeleteContainer(c.ID)
	if err != nil {
		t.Fatalf("删除容器失败: %v", err)
	}
}

func TestManagerTemplateManager(t *testing.T) {
	mgr := newTestManager(t)
	tm := mgr.TemplateManager()

	if tm == nil {
		t.Fatal("TemplateManager 不应为 nil")
	}
	if tm.Count() < 10 {
		t.Errorf("模板数量不足，got %d", tm.Count())
	}
}

// ===== Benchmark =====

func BenchmarkCreateContainer(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StoragePath = filepath.Join(tmpDir, "lxc")
	cfg.SnapshotPath = filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(cfg.StoragePath, 0755)
	os.MkdirAll(cfg.SnapshotPath, 0755)
	mgr := NewManager(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateContainer(CreateRequest{
			Name:     "bench-ct",
			Template: "alpine-3.20",
			Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128},
		})
	}
}

func BenchmarkListContainers(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StoragePath = filepath.Join(tmpDir, "lxc")
	cfg.SnapshotPath = filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(cfg.StoragePath, 0755)
	os.MkdirAll(cfg.SnapshotPath, 0755)
	mgr := NewManager(cfg)
	for i := 0; i < 100; i++ {
		mgr.CreateContainer(CreateRequest{
			Name:     "bench-list",
			Template: "alpine-3.20",
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ListContainers()
	}
}
