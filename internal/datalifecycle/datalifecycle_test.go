package datalifecycle

import (
	"context"
	"testing"
	"time"
)

// ========== 辅助函数 ==========

func newTestManager() *Manager {
	return NewManager(nil, nil)
}

func registerTestItems(m *Manager) {
	items := []*DataItem{
		{
			ID:          "item-1",
			Path:        "/data/hot/report.pdf",
			Name:        "report.pdf",
			Size:        1024 * 1024 * 100, // 100MB
			CurrentTier: TierHot,
			Tags:        []string{"important"},
			CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
			ModifiedAt:  time.Now().Add(-2 * 24 * time.Hour),
			AccessedAt:  time.Now().Add(-1 * 24 * time.Hour),
		},
		{
			ID:          "item-2",
			Path:        "/data/warm/archive-2025.tar.gz",
			Name:        "archive-2025.tar.gz",
			Size:        1024 * 1024 * 500, // 500MB
			CurrentTier: TierWarm,
			CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
			ModifiedAt:  time.Now().Add(-45 * 24 * time.Hour),
			AccessedAt:  time.Now().Add(-40 * 24 * time.Hour),
		},
		{
			ID:          "item-3",
			Path:        "/data/cold/old-backup.tar",
			Name:        "old-backup.tar",
			Size:        1024 * 1024 * 1024, // 1GB
			CurrentTier: TierCold,
			CreatedAt:   time.Now().Add(-200 * 24 * time.Hour),
			ModifiedAt:  time.Now().Add(-180 * 24 * time.Hour),
			AccessedAt:  time.Now().Add(-120 * 24 * time.Hour),
		},
	}
	for _, item := range items {
		_ = m.RegisterDataItem(item)
	}
}

// ========== 测试场景 ==========

// TestCreateAndListPolicies 测试创建和列出生命周期策略.
func TestCreateAndListPolicies(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	req := CreatePolicyRequest{
		Name:        "archive-old-data",
		Description: "归档90天未访问的数据",
		Enabled:     true,
		Priority:    10,
		PathPattern: "/data/**",
		TriggerDays: 90,
		Actions: []PolicyAction{
			{Type: ActionTierDown, TargetTier: TierCold},
			{Type: ActionArchive},
		},
	}

	policy, err := m.CreatePolicy(req)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if policy.ID == "" {
		t.Fatal("expected non-empty policy ID")
	}
	if policy.Name != "archive-old-data" {
		t.Errorf("expected name 'archive-old-data', got '%s'", policy.Name)
	}

	policies := m.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].ID != policy.ID {
		t.Errorf("expected policy ID '%s', got '%s'", policy.ID, policies[0].ID)
	}
}

// TestCreatePolicyInvalidAction 测试无效动作创建策略.
func TestCreatePolicyInvalidAction(t *testing.T) {
	m := newTestManager()

	req := CreatePolicyRequest{
		Name:        "bad-policy",
		PathPattern: "/data/*",
		Actions:     []PolicyAction{{Type: "invalid_action"}},
	}

	_, err := m.CreatePolicy(req)
	if err != ErrInvalidAction {
		t.Errorf("expected ErrInvalidAction, got %v", err)
	}
}

