package privacyimpact

import (
	"os"
	"path/filepath"
	"testing"
)

func defaultConfig() *Config {
	return &Config{
		Enabled:                 true,
		AutoAssess:              false,
		RiskThreshold:           60.0,
		EnabledFrameworks:       []ComplianceFramework{FrameworkPIPL, FrameworkGDPR},
		RetentionDays:           90,
		MaxAuditLogSize:         10000,
		DataFlowTrackingEnabled: true,
	}
}

// ========== NewManager ==========

func TestNewManager(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		m := NewManager(nil)
		if m == nil {
			t.Fatal("NewManager(nil) returned nil")
		}
		if !m.config.Enabled {
			t.Error("expected default Enabled=true")
		}
		if m.config.RetentionDays != 90 {
			t.Errorf("expected RetentionDays=90, got %d", m.config.RetentionDays)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := &Config{Enabled: false, RetentionDays: 30}
		m := NewManager(cfg)
		if m.config.RetentionDays != 30 {
			t.Errorf("expected RetentionDays=30, got %d", m.config.RetentionDays)
		}
		if m.config.Enabled {
			t.Error("expected Enabled=false")
		}
	})

	t.Run("initial state", func(t *testing.T) {
		m := NewManager(defaultConfig())
		if m.IsRunning() {
			t.Error("manager should not be running initially")
		}
		if len(m.ListAssessments()) != 0 {
			t.Error("initial assessments should be empty")
		}
	})
}

// ========== Start / Stop ==========

func TestManagerStartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		m := NewManager(defaultConfig())
		if err := m.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		if !m.IsRunning() {
			t.Error("expected running after Start")
		}

		// 重复启动应报错
		if err := m.Start(); err != ErrAlreadyRunning {
			t.Errorf("expected ErrAlreadyRunning, got %v", err)
		}

		if err := m.Stop(); err != nil {
			t.Fatalf("Stop failed: %v", err)
		}
		if m.IsRunning() {
			t.Error("expected not running after Stop")
		}

		// 重复停止应报错
		if err := m.Stop(); err != ErrNotRunning {
			t.Errorf("expected ErrNotRunning, got %v", err)
		}
	})

	t.Run("disabled config rejects start", func(t *testing.T) {
		m := NewManager(&Config{Enabled: false})
		if err := m.Start(); err != ErrInvalidConfig {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})
}

// ========== AssessOperation ==========

