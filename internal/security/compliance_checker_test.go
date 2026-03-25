package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultComplianceCheckerConfig(t *testing.T) {
	config := DefaultComplianceCheckerConfig()

	assert.True(t, config.AutoCheck)
	assert.True(t, config.Enabled)
	assert.True(t, len(config.EnabledStandards) >= 0)
}

func TestNewComplianceChecker(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	assert.NotNil(t, cc)
	assert.NotNil(t, cc.standards)
	assert.NotNil(t, cc.reports)
}

func TestComplianceChecker_GetReport_NotFound(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	report, err := cc.GetReport("non-existent-id")
	assert.Nil(t, report)
	assert.NotNil(t, err)
}

func TestComplianceChecker_GetLatestReport_NoReports(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	report, err := cc.GetLatestReport(StandardGDPR)
	assert.Nil(t, report)
	assert.NotNil(t, err)
}

func TestComplianceChecker_ListReports_Empty(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	reports := cc.ListReports(StandardGDPR, 10)
	assert.Empty(t, reports)
}

func TestComplianceChecker_InitStandards(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	config.EnabledStandards = []ComplianceStandard{StandardGDPR, StandardSOC2}
	cc := NewComplianceChecker(config)

	// 验证标准已初始化
	assert.NotNil(t, cc.standards[StandardGDPR])
	assert.NotNil(t, cc.standards[StandardSOC2])
}

func TestComplianceChecker_RunCheck_InvalidItem(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	ctx := context.Background()
	result, err := cc.RunCheck(ctx, "invalid-item-id")

	assert.Nil(t, result)
	assert.NotNil(t, err)
}

func TestComplianceChecker_DetermineLevel(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	tests := []struct {
		score    int
		expected ComplianceLevel
	}{
		{95, LevelFull},
		{90, LevelFull},
		{75, LevelPartial},
		{60, LevelPartial},
		{50, LevelNonCompliant},
		{30, LevelNonCompliant},
	}

	for _, tt := range tests {
		level := cc.determineLevel(tt.score)
		assert.Equal(t, tt.expected, level)
	}
}

func TestComplianceChecker_CalculateItemScore(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:     "test-item",
		Weight: 10,
	}

	tests := []struct {
		status   ComplianceStatus
		expected int
	}{
		{StatusPassed, 100},
		{StatusWarning, 70},
		{StatusFailed, 0},
		{StatusSkipped, -1},
		{StatusNotApplicable, -1},
	}

	for _, tt := range tests {
		score := cc.calculateItemScore(item, tt.status)
		assert.Equal(t, tt.expected, score)
	}
}

func TestComplianceChecker_GetCheckItem_NotFound(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := cc.getCheckItem("non-existent-item")
	assert.Nil(t, item)
}

func TestComplianceChecker_CalculateCategoryScore(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	results := []*ComplianceCheckResult{
		{Category: CategoryAccessControl, Score: 80},
		{Category: CategoryAccessControl, Score: 90},
		{Category: CategoryEncryption, Score: 70},
	}

	score := cc.calculateCategoryScore(results)
	assert.True(t, score >= 0 && score <= 100)
}

func TestComplianceChecker_CalculateOverallScore(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	results := []*ComplianceCheckResult{
		{Score: 80, Status: StatusPassed},
		{Score: 90, Status: StatusPassed},
	}

	items := []*ComplianceCheckItem{
		{ID: "item1", Weight: 10},
		{ID: "item2", Weight: 10},
	}

	score := cc.calculateOverallScore(results, items)
	assert.True(t, score >= 0 && score <= 100)
}

func TestComplianceChecker_GetOverallLevel(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	tests := []struct {
		score    int
		expected ComplianceLevel
	}{
		{95, LevelFull},
		{90, LevelFull},
		{80, LevelPartial},
		{70, LevelPartial},
		{60, LevelNonCompliant},
		{50, LevelNonCompliant},
		{40, LevelUnknown},
		{0, LevelUnknown},
	}

	for _, tt := range tests {
		level := cc.getOverallLevel(tt.score)
		assert.Equal(t, tt.expected, level)
	}
}

func TestComplianceChecker_ExecuteCheck(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	ctx := context.Background()
	item := &ComplianceCheckItem{
		ID:          "test-item",
		Name:        "Test Check",
		Category:    CategoryAccessControl,
		Description: "Test check item",
	}

	status, message, details := cc.executeCheck(ctx, item)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, details)
}

