package rsync

import "testing"

func TestNewManager(t *testing.T) {
	cfg := RsyncConfig{
		MaxConcurrent: 3,
		Compress:      true,
		Archive:       true,
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.MaxConcurrent != 3 {
		t.Errorf("期望 MaxConcurrent=3, 实际 %d", m.config.MaxConcurrent)
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := RsyncConfig{}
	m := NewManager(cfg)
	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !m.running {
		t.Error("期望 running=true")
	}
	m.Stop()
	if m.running {
		t.Error("期望 running=false")
	}
}

func TestManager_TargetLifecycle(t *testing.T) {
	cfg := RsyncConfig{}
	m := NewManager(cfg)

	// 添加目标
	target := &RsyncTarget{
		ID:          "t1",
		Name:        "备份到NAS",
		Source:      "/data/",
		Destination: "nas:/backup/",
		Type:        TargetTypeSSH,
		Host:        "nas.local",
		Port:        22,
	}
	if err := m.AddTarget(target); err != nil {
		t.Fatalf("AddTarget 失败: %v", err)
	}

	// 获取目标
	got, err := m.GetTarget("t1")
	if err != nil {
		t.Fatalf("GetTarget 失败: %v", err)
	}
	if got.Name != "备份到NAS" {
		t.Errorf("期望 name=备份到NAS, 实际 %s", got.Name)
	}

	// 列表
	targets := m.ListTargets()
	if len(targets) != 1 {
		t.Errorf("期望 1 个目标, 实际 %d", len(targets))
	}

	// 重复添加
	if err := m.AddTarget(target); err == nil {
		t.Error("重复添加应报错")
	}

	// 移除
	if err := m.RemoveTarget("t1"); err != nil {
		t.Fatalf("RemoveTarget 失败: %v", err)
	}
}

func TestManager_JobLifecycle(t *testing.T) {
	cfg := RsyncConfig{}
	m := NewManager(cfg)
	m.AddTarget(&RsyncTarget{ID: "t1", Name: "test", Source: "/src/", Destination: "/dst/"})

	// 创建任务
	job := &RsyncJob{TargetID: "t1", Name: "每日备份"}
	if err := m.CreateJob(job); err != nil {
		t.Fatalf("CreateJob 失败: %v", err)
	}
	if job.ID == "" {
		t.Error("任务 ID 不应为空")
	}

	// 执行任务
	result, err := m.RunJob(job.ID)
	if err != nil {
		t.Fatalf("RunJob 失败: %v", err)
	}
	if result.FilesTransferred != 150 {
		t.Errorf("期望传输 150 文件, 实际 %d", result.FilesTransferred)
	}

	// 检查历史
	history := m.GetHistory(10)
	if len(history) != 1 {
		t.Errorf("期望 1 条历史记录, 实际 %d", len(history))
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := RsyncConfig{}
	m := NewManager(cfg)
	m.AddTarget(&RsyncTarget{ID: "t1", Name: "test", Source: "/src/", Destination: "/dst/"})
	job := &RsyncJob{TargetID: "t1", Name: "test"}
	m.CreateJob(job)
	m.RunJob(job.ID)

	stats := m.GetStats()
	if stats.TotalTargets != 1 {
		t.Errorf("期望 1 个目标, 实际 %d", stats.TotalTargets)
	}
	if stats.CompletedJobs != 1 {
		t.Errorf("期望 1 个完成任务, 实际 %d", stats.CompletedJobs)
	}
}
