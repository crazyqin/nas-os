package retention

import (
	"testing"
	"time"
)

func TestCreateAndListPolicies(t *testing.T) {
	engine := NewRetentionEngine()

	p := &RetentionPolicy{
		Name:        "30-day-delete-tmp",
		Description: "Delete temp files after 30 days",
		Enabled:     true,
		Priority:    10,
		Period:      Period30Days,
		Mode:        ModeDelete,
		Conditions: []PolicyCondition{
			{Field: "path", Operator: "prefix", Values: []string{"/tmp"}},
			{Field: "fileType", Operator: "eq", Values: []string{".tmp", ".log"}},
		},
		ConditionLogic: OpAnd,
	}

	created, err := engine.CreatePolicy(p)
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty policy ID")
	}
	if created.Name != "30-day-delete-tmp" {
		t.Errorf("expected name '30-day-delete-tmp', got '%s'", created.Name)
	}

	policies := engine.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].ID != created.ID {
		t.Errorf("expected policy ID '%s', got '%s'", created.ID, policies[0].ID)
	}
}

func TestLegalHoldBlocksDeletion(t *testing.T) {
	engine := NewRetentionEngine()

	// 创建策略：7天后删除
	policy := &RetentionPolicy{
		Name:     "7d-delete",
		Enabled:  true,
		Priority: 10,
		Period:   Period7Days,
		Mode:     ModeDelete,
		Conditions: []PolicyCondition{
			{Field: "path", Operator: "prefix", Values: []string{"/data"}},
		},
	}
	created, _ := engine.CreatePolicy(policy)

	// 注册一个30天前的文件
	engine.RegisterFile(&FileRecord{
		Path:    "/data/case-evidence.pdf",
		Name:    "case-evidence.pdf",
		Size:    1024 * 1024,
		ModTime: time.Now().Add(-30 * 24 * time.Hour),
		FileType: ".pdf",
	})

	// 应用策略前先模拟：文件应匹配
	result := engine.Simulate(created)
	if result.MatchedCount != 1 {
		t.Fatalf("expected 1 matched file, got %d", result.MatchedCount)
	}

	// 创建法律保留
	hold := &LegalHold{
		Name:       "litigation-hold-001",
		FilePaths:  []string{"/data/case-evidence.pdf"},
		CaseNumber: "CASE-2026-001",
		IssuedBy:   "legal-dept",
	}
	_, err := engine.CreateLegalHold(hold)
	if err != nil {
		t.Fatalf("CreateLegalHold failed: %v", err)
	}

	// 再次模拟：文件应被保护
	result = engine.Simulate(created)
	if result.MatchedCount != 0 {
		t.Errorf("expected 0 matched files (protected by legal hold), got %d", result.MatchedCount)
	}
	if len(result.ProtectedFiles) != 1 {
		t.Errorf("expected 1 protected file, got %d", len(result.ProtectedFiles))
	}

	// 验证 IsFileProtected
	if !engine.IsFileProtected("/data/case-evidence.pdf") {
		t.Error("expected file to be protected by legal hold")
	}
	if engine.IsFileProtected("/data/other-file.txt") {
		t.Error("expected other file to NOT be protected")
	}
}

func TestPolicyPriority(t *testing.T) {
	engine := NewRetentionEngine()

	// 低优先级：90天归档
	engine.CreatePolicy(&RetentionPolicy{
		Name:     "archive-old",
		Enabled:  true,
		Priority: 1,
		Period:   Period90Days,
		Mode:     ModeArchive,
		Conditions: []PolicyCondition{
			{Field: "fileType", Operator: "eq", Values: []string{".pdf"}},
		},
	})

	// 高优先级：30天删除
	engine.CreatePolicy(&RetentionPolicy{
		Name:     "delete-pdfs",
		Enabled:  true,
		Priority: 100,
		Period:   Period30Days,
		Mode:     ModeDelete,
		Conditions: []PolicyCondition{
			{Field: "fileType", Operator: "eq", Values: []string{".pdf"}},
		},
	})

	policies := engine.ListPolicies()
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
	// 高优先级排在前面
	if policies[0].Priority <= policies[1].Priority {
		t.Errorf("expected policies sorted by priority desc, got %d then %d", policies[0].Priority, policies[1].Priority)
	}
}