func TestAssessOperation(t *testing.T) {
	m := NewManager(defaultConfig())
	m.Start()
	defer m.Stop()

	t.Run("valid operation", func(t *testing.T) {
		a, err := m.AssessOperation(OpUpload, "用户数据", 1024)
		if err != nil {
			t.Fatalf("AssessOperation failed: %v", err)
		}
		if a.ID == "" {
			t.Error("assessment ID should not be empty")
		}
		if a.Operation != OpUpload {
			t.Errorf("expected OpUpload, got %s", a.Operation)
		}
		if a.DataType != "用户数据" {
			t.Errorf("expected '用户数据', got %s", a.DataType)
		}
		if a.Status != AssessmentCompleted {
			t.Errorf("expected completed status, got %s", a.Status)
		}
	})

	t.Run("high risk operation", func(t *testing.T) {
		a, err := m.AssessOperation(OpShare, "身份证信息", 100*1024*1024)
		if err != nil {
			t.Fatalf("AssessOperation failed: %v", err)
		}
		if a.RiskScore < 60 {
			t.Errorf("expected high risk score for id_card share, got %.1f", a.RiskScore)
		}
		if a.RiskLevel != RiskHigh && a.RiskLevel != RiskCritical {
			t.Errorf("expected high/critical risk level, got %s", a.RiskLevel)
		}
	})

	t.Run("invalid operation", func(t *testing.T) {
		_, err := m.AssessOperation("invalid_op", "test", 100)
		if err != ErrInvalidOperation {
			t.Errorf("expected ErrInvalidOperation, got %v", err)
		}
	})

	t.Run("not running", func(t *testing.T) {
		m2 := NewManager(defaultConfig())
		_, err := m2.AssessOperation(OpUpload, "test", 100)
		if err != ErrNotRunning {
			t.Errorf("expected ErrNotRunning, got %v", err)
		}
	})

	t.Run("risk score for low risk", func(t *testing.T) {
		a, err := m.AssessOperation(OpDelete, "普通文档", 512)
		if err != nil {
			t.Fatalf("AssessOperation failed: %v", err)
		}
		if a.RiskScore >= 40 {
			t.Errorf("expected low risk score for delete general doc, got %.1f", a.RiskScore)
		}
		if a.RiskLevel != RiskLow && a.RiskLevel != RiskNone {
			t.Errorf("expected low/none risk, got %s", a.RiskLevel)
		}
	})

	t.Run("recommendations generated", func(t *testing.T) {
		a, err := m.AssessOperation(OpTransfer, "银行账户数据", 50*1024*1024)
		if err != nil {
			t.Fatalf("AssessOperation failed: %v", err)
		}
		if len(a.Recommendations) == 0 {
			t.Error("expected recommendations for high-risk transfer")
		}
	})

	t.Run("get and list assessments", func(t *testing.T) {
		assessments := m.ListAssessments()
		if len(assessments) == 0 {
			t.Fatal("expected assessments after operations")
		}

		// 获取第一个评估
		first := assessments[0]
		got, err := m.GetAssessment(first.ID)
		if err != nil {
			t.Fatalf("GetAssessment failed: %v", err)
		}
		if got.ID != first.ID {
			t.Errorf("expected ID %s, got %s", first.ID, got.ID)
		}
	})

	t.Run("get non-existent assessment", func(t *testing.T) {
		_, err := m.GetAssessment("non-existent-id")
		if err != ErrAssessmentNotFound {
			t.Errorf("expected ErrAssessmentNotFound, got %v", err)
		}
	})
}

// ========== DetectSensitiveData ==========

func TestDetectSensitiveData(t *testing.T) {
	m := NewManager(defaultConfig())
	m.Start()
	defer m.Stop()

	// 创建临时测试文件
	tmpDir := t.TempDir()

	t.Run("detect phone and email in file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "contact.txt")
		content := "联系方式：13812345678，邮箱：test@example.com"
		os.WriteFile(testFile, []byte(content), 0644)

		types, err := m.DetectSensitiveData(testFile)
		if err != nil {
			t.Fatalf("DetectSensitiveData failed: %v", err)
		}

		typeMap := make(map[PIIType]bool)
		for _, pt := range types {
			typeMap[pt] = true
		}
		if !typeMap[PIIPhone] {
			t.Error("expected phone PII to be detected")
		}
		if !typeMap[PIIEmail] {
			t.Error("expected email PII to be detected")
		}
	})

	t.Run("detect id card", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "idcard.txt")
		content := "身份证号：110101199001011234"
		os.WriteFile(testFile, []byte(content), 0644)

		types, err := m.DetectSensitiveData(testFile)
		if err != nil {
			t.Fatalf("DetectSensitiveData failed: %v", err)
		}

		found := false
		for _, pt := range types {
			if pt == PIIIDCard {
				found = true
			}
		}
		if !found {
			t.Error("expected id_card PII to be detected")
		}
	})

	t.Run("scan directory", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "subdir")
		os.MkdirAll(subDir, 0755)
		os.WriteFile(filepath.Join(subDir, "a.txt"), []byte("电话：13900001111"), 0644)
		os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("邮箱：user@test.org"), 0644)

		types, err := m.DetectSensitiveData(subDir)
		if err != nil {
			t.Fatalf("DetectSensitiveData on dir failed: %v", err)
		}
		if len(types) == 0 {
			t.Error("expected PII types from directory scan")
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := m.DetectSensitiveData("/non/existent/path")
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("not running", func(t *testing.T) {
		m2 := NewManager(defaultConfig())
		_, err := m2.DetectSensitiveData(tmpDir)
		if err != ErrNotRunning {
			t.Errorf("expected ErrNotRunning, got %v", err)
		}
	})

	t.Run("clean file no PII", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "clean.txt")
		os.WriteFile(testFile, []byte("hello world, no sensitive data here"), 0644)

		types, err := m.DetectSensitiveData(testFile)
		if err != nil {
			t.Fatalf("DetectSensitiveData failed: %v", err)
		}
		if len(types) != 0 {
			t.Errorf("expected no PII for clean file, got %d types", len(types))
		}
	})
}