// TestEvaluatePolicyDryRun 测试策略评估干跑模式.
func TestEvaluatePolicyDryRun(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	registerTestItems(m)

	// 创建策略：30天未访问迁移到冷数据层
	policy, err := m.CreatePolicy(CreatePolicyRequest{
		Name:        "move-to-cold",
		Description: "30天未访问数据迁移到冷数据层",
		Enabled:     true,
		PathPattern: "/data/**",
		TriggerDays: 30,
		Actions: []PolicyAction{
			{Type: ActionTierDown, TargetTier: TierCold},
		},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	result, err := m.EvaluatePolicy(context.Background(), policy.ID, true)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	// item-2 (40天) 和 item-3 (120天) 应该匹配
	if result.MatchedItems < 1 {
		t.Errorf("expected at least 1 matched item, got %d", result.MatchedItems)
	}
}

// TestEvaluatePolicyExecute 测试策略评估执行模式.
func TestEvaluatePolicyExecute(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	registerTestItems(m)

	policy, err := m.CreatePolicy(CreatePolicyRequest{
		Name:        "archive-cold-data",
		Enabled:     true,
		PathPattern: "/data/cold/*",
		TriggerDays: 30,
		Actions: []PolicyAction{
			{Type: ActionArchive},
		},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	result, err := m.EvaluatePolicy(context.Background(), policy.ID, false)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
	if result.DryRun {
		t.Error("expected DryRun=false")
	}
	if result.MatchedItems < 1 {
		t.Errorf("expected at least 1 matched item, got %d", result.MatchedItems)
	}
	// 检查审计日志
	events := m.ListAuditEvents(EventArchive, 10)
	if len(events) == 0 {
		t.Error("expected audit events for archive action")
	}
}

// TestRetentionPolicyTimeBased 测试基于时间的保留策略.
func TestRetentionPolicyTimeBased(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	// 注册一个很老的数据项
	_ = m.RegisterDataItem(&DataItem{
		ID:         "old-item",
		Path:       "/data/tmp/old-file.tmp",
		Name:       "old-file.tmp",
		Size:       1024,
		CurrentTier: TierHot,
		CreatedAt:  time.Now().Add(-60 * 24 * time.Hour),
		ModifiedAt: time.Now().Add(-45 * 24 * time.Hour),
		AccessedAt: time.Now().Add(-45 * 24 * time.Hour),
	})
	_ = m.RegisterDataItem(&DataItem{
		ID:         "new-item",
		Path:       "/data/tmp/new-file.tmp",
		Name:       "new-file.tmp",
		Size:       1024,
		CurrentTier: TierHot,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
		AccessedAt: time.Now(),
	})

	policy, err := m.CreateRetentionPolicy(CreateRetentionPolicyRequest{
		Name:          "clean-tmp",
		Description:   "清理30天前的临时文件",
		Enabled:       true,
		Mode:          RetentionModeTime,
		RetentionDays: 30,
		PathPattern:   "/data/tmp/*",
	})
	if err != nil {
		t.Fatalf("CreateRetentionPolicy failed: %v", err)
	}

	removed, err := m.EnforceRetentionPolicy(context.Background(), policy.ID)
	if err != nil {
		t.Fatalf("EnforceRetentionPolicy failed: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed file, got %d", len(removed))
	}
	if len(removed) > 0 && removed[0] != "/data/tmp/old-file.tmp" {
		t.Errorf("expected '/data/tmp/old-file.tmp', got '%s'", removed[0])
	}
}

// TestDataLineage 测试数据血缘追踪.
func TestDataLineage(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	// 创建血缘链: source.csv -> processed.csv -> report.pdf
	l1, err := m.CreateLineage(CreateLineageRequest{
		FilePath:  "/data/processed.csv",
		SourcePath: "/data/source.csv",
		Operation: "transform",
		Operator:  "etl-job-001",
		Metadata:  map[string]string{"tool": "python"},
	})
	if err != nil {
		t.Fatalf("CreateLineage failed: %v", err)
	}

	l2, err := m.CreateLineage(CreateLineageRequest{
		FilePath:  "/data/report.pdf",
		SourcePath: "/data/processed.csv",
		Operation: "generate",
		Operator:  "report-gen",
	})
	if err != nil {
		t.Fatalf("CreateLineage failed: %v", err)
	}

	// 查询血缘
	fetched, err := m.GetLineage(l1.ID)
	if err != nil {
		t.Fatalf("GetLineage failed: %v", err)
	}
	if fetched.Operation != "transform" {
		t.Errorf("expected operation 'transform', got '%s'", fetched.Operation)
	}

	// 按路径查询
	byPath := m.GetLineageByPath("/data/processed.csv")
	if len(byPath) < 2 {
		t.Errorf("expected at least 2 lineage records for processed.csv, got %d", len(byPath))
	}

	// 获取血缘图
	graph := m.GetLineageGraph("/data/report.pdf")
	if graph == nil {
		t.Fatal("expected non-nil lineage graph")
	}
	if graph.Root == nil {
		t.Fatal("expected non-nil root node")
	}
	_ = l2
}

// TestCostAnalysis 测试存储成本分析.
func TestCostAnalysis(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	registerTestItems(m)

	summary := m.AnalyzeCosts(context.Background())
	if summary == nil {
		t.Fatal("expected non-nil cost summary")
	}
	if summary.TotalItems != 3 {
		t.Errorf("expected 3 items, got %d", summary.TotalItems)
	}
	if summary.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if _, ok := summary.ByTier[TierHot]; !ok {
		t.Error("expected TierHot in ByTier map")
	}
	if _, ok := summary.ByTier[TierWarm]; !ok {
		t.Error("expected TierWarm in ByTier map")
	}

	// 检查建议是否生成
	suggestions := m.ListCostSuggestions()
	// item-3 访问在120天前，冷数据层，应建议归档
	found := false
	for _, s := range suggestions {
		if s.FilePath == "/data/cold/old-backup.tar" && s.SuggestedTier == TierArchive {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cost suggestion for old-backup.tar to archive tier")
	}
}

// TestAuditEvents 测试审计事件记录.
func TestAuditEvents(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	// 创建策略触发审计
	_, _ = m.CreatePolicy(CreatePolicyRequest{
		Name:        "audit-test",
		Enabled:     true,
		PathPattern: "/data/*",
		Actions:     []PolicyAction{{Type: ActionCompress}},
	})

	// 创建血缘触发审计
	_, _ = m.CreateLineage(CreateLineageRequest{
		FilePath:  "/data/test.csv",
		SourcePath: "/data/raw.csv",
		Operation: "transform",
	})

	allEvents := m.ListAuditEvents("", 100)
	if len(allEvents) < 2 {
		t.Errorf("expected at least 2 audit events, got %d", len(allEvents))
	}

	lineageEvents := m.ListAuditEvents(EventLineageUpdate, 10)
	if len(lineageEvents) < 1 {
		t.Error("expected at least 1 lineage audit event")
	}

	policyEvents := m.ListAuditEvents(EventRetentionPolicy, 10)
	if len(policyEvents) < 1 {
		t.Error("expected at least 1 policy audit event")
	}
}

// TestUpdateAndDeletePolicy 测试更新和删除策略.
func TestUpdateAndDeletePolicy(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	policy, err := m.CreatePolicy(CreatePolicyRequest{
		Name:        "temp-policy",
		Enabled:     true,
		PathPattern: "/tmp/*",
		Actions:     []PolicyAction{{Type: ActionDelete}},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 更新
	updated, err := m.UpdatePolicy(policy.ID, CreatePolicyRequest{
		Name:        "updated-policy",
		Enabled:     false,
		PathPattern: "/data/*",
		Actions:     []PolicyAction{{Type: ActionCompress}},
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

	// 删除
	if err := m.DeletePolicy(policy.ID); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = m.GetPolicy(policy.ID)
	if err != ErrPolicyNotFound {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}
}

// TestDisabledPolicyBlocked 测试禁用策略不可执行.
func TestDisabledPolicyBlocked(t *testing.T) {
	m := newTestManager()
	m.Start()
	defer m.Stop()

	policy, err := m.CreatePolicy(CreatePolicyRequest{
		Name:        "disabled-policy",
		Enabled:     false,
		PathPattern: "/data/*",
		Actions:     []PolicyAction{{Type: ActionDelete}},
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	_, err = m.EvaluatePolicy(context.Background(), policy.ID, false)
	if err != ErrPolicyDisabled {
		t.Errorf("expected ErrPolicyDisabled, got %v", err)
	}
}

// TestTierValidation 测试存储层级验证.
func TestTierValidation(t *testing.T) {
	if !IsValidTier(TierHot) {
		t.Error("TierHot should be valid")
	}
	if !IsValidTier(TierWarm) {
		t.Error("TierWarm should be valid")
	}
	if !IsValidTier(TierCold) {
		t.Error("TierCold should be valid")
	}
	if !IsValidTier(TierArchive) {
		t.Error("TierArchive should be valid")
	}
	if IsValidTier(Tier("invalid")) {
		t.Error("invalid tier should not be valid")
	}

	if !IsValidAction(ActionTierDown) {
		t.Error("ActionTierDown should be valid")
	}
	if IsValidAction(ActionType("bogus")) {
		t.Error("bogus action should not be valid")
	}
}
