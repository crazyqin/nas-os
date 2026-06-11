package compliance

import (
	"testing"
)

func TestNewComplianceScanner(t *testing.T) {
	scanner := NewComplianceScanner()

	if scanner == nil {
		t.Fatal("NewComplianceScanner should not return nil")
	}

	if scanner.checks == nil {
		t.Error("checks map should be initialized")
	}

	if scanner.stigChecks == nil {
		t.Error("stigChecks map should be initialized")
	}

	if scanner.gdprArticles == nil {
		t.Error("gdprArticles map should be initialized")
	}
}

func TestCISBenchmarkInitialization(t *testing.T) {
	scanner := NewComplianceScanner()

	benchmark := scanner.GetCISBenchmark()
	if benchmark == nil {
		t.Fatal("CIS benchmark should be initialized")
	}

	if benchmark.ID != "cis-nas-os-v1.0" {
		t.Errorf("expected benchmark ID 'cis-nas-os-v1.0', got '%s'", benchmark.ID)
	}

	if benchmark.Name != "CIS NAS-OS Benchmark" {
		t.Errorf("expected benchmark name 'CIS NAS-OS Benchmark', got '%s'", benchmark.Name)
	}

	if benchmark.Level != 1 {
		t.Errorf("expected level 1, got %d", benchmark.Level)
	}
}

func TestRunCISCheck(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunCISCheck()

	if report == nil {
		t.Fatal("CIS report should not be nil")
	}

	if report.ID == "" {
		t.Error("report ID should not be empty")
	}

	if report.Standard != StandardCIS {
		t.Errorf("expected standard CIS, got %s", report.Standard)
	}

	if len(report.Results) == 0 {
		t.Error("CIS report should have results")
	}

	// 检查摘要
	if report.Summary.TotalChecks != len(report.Results) {
		t.Errorf("total checks mismatch: expected %d, got %d", len(report.Results), report.Summary.TotalChecks)
	}
}

func TestRunSTIGCheck(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunSTIGCheck()

	if report == nil {
		t.Fatal("STIG report should not be nil")
	}

	if report.Standard != StandardSTIG {
		t.Errorf("expected standard STIG, got %s", report.Standard)
	}

	if len(report.Results) == 0 {
		t.Error("STIG report should have results")
	}
}

func TestRunGDPRCheck(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunGDPRCheck()

	if report == nil {
		t.Fatal("GDPR report should not be nil")
	}

	if report.Standard != StandardGDPR {
		t.Errorf("expected standard GDPR, got %s", report.Standard)
	}

	if len(report.Results) == 0 {
		t.Error("GDPR report should have results")
	}
}

func TestRunFullComplianceScan(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunFullComplianceScan()

	if report == nil {
		t.Fatal("full compliance report should not be nil")
	}

	if report.ID == "" {
		t.Error("report ID should not be empty")
	}

	if report.CISReport == nil {
		t.Error("CIS report should not be nil")
	}

	if report.STIGReport == nil {
		t.Error("STIG report should not be nil")
	}

	if report.GDPRReport == nil {
		t.Error("GDPR report should not be nil")
	}

	if report.OverallScore < 0 || report.OverallScore > 100 {
		t.Errorf("overall score should be between 0 and 100, got %f", report.OverallScore)
	}

	if len(report.Recommendations) == 0 {
		t.Error("report should have recommendations")
	}
}

func TestGetComplianceChecks(t *testing.T) {
	scanner := NewComplianceScanner()

	checks := scanner.GetComplianceChecks()

	if len(checks) == 0 {
		t.Error("should have compliance checks")
	}

	// 检查是否包含 CIS 检查
	foundCIS := false
	for _, check := range checks {
		if check.Standard == StandardCIS {
			foundCIS = true
			break
		}
	}

	if !foundCIS {
		t.Error("should contain CIS checks")
	}
}

func TestGetSTIGChecks(t *testing.T) {
	scanner := NewComplianceScanner()

	checks := scanner.GetSTIGChecks()

	if len(checks) == 0 {
		t.Error("should have STIG checks")
	}

	// 检查是否有预期的检查项
	found := false
	for _, check := range checks {
		if check.ID == "STIG-V-230221" {
			found = true
			break
		}
	}

	if !found {
		t.Error("should contain STIG-V-230221")
	}
}

func TestGetGDPRArticles(t *testing.T) {
	scanner := NewComplianceScanner()

	articles := scanner.GetGDPRArticles()

	if len(articles) == 0 {
		t.Error("should have GDPR articles")
	}

	// 检查是否有预期的条款
	found := false
	for _, article := range articles {
		if article.Article == "Article 5" {
			found = true
			break
		}
	}

	if !found {
		t.Error("should contain Article 5")
	}
}

func TestCISCheckResultStatuses(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunCISCheck()

	compliant := 0
	nonCompliant := 0

	for _, result := range report.Results {
		switch result.Status {
		case StatusCompliant:
			compliant++
		case StatusNonCompliant:
			nonCompliant++
		}
	}

	// 验证摘要计数
	if report.Summary.Compliant != compliant {
		t.Errorf("compliant count mismatch: expected %d, got %d", compliant, report.Summary.Compliant)
	}

	if report.Summary.NonCompliant != nonCompliant {
		t.Errorf("non-compliant count mismatch: expected %d, got %d", nonCompliant, report.Summary.NonCompliant)
	}
}

