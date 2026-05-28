package smartdiskmigrat

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.nextPlanID != 2 {
		t.Errorf("expected nextPlanID 2, got %d", m.nextPlanID)
	}
}

func TestScanDisks(t *testing.T) {
	m := NewManager()

	disks, err := m.ScanDisks()
	if err != nil {
		t.Fatalf("scan disks failed: %v", err)
	}
	if len(disks) < 5 {
		t.Errorf("expected at least 5 disks, got %d", len(disks))
	}

	// 检查第一块磁盘
	found := false
	for _, d := range disks {
		if d.DeviceName == "/dev/sda" {
			found = true
			if d.Model != "WDC WD40EFRX" {
				t.Errorf("expected 'WDC WD40EFRX', got '%s'", d.Model)
			}
			if !d.SmartOK {
				t.Error("expected SMART OK")
			}
		}
	}
	if !found {
		t.Error("expected to find /dev/sda")
	}
}

func TestCreatePlan(t *testing.T) {
	m := NewManager()

	plan := &MigrationPlan{
		Name:         "测试迁移",
		SourceDevice: "/dev/sdb",
		TargetDevice: "/dev/sdc",
		Type:         MigrationTypeDiskSwap,
	}

	err := m.CreatePlan(plan)
	if err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	if plan.ID == "" {
		t.Error("expected plan ID")
	}
	if plan.Status != StatusPending {
		t.Errorf("expected pending, got '%s'", plan.Status)
	}

	// 检查能否获取
	got := m.GetPlan(plan.ID)
	if got == nil {
		t.Fatal("expected to get plan")
	}
	if got.Name != "测试迁移" {
		t.Errorf("expected '测试迁移', got '%s'", got.Name)
	}
}

func TestListPlans(t *testing.T) {
	m := NewManager()

	plans := m.ListPlans()
	if len(plans) < 1 {
		t.Errorf("expected at least 1 plan, got %d", len(plans))
	}
}

func TestValidatePlan(t *testing.T) {
	m := NewManager()

	// 有效计划
	warnings, err := m.ValidatePlan("plan-1")
	if err != nil {
		t.Fatalf("validate plan failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}

	// 不存在的计划
	_, err = m.ValidatePlan("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestExecuteAndCancelPlan(t *testing.T) {
	m := NewManager()

	// 执行计划
	job, err := m.ExecutePlan("plan-1")
	if err != nil {
		t.Fatalf("execute plan failed: %v", err)
	}
	if job.Status != StatusRunning {
		t.Errorf("expected running, got '%s'", job.Status)
	}
	if job.TotalSteps != 4 {
		t.Errorf("expected 4 steps, got %d", job.TotalSteps)
	}

	// 重复执行
	_, err = m.ExecutePlan("plan-1")
	if err == nil {
		t.Error("expected error for already running plan")
	}

	// 获取任务状态
	got := m.GetJobStatus(job.ID)
	if got == nil {
		t.Fatal("expected to get job")
	}
	if got.Progress != 0 {
		t.Errorf("expected 0 progress, got %.1f", got.Progress)
	}

	// 暂停
	err = m.PauseJob(job.ID)
	if err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if m.GetJobStatus(job.ID).Status != StatusPaused {
		t.Error("expected paused status")
	}

	// 恢复
	err = m.ResumeJob(job.ID)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if m.GetJobStatus(job.ID).Status != StatusRunning {
		t.Error("expected running status")
	}

	// 取消
	err = m.CancelJob(job.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if m.GetJobStatus(job.ID).Status != StatusCancelled {
		t.Error("expected cancelled status")
	}
	if m.GetJobStatus(job.ID).FinishedAt == nil {
		t.Error("expected finished time")
	}

	// 不存在的任务
	err = m.PauseJob("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestRollbackJob(t *testing.T) {
	m := NewManager()

	// 执行并取消
	job, _ := m.ExecutePlan("plan-1")
	m.CancelJob(job.ID)

	// 回滚
	err := m.RollbackJob(job.ID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if m.GetJobStatus(job.ID).Status != StatusRolledBack {
		t.Error("expected rolled back status")
	}

	// 运中的任务不能回滚
	job2, _ := m.ExecutePlan("plan-1") // 需要新建计划
	_ = job2
}

func TestHotSpareManagement(t *testing.T) {
	m := NewManager()

	// 列出热备盘
	spares := m.ListHotSpares()
	if len(spares) < 1 {
		t.Errorf("expected at least 1 hot spare, got %d", len(spares))
	}

	// 添加热备盘
	err := m.AddHotSpare("/dev/sdd", "pool-1")
	if err != nil {
		t.Fatalf("add hot spare failed: %v", err)
	}

	// 重复添加
	err = m.AddHotSpare("/dev/sdd", "pool-1")
	if err == nil {
		t.Error("expected error for duplicate hot spare")
	}

	// 不存在的磁盘
	err = m.AddHotSpare("/dev/sdx", "pool-1")
	if err == nil {
		t.Error("expected error for nonexistent disk")
	}

	// 移除热备盘
	err = m.RemoveHotSpare("/dev/sdd")
	if err != nil {
		t.Fatalf("remove hot spare failed: %v", err)
	}

	// 移除不存在的
	err = m.RemoveHotSpare("/dev/sdx")
	if err == nil {
		t.Error("expected error for nonexistent hot spare")
	}
}

func TestEstimateTime(t *testing.T) {
	m := NewManager()

	// NAS 到 NAS
	est, err := m.EstimateTime(4000787030016, MigrationTypeNasToNas)
	if err != nil {
		t.Fatalf("estimate time failed: %v", err)
	}
	if est.EstimatedTime == 0 {
		t.Error("expected non-zero time")
	}
	if est.Bottleneck != "network" {
		t.Errorf("expected 'network', got '%s'", est.Bottleneck)
	}

	// 磁盘交换
	est, err = m.EstimateTime(4000787030016, MigrationTypeDiskSwap)
	if err != nil {
		t.Fatalf("estimate time failed: %v", err)
	}
	if est.Bottleneck != "disk" {
		t.Errorf("expected 'disk', got '%s'", est.Bottleneck)
	}

	// 无效大小
	_, err = m.EstimateTime(0, MigrationTypeDiskSwap)
	if err == nil {
		t.Error("expected error for zero size")
	}

	// 未知类型
	_, err = m.EstimateTime(1000, "unknown")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestVerifyIntegrity(t *testing.T) {
	m := NewManager()

	// 先创建任务
	job, _ := m.ExecutePlan("plan-1")

	check, err := m.VerifyIntegrity(job.ID)
	if err != nil {
		t.Fatalf("verify integrity failed: %v", err)
	}
	if check.TotalFiles == 0 {
		t.Error("expected non-zero total files")
	}
	if check.PassedFiles == 0 {
		t.Error("expected non-zero passed files")
	}
	if check.Duration == 0 {
		t.Error("expected non-zero duration")
	}

	// 不存在的任务
	_, err = m.VerifyIntegrity("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}