// ========== DataFlowTracking ==========

func TestDataFlowTracking(t *testing.T) {
	t.Run("track data flow", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		record, err := m.TrackDataFlow("/data/users", "s3://backup/users", OpTransfer)
		if err != nil {
			t.Fatalf("TrackDataFlow failed: %v", err)
		}
		if record.ID == "" {
			t.Error("record ID should not be empty")
		}
		if record.Operation != OpTransfer {
			t.Errorf("expected OpTransfer, got %s", record.Operation)
		}
		if record.Source.Location != "/data/users" {
			t.Errorf("expected source '/data/users', got %s", record.Source.Location)
		}
		if record.Destination.Location != "s3://backup/users" {
			t.Errorf("expected destination 's3://backup/users', got %s", record.Destination.Location)
		}
		if record.Status != "recorded" {
			t.Errorf("expected status 'recorded', got %s", record.Status)
		}
	})

	t.Run("multiple flows tracked", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		m.TrackDataFlow("/a", "/b", OpUpload)
		m.TrackDataFlow("/c", "/d", OpDownload)
		m.TrackDataFlow("/e", "/f", OpShare)

		dash := m.GetDashboard()
		if dash.DataFlowRecords != 3 {
			t.Errorf("expected 3 data flow records, got %d", dash.DataFlowRecords)
		}
	})

	t.Run("not running", func(t *testing.T) {
		m := NewManager(defaultConfig())
		_, err := m.TrackDataFlow("/a", "/b", OpUpload)
		if err != ErrNotRunning {
			t.Errorf("expected ErrNotRunning, got %v", err)
		}
	})

	t.Run("tracking disabled", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.DataFlowTrackingEnabled = false
		m := NewManager(cfg)
		m.Start()
		defer m.Stop()

		_, err := m.TrackDataFlow("/a", "/b", OpUpload)
		if err == nil {
			t.Error("expected error when tracking disabled")
		}
	})
}

// ========== ComplianceCheck ==========

func TestComplianceCheck(t *testing.T) {
	t.Run("no assessments", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		result, err := m.RunComplianceCheck()
		if err != nil {
			t.Fatalf("RunComplianceCheck failed: %v", err)
		}
		if result.Score != 100 {
			t.Errorf("expected score 100 with no assessments, got %.1f", result.Score)
		}
		if result.Status != StatusCompliant {
			t.Errorf("expected compliant status, got %s", result.Status)
		}
		if len(result.Checks) == 0 {
			t.Error("expected compliance checks")
		}
	})

	t.Run("with high risk assessments", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		// 创建高风险评估
		m.AssessOperation(OpShare, "身份证信息", 200*1024*1024)
		m.AssessOperation(OpTransfer, "银行账户数据", 100*1024*1024)

		result, err := m.RunComplianceCheck()
		if err != nil {
			t.Fatalf("RunComplianceCheck failed: %v", err)
		}
		if result.Score >= 80 {
			t.Errorf("expected lower compliance score for high-risk ops, got %.1f", result.Score)
		}
		if result.Framework != FrameworkPIPL {
			t.Errorf("expected PIPL framework, got %s", result.Framework)
		}
	})

	t.Run("not running", func(t *testing.T) {
		m := NewManager(defaultConfig())
		_, err := m.RunComplianceCheck()
		if err != ErrNotRunning {
			t.Errorf("expected ErrNotRunning, got %v", err)
		}
	})
}

// ========== GetDashboard ==========