func TestSTIGCheckVulnIDs(t *testing.T) {
	scanner := NewComplianceScanner()

	checks := scanner.GetSTIGChecks()

	for _, check := range checks {
		if check.VulnID == "" {
			t.Errorf("STIG check %s should have VulnID", check.ID)
		}

		if check.GroupID == "" {
			t.Errorf("STIG check %s should have GroupID", check.ID)
		}

		if check.Severity == "" {
			t.Errorf("STIG check %s should have Severity", check.ID)
		}
	}
}

func TestGDPRRequirements(t *testing.T) {
	scanner := NewComplianceScanner()

	articles := scanner.GetGDPRArticles()

	for _, article := range articles {
		if len(article.Requirements) == 0 {
			t.Errorf("GDPR article %s should have requirements", article.Article)
		}
	}
}

func TestComplianceSummaryCalculation(t *testing.T) {
	results := []ComplianceCheckResult{
		{Status: StatusCompliant},
		{Status: StatusCompliant},
		{Status: StatusNonCompliant},
		{Status: StatusPartial},
		{Status: StatusNotApplicable},
	}

	scanner := NewComplianceScanner()
	summary := scanner.calculateSummary(results)

	if summary.TotalChecks != 5 {
		t.Errorf("expected 5 total checks, got %d", summary.TotalChecks)
	}

	if summary.Compliant != 2 {
		t.Errorf("expected 2 compliant, got %d", summary.Compliant)
	}

	if summary.NonCompliant != 1 {
		t.Errorf("expected 1 non-compliant, got %d", summary.NonCompliant)
	}

	if summary.Partial != 1 {
		t.Errorf("expected 1 partial, got %d", summary.Partial)
	}

	if summary.NotApplicable != 1 {
		t.Errorf("expected 1 not-applicable, got %d", summary.NotApplicable)
	}

	// 合规率 = 2/5 * 100 = 40%
	expectedRate := 40.0
	if summary.ComplianceRate != expectedRate {
		t.Errorf("expected compliance rate %f, got %f", expectedRate, summary.ComplianceRate)
	}
}

func TestComplianceStandardConstants(t *testing.T) {
	tests := []struct {
		standard ComplianceStandard
		expected string
	}{
		{StandardCIS, "CIS"},
		{StandardSTIG, "STIG"},
		{StandardGDPR, "GDPR"},
		{StandardCCPA, "CCPA"},
		{StandardHIPAA, "HIPAA"},
		{StandardSOC2, "SOC2"},
		{StandardISO27001, "ISO27001"},
	}

	for _, tt := range tests {
		if string(tt.standard) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.standard))
		}
	}
}

func TestComplianceCheckStatusConstants(t *testing.T) {
	tests := []struct {
		status   ComplianceCheckStatus
		expected string
	}{
		{StatusCompliant, "compliant"},
		{StatusNonCompliant, "non-compliant"},
		{StatusPartial, "partial"},
		{StatusNotApplicable, "not-applicable"},
		{StatusError, "error"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

func TestBenchmarkCategoryConstants(t *testing.T) {
	tests := []struct {
		category BenchmarkCategory
		expected string
	}{
		{CategoryInitialSetup, "initial_setup"},
		{CategoryServices, "services"},
		{CategoryNetworkConfig, "network_config"},
		{CategoryLogging, "logging"},
		{CategoryAccessControl, "access_control"},
		{CategoryDataProtection, "data_protection"},
		{CategoryPrivacy, "privacy"},
		{CategoryEncryption, "encryption"},
	}

	for _, tt := range tests {
		if string(tt.category) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.category))
		}
	}
}

func TestFullComplianceReportRecommendations(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunFullComplianceScan()

	// 应该有一些不合规项的建议
	hasRecommendations := false
	for _, rec := range report.Recommendations {
		if rec != "" {
			hasRecommendations = true
			break
		}
	}

	if !hasRecommendations {
		t.Error("should have recommendations")
	}
}

func TestOverallScoreCalculation(t *testing.T) {
	scanner := NewComplianceScanner()

	report := scanner.RunFullComplianceScan()

	// 计算预期的总体分数
	totalChecks := report.CISReport.Summary.TotalChecks +
		report.STIGReport.Summary.TotalChecks +
		report.GDPRReport.Summary.TotalChecks

	totalCompliant := report.CISReport.Summary.Compliant +
		report.STIGReport.Summary.Compliant +
		report.GDPRReport.Summary.Compliant

	expectedScore := float64(0)
	if totalChecks > 0 {
		expectedScore = float64(totalCompliant) / float64(totalChecks) * 100
	}

	if report.OverallScore != expectedScore {
		t.Errorf("expected overall score %f, got %f", expectedScore, report.OverallScore)
	}
}

func TestComplianceCheckHasSeverity(t *testing.T) {
	scanner := NewComplianceScanner()

	checks := scanner.GetComplianceChecks()

	for _, check := range checks {
		if check.Severity == "" {
			t.Errorf("check %s should have severity", check.ID)
		}
	}
}

func TestComplianceCheckHasRemediation(t *testing.T) {
	scanner := NewComplianceScanner()

	checks := scanner.GetComplianceChecks()

	for _, check := range checks {
		if check.Remediation == "" {
			t.Errorf("check %s should have remediation", check.ID)
		}
	}
}
