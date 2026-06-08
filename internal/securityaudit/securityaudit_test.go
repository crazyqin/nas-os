package securityaudit

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	config := mgr.GetConfig()
	if !config.Enabled {
		t.Error("Expected config to be enabled by default")
	}
	if !config.AutoScan {
		t.Error("Expected AutoScan to be enabled by default")
	}
}

func TestGetSecurityScore(t *testing.T) {
	mgr := NewManager()
	score := mgr.GetSecurityScore()

	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("Score out of range: %d", score.Overall)
	}
	if score.Grade == "" {
		t.Error("Grade should not be empty")
	}
	if score.CalculatedAt.IsZero() {
		t.Error("CalculatedAt should be set")
	}
}

func TestRunSecurityChecks(t *testing.T) {
	mgr := NewManager()
	results := mgr.RunSecurityChecks()

	if len(results) == 0 {
		t.Error("Expected some check results")
	}

	for _, result := range results {
		if result.CheckID == "" {
			t.Error("Check ID should not be empty")
		}
		if result.Name == "" {
			t.Error("Check name should not be empty")
		}
		if result.Status == "" {
			t.Error("Check status should not be empty")
		}
	}
}

func TestRunSecurityChecksByCategory(t *testing.T) {
	mgr := NewManager()

	categories := []SecurityCheckCategory{
		CategoryAuth,
		CategoryNetwork,
		CategorySystem,
	}

	for _, category := range categories {
		results := mgr.RunSecurityChecksByCategory(category)
		for _, result := range results {
			if result.Category != category {
				t.Errorf("Expected category %s, got %s", category, result.Category)
			}
		}
	}
}

func TestVulnerabilityScan(t *testing.T) {
	mgr := NewManager()
	report := mgr.RunVulnerabilityScan()

	if report.ReportID == "" {
		t.Error("Report ID should not be empty")
	}
	if report.TotalFound < 0 {
		t.Error("Total found should not be negative")
	}
	if report.ScanTime.IsZero() {
		t.Error("Scan time should be set")
	}
}

func TestGetVulnerabilities(t *testing.T) {
	mgr := NewManager()
	_ = mgr.RunVulnerabilityScan()

	vulns := mgr.GetVulnerabilities("", "")
	if len(vulns) == 0 {
		t.Error("Expected some vulnerabilities")
	}
}

func TestUpdateVulnerabilityStatus(t *testing.T) {
	mgr := NewManager()
	_ = mgr.RunVulnerabilityScan()

	vulns := mgr.GetVulnerabilities("", "")
	if len(vulns) == 0 {
		t.Skip("No vulnerabilities to test")
	}

	err := mgr.UpdateVulnerabilityStatus(vulns[0].ID, VulnStatusFixed, "test-user")
	if err != nil {
		t.Errorf("Failed to update vulnerability status: %v", err)
	}
}

func TestHardeningSuggestions(t *testing.T) {
	mgr := NewManager()
	suggestions := mgr.GetHardeningSuggestions()

	if len(suggestions) == 0 {
		t.Error("Expected some hardening suggestions")
	}

	for _, s := range suggestions {
		if s.ID == "" {
			t.Error("Suggestion ID should not be empty")
		}
		if s.Title == "" {
			t.Error("Suggestion title should not be empty")
		}
		if s.Priority == "" {
			t.Error("Suggestion priority should not be empty")
		}
	}
}

func TestHardeningReport(t *testing.T) {
	mgr := NewManager()
	report := mgr.GetHardeningReport()

	if report.ReportID == "" {
		t.Error("Report ID should not be empty")
	}
	if report.TotalItems < 0 {
		t.Error("Total items should not be negative")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should be set")
	}
}

func TestApplyHardeningSuggestion(t *testing.T) {
	mgr := NewManager()
	suggestions := mgr.GetHardeningSuggestions()

	if len(suggestions) == 0 {
		t.Skip("No suggestions to test")
	}

	err := mgr.ApplyHardeningSuggestion(suggestions[0].ID, "test-user")
	if err != nil {
		t.Errorf("Failed to apply suggestion: %v", err)
	}
}

