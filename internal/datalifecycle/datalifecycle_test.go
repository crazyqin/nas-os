package datalifecycle

import (
	"testing"
	"time"
)

// ========== 辅助函数 ==========

func newTestManager() *Manager {
	return NewManager()
}

// ========== 策略管理测试 ==========

func TestCreateAndListPolicies(t *testing.T) {
	m := newTestManager()

	policy := LifecyclePolicy{
		Name:        "archive-old-data",
		Description: "归档90天未访问的数据",
		Enabled:     true,
		Priority:    10,
		Type:        PolicyTypeArchival,
		PathPatterns: []string{"/data/**"},
		Phases: []PhaseDefinition{
			{
				Phase:    PhaseArchive,
				Duration: 90 * 24 * time.Hour,
			},
		},
		Retention: RetentionPolicy{
			Type:     RetentionTypeTime,
			Duration: 365 * 24 * time.Hour,
		},
	}

	created, err := m.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty policy ID")
	}
	if created.Name != "archive-old-data" {
		t.Errorf("expected name 'archive-old-data', got '%s'", created.Name)
	}

	policies := m.ListPolicies(nil)
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].ID != created.ID {
		t.Errorf("expected policy ID '%s', got '%s'", created.ID, policies[0].ID)
	}
}

func TestCreatePolicyDuplicate(t *testing.T) {
	m := newTestManager()

	policy := LifecyclePolicy{
		ID:      "dup-test",
		Name:    "test",
		Enabled: true,
		Type:    PolicyTypeRetention,
	}

	_, err := m.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("first CreatePolicy failed: %v", err)
	}

	_, err = m.CreatePolicy(policy)
	if err == nil {
		t.Error("expected error for duplicate policy, got nil")
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.GetPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestListPoliciesFilter(t *testing.T) {
	m := newTestManager()

	enabled := true
	disabled := false

	_, _ = m.CreatePolicy(LifecyclePolicy{Name: "enabled", Enabled: true, Type: PolicyTypeRetention})
	_, _ = m.CreatePolicy(LifecyclePolicy{Name: "disabled", Enabled: false, Type: PolicyTypeRetention})

	all := m.ListPolicies(nil)
	if len(all) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(all))
	}

	enabledList := m.ListPolicies(&enabled)
	if len(enabledList) != 1 {
		t.Fatalf("expected 1 enabled policy, got %d", len(enabledList))
	}

	disabledList := m.ListPolicies(&disabled)
	if len(disabledList) != 1 {
		t.Fatalf("expected 1 disabled policy, got %d", len(disabledList))
	}
}

