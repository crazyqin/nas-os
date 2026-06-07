package smartbackupadvisor

import (
	"testing"
	"time"
)

func TestNewAdvisor(t *testing.T) {
	advisor := NewAdvisor()
	if advisor == nil {
		t.Fatal("NewAdvisor returned nil")
	}
}

func TestRegisterSource(t *testing.T) {
	advisor := NewAdvisor()

	source := &DataSource{
		Name:       "重要文档",
		Type:       "file",
		SizeGB:     100,
		ChangeRate: 0.1,
		Importance: 8,
		Encrypted:  true,
		Compressed: true,
	}

	if err := advisor.RegisterSource(source); err != nil {
		t.Fatalf("RegisterSource failed: %v", err)
	}

	if source.ID == "" {
		t.Error("Source ID not generated")
	}

	sources := advisor.GetSources()
	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}
}

func TestCreatePlan(t *testing.T) {
	advisor := NewAdvisor()

	plan := &BackupPlan{
		Name:          "每日全量备份",
		Strategy:      AdvisorStrategyFull,
		Schedule:      "0 2 * * *",
		Destination:   "/backup/daily",
		RetentionDays: 30,
		Enabled:       true,
	}

	if err := advisor.CreatePlan(plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan.ID == "" {
		t.Error("Plan ID not generated")
	}

	plans := advisor.GetPlans()
	if len(plans) != 1 {
		t.Errorf("Expected 1 plan, got %d", len(plans))
	}
}

func TestAssessAdvisorRiskHigh(t *testing.T) {
	advisor := NewAdvisor()

	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	source := &DataSource{
		Name:       "数据库",
		Type:       "database",
		SizeGB:     500,
		ChangeRate: 0.5,
		Importance: 9,
		LastBackup: &oldTime,
		Encrypted:  false,
		Compressed: false,
	}

	advisor.RegisterSource(source)

	assessment, err := advisor.AssessRisk(source.ID)
	if err != nil {
		t.Fatalf("AssessRisk failed: %v", err)
	}

	if assessment.AdvisorRiskLevel != AdvisorRiskHigh && assessment.AdvisorRiskLevel != AdvisorRiskCritical {
		t.Errorf("Expected high or critical risk, got %s", assessment.AdvisorRiskLevel)
	}

	if assessment.RiskScore < 30 {
		t.Errorf("Expected risk score >= 30, got %f", assessment.RiskScore)
	}
}

func TestAssessAdvisorRiskLow(t *testing.T) {
	advisor := NewAdvisor()

	recentTime := time.Now().Add(-1 * 24 * time.Hour)
	source := &DataSource{
		Name:       "临时文件",
		Type:       "file",
		SizeGB:     10,
		ChangeRate: 0.01,
		Importance: 2,
		LastBackup: &recentTime,
		Encrypted:  true,
		Compressed: true,
	}

	advisor.RegisterSource(source)

	assessment, err := advisor.AssessRisk(source.ID)
	if err != nil {
		t.Fatalf("AssessRisk failed: %v", err)
	}

	if assessment.AdvisorRiskLevel != AdvisorRiskLow {
		t.Errorf("Expected low risk, got %s", assessment.AdvisorRiskLevel)
	}
}

func TestRecommendStrategy(t *testing.T) {
	advisor := NewAdvisor()

	tests := []struct {
		name       string
		changeRate float64
		sizeGB     float64
		expected   AdvisorBackupStrategy
	}{
		{"低变更率", 0.01, 50, AdvisorStrategyMirror},
		{"中变更率", 0.1, 50, AdvisorStrategyIncremental},
		{"高变更率大容量", 0.5, 200, AdvisorStrategyDifferential},
		{"高变更率小容量", 0.5, 50, AdvisorStrategyFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &DataSource{
				Name:       tt.name,
				SizeGB:     tt.sizeGB,
				ChangeRate: tt.changeRate,
			}
			advisor.RegisterSource(source)

			strategy, reason, err := advisor.RecommendStrategy(source.ID)
			if err != nil {
				t.Fatalf("RecommendStrategy failed: %v", err)
			}

			if strategy != tt.expected {
				t.Errorf("Expected strategy %s, got %s", tt.expected, strategy)
			}

			if reason == "" {
				t.Error("Expected non-empty recommendation reason")
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	advisor := NewAdvisor()

	// 注册多个数据源
	advisor.RegisterSource(&DataSource{
		Name: "数据源1", Type: "file", SizeGB: 100,
		LastBackup: timePtr(time.Now().Add(-1 * 24 * time.Hour)),
	})
	advisor.RegisterSource(&DataSource{
		Name: "数据源2", Type: "database", SizeGB: 200,
	})
	advisor.RegisterSource(&DataSource{
		Name: "数据源3", Type: "vm", SizeGB: 150,
		LastBackup: timePtr(time.Now().Add(-10 * 24 * time.Hour)),
	})

	report := advisor.GenerateReport()

	if report.TotalSources != 3 {
		t.Errorf("Expected 3 total sources, got %d", report.TotalSources)
	}

	if report.BackedUp != 1 {
		t.Errorf("Expected 1 backed up, got %d", report.BackedUp)
	}

	if report.AtRisk != 2 {
		t.Errorf("Expected 2 at risk, got %d", report.AtRisk)
	}
}

func TestAssessRiskNotFound(t *testing.T) {
	advisor := NewAdvisor()

	_, err := advisor.AssessRisk("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent source")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