func TestAuditLogs(t *testing.T) {
	mgr := NewManager()

	// 记录一些事件
	mgr.LogEvent(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityMedium,
		Actor:     "test-user",
		Action:    "test_action",
		Status:    "success",
		Message:   "Test event",
	})

	// 获取日志
	logs := mgr.GetAuditLogs(10, 0, nil)
	if len(logs) == 0 {
		t.Error("Expected some audit logs")
	}
}

func TestAuditReport(t *testing.T) {
	mgr := NewManager()

	// 记录一些事件
	for i := 0; i < 5; i++ {
		mgr.LogEvent(AuditEvent{
			EventType: EventSecurityCheck,
			Severity:  SeverityMedium,
			Actor:     "test-user",
			Action:    "test_action",
			Status:    "success",
			Message:   "Test event",
		})
	}

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	report := mgr.GetAuditReport(startTime, endTime)

	if report.ReportID == "" {
		t.Error("Report ID should not be empty")
	}
	if report.TotalEvents < 0 {
		t.Error("Total events should not be negative")
	}
}

func TestFullAudit(t *testing.T) {
	mgr := NewManager()
	result := mgr.RunFullAudit()

	if result == nil {
		t.Fatal("Full audit returned nil")
	}

	auditID, ok := result["audit_id"]
	if !ok || auditID == "" {
		t.Error("Audit ID should be present")
	}

	summary, ok := result["summary"]
	if !ok {
		t.Error("Summary should be present")
	}

	summaryMap, ok := summary.(map[string]interface{})
	if !ok {
		t.Error("Summary should be a map")
	}

	if _, ok := summaryMap["overall_score"]; !ok {
		t.Error("Overall score should be in summary")
	}
}

func TestDashboard(t *testing.T) {
	mgr := NewManager()
	dashboard := mgr.GetDashboard()

	if dashboard == nil {
		t.Fatal("Dashboard returned nil")
	}

	if _, ok := dashboard["security_score"]; !ok {
		t.Error("Security score should be in dashboard")
	}

	if _, ok := dashboard["timestamp"]; !ok {
		t.Error("Timestamp should be in dashboard")
	}
}

func TestUpdateConfig(t *testing.T) {
	mgr := NewManager()

	config := mgr.GetConfig()
	config.AlertThreshold = 80

	err := mgr.UpdateConfig(config)
	if err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	newConfig := mgr.GetConfig()
	if newConfig.AlertThreshold != 80 {
		t.Errorf("Expected alert threshold 80, got %d", newConfig.AlertThreshold)
	}
}

func TestScoreHistory(t *testing.T) {
	mgr := NewManager()

	// 运行多次评分以生成历史
	for i := 0; i < 3; i++ {
		mgr.GetSecurityScore()
		time.Sleep(10 * time.Millisecond)
	}

	history := mgr.GetScoreHistory(30)
	if len(history) == 0 {
		t.Error("Expected some score history")
	}
}

func TestSecurityChecker(t *testing.T) {
	checker := NewSecurityChecker()

	checks := checker.GetCheckList()
	if len(checks) == 0 {
		t.Error("Expected some checks")
	}

	// 测试添加检查
	newCheck := SecurityCheck{
		ID:          "test-001",
		Name:        "Test Check",
		Description: "A test check",
		Category:    CategorySystem,
		Severity:    SeverityLow,
		Enabled:     true,
	}

	err := checker.AddCheck(newCheck)
	if err != nil {
		t.Errorf("Failed to add check: %v", err)
	}

	// 测试重复添加
	err = checker.AddCheck(newCheck)
	if err == nil {
		t.Error("Expected error for duplicate check")
	}

	// 测试启用/禁用
	err = checker.DisableCheck("test-001")
	if err != nil {
		t.Errorf("Failed to disable check: %v", err)
	}

	err = checker.EnableCheck("test-001")
	if err != nil {
		t.Errorf("Failed to enable check: %v", err)
	}

	// 测试删除
	err = checker.RemoveCheck("test-001")
	if err != nil {
		t.Errorf("Failed to remove check: %v", err)
	}

	// 测试删除不存在的
	err = checker.RemoveCheck("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent check")
	}
}