func TestUpdateAndDeletePolicy(t *testing.T) {
	m := newTestManager()

	created, err := m.CreatePolicy(LifecyclePolicy{
		Name:    "temp-policy",
		Enabled: true,
		Type:    PolicyTypeRetention,
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	updated, err := m.UpdatePolicy(created.ID, LifecyclePolicy{
		Name:    "updated-policy",
		Enabled: false,
		Type:    PolicyTypeArchival,
	})
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}
	if updated.Name != "updated-policy" {
		t.Errorf("expected name 'updated-policy', got '%s'", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected Enabled=false")
	}

	err = m.DeletePolicy(created.ID)
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = m.GetPolicy(created.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// ========== 数据记录测试 ==========

func TestCreateAndGetRecord(t *testing.T) {
	m := newTestManager()

	record := DataRecord{
		Path:           "/data/hot/report.pdf",
		Name:           "report.pdf",
		Size:           1024 * 1024 * 100,
		CurrentTier:    TierHot,
		Classification: ClassificationInternal,
		Tags:           []string{"important"},
	}

	created, err := m.CreateRecord(record)
	if err != nil {
		t.Fatalf("CreateRecord failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty record ID")
	}
	if created.CurrentPhase != PhaseActive {
		t.Errorf("expected PhaseActive, got '%s'", created.CurrentPhase)
	}

	fetched, err := m.GetRecord(created.ID)
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}
	if fetched.Path != record.Path {
		t.Errorf("expected path '%s', got '%s'", record.Path, fetched.Path)
	}
}

func TestListRecordsFilter(t *testing.T) {
	m := newTestManager()

	r1, _ := m.CreateRecord(DataRecord{Path: "/hot/file1"})
	r2, _ := m.CreateRecord(DataRecord{Path: "/warm/file2"})
	_, _ = m.CreateRecord(DataRecord{Path: "/hot/file3"})
	_ = r1

	// 转换 r2 到 warm 层
	_ = m.TransitionPhase(r2.ID, PhaseReference, "转换到温存储")

	hotRecords := m.ListRecords("", TierHot)
	if len(hotRecords) != 2 {
		t.Errorf("expected 2 hot records, got %d", len(hotRecords))
	}

	warmRecords := m.ListRecords("", TierWarm)
	if len(warmRecords) != 1 {
		t.Errorf("expected 1 warm record, got %d", len(warmRecords))
	}
}

// ========== 阶段转换测试 ==========

func TestTransitionPhaseUnit(t *testing.T) {
	m := newTestManager()

	record, _ := m.CreateRecord(DataRecord{
		Path: "/data/file.txt",
		Name: "file.txt",
		Size: 1024,
	})

	err := m.TransitionPhase(record.ID, PhaseArchive, "超过90天未访问")
	if err != nil {
		t.Fatalf("TransitionPhase failed: %v", err)
	}

	updated, _ := m.GetRecord(record.ID)
	if updated.CurrentPhase != PhaseArchive {
		t.Errorf("expected PhaseArchive, got '%s'", updated.CurrentPhase)
	}
	if updated.CurrentTier != TierCold {
		t.Errorf("expected TierCold, got '%s'", updated.CurrentTier)
	}
	if len(updated.PhaseHistory) != 1 {
		t.Errorf("expected 1 phase transition, got %d", len(updated.PhaseHistory))
	}
}

func TestTransitionPhaseInvalidUnit(t *testing.T) {
	m := newTestManager()

	record, _ := m.CreateRecord(DataRecord{
		Path: "/data/file.txt",
		Name: "file.txt",
	})

	// 先转到 Archive
	_ = m.TransitionPhase(record.ID, PhaseArchive, "归档")

	// 尝试回退到 Active（应该失败）
	err := m.TransitionPhase(record.ID, PhaseActive, "回退")
	if err == nil {
		t.Error("expected error for backward transition")
	}
}

// ========== 合规保留测试 ==========

func TestCreateAndReleaseHold(t *testing.T) {
	m := newTestManager()

	record, _ := m.CreateRecord(DataRecord{
		Path: "/data/legal/doc.pdf",
		Name: "doc.pdf",
	})

	hold := ComplianceHold{
		Type:      RetentionTypeLegal,
		Name:      "诉讼保留",
		FilePaths: []string{"/data/legal/doc.pdf"},
		CaseNumber: "CASE-2026-001",
		IssuedBy:   "法务部",
		Regulation: "民事诉讼法",
	}

	created, err := m.CreateHold(hold)
	if err != nil {
		t.Fatalf("CreateHold failed: %v", err)
	}
	if !created.Active {
		t.Error("expected hold to be active")
	}

	// 检查记录关联
	updated, _ := m.GetRecord(record.ID)
	if len(updated.HoldIDs) != 1 {
		t.Errorf("expected 1 hold ID on record, got %d", len(updated.HoldIDs))
	}

	// 释放
	err = m.ReleaseHold(created.ID, "法务主管")
	if err != nil {
		t.Fatalf("ReleaseHold failed: %v", err)
	}

	// 重复释放应报错
	err = m.ReleaseHold(created.ID, "法务主管")
	if err == nil {
		t.Error("expected error for double release")
	}
}

func TestListHoldsFilter(t *testing.T) {
	m := newTestManager()

	active := true
	inactive := false

	_, _ = m.CreateHold(ComplianceHold{Name: "active", Active: true, FilePaths: []string{"/a"}})
	hold2, _ := m.CreateHold(ComplianceHold{Name: "to-release", Active: true, FilePaths: []string{"/b"}})
	_ = m.ReleaseHold(hold2.ID, "admin")

	all := m.ListHolds(nil)
	if len(all) != 2 {
		t.Fatalf("expected 2 holds, got %d", len(all))
	}

	activeList := m.ListHolds(&active)
	if len(activeList) != 1 {
		t.Errorf("expected 1 active hold, got %d", len(activeList))
	}

	inactiveList := m.ListHolds(&inactive)
	if len(inactiveList) != 1 {
		t.Errorf("expected 1 inactive hold, got %d", len(inactiveList))
	}
}

// ========== 数据迁移测试 ==========

func TestCreateAndStartMigration(t *testing.T) {
	m := newTestManager()

	migration := DataMigration{
		SourceTier: TierHot,
		TargetTier: TierCold,
		SourcePath: "/data/hot",
		TargetPath: "/data/cold",
		Files: []MigrationFile{
			{SourcePath: "/data/hot/file1.dat", TargetPath: "/data/cold/file1.dat", Size: 1024},
			{SourcePath: "/data/hot/file2.dat", TargetPath: "/data/cold/file2.dat", Size: 2048},
		},
		TotalFiles: 2,
		TotalBytes: 3072,
	}

	created, err := m.CreateMigration(migration)
	if err != nil {
		t.Fatalf("CreateMigration failed: %v", err)
	}
	if created.Status != MigrationPending {
		t.Errorf("expected MigrationPending, got '%s'", created.Status)
	}

	err = m.StartMigration(created.ID)
	if err != nil {
		t.Fatalf("StartMigration failed: %v", err)
	}

	// 等待迁移完成
	time.Sleep(200 * time.Millisecond)

	updated, _ := m.GetMigration(created.ID)
	if updated.Status != MigrationCompleted {
		t.Errorf("expected MigrationCompleted, got '%s'", updated.Status)
	}
}

func TestListMigrationsFilter(t *testing.T) {
	m := newTestManager()

	_, _ = m.CreateMigration(DataMigration{SourceTier: TierHot, TargetTier: TierCold})
	_, _ = m.CreateMigration(DataMigration{SourceTier: TierWarm, TargetTier: TierArchive})

	all := m.ListMigrations("")
	if len(all) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(all))
	}

	pending := m.ListMigrations(MigrationPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

// ========== 数据销毁测试 ==========

func TestCreateAndExecuteDestruction(t *testing.T) {
	m := newTestManager()

	destruction := DestructionRecord{
		FilePaths: []string{"/data/sensitive/secret.doc"},
		Method:    MethodSecureDelete,
		TotalSize: 1024 * 512,
	}

	created, err := m.CreateDestruction(destruction)
	if err != nil {
		t.Fatalf("CreateDestruction failed: %v", err)
	}
	if created.Status != DestructionPending {
		t.Errorf("expected DestructionPending, got '%s'", created.Status)
	}

	// 批准
	err = m.ApproveDestruction(created.ID, "安全主管")
	if err != nil {
		t.Fatalf("ApproveDestruction failed: %v", err)
	}

	// 执行
	err = m.ExecuteDestruction(created.ID)
	if err != nil {
		t.Fatalf("ExecuteDestruction failed: %v", err)
	}

	updated, _ := m.GetDestruction(created.ID)
	if updated.Status != DestructionCompleted {
		t.Errorf("expected DestructionCompleted, got '%s'", updated.Status)
	}
	if updated.Certification == nil {
		t.Error("expected destruction certification")
	}
}

func TestDestructionWithComplianceHold(t *testing.T) {
	m := newTestManager()

	// 创建合规保留
	_, _ = m.CreateHold(ComplianceHold{
		Name:      "诉讼保留",
		Active:    true,
		FilePaths: []string{"/data/legal/evidence.doc"},
		CaseNumber: "CASE-001",
	})

	// 创建销毁（应自动关联保留）
	created, _ := m.CreateDestruction(DestructionRecord{
		FilePaths: []string{"/data/legal/evidence.doc"},
		Method:    MethodSecureDelete,
	})

	if !created.RequiresApproval {
		t.Error("expected RequiresApproval=true due to compliance hold")
	}
	if created.HoldID == "" {
		t.Error("expected HoldID to be set")
	}
}

// ========== 策略模板测试 ==========

func TestCreateAndListTemplates(t *testing.T) {
	m := newTestManager()

	template := PolicyTemplate{
		Name:        "医疗数据保留",
		Description: "HIPAA合规的医疗数据保留策略",
		Category:    "healthcare",
		Policy: LifecyclePolicy{
			Name:    "hipaa-retention",
			Enabled: true,
			Type:    PolicyTypeCompliance,
		},
		IsSystem: true,
	}

	created, err := m.CreateTemplate(template)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	templates := m.ListTemplates()
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	if templates[0].ID != created.ID {
		t.Errorf("expected template ID '%s', got '%s'", created.ID, templates[0].ID)
	}
}

// ========== 批量操作测试 ==========

func TestBatchApplyPolicy(t *testing.T) {
	m := newTestManager()

	policy, _ := m.CreatePolicy(LifecyclePolicy{
		Name:    "batch-test",
		Enabled: true,
		Type:    PolicyTypeRetention,
	})

	_, _ = m.CreateRecord(DataRecord{Path: "/data/file1.txt"})
	_, _ = m.CreateRecord(DataRecord{Path: "/data/file2.txt"})
	_, _ = m.CreateRecord(DataRecord{Path: "/data/file3.txt"})

	result, err := m.BatchApplyPolicy(BatchApplyRequest{
		PolicyID: policy.ID,
		Paths:    []string{"/data/file1.txt", "/data/file2.txt"},
	})
	if err != nil {
		t.Fatalf("BatchApplyPolicy failed: %v", err)
	}
	if result.AppliedFiles != 2 {
		t.Errorf("expected 2 applied, got %d", result.AppliedFiles)
	}
}

func TestBatchApplyPolicyForce(t *testing.T) {
	m := newTestManager()

	policy1, _ := m.CreatePolicy(LifecyclePolicy{Name: "p1", Enabled: true, Type: PolicyTypeRetention})
	policy2, _ := m.CreatePolicy(LifecyclePolicy{Name: "p2", Enabled: true, Type: PolicyTypeArchival})

	record, _ := m.CreateRecord(DataRecord{Path: "/data/file.txt"})

	// 先应用 p1
	_, _ = m.BatchApplyPolicy(BatchApplyRequest{
		PolicyID: policy1.ID,
		Paths:    []string{"/data/file.txt"},
	})

	// 不强制应用 p2（应跳过）
	result1, _ := m.BatchApplyPolicy(BatchApplyRequest{
		PolicyID: policy2.ID,
		Paths:    []string{"/data/file.txt"},
		Force:    false,
	})
	if result1.SkippedFiles != 1 {
		t.Errorf("expected 1 skipped, got %d", result1.SkippedFiles)
	}

	// 强制应用 p2
	result2, _ := m.BatchApplyPolicy(BatchApplyRequest{
		PolicyID: policy2.ID,
		Paths:    []string{"/data/file.txt"},
		Force:    true,
	})
	if result2.AppliedFiles != 1 {
		t.Errorf("expected 1 applied with force, got %d", result2.AppliedFiles)
	}

	updated, _ := m.GetRecord(record.ID)
	if updated.PolicyID != policy2.ID {
		t.Errorf("expected policy2, got '%s'", updated.PolicyID)
	}
}

// ========== 访问分析测试 ==========

func TestGenerateAccessReport(t *testing.T) {
	m := newTestManager()

	r1, _ := m.CreateRecord(DataRecord{Path: "/hot/file1", Size: 1024})
	r2, _ := m.CreateRecord(DataRecord{Path: "/warm/file2", Size: 2048})
	r3, _ := m.CreateRecord(DataRecord{Path: "/cold/file3", Size: 4096})
	_ = r1

	// 转换不同阶段以获得不同 tier
	_ = m.TransitionPhase(r2.ID, PhaseReference, "偶尔访问")
	_ = m.TransitionPhase(r3.ID, PhaseArchive, "归档")

	report := m.GenerateAccessReport()
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TotalFiles != 3 {
		t.Errorf("expected 3 files, got %d", report.TotalFiles)
	}
	if report.TotalSize != 7168 {
		t.Errorf("expected 7168 bytes, got %d", report.TotalSize)
	}
	if len(report.TierStats) != 3 {
		t.Errorf("expected 3 tier stats, got %d", len(report.TierStats))
	}
}

// ========== 审计日志测试 ==========

func TestAuditLog(t *testing.T) {
	m := newTestManager()

	// 创建策略（触发审计）
	_, _ = m.CreatePolicy(LifecyclePolicy{Name: "audit-policy", Enabled: true, Type: PolicyTypeRetention})

	// 创建记录（触发审计）
	_, _ = m.CreateRecord(DataRecord{Path: "/data/file.txt"})

	log := m.GetAuditLog(100)
	if len(log) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(log))
	}

	// 检查审计条目内容
	found := false
	for _, entry := range log {
		if entry.Action == "create_policy" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected create_policy audit entry")
	}
}

// ========== 状态查询测试 ==========

func TestGetStatusUnit(t *testing.T) {
	m := newTestManager()

	_, _ = m.CreatePolicy(LifecyclePolicy{Name: "p1", Enabled: true, Type: PolicyTypeRetention})
	_, _ = m.CreatePolicy(LifecyclePolicy{Name: "p2", Enabled: false, Type: PolicyTypeArchival})
	_, _ = m.CreateRecord(DataRecord{Path: "/data/file1", CurrentTier: TierHot})
	_, _ = m.CreateHold(ComplianceHold{Name: "hold1", Active: true, FilePaths: []string{"/a"}})

	status := m.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled=true")
	}
	if status.TotalPolicies != 2 {
		t.Errorf("expected 2 policies, got %d", status.TotalPolicies)
	}
	if status.ActivePolicies != 1 {
		t.Errorf("expected 1 active policy, got %d", status.ActivePolicies)
	}
	if status.TotalRecords != 1 {
		t.Errorf("expected 1 record, got %d", status.TotalRecords)
	}
	if status.ActiveHolds != 1 {
		t.Errorf("expected 1 active hold, got %d", status.ActiveHolds)
	}
}