func TestComplianceChecker_CheckAccessControl(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "access-001",
		Name:     "Access Control Test",
		Category: CategoryAccessControl,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkAccessControl(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckEncryption(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "enc-001",
		Name:     "Encryption Test",
		Category: CategoryEncryption,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkEncryption(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckAudit(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "audit-001",
		Name:     "Audit Test",
		Category: CategoryAudit,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkAudit(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckDataProtection(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "dp-001",
		Name:     "Data Protection Test",
		Category: CategoryDataProtection,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkDataProtection(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckVulnerability(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "vuln-001",
		Name:     "Vulnerability Test",
		Category: CategoryVulnerability,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkVulnerability(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckIncidentResponse(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "ir-001",
		Name:     "Incident Response Test",
		Category: CategoryIncidentResponse,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkIncidentResponse(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckBreachNotification(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "bn-001",
		Name:     "Breach Notification Test",
		Category: CategoryBreachNotification,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkBreachNotification(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

func TestComplianceChecker_CheckPrivacy(t *testing.T) {
	config := DefaultComplianceCheckerConfig()
	cc := NewComplianceChecker(config)

	item := &ComplianceCheckItem{
		ID:       "privacy-001",
		Name:     "Privacy Test",
		Category: CategoryPrivacy,
	}

	details := make(map[string]interface{})
	status, message, resultDetails := cc.checkPrivacy(item, details)

	assert.NotEmpty(t, status)
	assert.NotEmpty(t, message)
	assert.NotNil(t, resultDetails)
}

// 测试类型常量.
func TestComplianceStandard_Constants(t *testing.T) {
	assert.Equal(t, ComplianceStandard("gdpr"), StandardGDPR)
	assert.Equal(t, ComplianceStandard("soc2"), StandardSOC2)
	assert.Equal(t, ComplianceStandard("iso27001"), StandardISO27001)
	assert.Equal(t, ComplianceStandard("hipaa"), StandardHIPAA)
	assert.Equal(t, ComplianceStandard("pci"), StandardPCI)
	assert.Equal(t, ComplianceStandard("csl"), StandardCSL)
	assert.Equal(t, ComplianceStandard("pipl"), StandardPIPL)
}

func TestComplianceLevel_Constants(t *testing.T) {
	assert.Equal(t, ComplianceLevel("full"), LevelFull)
	assert.Equal(t, ComplianceLevel("partial"), LevelPartial)
	assert.Equal(t, ComplianceLevel("non_compliant"), LevelNonCompliant)
	assert.Equal(t, ComplianceLevel("unknown"), LevelUnknown)
}

func TestComplianceStatus_Constants(t *testing.T) {
	assert.Equal(t, ComplianceStatus("passed"), StatusPassed)
	assert.Equal(t, ComplianceStatus("failed"), StatusFailed)
	assert.Equal(t, ComplianceStatus("warning"), StatusWarning)
	assert.Equal(t, ComplianceStatus("skipped"), StatusSkipped)
	assert.Equal(t, ComplianceStatus("not_applicable"), StatusNotApplicable)
}

func TestComplianceCategory_Constants(t *testing.T) {
	assert.Equal(t, ComplianceCategory("access_control"), CategoryAccessControl)
	assert.Equal(t, ComplianceCategory("data_protection"), CategoryDataProtection)
	assert.Equal(t, ComplianceCategory("encryption"), CategoryEncryption)
	assert.Equal(t, ComplianceCategory("audit"), CategoryAudit)
}

func TestComplianceCheckItem_Fields(t *testing.T) {
	item := &ComplianceCheckItem{
		ID:              "test-001",
		Standard:        StandardGDPR,
		ControlID:       "GDPR-5.1",
		Category:        CategoryDataProtection,
		Name:            "Data Processing Lawfulness",
		Description:     "Ensure data processing has legal basis",
		Requirement:     "Article 5 GDPR",
		Weight:          10,
		Severity:        "high",
		Remediation:     "Review data processing activities",
		References:      []string{"GDPR Art. 5", "GDPR Art. 6"},
		ApplicableRoles: []string{"admin", "data_controller"},
		Tags:            []string{"privacy", "legal"},
	}

	assert.Equal(t, "test-001", item.ID)
	assert.Equal(t, StandardGDPR, item.Standard)
	assert.Equal(t, "GDPR-5.1", item.ControlID)
	assert.Equal(t, 10, item.Weight)
	assert.Equal(t, "high", item.Severity)
}

func TestComplianceCheckResult_Fields(t *testing.T) {
	result := &ComplianceCheckResult{
		ItemID:    "item-001",
		Standard:  StandardGDPR,
		ControlID: "GDPR-5.1",
		Category:  CategoryDataProtection,
		Status:    StatusPassed,
		Score:     90,
		Message:   "All checks passed",
		Details:   map[string]interface{}{"test": "evidence"},
	}

	assert.Equal(t, "item-001", result.ItemID)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, 90, result.Score)
}

func TestComplianceReport_Fields(t *testing.T) {
	report := &ComplianceReport{
		ReportID:     "report-001",
		Standard:     StandardGDPR,
		OverallLevel: LevelFull,
		OverallScore: 95,
		Summary: ComplianceSummary{
			TotalChecks:   10,
			PassedChecks:  9,
			WarningChecks: 1,
			FailedChecks:  0,
			SkippedChecks: 0,
		},
	}

	assert.Equal(t, "report-001", report.ReportID)
	assert.Equal(t, StandardGDPR, report.Standard)
	assert.Equal(t, LevelFull, report.OverallLevel)
	assert.Equal(t, 95, report.OverallScore)
}

func TestComplianceSummary_Fields(t *testing.T) {
	summary := ComplianceSummary{
		TotalChecks:    100,
		PassedChecks:   80,
		WarningChecks:  10,
		FailedChecks:   5,
		SkippedChecks:  3,
		NotApplicable:  2,
		CriticalIssues: 2,
		HighIssues:     5,
		MediumIssues:   8,
		LowIssues:      10,
	}

	assert.Equal(t, 100, summary.TotalChecks)
	assert.Equal(t, 80, summary.PassedChecks)
	assert.Equal(t, 10, summary.WarningChecks)
	assert.Equal(t, 5, summary.FailedChecks)
}

func TestComplianceCheckerConfig_Fields(t *testing.T) {
	config := ComplianceCheckerConfig{
		Enabled:          true,
		AutoCheck:        true,
		CheckInterval:    3600000000000,
		ReportRetention:  30,
		MaxReports:       100,
		NotifyOnFailure:  true,
		NotifyChannels:   []string{"email", "webhook"},
		EnabledStandards: []ComplianceStandard{StandardGDPR, StandardSOC2},
	}

	assert.True(t, config.Enabled)
	assert.True(t, config.AutoCheck)
	assert.Equal(t, 30, config.ReportRetention)
	assert.Equal(t, 100, config.MaxReports)
}