func TestVulnerabilityScanner(t *testing.T) {
	scanner := NewVulnerabilityScanner()

	config := VulnerabilityScanConfig{
		ScanPackages: true,
		ScanServices: true,
		ScanConfig:   true,
		ScanNetwork:  true,
		ScanPorts:    true,
	}

	report := scanner.Scan(config)
	if report.ReportID == "" {
		t.Error("Report ID should not be empty")
	}

	// 获取漏洞
	vulns := scanner.GetVulnerabilities("", "")
	if len(vulns) == 0 {
		t.Error("Expected some vulnerabilities")
	}

	// 获取单个漏洞
	if len(vulns) > 0 {
		vuln, err := scanner.GetVulnerability(vulns[0].ID)
		if err != nil {
			t.Errorf("Failed to get vulnerability: %v", err)
		}
		if vuln == nil {
			t.Error("Vulnerability should not be nil")
		}
	}

	// 测试不存在的漏洞
	_, err := scanner.GetVulnerability("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent vulnerability")
	}
}

func TestHardeningAdvisor(t *testing.T) {
	advisor := NewHardeningAdvisor()

	// 测试获取建议
	suggestions := advisor.GetSuggestionsByCategory(HardeningAuth)
	if len(suggestions) == 0 {
		t.Error("Expected some auth suggestions")
	}

	// 测试应用建议
	if len(suggestions) > 0 {
		err := advisor.Apply(suggestions[0].ID)
		if err != nil {
			t.Errorf("Failed to apply suggestion: %v", err)
		}
	}

	// 测试忽略建议
	if len(suggestions) > 1 {
		err := advisor.Dismiss(suggestions[1].ID)
		if err != nil {
			t.Errorf("Failed to dismiss suggestion: %v", err)
		}
	}
}

func TestScoreEngine(t *testing.T) {
	engine := NewScoreEngine()

	// 创建测试检查结果
	results := []SecurityCheckResult{
		{Category: CategoryAuth, Status: StatusPass},
		{Category: CategoryAuth, Status: StatusPass},
		{Category: CategoryNetwork, Status: StatusFail},
		{Category: CategorySystem, Status: StatusPass},
	}

	score := engine.CalculateScore(results)
	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("Score out of range: %d", score.Overall)
	}

	// 测试历史
	history := engine.GetHistory(30)
	if len(history) == 0 {
		t.Error("Expected some history")
	}

	// 测试趋势
	trend := engine.GetScoreTrend(30)
	if trend == nil {
		t.Error("Trend should not be nil")
	}
}

func TestAuditLogger(t *testing.T) {
	logger := NewAuditLogger()

	// 记录事件
	logger.Log(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityMedium,
		Actor:     "test-user",
		Action:    "test_action",
		Status:    "success",
		Message:   "Test event",
	})

	// 获取日志
	logs := logger.GetLogs(10, 0, nil)
	if len(logs) == 0 {
		t.Error("Expected some logs")
	}

	// 测试过滤
	filters := map[string]string{
		"actor": "test-user",
	}
	filteredLogs := logger.GetLogs(10, 0, filters)
	if len(filteredLogs) == 0 {
		t.Error("Expected filtered logs")
	}

	// 测试统计
	stats := logger.GetStats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}

	// 测试搜索
	results := logger.SearchEvents("test", 10)
	if len(results) == 0 {
		t.Error("Expected search results")
	}

	// 测试导出
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	jsonData, err := logger.ExportLogs(startTime, endTime, "json")
	if err != nil {
		t.Errorf("Failed to export JSON: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON export should not be empty")
	}

	csvData, err := logger.ExportLogs(startTime, endTime, "csv")
	if err != nil {
		t.Errorf("Failed to export CSV: %v", err)
	}
	if len(csvData) == 0 {
		t.Error("CSV export should not be empty")
	}
}