func TestComplianceReport(t *testing.T) {
	engine := NewRetentionEngine()

	// 创建策略
	engine.CreatePolicy(&RetentionPolicy{
		Name:     "cleanup-tmp",
		Enabled:  true,
		Priority: 10,
		Period:   Period7Days,
		Mode:     ModeDelete,
		Conditions: []PolicyCondition{
			{Field: "path", Operator: "prefix", Values: []string{"/tmp"}},
		},
	})

	// 注册文件
	engine.RegisterFile(&FileRecord{
		Path:     "/tmp/a.log",
		Name:     "a.log",
		Size:     500,
		ModTime:  time.Now().Add(-10 * 24 * time.Hour),
		FileType: ".log",
	})
	engine.RegisterFile(&FileRecord{
		Path:     "/data/important.doc",
		Name:     "important.doc",
		Size:     2000,
		ModTime:  time.Now().Add(-1 * 24 * time.Hour),
		FileType: ".doc",
	})

	report := engine.GetComplianceReport()
	if report.TotalPolicies != 1 {
		t.Errorf("expected 1 total policy, got %d", report.TotalPolicies)
	}
	if report.TotalFiles != 2 {
		t.Errorf("expected 2 total files, got %d", report.TotalFiles)
	}
	if report.CoveredFiles != 1 {
		t.Errorf("expected 1 covered file, got %d", report.CoveredFiles)
	}
	// /data/important.doc 不匹配 /tmp 前缀，应为违规文件
	if len(report.ViolatingFiles) != 1 {
		t.Errorf("expected 1 violating file, got %d", len(report.ViolatingFiles))
	}
}

func TestAuditLog(t *testing.T) {
	engine := NewRetentionEngine()

	engine.CreatePolicy(&RetentionPolicy{
		Name:    "test-policy",
		Enabled: true,
		Period:  Period30Days,
		Mode:    ModeDelete,
	})
	engine.CreateLegalHold(&LegalHold{
		Name:       "test-hold",
		FilePaths:  []string{"/data/*"},
		CaseNumber: "C001",
		IssuedBy:   "admin",
	})

	logs := engine.GetAuditLog(100)
	if len(logs) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(logs))
	}
	// 最新的在前
	if logs[0].Action != "create_hold" {
		t.Errorf("expected first entry to be 'create_hold', got '%s'", logs[0].Action)
	}
	if logs[1].Action != "create_policy" {
		t.Errorf("expected second entry to be 'create_policy', got '%s'", logs[1].Action)
	}
}

func TestExpiringFiles(t *testing.T) {
	engine := NewRetentionEngine()

	engine.CreatePolicy(&RetentionPolicy{
		Name:     "30d-cleanup",
		Enabled:  true,
		Priority: 10,
		Period:   Period30Days,
		Mode:     ModeDelete,
	})

	// 文件在25天前修改，保留30天 → 5天后过期
	engine.RegisterFile(&FileRecord{
		Path:     "/data/expiring-soon.pdf",
		Name:     "expiring-soon.pdf",
		Size:     1024,
		ModTime:  time.Now().Add(-25 * 24 * time.Hour),
		FileType: ".pdf",
	})
	// 文件昨天修改，还早
	engine.RegisterFile(&FileRecord{
		Path:     "/data/fresh.pdf",
		Name:     "fresh.pdf",
		Size:     1024,
		ModTime:  time.Now().Add(-1 * 24 * time.Hour),
		FileType: ".pdf",
	})

	expiring := engine.GetExpiringFiles(7)
	if len(expiring) != 1 {
		t.Fatalf("expected 1 expiring file, got %d", len(expiring))
	}
	if expiring[0].Path != "/data/expiring-soon.pdf" {
		t.Errorf("expected /data/expiring-soon.pdf, got %s", expiring[0].Path)
	}
}

func TestSimulatePolicy(t *testing.T) {
	engine := NewRetentionEngine()

	policy := &RetentionPolicy{
		Name:     "archive-large",
		Enabled:  true,
		Priority: 10,
		Period:   Period30Days,
		Mode:     ModeArchive,
		Conditions: []PolicyCondition{
			{Field: "size", Operator: "gte", Values: []string{"1048576"}}, // >= 1MB
			{Field: "age", Operator: "gte", Values: []string{"30"}},       // >= 30 days
		},
		ConditionLogic: OpAnd,
	}

	// 大文件，35天前
	engine.RegisterFile(&FileRecord{
		Path:     "/data/big-video.mp4",
		Name:     "big-video.mp4",
		Size:     500 * 1024 * 1024,
		ModTime:  time.Now().Add(-35 * 24 * time.Hour),
		FileType: ".mp4",
	})
	// 小文件，40天前（不匹配size条件）
	engine.RegisterFile(&FileRecord{
		Path:     "/data/small.txt",
		Name:     "small.txt",
		Size:     100,
		ModTime:  time.Now().Add(-40 * 24 * time.Hour),
		FileType: ".txt",
	})
	// 大文件，10天前（不匹配age条件）
	engine.RegisterFile(&FileRecord{
		Path:     "/data/new-big.zip",
		Name:     "new-big.zip",
		Size:     2 * 1024 * 1024,
		ModTime:  time.Now().Add(-10 * 24 * time.Hour),
		FileType: ".zip",
	})

	result := engine.Simulate(policy)
	if result.MatchedCount != 1 {
		t.Fatalf("expected 1 matched file, got %d", result.MatchedCount)
	}
	if result.MatchedFiles[0].Path != "/data/big-video.mp4" {
		t.Errorf("expected /data/big-video.mp4, got %s", result.MatchedFiles[0].Path)
	}
	if result.Action != ModeArchive {
		t.Errorf("expected action '%s', got '%s'", ModeArchive, result.Action)
	}
}