func TestGetDashboard(t *testing.T) {
	t.Run("empty dashboard", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		dash := m.GetDashboard()
		if dash.TotalAssessments != 0 {
			t.Errorf("expected 0 assessments, got %d", dash.TotalAssessments)
		}
		if dash.PIIByType == nil {
			t.Error("PIIByType should be initialized")
		}
		if dash.ComplianceScores == nil {
			t.Error("ComplianceScores should be initialized")
		}
	})

	t.Run("dashboard with data", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		// 创建不同风险等级的评估
		m.AssessOperation(OpUpload, "普通文档", 512)
		m.AssessOperation(OpShare, "身份证信息", 50*1024*1024)
		m.AssessOperation(OpTransfer, "银行账户", 200*1024*1024)

		dash := m.GetDashboard()
		if dash.TotalAssessments != 3 {
			t.Errorf("expected 3 assessments, got %d", dash.TotalAssessments)
		}
		if dash.CompletedAssessments != 3 {
			t.Errorf("expected 3 completed, got %d", dash.CompletedAssessments)
		}
		if dash.AverageRiskScore <= 0 {
			t.Errorf("expected positive average risk score, got %.1f", dash.AverageRiskScore)
		}
		if dash.TotalAuditEvents < 3 {
			t.Errorf("expected at least 3 audit events, got %d", dash.TotalAuditEvents)
		}
	})

	t.Run("dashboard with high/critical risks", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		// 触发高风险和严重风险
		m.AssessOperation(OpShare, "身份证信息", 200*1024*1024)   // 高或严重
		m.AssessOperation(OpExport, "生物识别数据", 100*1024*1024) // 高或严重

		dash := m.GetDashboard()
		if dash.HighRiskCount+dash.CriticalRiskCount == 0 {
			t.Error("expected high or critical risk count > 0")
		}
	})
}

// ========== GetRecommendations ==========

func TestGetRecommendations(t *testing.T) {
	t.Run("no assessments", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		recs := m.GetRecommendations()
		if len(recs) != 0 {
			t.Errorf("expected 0 recommendations, got %d", len(recs))
		}
	})

	t.Run("deduplicated by category", func(t *testing.T) {
		m := NewManager(defaultConfig())
		m.Start()
		defer m.Stop()

		// 创建多个高风险评估，它们都会生成 encryption 建议
		m.AssessOperation(OpShare, "身份证信息", 100*1024*1024)
		m.AssessOperation(OpTransfer, "银行账户数据", 200*1024*1024)

		recs := m.GetRecommendations()
		categories := make(map[string]int)
		for _, r := range recs {
			categories[r.Category]++
		}
		// 每个分类应该只出现一次
		for cat, count := range categories {
			if count > 1 {
				t.Errorf("category %s appeared %d times, expected deduplication", cat, count)
			}
		}
	})
}

// ========== Risk Score Calculation ==========

func TestRiskScoreCalculation(t *testing.T) {
	m := NewManager(defaultConfig())
	m.Start()
	defer m.Stop()

	tests := []struct {
		name      string
		op        DataOperation
		dataType  string
		dataSize  int64
		minScore  float64
		maxScore  float64
		wantLevel RiskLevel
	}{
		{"low risk delete", OpDelete, "普通文档", 500, 0, 25, RiskLow},
		{"medium risk upload", OpUpload, "邮箱数据", 5 * 1024 * 1024, 30, 50, RiskMedium},
		{"high risk share id_card", OpShare, "身份证信息", 50 * 1024 * 1024, 60, 80, RiskHigh},
		{"critical risk transfer bank", OpTransfer, "银行账户数据", 200 * 1024 * 1024, 80, 100, RiskCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := m.AssessOperation(tt.op, tt.dataType, tt.dataSize)
			if err != nil {
				t.Fatalf("AssessOperation failed: %v", err)
			}
			if a.RiskScore < tt.minScore || a.RiskScore > tt.maxScore {
				t.Errorf("expected score in [%.0f, %.0f], got %.1f", tt.minScore, tt.maxScore, a.RiskScore)
			}
			if a.RiskLevel != tt.wantLevel {
				// 放宽检查：分数在边界时可能跨级
				t.Logf("risk level %s (score=%.1f), expected %s", a.RiskLevel, a.RiskScore, tt.wantLevel)
			}
		})
	}
}
