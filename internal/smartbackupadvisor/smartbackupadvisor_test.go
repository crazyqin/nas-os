package smartbackupadvisor

import (
	"testing"
	"time"
)

func TestSmartBackupAdvisor_AssessDataRisk(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	lastBackup := time.Now().Add(-48 * time.Hour)
	assessment := sba.AssessDataRisk("/data/critical.db", 1024*1024*100, CriticalityHigh, &lastBackup)

	if assessment.RiskLevel == "" {
		t.Error("Expected risk level to be set")
	}

	if assessment.RiskScore == 0 {
		t.Error("Expected risk score to be non-zero")
	}

	if assessment.RecommendedPolicy == nil {
		t.Error("Expected recommended policy")
	}
}

func TestSmartBackupAdvisor_CreatePolicy(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	policy := &BackupPolicy{
		ID:             "policy-1",
		Name:           "每日备份",
		Strategy:       StrategyIncremental,
		FrequencyHours: 24,
		RetentionDays:  30,
		RPOMinutes:     60,
		RTOMinutes:     240,
	}

	err := sba.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
}

func TestSmartBackupAdvisor_CreateDRPlan(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	plan := GenerateDefaultDRPlan("生产环境灾难恢复")
	err := sba.CreateDRPlan(plan)
	if err != nil {
		t.Fatalf("Failed to create DR plan: %v", err)
	}
}

func TestSmartBackupAdvisor_VerifyBackup(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	verification, err := sba.VerifyBackup("backup-123")
	if err != nil {
		t.Fatalf("Failed to verify backup: %v", err)
	}

	if !verification.IntegrityOK {
		t.Error("Expected integrity to be OK")
	}

	if !verification.RestoreTest {
		t.Error("Expected restore test to pass")
	}
}

func TestSmartBackupAdvisor_GetHighRiskItems(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	// 添加高风险评估
	lastBackup := time.Now().Add(-720 * time.Hour) // 30天前
	sba.AssessDataRisk("/data/important.db", 1024*1024*500, CriticalityCritical, &lastBackup)

	items := sba.GetHighRiskItems()
	if len(items) == 0 {
		t.Error("Expected at least one high risk item")
	}
}

func TestSmartBackupAdvisor_Stats(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	// 添加一些数据
	sba.CreatePolicy(&BackupPolicy{ID: "p1", Name: "Policy 1"})
	sba.AssessDataRisk("/data/file1", 1024, CriticalityLow, nil)
	sba.AssessDataRisk("/data/file2", 1024*1024, CriticalityHigh, nil)

	stats := sba.GetStats()

	if stats.TotalPolicies != 1 {
		t.Errorf("Expected 1 policy, got %d", stats.TotalPolicies)
	}

	if stats.TotalAssessments != 2 {
		t.Errorf("Expected 2 assessments, got %d", stats.TotalAssessments)
	}
}

func TestRiskLevel_Constants(t *testing.T) {
	levels := []RiskLevel{RiskLow, RiskMedium, RiskHigh, RiskCritical}
	for _, l := range levels {
		if l == "" {
			t.Error("RiskLevel constant should not be empty")
		}
	}
}

func TestBackupStrategy_Constants(t *testing.T) {
	strategies := []BackupStrategy{
		StrategyFull, StrategyIncremental, StrategyDifferential,
		StrategyMirror, StrategySnapshot,
	}
	for _, s := range strategies {
		if s == "" {
			t.Error("BackupStrategy constant should not be empty")
		}
	}
}

func TestEstimateRecoveryTime(t *testing.T) {
	// 100GB 全量备份
	time1 := EstimateRecoveryTime(100, StrategyFull)
	if time1 != 60 { // 100GB / 100GB/h = 1h = 60min
		t.Errorf("Expected 60 minutes, got %d", time1)
	}

	// 100GB 快照恢复
	time2 := EstimateRecoveryTime(100, StrategySnapshot)
	if time2 != 30 { // 最少30分钟
		t.Errorf("Expected 30 minutes, got %d", time2)
	}

	// 500GB 增量备份
	time3 := EstimateRecoveryTime(500, StrategyIncremental)
	if time3 == 0 {
		t.Error("Expected non-zero recovery time")
	}
}

func TestGenerateDefaultDRPlan(t *testing.T) {
	plan := GenerateDefaultDRPlan("测试灾难恢复")

	if plan.Name != "测试灾难恢复" {
		t.Errorf("Expected name '测试灾难恢复', got '%s'", plan.Name)
	}

	if len(plan.BackupSites) == 0 {
		t.Error("Expected backup sites")
	}

	if len(plan.RecoverySteps) == 0 {
		t.Error("Expected recovery steps")
	}

	if len(plan.Contacts) == 0 {
		t.Error("Expected contacts")
	}
}

func TestSmartBackupAdvisor_MarshalJSON(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	data, err := sba.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestRiskFactor_Calculation(t *testing.T) {
	sba := NewSmartBackupAdvisor(nil)

	// 测试关键性评分
	tests := []struct {
		criticality DataCriticality
		expected    float64
	}{
		{CriticalityLow, 0.2},
		{CriticalityMedium, 0.5},
		{CriticalityHigh, 0.8},
		{CriticalityCritical, 1.0},
	}

	for _, tt := range tests {
		score := sba.calculateCriticalityScore(tt.criticality)
		if score != tt.expected {
			t.Errorf("calculateCriticalityScore(%s): expected %f, got %f", tt.criticality, tt.expected, score)
		}
	}
}
