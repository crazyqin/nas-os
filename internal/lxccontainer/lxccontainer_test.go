package lxccontainer

import (
	"fmt"
	"testing"
)

// ===== 状态机测试 =====

func TestValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{"stopped->starting", StatusStopped, StatusStarting, true},
		{"starting->running", StatusStarting, StatusRunning, true},
		{"running->stopped", StatusRunning, StatusStopped, true},
		{"running->rebooting", StatusRunning, StatusRebooting, true},
		{"running->deleting", StatusRunning, StatusDeleting, true},
		{"stopped->deleting", StatusStopped, StatusDeleting, true},
		{"failed->starting", StatusFailed, StatusStarting, true},
		{"running->starting", StatusRunning, StatusStarting, false},
		{"stopped->running", StatusStopped, StatusRunning, true},
		{"deleting->running", StatusDeleting, StatusRunning, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("ValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// ===== 资源限制验证测试 =====

func TestValidateResources(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResources(tt.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ===== 模板管理测试 =====

func TestTemplateManager(t *testing.T) {
	tm := NewTemplateManager()

	// 默认模板数量
	if tm.Count() < 3 {
		t.Errorf("默认模板数量不足，got %d", tm.Count())
	}

	// 获取模板
	tpl, err := tm.Get("ubuntu-22.04")
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}
	if tpl.Distro != "ubuntu" {
		t.Errorf("期望 distro=ubuntu, got %s", tpl.Distro)
	}

	// 注册新模板
	err = tm.Register(&Template{
		Name:    "custom-1",
		Distro:  "custom",
		Version: "1.0",
	})
	if err != nil {
		t.Fatalf("注册模板失败: %v", err)
	}

	// 重复注册
	err = tm.Register(&Template{Name: "custom-1", Distro: "x"})
	if err == nil {
		t.Error("重复注册应返回错误")
	}

	// 按发行版过滤
	ubuntus := tm.ListByDistro("ubuntu")
	if len(ubuntus) < 2 {
		t.Errorf("ubuntu 模板数量不足，got %d", len(ubuntus))
	}

	// 删除模板
	err = tm.Delete("custom-1")
	if err != nil {
		t.Fatalf("删除模板失败: %v", err)
	}
	if tm.Exists("custom-1") {
		t.Error("模板应已被删除")
	}

	// 删除不存在的模板
	err = tm.Delete("nonexistent")
	if err == nil {
		t.Error("删除不存在的模板应返回错误")
	}

	// 注册空名称
	err = tm.Register(&Template{})
	if err == nil {
		t.Error("空名称注册应返回错误")
	}
}

// ===== 网络管理测试 =====

func TestNetworkManager(t *testing.T) {
	nm := NewNetworkManager()

	// 创建网桥
	bridge, err := nm.CreateBridge("lxcbr0", "10.0.0.0/24", "10.0.0.1")
	if err != nil {
		t.Fatalf("创建网桥失败: %v", err)
	}
	if bridge.Name != "lxcbr0" {
		t.Errorf("期望 name=lxcbr0, got %s", bridge.Name)
	}

	// 重复创建
	_, err = nm.CreateBridge("lxcbr0", "10.0.0.0/24", "10.0.0.1")
	if err == nil {
		t.Error("重复创建网桥应返回错误")
	}

	// 分配 IP
	ip, err := nm.AllocateIP("lxcbr0")
	if err != nil {
		t.Fatalf("分配 IP 失败: %v", err)
	}
	if ip == "" {
		t.Error("分配的 IP 不应为空")
	}
	if ip == "10.0.0.1" {
		t.Error("不应分配网关地址")
	}

	// 释放 IP
	err = nm.ReleaseIP("lxcbr0", ip)
	if err != nil {
		t.Fatalf("释放 IP 失败: %v", err)
	}

	// 释放不存在的 IP
	err = nm.ReleaseIP("lxcbr0", "10.0.0.99")
	if err == nil {
		t.Error("释放未分配的 IP 应返回错误")
	}

	// 列出网桥
	bridges := nm.ListBridges()
	if len(bridges) != 1 {
		t.Errorf("期望 1 个网桥, got %d", len(bridges))
	}

	// 删除网桥
	err = nm.DeleteBridge("lxcbr0")
	if err != nil {
		t.Fatalf("删除网桥失败: %v", err)
	}

	// 删除不存在的网桥
	err = nm.DeleteBridge("nonexistent")
	if err == nil {
		t.Error("删除不存在的网桥应返回错误")
	}
}

func TestNetworkManagerIPExhaustion(t *testing.T) {
	nm := NewNetworkManager()
	// /30 网段有 4 个地址 (.0 网络, .1 网关, .2, .3 广播)
	// generateIPPool 生成 [.1, .2, .3]，排除网关后剩余 [.2, .3]
	_, err := nm.CreateBridge("tiny", "192.168.100.0/30", "192.168.100.1")
	if err != nil {
		t.Fatalf("创建网桥失败: %v", err)
	}

	// 分配所有可用 IP
	ip1, err := nm.AllocateIP("tiny")
	if err != nil {
		t.Fatalf("分配 IP1 失败: %v", err)
	}
	if ip1 == "" {
		t.Error("IP1 不应为空")
	}

	ip2, err := nm.AllocateIP("tiny")
	if err != nil {
		t.Fatalf("分配 IP2 失败: %v", err)
	}
	if ip2 == "" {
		t.Error("IP2 不应为空")
	}

	// 应该没有可用 IP 了
	_, err = nm.AllocateIP("tiny")
	if err == nil {
		t.Error("IP 耗尽应返回错误")
	}
}

func TestNetworkManagerInUse(t *testing.T) {
	nm := NewNetworkManager()
	nm.CreateBridge("br-inuse", "10.10.0.0/24", "10.10.0.1")
	ip, _ := nm.AllocateIP("br-inuse")

	// 有 IP 在使用时不能删除网桥
	err := nm.DeleteBridge("br-inuse")
	if err == nil {
		t.Error("有 IP 使用中的网桥不应被删除")
	}

	// 释放后可删除
	nm.ReleaseIP("br-inuse", ip)
	err = nm.DeleteBridge("br-inuse")
	if err != nil {
		t.Errorf("释放后删除网桥失败: %v", err)
	}
}

func TestValidateNetworkConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     NetworkConfig
		wantErr bool
	}{
		{"bridge valid", NetworkConfig{Mode: NetworkModeBridge, Bridge: "br0"}, false},
		{"bridge no bridge name", NetworkConfig{Mode: NetworkModeBridge}, true},
		{"nat valid", NetworkConfig{Mode: NetworkModeNAT, Bridge: "virbr0"}, false},
		{"static valid", NetworkConfig{Mode: NetworkModeStatic, IPAddress: "10.0.0.5"}, false},
		{"static no ip", NetworkConfig{Mode: NetworkModeStatic}, true},
		{"static bad ip", NetworkConfig{Mode: NetworkModeStatic, IPAddress: "bad"}, true},
		{"static bad gateway", NetworkConfig{Mode: NetworkModeStatic, IPAddress: "10.0.0.5", Gateway: "bad"}, true},
		{"static bad dns", NetworkConfig{Mode: NetworkModeStatic, IPAddress: "10.0.0.5", DNS: []string{"bad"}}, true},
		{"none", NetworkConfig{Mode: NetworkModeNone}, false},
		{"invalid mode", NetworkConfig{Mode: "invalid"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetworkConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNetworkConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ===== 容器生命周期测试 =====

func TestContainerLifecycle(t *testing.T) {
	cm := NewContainerManager()

	// 创建
	c, err := cm.Create(CreateRequest{
		Name:      "test-ct",
		Template:  "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 20},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	if c.Status != StatusStopped {
		t.Errorf("新容器应为 stopped, got %s", c.Status)
	}
	if c.Hostname != "test-ct" {
		t.Errorf("默认 hostname 应为容器名, got %s", c.Hostname)
	}

	// 启动
	err = cm.Start(c.ID)
	if err != nil {
		t.Fatalf("启动容器失败: %v", err)
	}

	// 获取统计
	stats, err := cm.GetStats(c.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats.MemoryLimit != 1024 {
		t.Errorf("内存限制应为 1024, got %d", stats.MemoryLimit)
	}

	// 重启
	err = cm.Restart(c.ID)
	if err != nil {
		t.Fatalf("重启容器失败: %v", err)
	}

	// 停止
	err = cm.Stop(c.ID)
	if err != nil {
		t.Fatalf("停止容器失败: %v", err)
	}

	// 删除
	err = cm.Delete(c.ID)
	if err != nil {
		t.Fatalf("删除容器失败: %v", err)
	}

	// 不存在
	_, err = cm.Get(c.ID)
	if err == nil {
		t.Error("获取已删除容器应返回错误")
	}
}

func TestContainerCreateDefaults(t *testing.T) {
	cm := NewContainerManager()
	c, err := cm.Create(CreateRequest{
		Name:     "defaults-test",
		Template: "alpine-3.19",
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if c.Resources.CPUCores != 1 {
		t.Errorf("默认 CPU 应为 1, got %d", c.Resources.CPUCores)
	}
	if c.Resources.MemoryMB != 512 {
		t.Errorf("默认内存应为 512, got %d", c.Resources.MemoryMB)
	}
	if c.Resources.DiskGB != 10 {
		t.Errorf("默认磁盘应为 10, got %d", c.Resources.DiskGB)
	}
}

func TestContainerCreateValidation(t *testing.T) {
	cm := NewContainerManager()

	// 空名称
	_, err := cm.Create(CreateRequest{Template: "ubuntu-22.04"})
	if err == nil {
		t.Error("空名称应返回错误")
	}

	// 空模板
	_, err = cm.Create(CreateRequest{Name: "x"})
	if err == nil {
		t.Error("空模板应返回错误")
	}
}

func TestContainerDuplicate(t *testing.T) {
	cm := NewContainerManager()
	cm.Create(CreateRequest{Name: "dup", Template: "ubuntu-22.04"})
	_, err := cm.Create(CreateRequest{Name: "dup", Template: "ubuntu-22.04"})
	if err == nil {
		t.Error("重复创建应返回错误")
	}
}

func TestContainerStartInvalidState(t *testing.T) {
	cm := NewContainerManager()
	c, _ := cm.Create(CreateRequest{Name: "inv", Template: "ubuntu-22.04"})
	cm.Start(c.ID)

	// 运行中不能再次启动
	err := cm.Start(c.ID)
	if err == nil {
		t.Error("运行中启动应返回错误")
	}
}

func TestContainerStopInvalidState(t *testing.T) {
	cm := NewContainerManager()
	c, _ := cm.Create(CreateRequest{Name: "stop-inv", Template: "ubuntu-22.04"})

	// 已停止不能再次停止
	err := cm.Stop(c.ID)
	if err == nil {
		t.Error("已停止容器再次停止应返回错误")
	}
}

func TestContainerRestartNotRunning(t *testing.T) {
	cm := NewContainerManager()
	c, _ := cm.Create(CreateRequest{Name: "restart-inv", Template: "ubuntu-22.04"})

	err := cm.Restart(c.ID)
	if err == nil {
		t.Error("未运行容器重启应返回错误")
	}
}

func TestContainerListByStatus(t *testing.T) {
	cm := NewContainerManager()
	cm.Create(CreateRequest{Name: "s1", Template: "ubuntu-22.04"})
	c2, _ := cm.Create(CreateRequest{Name: "s2", Template: "ubuntu-22.04"})
	err := cm.Start(c2.ID)
	if err != nil {
		t.Fatalf("启动容器失败: %v", err)
	}

	stopped := cm.ListByStatus(StatusStopped)
	running := cm.ListByStatus(StatusRunning)
	if len(stopped) != 1 {
		t.Errorf("期望 1 个停止容器, got %d", len(stopped))
	}
	if len(running) != 1 {
		t.Errorf("期望 1 个运行容器, got %d", len(running))
	}
}

func TestContainerCount(t *testing.T) {
	cm := NewContainerManager()
	if cm.Count() != 0 {
		t.Error("初始容器数应为 0")
	}
	cm.Create(CreateRequest{Name: "cnt1", Template: "ubuntu-22.04"})
	cm.Create(CreateRequest{Name: "cnt2", Template: "ubuntu-22.04"})
	if cm.Count() != 2 {
		t.Errorf("期望 2 个容器, got %d", cm.Count())
	}
}

func TestContainerUpdateResources(t *testing.T) {
	cm := NewContainerManager()
	c, _ := cm.Create(CreateRequest{Name: "res", Template: "ubuntu-22.04"})

	err := cm.UpdateResources(c.ID, ResourceLimit{CPUCores: 4, MemoryMB: 2048})
	if err != nil {
		t.Fatalf("更新资源失败: %v", err)
	}

	ct, _ := cm.Get(c.ID)
	if ct.Resources.CPUCores != 4 || ct.Resources.MemoryMB != 2048 {
		t.Error("资源未正确更新")
	}

	// 无效资源
	err = cm.UpdateResources(c.ID, ResourceLimit{CPUCores: 1, MemoryMB: 0})
	if err == nil {
		t.Error("内存为 0 应返回错误")
	}
}

func TestContainerAddVolume(t *testing.T) {
	cm := NewContainerManager()
	c, _ := cm.Create(CreateRequest{Name: "vol", Template: "ubuntu-22.04"})

	err := cm.AddVolume(c.ID, VolumeMount{Source: "/data", Destination: "/mnt/data"})
	if err != nil {
		t.Fatalf("添加卷失败: %v", err)
	}

	ct, _ := cm.Get(c.ID)
	if len(ct.Volumes) != 1 {
		t.Errorf("期望 1 个卷, got %d", len(ct.Volumes))
	}

	// 空路径
	err = cm.AddVolume(c.ID, VolumeMount{})
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

// ===== Manager 集成测试 =====

func TestManagerIntegration(t *testing.T) {
	mgr := NewManager()

	// 创建网桥
	_, err := mgr.CreateBridge("br-test", "172.16.0.0/24", "172.16.0.1")
	if err != nil {
		t.Fatalf("创建网桥失败: %v", err)
	}

	// 分配 IP
	ip, err := mgr.AllocateIP("br-test")
	if err != nil {
		t.Fatalf("分配 IP 失败: %v", err)
	}

	// 创建容器
	ct, err := mgr.CreateContainer(CreateRequest{
		Name:      "integration-test",
		Template:  "debian-12",
		Resources: ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 20},
		Network: NetworkConfig{
			Mode:      NetworkModeStatic,
			IPAddress: ip,
			Gateway:   "172.16.0.1",
		},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}

	// 启动
	err = mgr.StartContainer(ct.ID)
	if err != nil {
		t.Fatalf("启动容器失败: %v", err)
	}

	// 统计
	stats, err := mgr.GetStats(ct.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats.PIDs == 0 {
		t.Error("进程数不应为 0")
	}

	// 停止并删除
	mgr.StopContainer(ct.ID)
	mgr.DeleteContainer(ct.ID)

	// 释放 IP
	err = mgr.ReleaseIP("br-test", ip)
	if err != nil {
		t.Fatalf("释放 IP 失败: %v", err)
	}

	// 删除网桥
	err = mgr.DeleteBridge("br-test")
	if err != nil {
		t.Fatalf("删除网桥失败: %v", err)
	}

	// 确认清空
	if mgr.ContainerCount() != 0 {
		t.Error("容器应已清空")
	}
}

func TestManagerTemplateWorkflow(t *testing.T) {
	mgr := NewManager()

	// 列出默认模板
	templates := mgr.ListTemplates()
	if len(templates) < 5 {
		t.Errorf("默认模板不足 5 个, got %d", len(templates))
	}

	// 注册自定义模板
	err := mgr.RegisterTemplate(&Template{
		Name:    "custom-arch",
		Distro:  "archlinux",
		Version: "latest",
		SizeMB:  150,
	})
	if err != nil {
		t.Fatalf("注册模板失败: %v", err)
	}

	// 用自定义模板创建容器
	ct, err := mgr.CreateContainer(CreateRequest{
		Name:      "arch-ct",
		Template:  "custom-arch",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 256},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}
	if ct.Template != "custom-arch" {
		t.Errorf("模板名应为 custom-arch, got %s", ct.Template)
	}

	// 删除模板
	err = mgr.DeleteTemplate("custom-arch")
	if err != nil {
		t.Fatalf("删除模板失败: %v", err)
	}
}

func TestManagerCreateWithInvalidTemplate(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.CreateContainer(CreateRequest{
		Name:      "bad-tpl",
		Template:  "nonexistent-template",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 256},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})
	if err == nil {
		t.Error("不存在的模板应返回错误")
	}
}

func TestManagerCreateWithInvalidNetwork(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.CreateContainer(CreateRequest{
		Name:      "bad-net",
		Template:  "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 256},
		Network:   NetworkConfig{Mode: NetworkModeStatic}, // 缺少 IP
	})
	if err == nil {
		t.Error("无效网络配置应返回错误")
	}
}

// ===== 快照管理测试 =====

func TestSnapshotLifecycle(t *testing.T) {
	mgr := NewManager()

	// 创建容器
	ct, err := mgr.CreateContainer(CreateRequest{
		Name:      "snap-test",
		Template:  "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 256, DiskGB: 10},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})
	if err != nil {
		t.Fatalf("创建容器失败: %v", err)
	}

	// 创建快照
	snap, err := mgr.CreateSnapshot(SnapshotCreateRequest{
		ContainerID: ct.ID,
		Name:        "snap-1",
	})
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}
	if snap.Status != SnapshotReady {
		t.Errorf("快照应为 ready，实际 %s", snap.Status)
	}
	if snap.ContainerID != ct.ID {
		t.Errorf("快照容器 ID 不匹配")
	}

	// 列出快照
	snaps := mgr.ListSnapshots(ct.ID)
	if len(snaps) != 1 {
		t.Errorf("期望 1 个快照，实际 %d", len(snaps))
	}

	// 获取快照
	got, err := mgr.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("获取快照失败: %v", err)
	}
	if got.Name != "snap-1" {
		t.Errorf("快照名称应为 snap-1，实际 %s", got.Name)
	}

	// 恢复快照
	err = mgr.RestoreSnapshot(SnapshotRestoreRequest{
		SnapshotID:  snap.ID,
		ContainerID: ct.ID,
	})
	if err != nil {
		t.Fatalf("恢复快照失败: %v", err)
	}

	// 删除快照
	err = mgr.DeleteSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("删除快照失败: %v", err)
	}

	if mgr.ListSnapshots(ct.ID) != nil && len(mgr.ListSnapshots(ct.ID)) != 0 {
		t.Error("快照应已清空")
	}
}

func TestSnapshotDuplicateName(t *testing.T) {
	mgr := NewManager()
	ct, _ := mgr.CreateContainer(CreateRequest{
		Name: "dup-snap", Template: "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128, DiskGB: 5},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})

	_, err := mgr.CreateSnapshot(SnapshotCreateRequest{ContainerID: ct.ID, Name: "same"})
	if err != nil {
		t.Fatalf("第一次创建快照失败: %v", err)
	}
	_, err = mgr.CreateSnapshot(SnapshotCreateRequest{ContainerID: ct.ID, Name: "same"})
	if err == nil {
		t.Error("重复快照名应返回错误")
	}
}

func TestSnapshotRestoreNotFound(t *testing.T) {
	mgr := NewManager()
	ct, _ := mgr.CreateContainer(CreateRequest{
		Name: "no-snap", Template: "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128, DiskGB: 5},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})

	err := mgr.RestoreSnapshot(SnapshotRestoreRequest{
		SnapshotID: "nonexistent", ContainerID: ct.ID,
	})
	if err == nil {
		t.Error("恢复不存在的快照应返回错误")
	}
}

func TestSnapshotDeleteNonexistent(t *testing.T) {
	mgr := NewManager()
	err := mgr.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Error("删除不存在的快照应返回错误")
	}
}

func TestSnapshotContainerNotFound(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.CreateSnapshot(SnapshotCreateRequest{
		ContainerID: "nonexistent", Name: "x",
	})
	if err == nil {
		t.Error("为不存在的容器创建快照应返回错误")
	}
}

func TestSnapshotEmptyName(t *testing.T) {
	mgr := NewManager()
	ct, _ := mgr.CreateContainer(CreateRequest{
		Name: "no-name", Template: "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128, DiskGB: 5},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})
	_, err := mgr.CreateSnapshot(SnapshotCreateRequest{ContainerID: ct.ID})
	if err == nil {
		t.Error("空快照名应返回错误")
	}
}

func TestSnapshotMultiple(t *testing.T) {
	mgr := NewManager()
	ct, _ := mgr.CreateContainer(CreateRequest{
		Name: "multi-snap", Template: "ubuntu-22.04",
		Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128, DiskGB: 5},
		Network:   NetworkConfig{Mode: NetworkModeNone},
	})

	for i := 0; i < 5; i++ {
		_, err := mgr.CreateSnapshot(SnapshotCreateRequest{
			ContainerID: ct.ID, Name: fmt.Sprintf("snap-%d", i),
		})
		if err != nil {
			t.Fatalf("创建快照 %d 失败: %v", i, err)
		}
	}

	if len(mgr.ListSnapshots(ct.ID)) != 5 {
		t.Errorf("期望 5 个快照，实际 %d", len(mgr.ListSnapshots(ct.ID)))
	}
}

// ===== Benchmark =====

func BenchmarkCreateContainer(b *testing.B) {
	mgr := NewManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateContainer(CreateRequest{
			Name:      "bench-ct",
			Template:  "alpine-3.19",
			Resources: ResourceLimit{CPUCores: 1, MemoryMB: 128},
			Network:   NetworkConfig{Mode: NetworkModeNone},
		})
	}
}
